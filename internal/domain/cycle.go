package domain

import "time"

// Cycle is a Whoop physiological cycle (`/v2/cycle`).
// End is nil while the cycle is still in progress.
type Cycle struct {
	ID         int64
	Start      time.Time
	End        *time.Time
	TZOffset   string
	Strain     float64 // 0..21
	AvgHR      float64
	MaxHR      float64
	Kilojoule  float64
	ScoreState string
}
