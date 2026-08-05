package fatsecret

import (
	"context"
	"errors"
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
}

func NewTokenProvider(tokens *auth.TokenStore, consumerKey, consumerSecret, fallbackToken, fallbackSecret string) *TokenProvider {
	return &TokenProvider{tokens: tokens, consumerKey: consumerKey, consumerSecret: consumerSecret, fallbackToken: fallbackToken, fallbackSecret: fallbackSecret}
}

func (p *TokenProvider) client(ctx context.Context) (*Client, error) {
	token, secret := p.fallbackToken, p.fallbackSecret
	stored, err := p.tokens.Get(ctx, auth.SourceFatSecret)
	if err == nil {
		token, secret = stored.AccessToken, stored.RefreshToken
	} else if !errors.Is(err, auth.ErrNotFound) {
		return nil, err
	}
	if token == "" || secret == "" {
		return nil, ErrNotConnected
	}
	return NewClient(NewSigner(p.consumerKey, p.consumerSecret, token, secret), Options{}), nil
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
