package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMonthsSpanned(t *testing.T) {
	loc := time.UTC
	d := func(y int, m time.Month, day int) time.Time {
		return time.Date(y, m, day, 0, 0, 0, 0, loc)
	}

	cases := []struct {
		name          string
		from, toLabel time.Time
		want          []time.Time
	}{
		{
			name: "within one month",
			from: d(2026, 6, 5), toLabel: d(2026, 6, 11),
			want: []time.Time{d(2026, 6, 1)},
		},
		{
			name: "straddles month boundary",
			from: d(2026, 5, 13), toLabel: d(2026, 6, 11),
			want: []time.Time{d(2026, 5, 1), d(2026, 6, 1)},
		},
		{
			name: "straddles year boundary",
			from: d(2025, 12, 28), toLabel: d(2026, 1, 3),
			want: []time.Time{d(2025, 12, 1), d(2026, 1, 1)},
		},
		{
			name: "spans three months",
			from: d(2026, 4, 20), toLabel: d(2026, 6, 2),
			want: []time.Time{d(2026, 4, 1), d(2026, 5, 1), d(2026, 6, 1)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, monthsSpanned(tc.from, tc.toLabel, loc))
		})
	}
}
