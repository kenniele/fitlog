package domain

import "time"

// SleepStages holds per-phase durations as reported by Whoop, in milliseconds.
type SleepStages struct {
	InBedMs int64
	AwakeMs int64
	LightMs int64
	SWSMs   int64 // slow-wave / deep
	REMMs   int64
}

// SleepNeed breaks down the Whoop "need" composition in milliseconds.
type SleepNeed struct {
	BaselineMs   int64
	FromNapMs    int64
	FromStrainMs int64
	FromDebtMs   int64
}

// Sleep is a single sleep activity from Whoop (`/v2/activity/sleep`).
type Sleep struct {
	ExternalID            string // UUID
	Start                 time.Time
	End                   time.Time
	IsNap                 bool
	SleepPerformancePct   float64
	SleepEfficiencyPct    float64
	SleepConsistencyPct   float64
	RespiratoryRate       float64
	Stages                SleepStages
	DisturbanceCount      int
	SleepCycleCount       int
	SleepNeed             SleepNeed
}
