package controlcenter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CompactSnapshot is a bounded, read-only view. Each section retains its own
// observation date: yesterday's sleep must never masquerade as today's data.
type CompactSnapshot struct {
	Version     int             `json:"version"`
	GeneratedAt time.Time       `json:"generated_at"`
	Date        string          `json:"date"`
	Timezone    string          `json:"timezone"`
	Recovery    *CompactDay     `json:"recovery"`
	Sleep       *CompactDay     `json:"sleep"`
	Nutrition   *CompactDay     `json:"nutrition"`
	Body        *CompactBody    `json:"body"`
	Training    TrainingSummary `json:"training"`
	NextWorkout *CompactWorkout `json:"next_workout"`
	Sources     []SourceStatus  `json:"sources"`
}

type CompactDay struct {
	Date                    string   `json:"date"`
	RecoveryScore           *float64 `json:"recovery_score"`
	HRVMs                   *float64 `json:"hrv_ms"`
	RestingHeartRateBPM     *float64 `json:"resting_heart_rate_bpm"`
	DailyStrain             *float64 `json:"daily_strain"`
	RespiratoryRate         *float64 `json:"respiratory_rate"`
	SpO2Percent             *float64 `json:"spo2_percent"`
	SkinTemperatureC        *float64 `json:"skin_temperature_c"`
	SleepSeconds            *int64   `json:"sleep_seconds"`
	SleepPerformancePercent *float64 `json:"sleep_performance_percent"`
	SleepEfficiencyPercent  *float64 `json:"sleep_efficiency_percent"`
	REMSeconds              *int64   `json:"rem_seconds"`
	DeepSeconds             *int64   `json:"deep_seconds"`
	CaloriesKcal            *float64 `json:"calories_kcal"`
	ProteinG                *float64 `json:"protein_g"`
	FatG                    *float64 `json:"fat_g"`
	CarbohydratesG          *float64 `json:"carbohydrates_g"`
}

type CompactBody struct {
	MeasuredAt           time.Time `json:"measured_at"`
	WeightKG             *float64  `json:"weight_kg"`
	BodyFatPercent       *float64  `json:"body_fat_percent"`
	SkeletalMuscleMassKG *float64  `json:"skeletal_muscle_mass_kg"`
}

func compactDay(point DailyPoint) CompactDay {
	return CompactDay{Date: point.Date, RecoveryScore: point.RecoveryScore, HRVMs: point.HRVMs,
		RestingHeartRateBPM: point.RestingHeartRateBPM, DailyStrain: point.DailyStrain,
		RespiratoryRate: point.RespiratoryRate, SpO2Percent: point.SpO2Percent, SkinTemperatureC: point.SkinTemperatureC,
		SleepSeconds: point.SleepSeconds, SleepPerformancePercent: point.SleepPerformancePercent,
		SleepEfficiencyPercent: point.SleepEfficiencyPercent, REMSeconds: point.REMSeconds, DeepSeconds: point.DeepSeconds,
		CaloriesKcal: point.CaloriesKcal, ProteinG: point.ProteinG, FatG: point.FatG, CarbohydratesG: point.CarbohydratesG}
}

type CompactWorkout struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	ScheduledAt *time.Time `json:"scheduled_at"`
}

func (h *apiHandler) compact(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.CompactSnapshot(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, result)
}

type compactWorkoutStore interface {
	NextCompactWorkout(context.Context, int64, time.Time) (*CompactWorkout, error)
}

func (s *Service) CompactSnapshot(ctx context.Context) (CompactSnapshot, error) {
	loc := s.location(ctx)
	now := s.now()
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	overview, err := s.Overview(ctx, DateRange{From: today.AddDate(0, 0, -29), To: today})
	if err != nil {
		return CompactSnapshot{}, err
	}
	result := CompactSnapshot{Version: 1, GeneratedAt: now.UTC(), Date: today.Format("2006-01-02"), Timezone: loc.String(), Sources: overview.Sources}
	if result.Sources == nil {
		result.Sources = []SourceStatus{}
	}
	weekFrom := today.AddDate(0, 0, -6).Format("2006-01-02")
	var rirSum float64
	var rirSamples int
	for _, point := range overview.Daily {
		if point.Date > result.Date {
			continue
		}
		if point.RecoveryScore != nil || point.HRVMs != nil || point.RestingHeartRateBPM != nil || point.DailyStrain != nil || point.RespiratoryRate != nil || point.SpO2Percent != nil || point.SkinTemperatureC != nil {
			if result.Recovery == nil || point.Date > result.Recovery.Date {
				copy := compactDay(point)
				result.Recovery = &copy
			}
		}
		if point.SleepSeconds != nil || point.SleepPerformancePercent != nil || point.SleepEfficiencyPercent != nil || point.REMSeconds != nil || point.DeepSeconds != nil {
			if result.Sleep == nil || point.Date > result.Sleep.Date {
				copy := compactDay(point)
				result.Sleep = &copy
			}
		}
		if point.CaloriesKcal != nil || point.ProteinG != nil || point.FatG != nil || point.CarbohydratesG != nil {
			if result.Nutrition == nil || point.Date > result.Nutrition.Date {
				copy := compactDay(point)
				result.Nutrition = &copy
			}
		}
		if point.Date >= weekFrom {
			result.Training.Sessions += point.WorkoutCount
			result.Training.WorkingSets += point.WorkingSets
			result.Training.Repetitions += point.Repetitions
			result.Training.VolumeKG += point.TrainingVolumeKG
			rirSum += point.RIRSum
			rirSamples += point.RIRSamples
		}
	}
	if rirSamples > 0 {
		value := rirSum / float64(rirSamples)
		result.Training.AverageRIR = &value
	}
	// A body scan may predate the daily window; use the existing latest-first list.
	body, err := s.store.List(ctx, s.ownerID, "body-measurements", Pagination{Page: 1, PageSize: 1, To: &today}, loc)
	if err != nil {
		return CompactSnapshot{}, fmt.Errorf("compact body: %w", err)
	}
	if len(body.Items) > 0 {
		var measurement CompactBody
		if err := json.Unmarshal(body.Items[0], &measurement); err != nil {
			return CompactSnapshot{}, fmt.Errorf("compact body decode: %w", err)
		}
		result.Body = &measurement
	}
	if store, ok := s.store.(compactWorkoutStore); ok {
		result.NextWorkout, err = store.NextCompactWorkout(ctx, s.ownerID, now)
		if err != nil {
			return CompactSnapshot{}, err
		}
	}
	return result, nil
}
