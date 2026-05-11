package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitlog/internal/auth"
)

// TokensRepo is the pgx-backed implementation of auth.Repository.
type TokensRepo struct {
	pool *pgxpool.Pool
}

// NewTokensRepo builds a TokensRepo over a pgxpool.
func NewTokensRepo(pool *pgxpool.Pool) *TokensRepo {
	return &TokensRepo{pool: pool}
}

// Get returns the encrypted blobs and expiry for src, or auth.ErrNotFound.
func (r *TokensRepo) Get(ctx context.Context, src auth.Source) ([]byte, []byte, time.Time, error) {
	const q = `SELECT access_token, refresh_token, expires_at FROM oauth_tokens WHERE source = $1`
	var access, refresh []byte
	var exp time.Time
	err := r.pool.QueryRow(ctx, q, string(src)).Scan(&access, &refresh, &exp)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, time.Time{}, auth.ErrNotFound
		}
		return nil, nil, time.Time{}, fmt.Errorf("select oauth_tokens: %w", err)
	}
	return access, refresh, exp, nil
}

// Upsert inserts or replaces the row for src.
func (r *TokensRepo) Upsert(ctx context.Context, src auth.Source, access, refresh []byte, expiresAt time.Time) error {
	const q = `
        INSERT INTO oauth_tokens (source, access_token, refresh_token, expires_at, updated_at)
        VALUES ($1, $2, $3, $4, now())
        ON CONFLICT (source) DO UPDATE SET
            access_token  = EXCLUDED.access_token,
            refresh_token = EXCLUDED.refresh_token,
            expires_at    = EXCLUDED.expires_at,
            updated_at    = now()
    `
	if _, err := r.pool.Exec(ctx, q, string(src), access, refresh, expiresAt); err != nil {
		return fmt.Errorf("upsert oauth_tokens: %w", err)
	}
	return nil
}

// Delete removes the token row for src; missing row is not an error.
func (r *TokensRepo) Delete(ctx context.Context, src auth.Source) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM oauth_tokens WHERE source = $1`, string(src)); err != nil {
		return fmt.Errorf("delete oauth_tokens: %w", err)
	}
	return nil
}
