package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveWhoopBackfillRangeIncludesToday(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	now := time.Date(2026, 8, 21, 23, 30, 0, 0, loc)
	from, to, err := resolveWhoopBackfillRange(now, loc, 250, "", "")
	require.NoError(t, err)
	require.Equal(t, "2025-12-15", from.Format("2006-01-02"))
	require.Equal(t, "2026-08-21", to.Format("2006-01-02"))
}

func TestResolveWhoopBackfillRangeExplicit(t *testing.T) {
	from, to, err := resolveWhoopBackfillRange(time.Now(), time.UTC, 1, "2026-01-01", "2026-08-21")
	require.NoError(t, err)
	require.Equal(t, "2026-01-01", from.Format("2006-01-02"))
	require.Equal(t, "2026-08-21", to.Format("2006-01-02"))

	_, _, err = resolveWhoopBackfillRange(time.Now(), time.UTC, 367, "", "")
	require.ErrorContains(t, err, "between 1 and 366")
}
