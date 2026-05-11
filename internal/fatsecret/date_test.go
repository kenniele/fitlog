package fatsecret

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestToDateInt_Epoch(t *testing.T) {
	require.Equal(t, 0, ToDateInt(time.Unix(0, 0).UTC()))
}

func TestToDateInt_KnownDay(t *testing.T) {
	// 2026-05-11 UTC → days since 1970-01-01 == 20584
	require.Equal(t, 20584, ToDateInt(time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)))
}

func TestToDateInt_FloorsToUTCMidnight(t *testing.T) {
	// 23:59 UTC of day 20584 should still be 20584.
	d := time.Date(2026, 5, 11, 23, 59, 59, 0, time.UTC)
	require.Equal(t, 20584, ToDateInt(d))
}

func TestRoundTrip(t *testing.T) {
	d := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	require.True(t, FromDateInt(ToDateInt(d)).Equal(d))
}

func TestToDateInt_UsesLocalCalendarDate(t *testing.T) {
	// Midnight 11 May Moscow == 21:00 10 May UTC. The OLD implementation
	// would return 10 May's dateInt; the new one must return 11 May's.
	moscow, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	midnightMoscow := time.Date(2026, 5, 11, 0, 0, 0, 0, moscow)
	require.Equal(t, 20584, ToDateInt(midnightMoscow))
}
