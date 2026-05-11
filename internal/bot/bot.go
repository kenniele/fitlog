package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	tele "gopkg.in/telebot.v3"

	"fitlog/internal/auth"
	"fitlog/internal/domain"
	"fitlog/internal/fatsecret"
	"fitlog/internal/whoop"
)

// Deps bundles the bot's external collaborators. Held in a struct so handlers
// can be methods on Bot rather than closures over a half-dozen vars.
type Deps struct {
	Tokens         *auth.TokenStore
	Notes          NotesRepo
	OAuthConfig    *oauth2.Config
	FatSecret      *fatsecret.Client
	States         StateIssuer
	Location       *time.Location
	Logger         *slog.Logger
	NewWhoopClient func(ctx context.Context, ts oauth2.TokenSource) *whoop.Client
}

// NotesRepo is the slice of storage.NotesRepo the bot needs. Defined as an
// interface so the bot package stays independent of pgx.
type NotesRepo interface {
	Insert(ctx context.Context, n domain.Note) (int64, error)
	ListBetween(ctx context.Context, from, to time.Time) ([]domain.Note, error)
}

// StateIssuer is the slice of server.StateStore the bot needs.
// Defined as an interface to keep the bot package independent of the server
// package (avoids an import cycle: server depends on bot via Notifier).
type StateIssuer interface {
	Issue(chatID int64) (string, error)
}

// Bot is the long-poll telebot wrapper plus the deps it dispatches over.
type Bot struct {
	b    *tele.Bot
	deps Deps
}

// New builds a Bot. The telebot is created with long-polling; no webhook.
func New(token string, allowlist *Allowlist, deps Deps) (*Bot, error) {
	tb, err := tele.NewBot(tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		OnError: func(err error, c tele.Context) {
			deps.Logger.Error("telebot handler error", "err", err)
			if c != nil {
				_ = c.Send("Что-то пошло не так. Загляни в логи.")
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("telebot init: %w", err)
	}

	tb.Use(allowlist.Middleware())

	bot := &Bot{b: tb, deps: deps}
	bot.registerHandlers()
	return bot, nil
}

// Start runs the long-poll loop. Blocks until ctx is cancelled or Stop is called.
func (b *Bot) Start(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		b.b.Stop()
		close(done)
	}()
	b.b.Start()
	<-done
}

// Stop signals the long-poll loop to exit. Safe to call from another goroutine.
func (b *Bot) Stop() { b.b.Stop() }

// NotifyOAuthSuccess implements server.Notifier.
func (b *Bot) NotifyOAuthSuccess(_ context.Context, chatID int64) {
	if _, err := b.b.Send(&tele.Chat{ID: chatID}, "✓ Whoop подключён"); err != nil {
		b.deps.Logger.Warn("notify oauth success", "err", err)
	}
}

// NotifyOAuthFailure implements server.Notifier.
func (b *Bot) NotifyOAuthFailure(_ context.Context, chatID int64, reason string) {
	if _, err := b.b.Send(&tele.Chat{ID: chatID}, "✗ "+reason); err != nil {
		b.deps.Logger.Warn("notify oauth failure", "err", err)
	}
}

func (b *Bot) registerHandlers() {
	b.b.Handle("/start", b.handleStart)
	b.b.Handle("/connect_whoop", b.handleConnectWhoop)
	b.b.Handle("/info", b.handleInfo)
	b.b.Handle("/log", b.handleLog)
	b.b.Handle("/week", b.makePeriodHandler(7))
	b.b.Handle("/month", b.makePeriodHandler(30))
	b.b.Handle("/sleep", b.handleSleep)
	b.b.Handle("/recovery", b.handleRecovery)
	b.b.Handle("/workouts", b.handleWorkouts)
	b.b.Handle("/food", b.handleFood)
	b.b.Handle("/status", b.handleStatus)
}

// argInt parses the first positional argument as an int, or returns def.
func argInt(c tele.Context, def int) int {
	args := c.Args()
	if len(args) == 0 {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// reply sends s in MarkdownV2 mode and logs send errors.
func (b *Bot) reply(c tele.Context, s string) error {
	return c.Send(s, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, DisableWebPagePreview: true})
}

// loadWhoopClient fetches the encrypted Whoop token, builds a persisting
// token source, and returns a Whoop client that will auto-refresh+persist.
// If no token is stored, returns a sentinel "not connected" error.
var errWhoopNotConnected = errors.New("whoop not connected")

func (b *Bot) loadWhoopClient(ctx context.Context) (*whoop.Client, error) {
	tok, err := b.deps.Tokens.Get(ctx, auth.SourceWhoop)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return nil, errWhoopNotConnected
		}
		return nil, fmt.Errorf("load whoop token: %w", err)
	}

	initial := &oauth2.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.ExpiresAt,
	}
	ts := whoop.MakeTokenSource(ctx, b.deps.OAuthConfig, initial, func(updated *oauth2.Token) {
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := b.deps.Tokens.Save(saveCtx, auth.SourceWhoop, &auth.Token{
			AccessToken:  updated.AccessToken,
			RefreshToken: updated.RefreshToken,
			ExpiresAt:    updated.Expiry,
		}); err != nil {
			b.deps.Logger.Error("persist refreshed whoop token", "err", err)
		}
	})
	return b.deps.NewWhoopClient(ctx, ts), nil
}
