package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveFatSecretBackfillRangeDefaultsToCompletedDays(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	now := time.Date(2026, 8, 21, 17, 30, 0, 0, loc)

	from, to, err := resolveFatSecretBackfillRange(now, loc, 100, "", "")
	require.NoError(t, err)
	require.Equal(t, "2026-05-13", from.Format("2006-01-02"))
	require.Equal(t, "2026-08-20", to.Format("2006-01-02"))
}

func TestResolveFatSecretBackfillRangeExplicit(t *testing.T) {
	from, to, err := resolveFatSecretBackfillRange(time.Now(), time.UTC, 100, "2026-06-01", "2026-08-20")
	require.NoError(t, err)
	require.Equal(t, "2026-06-01", from.Format("2006-01-02"))
	require.Equal(t, "2026-08-20", to.Format("2006-01-02"))
}

func TestResolveFatSecretBackfillRangeRejectsIncompleteOrOversizedRange(t *testing.T) {
	_, _, err := resolveFatSecretBackfillRange(time.Now(), time.UTC, 100, "2026-06-01", "")
	require.ErrorContains(t, err, "together")
	_, _, err = resolveFatSecretBackfillRange(time.Now(), time.UTC, 100, "2025-01-01", "2026-08-20")
	require.ErrorContains(t, err, "366")
}

func TestFatSecretBackfillCommandRequiresStorageAuthorizationBeforeConfig(t *testing.T) {
	command := fatSecretBackfillCmd()
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs(nil)

	err := command.Execute()
	require.ErrorContains(t, err, "--storage-authorized")
	require.ErrorContains(t, err, fatSecretStorageGuide)
}
