package domain

import "time"

// TimeRange is a half-open [From, To) interval used for Whoop range queries
// and for "last N days" rollups.
type TimeRange struct {
	From time.Time
	To   time.Time
}

// Days returns a TimeRange covering the last n*24h ending at now.
// now is taken as a parameter so callers can inject a clock in tests.
func Days(now time.Time, n int) TimeRange {
	return TimeRange{From: now.Add(-time.Duration(n) * 24 * time.Hour), To: now}
}
