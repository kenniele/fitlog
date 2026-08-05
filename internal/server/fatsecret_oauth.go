package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"fitlog/internal/auth"
	"fitlog/internal/fatsecret"
)

type fatSecretRequest struct {
	secret    string
	chatID    int64
	createdAt time.Time
}

// FatSecretOAuth manages the short-lived request-token state for OAuth 1.0.
type FatSecretOAuth struct {
	client   *fatsecret.OAuthClient
	tokens   *auth.TokenStore
	logger   *slog.Logger
	notifier Notifier

	mu       sync.Mutex
	requests map[string]fatSecretRequest
	now      func() time.Time
}

func NewFatSecretOAuth(client *fatsecret.OAuthClient, tokens *auth.TokenStore, logger *slog.Logger) *FatSecretOAuth {
	return &FatSecretOAuth{client: client, tokens: tokens, logger: logger, requests: make(map[string]fatSecretRequest), now: time.Now}
}

func (h *FatSecretOAuth) SetNotifier(notifier Notifier) { h.notifier = notifier }

// Begin obtains a request token and returns the FatSecret approval URL.
func (h *FatSecretOAuth) Begin(ctx context.Context, chatID int64) (string, error) {
	token, err := h.client.RequestToken(ctx)
	if err != nil {
		return "", err
	}
	h.mu.Lock()
	h.gcLocked()
	h.requests[token.Token] = fatSecretRequest{secret: token.Secret, chatID: chatID, createdAt: h.now()}
	h.mu.Unlock()
	return h.client.AuthorizationURL(token.Token), nil
}

func (h *FatSecretOAuth) HandleCallback(w http.ResponseWriter, r *http.Request) {
	requestToken, verifier := r.URL.Query().Get("oauth_token"), r.URL.Query().Get("oauth_verifier")
	h.mu.Lock()
	h.gcLocked()
	pending, ok := h.requests[requestToken]
	if ok {
		delete(h.requests, requestToken)
	}
	h.mu.Unlock()
	if !ok || verifier == "" {
		h.fail(w, r, 0, "Некорректная или просроченная авторизация FatSecret")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	token, err := h.client.AccessToken(ctx, requestToken, pending.secret, verifier)
	if err != nil {
		h.logger.Error("fatsecret access token exchange failed", "err", err)
		h.fail(w, r, pending.chatID, "FatSecret отказался выдать access token")
		return
	}
	if err := h.tokens.Save(ctx, auth.SourceFatSecret, &auth.Token{AccessToken: token.Token, RefreshToken: token.Secret}); err != nil {
		h.logger.Error("save fatsecret token", "err", err)
		h.fail(w, r, pending.chatID, "Не получилось сохранить токен FatSecret")
		return
	}
	if h.notifier != nil {
		h.notifier.NotifyFatSecretOAuthSuccess(r.Context(), pending.chatID)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>fitlog</title><body style="font-family:system-ui;padding:2rem"><h1>✓ FatSecret подключён</h1><p>Можно возвращаться в Telegram.</p></body>`)
}

func (h *FatSecretOAuth) fail(w http.ResponseWriter, r *http.Request, chatID int64, reason string) {
	if chatID != 0 && h.notifier != nil {
		h.notifier.NotifyFatSecretOAuthFailure(r.Context(), chatID, reason)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = fmt.Fprintf(w, failPage, htmlEscape(reason))
}

func (h *FatSecretOAuth) gcLocked() {
	cutoff := h.now().Add(-stateTTL)
	for token, request := range h.requests {
		if request.createdAt.Before(cutoff) {
			delete(h.requests, token)
		}
	}
}
