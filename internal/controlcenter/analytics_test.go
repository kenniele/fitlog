package controlcenter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func floatPointer(value float64) *float64 {
	return &value
}

func TestCompletedWeightedSetVolume(t *testing.T) {
	sets := []VolumeSet{
		{Type: SetTypeWorking, WeightKG: floatPointer(80), Reps: 5, Completed: true},
		{Type: SetTypeDrop, WeightKG: floatPointer(40), Reps: 10, Completed: true},
		{Type: SetTypeWarmup, WeightKG: floatPointer(20), Reps: 10, Completed: true},
		{Type: SetTypeWorking, WeightKG: nil, Reps: 10, Completed: true},
		{Type: SetTypeWorking, WeightKG: floatPointer(80), Reps: 5, Completed: false},
		{Type: SetTypeWorking, WeightKG: floatPointer(0), Reps: 5, Completed: true},
		{Type: SetTypeWorking, WeightKG: floatPointer(80), Reps: 0, Completed: true},
		{Type: SetType("unknown"), WeightKG: floatPointer(100), Reps: 10, Completed: true},
	}

	require.InDelta(t, 800, CompletedWeightedSetVolume(sets), 0.001)
}

func TestEstimatedOneRepMax(t *testing.T) {
	tests := []struct {
		name     string
		weightKG float64
		reps     int
		want     float64
		valid    bool
	}{
		{name: "one rep", weightKG: 100, reps: 1, want: 103.333333, valid: true},
		{name: "ten reps", weightKG: 100, reps: 10, want: 133.333333, valid: true},
		{name: "twelve reps", weightKG: 100, reps: 12, want: 140, valid: true},
		{name: "zero weight", weightKG: 0, reps: 5, valid: false},
		{name: "negative weight", weightKG: -1, reps: 5, valid: false},
		{name: "zero reps", weightKG: 100, reps: 0, valid: false},
		{name: "too many reps", weightKG: 100, reps: 13, valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := EstimatedOneRepMax(test.weightKG, test.reps)
			require.Equal(t, test.valid, valid)
			if !test.valid {
				require.Zero(t, got)
				return
			}
			require.InDelta(t, test.want, got, 0.000001)
		})
	}
}

func TestCalculateAdherence(t *testing.T) {
	sessions := []AdherenceSession{
		{Scheduled: true, Finished: true},
		{Scheduled: true},
		{Scheduled: true, Cancelled: true},
		{Scheduled: true, Cancelled: true, Excused: true},
		{Scheduled: true, Excluded: true},
		{Finished: true},
	}

	got := CalculateAdherence(sessions)
	require.Equal(t, 1, got.Finished)
	require.Equal(t, 3, got.Scheduled)
	require.NotNil(t, got.Percent)
	require.InDelta(t, 100.0/3.0, *got.Percent, 0.000001)
}

func TestCalculateAdherenceWithoutEligibleSessions(t *testing.T) {
	got := CalculateAdherence([]AdherenceSession{
		{Scheduled: true, Cancelled: true, Excused: true},
		{Scheduled: true, Excluded: true},
	})

	require.Zero(t, got.Finished)
	require.Zero(t, got.Scheduled)
	require.Nil(t, got.Percent)
}

func TestCalculateAdherenceExcludesExcusedStatusWithoutCancelledFlag(t *testing.T) {
	got := CalculateAdherence([]AdherenceSession{
		{Scheduled: true, Finished: true},
		{Scheduled: true, Excused: true},
	})
	if got.Scheduled != 1 || got.Finished != 1 || got.Percent == nil || *got.Percent != 100 {
		t.Fatalf("unexpected adherence: %#v", got)
	}
}

func TestMovingAverageRequiresCompleteWindow(t *testing.T) {
	values := []*float64{
		floatPointer(1),
		floatPointer(2),
		nil,
		floatPointer(4),
		floatPointer(5),
		floatPointer(6),
	}

	got := MovingAverage(values, 3)
	require.Len(t, got, len(values))
	for index := 0; index < len(got)-1; index++ {
		require.Nil(t, got[index])
	}
	require.NotNil(t, got[5])
	require.InDelta(t, 5, *got[5], 0.001)
}

