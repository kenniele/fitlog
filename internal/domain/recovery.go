package domain

// Recovery is a Whoop recovery score for a cycle/sleep pair.
// Whoop returns no dedicated ID — the natural key is CycleID.
type Recovery struct {
	CycleID         int64
	SleepID         string // UUID
	Score           float64
	HRVMilli        float64
	RestingHR       float64
	SkinTempC       *float64
	SpO2Pct         *float64
	UserCalibrating bool
}
