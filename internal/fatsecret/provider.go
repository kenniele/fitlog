package fatsecret

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"fitlog/internal/auth"
	"fitlog/internal/domain"
)

var ErrNotConnected = errors.New("fatsecret not connected")

// TokenProvider loads the delegated OAuth 1.0 credentials for every report.
// Static credentials are retained as a migration fallback.
type TokenProvider struct {
	tokens         *auth.TokenStore
	consumerKey    string
	consumerSecret string
	fallbackToken  string
	fallbackSecret string
	logger         *slog.Logger
}

func NewTokenProvider(tokens *auth.TokenStore, consumerKey, consumerSecret, fallbackToken, fallbackSecret string, logger *slog.Logger) *TokenProvider {
	return &TokenProvider{tokens: tokens, consumerKey: consumerKey, consumerSecret: consumerSecret, fallbackToken: fallbackToken, fallbackSecret: fallbackSecret, logger: logger}
}

func (p *TokenProvider) client(ctx context.Context) (*Client, error) {
	token, secret := p.fallbackToken, p.fallbackSecret
	stored, err := p.tokens.Get(ctx, auth.SourceFatSecret)
	if err == nil {
		token, secret = stored.AccessToken, stored.RefreshToken
		if p.logger != nil {
			p.logger.Info("fatsecret credentials loaded", "source", "encrypted_database")
		}
	} else if !errors.Is(err, auth.ErrNotFound) {
		return nil, err
	}
	if token == "" || secret == "" {
		if p.logger != nil {
			p.logger.Warn("fatsecret credentials unavailable")
		}
		return nil, ErrNotConnected
	}
	if stored == nil && p.logger != nil {
		p.logger.Info("fatsecret credentials loaded", "source", "legacy_environment_fallback")
	}
	return NewClient(NewSigner(p.consumerKey, p.consumerSecret, token, secret), Options{Logger: p.logger}), nil
}

func (p *TokenProvider) FoodEntriesForDay(ctx context.Context, day time.Time) ([]domain.MealEntry, error) {
	c, err := p.client(ctx)
	if err != nil {
		return nil, err
	}
	return c.FoodEntriesForDay(ctx, day)
}

func (p *TokenProvider) FoodEntriesMonth(ctx context.Context, day time.Time) ([]domain.DailyNutrition, error) {
	c, err := p.client(ctx)
	if err != nil {
		return nil, err
	}
	return c.FoodEntriesMonth(ctx, day)
}
