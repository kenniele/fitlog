package domain

import "time"

// ZoneDurations holds the time (ms) spent in each of Whoop's 6 HR zones.
type ZoneDurations struct {
	Zone0 int64
	Zone1 int64
	Zone2 int64
	Zone3 int64
	Zone4 int64
	Zone5 int64
}

// Workout is a Whoop workout activity (`/v2/activity/workout`).
type Workout struct {
	ExternalID      string // UUID
	SportID         int
	SportName       string
	Start           time.Time
	End             time.Time
	TZOffset        string
	Strain          float64
	AvgHR           float64
	MaxHR           float64
	Kilojoule       float64
	DistanceM       *float64
	AltitudeGainM   *float64
	PercentRecorded float64
	ZoneDurations   ZoneDurations
}
