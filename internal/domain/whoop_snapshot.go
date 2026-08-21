package domain

import "time"

// WhoopRecoverySnapshot is a scored recovery joined to its physiological
// cycle and associated sleep, ready for an owner-scoped durable upsert.
type WhoopRecoverySnapshot struct {
	EntryDate           time.Time
	CycleID             int64
	RecoveryScore       *float64
	HRVMs               *float64
	RestingHeartRateBPM *float64
	RespiratoryRate     *float64
	SpO2Percent         *float64
	SkinTemperatureC    *float64
	DailyStrain         *float64
}

// WhoopSleepSnapshot preserves one main sleep or nap. Stage metrics are nil
// when WHOOP has not scored the activity yet; start/end remain useful.
type WhoopSleepSnapshot struct {
	SleepDate           time.Time
	ExternalID          string
	Start               time.Time
	End                 time.Time
	IsNap               bool
	TimeInBedSeconds    *int64
	ActualSleepSeconds  *int64
	AwakeSeconds        *int64
	REMSeconds          *int64
	DeepSeconds         *int64
	LightSeconds        *int64
	SleepPerformancePct *float64
	EfficiencyPct       *float64
	ConsistencyPct      *float64
	SleepDebtSeconds    *int64
	Disturbances        *int
}
