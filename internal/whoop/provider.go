package whoop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/oauth2"

	"fitlog/internal/auth"
	"fitlog/internal/domain"
)

// ErrNotConnected means that the user has not completed Whoop OAuth yet.
var ErrNotConnected = errors.New("whoop not connected")

// API is the read-only slice of the Whoop transport needed by report use cases.
type API interface {
	Cycles(context.Context, domain.TimeRange, int) ([]domain.Cycle, error)
	Recoveries(context.Context, domain.TimeRange, int) ([]domain.Recovery, error)
	Sleeps(context.Context, domain.TimeRange, int) ([]domain.Sleep, error)
	Workouts(context.Context, domain.TimeRange, int) ([]domain.Workout, error)
}

// Provider supplies an authenticated client without exposing OAuth details to
// the use case or Telegram layer.
type Provider interface {
	Client(context.Context) (API, error)
}

type OAuthProvider struct {
	tokens  *auth.TokenStore
	config  *oauth2.Config
	logger  *slog.Logger
	factory func(context.Context, oauth2.TokenSource) API
}

func NewOAuthProvider(tokens *auth.TokenStore, config *oauth2.Config, logger *slog.Logger) *OAuthProvider {
	return &OAuthProvider{
		tokens: tokens,
		config: config,
		logger: logger,
		factory: func(ctx context.Context, source oauth2.TokenSource) API {
			return NewClientWithTokenSource(ctx, source, Options{})
		},
	}
}

func (p *OAuthProvider) Client(ctx context.Context) (API, error) {
	stored, err := p.tokens.Get(ctx, auth.SourceWhoop)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return nil, ErrNotConnected
		}
		return nil, fmt.Errorf("load whoop token: %w", err)
	}

	initial := &oauth2.Token{
		AccessToken:  stored.AccessToken,
		RefreshToken: stored.RefreshToken,
		Expiry:       stored.ExpiresAt,
	}
	source := MakeTokenSource(ctx, p.config, initial, func(updated *oauth2.Token) {
		// A Whoop refresh token is single-use and rotates. Persist its replacement
		// with a context independent from the report request that triggered it.
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.tokens.Save(saveCtx, auth.SourceWhoop, &auth.Token{
			AccessToken:  updated.AccessToken,
			RefreshToken: updated.RefreshToken,
			ExpiresAt:    updated.Expiry,
		}); err != nil {
			p.logger.Error("persist refreshed whoop token", "err", err)
		}
	})
	return p.factory(ctx, source), nil
}
