package controlcenter

import (
	"context"
	"math"
	"time"
)

// These are sensitivity assumptions, not calibrated confidence intervals.
// The 7700 kcal/kg shortcut does not model metabolic adaptation or lean mass:
// https://pubmed.ncbi.nlm.nih.gov/17848938/
const (
	bodyFatKcalPerKG     = 7700.0
	bodyFatTDEEError     = 0.15
	bodyFatFoodError     = 0.10
	bodyFatBaselineError = 2.0 // percentage points
)

type BodyFatEstimate struct {
	Status               string     `json:"status"`
	MeasuredAt           *time.Time `json:"measured_at,omitempty"`
	From                 string     `json:"from,omitempty"`
	Through              string     `json:"through,omitempty"`
	Timezone             string     `json:"timezone"`
	BaselinePercent      *float64   `json:"baseline_percent,omitempty"`
	TDEEKcal             float64    `json:"tdee_kcal"`
	TotalDays            int        `json:"total_days"`
	ObservedDays         int        `json:"observed_days"`
	MissingDays          int        `json:"missing_days"`
	CoveragePercent      float64    `json:"coverage_percent"`
	ObservedDeficitKcal  *float64   `json:"observed_deficit_kcal,omitempty"`
	EstimatedDeficitKcal *float64   `json:"estimated_deficit_kcal,omitempty"`
	Percent              *float64   `json:"percent,omitempty"`
	LowerPercent         *float64   `json:"lower_percent,omitempty"`
	UpperPercent         *float64   `json:"upper_percent,omitempty"`
}

type bodyFatBaseline struct {
	MeasuredAt time.Time
	WeightKG   *float64
	FatMassKG  *float64
	FatPercent *float64
}

type bodyFatInputs struct {
	Baseline *bodyFatBaseline
	Days     []DailyPoint
}

type bodyFatEstimateStore interface {
	loadBodyFatInputs(context.Context, int64, time.Time, *time.Location) (bodyFatInputs, error)
}

func (s *Service) bodyFatEstimate(ctx context.Context) (BodyFatEstimate, error) {
	now, loc := s.now(), s.location(ctx)
	inputs := bodyFatInputs{}
	if store, ok := s.store.(bodyFatEstimateStore); ok {
		var err error
		inputs, err = store.loadBodyFatInputs(ctx, s.ownerID, now, loc)
		if err != nil {
			return BodyFatEstimate{}, err
		}
	}
	return estimateBodyFat(inputs, s.estimatedTDEE, now, loc), nil
}

func bodyFatDateRange(measuredAt, now time.Time, loc *time.Location) DateRange {
	measured, today := measuredAt.In(loc), now.In(loc)
	start := time.Date(measured.Year(), measured.Month(), measured.Day()+1, 0, 0, 0, 0, loc)
	end := time.Date(today.Year(), today.Month(), today.Day()-1, 0, 0, 0, 0, loc)
	return DateRange{From: start, To: end}
}

func estimateBodyFat(inputs bodyFatInputs, tdee float64, now time.Time, loc *time.Location) BodyFatEstimate {
	result := BodyFatEstimate{Status: "no_inbody", Timezone: loc.String()}
	if finite(tdee) && tdee > 0 {
		result.TDEEKcal = tdee
	}
	baseline := inputs.Baseline
	if baseline == nil || baseline.MeasuredAt.After(now) {
		return result
	}
	result.MeasuredAt = &baseline.MeasuredAt
	result.Status = "incomplete_inbody"
	if baseline.WeightKG == nil || !finite(*baseline.WeightKG) || *baseline.WeightKG <= 0 {
		return result
	}
	weight, fat := *baseline.WeightKG, 0.0
	if baseline.FatMassKG != nil {
		fat = *baseline.FatMassKG
	} else if baseline.FatPercent != nil {
		fat = weight * *baseline.FatPercent / 100
	}
	if !finite(fat) || fat <= 0 || fat >= weight {
		return result
	}
	initialPercent := fat / weight * 100
	result.BaselinePercent = &initialPercent
	rng := bodyFatDateRange(baseline.MeasuredAt, now, loc)
	result.From, result.Through = rng.From.Format("2006-01-02"), rng.To.Format("2006-01-02")
	result.TotalDays = rng.Days()
	if result.TDEEKcal == 0 {
		result.Status = "no_tdee"
		return result
	}
	if result.TotalDays == 0 {
		result.Status = "no_complete_days"
		return result
	}
	// A fixed expenditure and unchanged lean mass are too weak for an old baseline.
	if result.TotalDays > MaxRangeDays {
		result.Status = "stale_inbody"
		return result
	}
	calories := 0.0
	seen := make(map[string]bool)
	for _, day := range inputs.Days {
		if day.Date < result.From || day.Date > result.Through || seen[day.Date] {
			continue
		}
		seen[day.Date] = true
		if day.CaloriesKcal == nil || !finite(*day.CaloriesKcal) || *day.CaloriesKcal < 0 {
			continue
		}
		calories += *day.CaloriesKcal
		result.ObservedDays++
	}
	result.MissingDays = result.TotalDays - result.ObservedDays
	result.CoveragePercent = float64(result.ObservedDays) / float64(result.TotalDays) * 100
	observedDeficit := tdee*float64(result.ObservedDays) - calories
	if !finite(observedDeficit) {
		result.Status = "outside_model"
		return result
	}
	result.ObservedDeficitKcal = &observedDeficit
	if result.ObservedDays == 0 || result.CoveragePercent < 80 {
		result.Status = "insufficient_nutrition"
		return result
	}
	// Small gaps use the observed average, explicitly exposed in the UI. Wider
	// (50%) food uncertainty on those days prevents treating the imputation as fact.
	imputedCalories := calories / float64(result.ObservedDays) * float64(result.MissingDays)
	expenditure := tdee * float64(result.TotalDays)
	deficit := expenditure - calories - imputedCalories
	uncertainty := expenditure*bodyFatTDEEError + calories*bodyFatFoodError + imputedCalories*0.5
	if !finite(deficit) || !finite(uncertainty) {
		result.Status = "outside_model"
		return result
	}
	result.EstimatedDeficitKcal = &deficit
	percentAt := func(startPercent, energyDeficit float64) float64 {
		fatMass := weight*startPercent/100 - energyDeficit/bodyFatKcalPerKG
		leanMass := weight * (1 - startPercent/100)
		if fatMass <= 0 || leanMass <= 0 {
			return math.NaN()
		}
		return fatMass / (leanMass + fatMass) * 100
	}
	center := percentAt(initialPercent, deficit)
	lower := percentAt(initialPercent-bodyFatBaselineError, deficit+uncertainty)
	upper := percentAt(initialPercent+bodyFatBaselineError, deficit-uncertainty)
	if !finite(center) || !finite(lower) || !finite(upper) || lower >= upper {
		result.Status = "outside_model"
		return result
	}
	result.Status = "estimated"
	result.Percent, result.LowerPercent, result.UpperPercent = &center, &lower, &upper
	return result
}