func TestMovingAverageWindowOnePreservesMissingValues(t *testing.T) {
	values := []*float64{floatPointer(2), nil, floatPointer(6)}

	got := MovingAverage(values, 1)
	require.InDelta(t, 2, *got[0], 0.001)
	require.Nil(t, got[1])
	require.InDelta(t, 6, *got[2], 0.001)
}

func TestMovingAverageInvalidAndOversizedWindows(t *testing.T) {
	require.Nil(t, MovingAverage([]*float64{floatPointer(1)}, 0))

	got := MovingAverage([]*float64{floatPointer(1), floatPointer(2)}, 3)
	require.Len(t, got, 2)
	require.Nil(t, got[0])
	require.Nil(t, got[1])
}

func TestWeightedAverageRIRUsesCompletedSetSamples(t *testing.T) {
	got := weightedAverageRIR([]DailyPoint{
		{AverageRIR: floatPointer(1), RIRSum: 1, RIRSamples: 1},
		{AverageRIR: floatPointer(3), RIRSum: 30, RIRSamples: 10},
		{AverageRIR: floatPointer(99)}, // A displayed daily value without samples must not influence a period summary.
	})
	require.NotNil(t, got)
	require.InDelta(t, 31.0/11.0, *got, 0.000001)
	require.Nil(t, weightedAverageRIR([]DailyPoint{{}}))
}

func TestWeeklyTrainingUsesConfiguredFirstDay(t *testing.T) {
	points := []DailyPoint{
		{Date: "2026-08-16", WorkoutCount: 1, WorkingSets: 3, TrainingVolumeKG: 300}, // Sunday.
		{Date: "2026-08-17", WorkoutCount: 2, WorkingSets: 5, TrainingVolumeKG: 500}, // Monday.
	}

	mondayWeeks := weeklyTraining(points, time.UTC, 1)
	require.Len(t, mondayWeeks, 2)
	require.Equal(t, "2026-08-10", mondayWeeks[0]["date"])
	require.Equal(t, "2026-08-17", mondayWeeks[1]["date"])

	sundayWeeks := weeklyTraining(points, time.UTC, 7)
	require.Len(t, sundayWeeks, 1)
	require.Equal(t, "2026-08-16", sundayWeeks[0]["date"])
	require.Equal(t, 3, sundayWeeks[0]["sessions"])
	require.Equal(t, 8, sundayWeeks[0]["working_sets"])
	require.InDelta(t, 800, sundayWeeks[0]["volume_kg"], 0.001)
}

func TestPearsonCorrelationUsesCompletePairs(t *testing.T) {
	points := []PairedValue{
		{X: floatPointer(1), Y: floatPointer(2)},
		{X: floatPointer(2), Y: floatPointer(4)},
		{X: nil, Y: floatPointer(100)},
		{X: floatPointer(3), Y: floatPointer(6)},
		{X: floatPointer(4), Y: floatPointer(8)},
		{X: floatPointer(5), Y: floatPointer(10)},
		{X: floatPointer(6), Y: floatPointer(12)},
		{X: floatPointer(7), Y: floatPointer(14)},
		{X: floatPointer(100), Y: nil},
	}

	got := PearsonCorrelation(points)
	require.Equal(t, 7, got.SampleSize)
	require.False(t, got.InsufficientSample)
	require.NotNil(t, got.Coefficient)
	require.InDelta(t, 1, *got.Coefficient, 0.000001)
}

func TestPearsonCorrelationMarksSmallSample(t *testing.T) {
	points := []PairedValue{
		{X: floatPointer(1), Y: floatPointer(6)},
		{X: floatPointer(2), Y: floatPointer(5)},
		{X: floatPointer(3), Y: floatPointer(4)},
		{X: floatPointer(4), Y: floatPointer(3)},
		{X: floatPointer(5), Y: floatPointer(2)},
		{X: floatPointer(6), Y: floatPointer(1)},
	}

	got := PearsonCorrelation(points)
	require.Equal(t, 6, got.SampleSize)
	require.True(t, got.InsufficientSample)
	require.NotNil(t, got.Coefficient)
	require.InDelta(t, -1, *got.Coefficient, 0.000001)
}

