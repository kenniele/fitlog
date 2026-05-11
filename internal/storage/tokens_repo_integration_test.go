//go:build integration

package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fitlog/internal/auth"
)

// To run:  TEST_DATABASE_URL=postgres://fitlog:fitlog@localhost:5432/fitlog?sslmode=disable \
//          go test -tags=integration ./internal/storage/...
//
// The schema is expected to be migrated already (`make migrate-up`).
func TestTokensRepo_Integration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	repo := NewTokensRepo(pool)

	// Clean slate.
	require.NoError(t, repo.Delete(ctx, auth.SourceWhoop))

	// Missing row → ErrNotFound.
	_, _, _, err = repo.Get(ctx, auth.SourceWhoop)
	require.ErrorIs(t, err, auth.ErrNotFound)

	// Insert.
	exp1 := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	require.NoError(t, repo.Upsert(ctx, auth.SourceWhoop,
		[]byte{0x01, 0x02}, []byte{0x03, 0x04}, exp1))

	a, r, e, err := repo.Get(ctx, auth.SourceWhoop)
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02}, a)
	require.Equal(t, []byte{0x03, 0x04}, r)
	require.True(t, e.Equal(exp1))

	// Upsert overwrites — proves ON CONFLICT path.
	exp2 := exp1.Add(2 * time.Hour)
	require.NoError(t, repo.Upsert(ctx, auth.SourceWhoop,
		[]byte{0xaa}, []byte{0xbb}, exp2))

	a, r, e, err = repo.Get(ctx, auth.SourceWhoop)
	require.NoError(t, err)
	require.Equal(t, []byte{0xaa}, a)
	require.Equal(t, []byte{0xbb}, r)
	require.True(t, e.Equal(exp2))

	// Delete.
	require.NoError(t, repo.Delete(ctx, auth.SourceWhoop))
	_, _, _, err = repo.Get(ctx, auth.SourceWhoop)
	require.ErrorIs(t, err, auth.ErrNotFound)

	// Delete of missing row is a no-op (no error).
	require.NoError(t, repo.Delete(ctx, auth.SourceWhoop))
}
