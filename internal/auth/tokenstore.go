package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Source identifies which third-party the token belongs to.
type Source string

const (
	SourceWhoop     Source = "whoop"
	SourceFatSecret Source = "fatsecret"
)

// Token is the plaintext form of a stored OAuth token set.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// ErrNotFound is returned by Repository.Get when no row exists for the source.
var ErrNotFound = errors.New("token not found")

// Repository persists encrypted token blobs. Implementations live in
// internal/storage; this interface is here to keep the encrypting wrapper
// decoupled from pgx.
type Repository interface {
	Get(ctx context.Context, src Source) (access, refresh []byte, expiresAt time.Time, err error)
	Upsert(ctx context.Context, src Source, access, refresh []byte, expiresAt time.Time) error
	Delete(ctx context.Context, src Source) error
}

// TokenStore is the encrypting facade over a Repository. Whoop OAuth and the
// authenticated client provider always go through this layer, so plaintext
// never reaches the DB repository.
type TokenStore struct {
	repo   Repository
	cipher *Cipher
}

// NewTokenStore wires a repo and a cipher.
func NewTokenStore(repo Repository, cipher *Cipher) *TokenStore {
	return &TokenStore{repo: repo, cipher: cipher}
}

// Get loads and decrypts a token. Returns ErrNotFound if absent.
func (s *TokenStore) Get(ctx context.Context, src Source) (*Token, error) {
	accCt, refCt, exp, err := s.repo.Get(ctx, src)
	if err != nil {
		return nil, err
	}
	access, err := s.cipher.OpenString(accCt)
	if err != nil {
		return nil, fmt.Errorf("decrypt access: %w", err)
	}
	refresh, err := s.cipher.OpenString(refCt)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh: %w", err)
	}
	return &Token{AccessToken: access, RefreshToken: refresh, ExpiresAt: exp}, nil
}

// Save encrypts and upserts a token.
func (s *TokenStore) Save(ctx context.Context, src Source, t *Token) error {
	acc, err := s.cipher.SealString(t.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt access: %w", err)
	}
	ref, err := s.cipher.SealString(t.RefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh: %w", err)
	}
	return s.repo.Upsert(ctx, src, acc, ref, t.ExpiresAt)
}

// Delete removes the token row.
func (s *TokenStore) Delete(ctx context.Context, src Source) error {
	return s.repo.Delete(ctx, src)
}
