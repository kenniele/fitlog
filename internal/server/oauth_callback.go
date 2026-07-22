package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"

	"fitlog/internal/auth"
)

const stateTTL = 10 * time.Minute

type stateEntry struct {
	chatID    int64
	createdAt time.Time
}

// StateStore is an in-memory state→chat-id map with TTL. Single-user bot,
// so no need for Redis or anything more elaborate.
type StateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
	now     func() time.Time
}

// NewStateStore builds an empty store using time.Now as the clock.
func NewStateStore() *StateStore {
	return &StateStore{entries: make(map[string]stateEntry), now: time.Now}
}

// Issue creates a fresh random state bound to chatID and returns it.
func (s *StateStore) Issue(chatID int64) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	state := hex.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	s.entries[state] = stateEntry{chatID: chatID, createdAt: s.now()}
	return state, nil
}

// Consume returns the chatID for state and removes it. Returns ok=false
// if state is unknown or expired.
func (s *StateStore) Consume(state string) (chatID int64, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	e, found := s.entries[state]
	if !found {
		return 0, false
	}
	delete(s.entries, state)
	return e.chatID, true
}

func (s *StateStore) gcLocked() {
	cutoff := s.now().Add(-stateTTL)
	for k, e := range s.entries {
		if e.createdAt.Before(cutoff) {
			delete(s.entries, k)
		}
	}
}

// Notifier sends a confirmation message to a Telegram chat. The bot package
// implements this — kept as an interface here to avoid an import cycle.
type Notifier interface {
	NotifyOAuthSuccess(ctx context.Context, chatID int64)
	NotifyOAuthFailure(ctx context.Context, chatID int64, reason string)
}

// HealthChecker verifies the application's required persistence dependency.
// pgxpool.Pool satisfies this interface.
type HealthChecker interface {
	Ping(context.Context) error
}

// CallbackHandler wires Whoop's OAuth2 callback into our token store.
type CallbackHandler struct {
	cfg      *oauth2.Config
	states   *StateStore
	tokens   *auth.TokenStore
	notifier Notifier
	logger   *slog.Logger
}

// NewCallbackHandler builds a CallbackHandler. notifier may be nil if the
// caller doesn't want Telegram confirmation pings (e.g. in tests).
func NewCallbackHandler(cfg *oauth2.Config, states *StateStore, tokens *auth.TokenStore, notifier Notifier, logger *slog.Logger) *CallbackHandler {
	return &CallbackHandler{cfg: cfg, states: states, tokens: tokens, notifier: notifier, logger: logger}
}

// Router builds an http.Handler exposing health, OAuth, and public article pages.
func Router(h *CallbackHandler, database HealthChecker, articles http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if database == nil || database.Ping(ctx) != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// Both paths are wired so the registered redirect URI can be either the
	// short /callback or the namespaced /oauth/whoop/callback.
	r.Get("/oauth/whoop/callback", h.HandleWhoopCallback)
	r.Get("/callback", h.HandleWhoopCallback)
	if articles != nil {
		r.Mount("/articles", http.StripPrefix("/articles", articles))
	}
	return r
}

const okPage = `<!doctype html><meta charset="utf-8"><title>fitlog</title>
<body style="font-family:system-ui;padding:2rem">
<h1>✓ Whoop подключён</h1>
<p>Можно возвращаться в Telegram.</p>
</body>`

const failPage = `<!doctype html><meta charset="utf-8"><title>fitlog</title>
<body style="font-family:system-ui;padding:2rem">
<h1>✗ Не получилось</h1>
<p>%s</p>
</body>`

// HandleWhoopCallback exchanges the code for tokens and persists them.
func (h *CallbackHandler) HandleWhoopCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state := q.Get("state")
	code := q.Get("code")
	cbErr := q.Get("error")

	chatID, ok := h.states.Consume(state)
	if !ok {
		h.fail(w, r, 0, "Некорректный или просроченный state")
		return
	}

	if cbErr != "" {
		// Don't reflect the provider-supplied error verbatim into the user's
		// chat / HTML page; log it and show a fixed message.
		h.logger.Warn("whoop oauth callback returned error", "error", cbErr)
		h.fail(w, r, chatID, "Whoop вернул ошибку авторизации")
		return
	}
	if code == "" {
		h.fail(w, r, chatID, "Whoop не прислал code")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tok, err := h.cfg.Exchange(ctx, code)
	if err != nil {
		h.logger.Error("whoop token exchange failed", "err", err)
		h.fail(w, r, chatID, "Whoop отказался обменять code")
		return
	}
	if tok.RefreshToken == "" {
		h.fail(w, r, chatID, "Whoop не вернул refresh_token (нужен scope offline)")
		return
	}

	if err := h.tokens.Save(ctx, auth.SourceWhoop, &auth.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    tok.Expiry,
	}); err != nil {
		h.logger.Error("save whoop token", "err", err)
		h.fail(w, r, chatID, "Не получилось сохранить токен")
		return
	}

	if h.notifier != nil {
		h.notifier.NotifyOAuthSuccess(r.Context(), chatID)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(okPage))
}

func (h *CallbackHandler) fail(w http.ResponseWriter, r *http.Request, chatID int64, reason string) {
	if chatID != 0 && h.notifier != nil {
		h.notifier.NotifyOAuthFailure(r.Context(), chatID, reason)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = fmt.Fprintf(w, failPage, htmlEscape(reason))
}

// ErrUnknownState is returned by tests inspecting Consume's miss path.
var ErrUnknownState = errors.New("unknown or expired state")

// htmlEscape is a tiny escape for the failure page; we control the input
// strings so this is defense-in-depth, not the primary safety net.
func htmlEscape(s string) string {
	r := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			r = append(r, []byte("&lt;")...)
		case '>':
			r = append(r, []byte("&gt;")...)
		case '&':
			r = append(r, []byte("&amp;")...)
		default:
			r = append(r, s[i])
		}
	}
	return string(r)
}