func TestPearsonCorrelationIsUndefinedForConstantSeries(t *testing.T) {
	points := []PairedValue{
		{X: floatPointer(1), Y: floatPointer(1)},
		{X: floatPointer(1), Y: floatPointer(2)},
		{X: floatPointer(1), Y: floatPointer(3)},
		{X: floatPointer(1), Y: floatPointer(4)},
		{X: floatPointer(1), Y: floatPointer(5)},
		{X: floatPointer(1), Y: floatPointer(6)},
		{X: floatPointer(1), Y: floatPointer(7)},
	}

	got := PearsonCorrelation(points)
	require.Equal(t, 7, got.SampleSize)
	require.False(t, got.InsufficientSample)
	require.Nil(t, got.Coefficient)
}

func TestCorrelationsUseTheNextCalendarDateWhenFilteredDaysAreSparse(t *testing.T) {
	selected := []DailyPoint{
		{Date: "2026-08-01", DailyStrain: floatPointer(8)},
		{Date: "2026-08-03", DailyStrain: floatPointer(12)},
	}
	calendar := []DailyPoint{
		{Date: "2026-08-01", RecoveryScore: floatPointer(60)},
		{Date: "2026-08-02", RecoveryScore: floatPointer(70)},
		{Date: "2026-08-03", RecoveryScore: floatPointer(75)},
		{Date: "2026-08-04", RecoveryScore: floatPointer(80)},
	}
	correlations := correlationsFromDaily(selected, calendar)
	if got := correlations[1]["sample_size"]; got != 2 {
		t.Fatalf("next-day correlation sample size = %v, want 2", got)
	}
}

func TestTrainingStreakUsesCalendarDaysAndThirtyDayWindow(t *testing.T) {
	loc := time.UTC
	dateRange := DateRange{
		From: time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		To:   time.Date(2026, 8, 5, 0, 0, 0, 0, loc),
	}
	points := []DailyPoint{
		{Date: "2026-07-02", WorkoutCount: 1}, // Outside the trailing 30-day window.
		{Date: "2026-08-01", WorkoutCount: 1},
		{Date: "2026-08-02", WorkoutCount: 1},
		{Date: "2026-08-04", WorkoutCount: 1},
		{Date: "2026-08-05", WorkoutCount: 2},
	}

	got := trainingStreak(points, dateRange, loc)
	require.Equal(t, 2, got.CurrentDays)
	require.Equal(t, 2, got.LongestLast30Days)
	require.Equal(t, 4, got.ActiveLast30Days)
}

func TestAverageTrainingDurationWeightsSessionsInsteadOfDays(t *testing.T) {
	total30 := 30.0
	total180 := 180.0
	got := averageTrainingDuration([]trainingDurationPoint{
		{DurationMinutes: &total30, Sessions: 1},
		{DurationMinutes: &total180, Sessions: 3},
		{},
	})
	require.NotNil(t, got)
	require.InDelta(t, 52.5, *got, 0.000001)
	require.Nil(t, averageTrainingDuration([]trainingDurationPoint{{}}))
}

func TestAdherenceTotalsUsePlannedSessions(t *testing.T) {
	planned, completed := adherenceTotals([]trainingAdherencePoint{
		{Planned: 2, Completed: 1},
		{Planned: 1, Completed: 1},
	})
	require.Equal(t, 3, planned)
	require.Equal(t, 2, completed)
}

func TestSummarizeBodySegmentsKeepsRegionsSeparate(t *testing.T) {
	points := []DailyPoint{
		{Date: "2026-08-20", BodySegments: []BodySegmentSnapshot{
			{Segment: "left_arm", LeanMassKG: floatPointer(3.7), LeanPercent: floatPointer(101)},
			{Segment: "right_arm", LeanMassKG: floatPointer(3.8)},
		}},
		{Date: "2026-08-21", BodySegments: []BodySegmentSnapshot{
			{Segment: "left_arm", LeanMassKG: floatPointer(3.9), LeanPercent: floatPointer(104)},
		}},
	}

	got := summarizeBodySegments(points)
	require.Len(t, got, 2)
	require.Equal(t, "left_arm", got[0].Segment)
	require.Equal(t, 2, got[0].LeanMass.Samples)
	require.InDelta(t, 3.9, *got[0].LeanMass.Current, 0.000001)
	require.InDelta(t, 0.2, *got[0].LeanMass.Change, 0.000001)
	require.Equal(t, "right_arm", got[1].Segment)
	require.Equal(t, 1, got[1].LeanMass.Samples)
}
