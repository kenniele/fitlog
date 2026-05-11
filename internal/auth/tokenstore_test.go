package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	access, refresh []byte
	expires         time.Time
	present         bool
	lastSrc         Source
	deleted         bool
}

func (f *fakeRepo) Get(_ context.Context, src Source) ([]byte, []byte, time.Time, error) {
	f.lastSrc = src
	if !f.present {
		return nil, nil, time.Time{}, ErrNotFound
	}
	return f.access, f.refresh, f.expires, nil
}

func (f *fakeRepo) Upsert(_ context.Context, src Source, a, r []byte, exp time.Time) error {
	f.lastSrc = src
	f.access, f.refresh, f.expires, f.present = a, r, exp, true
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, src Source) error {
	f.lastSrc = src
	f.deleted = true
	f.present = false
	return nil
}

func newStore(t *testing.T) (*TokenStore, *fakeRepo) {
	t.Helper()
	c, err := NewCipher(newTestKey(t))
	require.NoError(t, err)
	repo := &fakeRepo{}
	return NewTokenStore(repo, c), repo
}

func TestTokenStore_SaveAndGet(t *testing.T) {
	s, repo := newStore(t)
	ctx := context.Background()
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	require.NoError(t, s.Save(ctx, SourceWhoop, &Token{
		AccessToken: "acc-1", RefreshToken: "ref-1", ExpiresAt: exp,
	}))

	require.True(t, repo.present)
	require.NotEqual(t, []byte("acc-1"), repo.access, "DB blob must be encrypted, not plaintext")

	got, err := s.Get(ctx, SourceWhoop)
	require.NoError(t, err)
	require.Equal(t, "acc-1", got.AccessToken)
	require.Equal(t, "ref-1", got.RefreshToken)
	require.True(t, got.ExpiresAt.Equal(exp))
}

func TestTokenStore_Get_NotFound(t *testing.T) {
	s, _ := newStore(t)
	_, err := s.Get(context.Background(), SourceWhoop)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestTokenStore_Delete(t *testing.T) {
	s, repo := newStore(t)
	ctx := context.Background()
	require.NoError(t, s.Save(ctx, SourceWhoop, &Token{AccessToken: "a", RefreshToken: "r", ExpiresAt: time.Now()}))
	require.NoError(t, s.Delete(ctx, SourceWhoop))
	require.True(t, repo.deleted)
	_, err := s.Get(ctx, SourceWhoop)
	require.True(t, errors.Is(err, ErrNotFound))
}
