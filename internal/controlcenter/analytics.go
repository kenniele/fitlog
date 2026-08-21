package controlcenter

import "math"

// SetType identifies how a training set contributes to a workout.
type SetType string

const (
	SetTypeWarmup  SetType = "warmup"
	SetTypeWorking SetType = "working"
	SetTypeDrop    SetType = "drop"
)

// VolumeSet contains the values needed to calculate training volume.
// A nil WeightKG represents a bodyweight set.
type VolumeSet struct {
	Type      SetType
	WeightKG  *float64
	Reps      int
	Completed bool
}

// CompletedWeightedSetVolume returns weight times repetitions for completed
// working and drop sets with a positive external weight.
func CompletedWeightedSetVolume(sets []VolumeSet) float64 {
	var volume float64
	for _, set := range sets {
		if !set.Completed || set.WeightKG == nil || *set.WeightKG <= 0 || set.Reps <= 0 {
			continue
		}
		if set.Type != SetTypeWorking && set.Type != SetTypeDrop {
			continue
		}
		volume += *set.WeightKG * float64(set.Reps)
	}
	return volume
}

// EstimatedOneRepMax calculates an Epley estimated one-repetition maximum.
// The estimate is valid only for a positive external weight and 1 to 12 reps.
func EstimatedOneRepMax(weightKG float64, reps int) (float64, bool) {
	if weightKG <= 0 || reps < 1 || reps > 12 {
		return 0, false
	}
	return weightKG * (1 + float64(reps)/30), true
}

// AdherenceSession describes whether a planned session belongs in adherence
// calculations and whether it was completed.
type AdherenceSession struct {
	Scheduled bool
	Finished  bool
	Cancelled bool
	Excused   bool
	Excluded  bool
}

// AdherenceResult reports the completed and eligible scheduled sessions.
// Percent is nil when there are no eligible scheduled sessions.
type AdherenceResult struct {
	Finished  int
	Scheduled int
	Percent   *float64
}

// CalculateAdherence calculates finished scheduled sessions as a percentage of
// eligible scheduled sessions. Explicit exclusions and excused cancellations
// are omitted from both counts.
func CalculateAdherence(sessions []AdherenceSession) AdherenceResult {
	result := AdherenceResult{}
	for _, session := range sessions {
		if !session.Scheduled || session.Excluded || session.Excused {
			continue
		}
		result.Scheduled++
		if session.Finished {
			result.Finished++
		}
	}
	if result.Scheduled == 0 {
		return result
	}

	percent := float64(result.Finished) / float64(result.Scheduled) * 100
	result.Percent = &percent
	return result
}

// MovingAverage calculates a nullable rolling average. An output point is nil
// until a complete window exists and whenever any value in that window is nil.
func MovingAverage(values []*float64, window int) []*float64 {
	if window <= 0 {
		return nil
	}

	averages := make([]*float64, len(values))
	var sum float64
	missing := 0
	for index, value := range values {
		if value == nil {
			missing++
		} else {
			sum += *value
		}

		if index >= window {
			outgoing := values[index-window]
			if outgoing == nil {
				missing--
			} else {
				sum -= *outgoing
			}
		}

		if index+1 >= window && missing == 0 {
			average := sum / float64(window)
			averages[index] = &average
		}
	}
	return averages
}

// PairedValue is one candidate observation for a correlation calculation.
// A pair is omitted when either value is nil.
type PairedValue struct {
	X *float64
	Y *float64
}

// CorrelationResult contains a Pearson coefficient and the number of complete
// pairs used to calculate it. Coefficient is nil when correlation is undefined.
type CorrelationResult struct {
	Coefficient        *float64
	SampleSize         int
	InsufficientSample bool
}

// PearsonCorrelation calculates Pearson's correlation over complete pairs.
// Samples with fewer than seven complete pairs are marked insufficient even
// when a coefficient can be calculated.
func PearsonCorrelation(points []PairedValue) CorrelationResult {
	complete := make([]PairedValue, 0, len(points))
	var sumX, sumY float64
	for _, point := range points {
		if point.X == nil || point.Y == nil {
			continue
		}
		complete = append(complete, point)
		sumX += *point.X
		sumY += *point.Y
	}

	result := CorrelationResult{
		SampleSize:         len(complete),
		InsufficientSample: len(complete) < 7,
	}
	if len(complete) < 2 {
		return result
	}

	meanX := sumX / float64(len(complete))
	meanY := sumY / float64(len(complete))
	var sumXY, sumXX, sumYY float64
	for _, point := range complete {
		deltaX := *point.X - meanX
		deltaY := *point.Y - meanY
		sumXY += deltaX * deltaY
		sumXX += deltaX * deltaX
		sumYY += deltaY * deltaY
	}
	if sumXX == 0 || sumYY == 0 {
		return result
	}

	coefficient := sumXY / math.Sqrt(sumXX*sumYY)
	coefficient = max(-1, min(1, coefficient))
	result.Coefficient = &coefficient
	return result
}
