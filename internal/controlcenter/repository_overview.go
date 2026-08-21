package controlcenter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

func (r *PostgresRepository) Overview(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location) (Overview, error) {
	daily, err := r.loadDaily(ctx, ownerID, dateRange, loc)
	if err != nil {
		return Overview{}, err
	}
	summary, err := r.summarize(ctx, ownerID, dateRange, daily, loc)
	if err != nil {
		return Overview{}, err
	}
	view := rangeView(dateRange, loc)
	overview := Overview{
		Range: view, Daily: daily, Highlights: []Highlight{}, Sessions: []json.RawMessage{},
		TodaySessions: []json.RawMessage{}, WeeklySessions: []json.RawMessage{}, Sources: []SourceStatus{},
	}
	overview.Summary = summary

	todayDate := time.Now().In(loc)
	todayKey := todayDate.Format("2006-01-02")
	for _, point := range daily {
		if point.Date == todayKey {
			overview.Today = point
			break
		}
	}
	if overview.Today.Date == "" {
		today := time.Date(todayDate.Year(), todayDate.Month(), todayDate.Day(), 0, 0, 0, 0, loc)
		points, loadErr := r.loadDaily(ctx, ownerID, DateRange{From: today, To: today}, loc)
		if loadErr != nil {
			return Overview{}, loadErr
		}
		if len(points) > 0 {
			overview.Today = points[0]
		}
	}

	today := time.Date(todayDate.Year(), todayDate.Month(), todayDate.Day(), 0, 0, 0, 0, loc)
	previousDay := today.AddDate(0, 0, -1)
	previousPoints, err := r.loadDaily(ctx, ownerID, DateRange{From: previousDay, To: previousDay}, loc)
	if err != nil {
		return Overview{}, err
	}
	if len(previousPoints) > 0 {
		overview.PreviousDay = previousPoints[0]
	}
	todaySessions, err := r.List(ctx, ownerID, "workout-sessions", Pagination{
		Page: 1, PageSize: 12, From: &today, To: &today, Filters: map[string]string{"date_basis": "calendar"},
	}, loc)
	if err != nil {
		return Overview{}, err
	}
	overview.TodaySessions = todaySessions.Items
	// Sessions remains a compatibility alias for older clients; its semantics are
	// deliberately "today", never the last rows of the selected analytics range.
	overview.Sessions = todaySessions.Items
	settings, err := r.Settings(ctx, ownerID, loc.String())
	if err != nil {
		return Overview{}, err
	}
	weekStart := startOfWeek(today, settings.FirstDayOfWeek)
	weekEnd := weekStart.AddDate(0, 0, 6)
	overview.WeeklyRange = rangeView(DateRange{From: weekStart, To: weekEnd}, loc)
	weeklySessions, err := r.List(ctx, ownerID, "workout-sessions", Pagination{
		Page: 1, PageSize: 25, From: &weekStart, To: &weekEnd, Filters: map[string]string{"date_basis": "calendar"},
	}, loc)
	if err != nil {
		return Overview{}, err
	}
	overview.WeeklySessions = weeklySessions.Items
	overview.Sources, err = r.Sources(ctx, ownerID)
	if err != nil {
		return Overview{}, err
	}
	if summary.Training.PersonalRecords > 0 {
		overview.Highlights = append(overview.Highlights, Highlight{
			ID: "personal-records", Type: "positive", Title: "Новые силовые рекорды",
			Description: fmt.Sprintf("За период зафиксировано новых рекордов: %d", summary.Training.PersonalRecords),
			Rule:        "e1RM по Epley выше всех предыдущих завершённых подходов того же упражнения",
		})
	}
	lowRecovery, _, _ := recoveryRangeBounds(settings.RecoveryRanges)
	if current := summary.Recovery.Recovery.Current; current != nil && *current < lowRecovery {
		overview.Highlights = append(overview.Highlights, Highlight{
			ID: "low-recovery", Type: "warning", Title: "Низкое восстановление",
			Description: fmt.Sprintf("Последняя оценка восстановления — %.0f%%", *current),
			Rule:        fmt.Sprintf("последний Recovery Score выбранного периода ниже сохранённого порога %.0f%%", lowRecovery),
		})
	}
	if overview.Today.HRVMs != nil && overview.Today.HRV28DAverage != nil && *overview.Today.HRV28DAverage > 0 && *overview.Today.HRVMs < *overview.Today.HRV28DAverage*.9 {
		overview.Highlights = append(overview.Highlights, Highlight{
			ID: "hrv-below-baseline", Type: "warning", Title: "HRV ниже 28-дневной базы", Date: todayKey,
			Description: fmt.Sprintf("Сегодня %.1f мс при 28-дневной базе %.1f мс", *overview.Today.HRVMs, *overview.Today.HRV28DAverage),
			Rule:        "текущее HRV ниже 90% полного 28-дневного moving average",
		})
	}
	if settings.SleepTargetMinSeconds != nil && overview.Today.SleepSeconds != nil && *overview.Today.SleepSeconds < int64(*settings.SleepTargetMinSeconds) {
		overview.Highlights = append(overview.Highlights, Highlight{
			ID: "sleep-below-target", Type: "warning", Title: "Сон короче цели", Date: todayKey,
			Description: fmt.Sprintf("Сегодня %d мин при минимальной цели %d мин", *overview.Today.SleepSeconds/60, *settings.SleepTargetMinSeconds/60),
			Rule:        "фактический сон ниже сохранённой минимальной цели",
		})
	}
	if settings.SleepTargetMaxSeconds != nil && overview.Today.SleepSeconds != nil && *overview.Today.SleepSeconds > int64(*settings.SleepTargetMaxSeconds) {
		overview.Highlights = append(overview.Highlights, Highlight{
			ID: "sleep-above-target", Type: "neutral", Title: "Сон длиннее целевого диапазона", Date: todayKey,
			Description: fmt.Sprintf("Сегодня %d мин при максимальной цели %d мин", *overview.Today.SleepSeconds/60, *settings.SleepTargetMaxSeconds/60),
			Rule:        "фактический сон выше сохранённой максимальной границы; это описательное наблюдение, не медицинский вывод",
		})
	}
	if settings.CalorieTargetKcal != nil && overview.Today.CaloriesKcal != nil {
		lower, upper := *settings.CalorieTargetKcal*.9, *settings.CalorieTargetKcal*1.1
		if *overview.Today.CaloriesKcal < lower || *overview.Today.CaloriesKcal > upper {
			overview.Highlights = append(overview.Highlights, Highlight{
				ID: "calories-outside-target", Type: "warning", Title: "Калории вне целевого диапазона", Date: todayKey,
				Description: fmt.Sprintf("Сегодня %.0f ккал при цели %.0f ккал", *overview.Today.CaloriesKcal, *settings.CalorieTargetKcal),
				Rule:        "дневной итог выходит за диапазон 90–110% сохранённой калорийной цели",
			})
		}
	}
	streak := 0
	for index := len(daily) - 1; index >= 0; index-- {
		if daily[index].WorkoutCount <= 0 {
			if streak > 0 {
				break
			}
			continue
		}
		streak++
	}
	if streak >= 3 {
		overview.Highlights = append(overview.Highlights, Highlight{
			ID: "training-streak", Type: "positive", Title: "Серия тренировочных дней",
			Description: fmt.Sprintf("Тренировки отмечены %d дней подряд", streak),
			Rule:        "последовательные календарные дни выбранного периода с завершённой тренировкой",
		})
	}
	if overview.Today.RecoveryScore == nil && overview.Today.SleepSeconds == nil && overview.Today.CaloriesKcal == nil && overview.Today.WeightKG == nil && overview.Today.WorkoutCount == 0 {
		overview.Highlights = append(overview.Highlights, Highlight{
			ID: "today-missing", Type: "neutral", Title: "Сегодня пока нет данных", Date: todayKey,
			Description: "Recovery, сон, питание, вес и тренировки за сегодня ещё не записаны",
			Rule:        "все основные сегодняшние показатели отсутствуют",
		})
	}
	if dateRange.Compare {
		previous := dateRange.Previous()
		previousDaily, previousErr := r.loadDaily(ctx, ownerID, previous, loc)
		if previousErr != nil {
			return Overview{}, previousErr
		}
		previousSummary, previousErr := r.summarize(ctx, ownerID, previous, previousDaily, loc)
		if previousErr != nil {
			return Overview{}, previousErr
		}
		overview.Comparison = &Comparison{Range: rangeView(previous, loc), Summary: previousSummary}
	}
	return overview, nil
}

func startOfWeek(day time.Time, firstDay int) time.Time {
	if firstDay < 1 || firstDay > 7 {
		firstDay = 1
	}
	weekday := firstDay % 7 // Settings use ISO 1=Monday ... 7=Sunday.
	delta := (int(day.Weekday()) - weekday + 7) % 7
	return day.AddDate(0, 0, -delta)
}

func rangeView(dateRange DateRange, loc *time.Location) RangeView {
	return RangeView{
		From: dateRange.From.In(loc).Format("2006-01-02"),
		To:   dateRange.To.In(loc).Format("2006-01-02"),
		Days: dateRange.Days(), Timezone: loc.String(), Comparison: dateRange.Compare,
	}
}

func (r *PostgresRepository) loadDaily(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location) ([]DailyPoint, error) {
	// Baselines need observations preceding the visible range. Fetch the largest
	// rolling window once, calculate all series on that contiguous calendar, and
	// return only the dates requested by the client.
	historyRange := dateRange
	historyRange.From = historyRange.From.AddDate(0, 0, -27)
	points, err := r.loadDailyWindow(ctx, ownerID, historyRange, loc)
	if err != nil {
		return nil, err
	}
	visible := make([]DailyPoint, 0, dateRange.Days())
	firstVisible := dateRange.From.In(loc).Format("2006-01-02")
	for _, point := range points {
		if point.Date >= firstVisible {
			visible = append(visible, point)
		}
	}
	return visible, nil
}

func (r *PostgresRepository) loadDailyWindow(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location) ([]DailyPoint, error) {
	rows, err := r.pool.Query(ctx, `
		WITH days AS (
			SELECT generate_series($2::date,$3::date,'1 day'::interval)::date AS day
		)
		SELECT to_char(days.day,'YYYY-MM-DD'),
			recovery.recovery_score, recovery.hrv_ms, recovery.resting_heart_rate_bpm,
			recovery.respiratory_rate, recovery.spo2_percent, recovery.skin_temperature_c, recovery.daily_strain,
			sleep.actual_sleep_seconds, sleep.time_in_bed_seconds, sleep.sleep_performance_percent,
			sleep.efficiency_percent, sleep.consistency_percent, sleep.sleep_debt_seconds, sleep.disturbances,
			sleep.rem_seconds, sleep.deep_seconds, sleep.light_seconds, sleep.awake_seconds,
			nutrition.calories_kcal, nutrition.protein_g, nutrition.fat_g, nutrition.carbohydrates_g, nutrition.fiber_g,
			nutrition.sugar_g, nutrition.saturated_fat_g, nutrition.sodium_mg, nutrition.potassium_mg, nutrition.water_ml,
			body.weight_kg, body.body_fat_percent, body.fat_mass_kg, body.lean_mass_kg,
			body.skeletal_muscle_mass_kg, body.waist_cm, body.chest_cm, body.biceps_cm, body.thigh_cm,
			body.total_body_water_l, body.intracellular_water_l, body.extracellular_water_l, body.ecw_tbw_ratio,
			body.protein_mass_kg, body.mineral_mass_kg, body.bmi, body.visceral_fat_level,
			body.visceral_fat_area_cm2, body.basal_metabolic_rate_kcal, body.inbody_score,
			body.phase_angle_degrees, body.segments,
			training.workout_count, schedule.scheduled_sessions, schedule.completed_scheduled_sessions,
			training.working_sets, training.repetitions,
			training.volume_kg, training.average_rir, training.rir_sum, training.rir_samples
		FROM days
		LEFT JOIN LATERAL (
			SELECT recovery_score::double precision,hrv_ms::double precision,
				resting_heart_rate_bpm::double precision,respiratory_rate::double precision,
				spo2_percent::double precision,skin_temperature_c::double precision,daily_strain::double precision
			FROM recovery_entries WHERE owner_id=$1 AND entry_date=days.day
			ORDER BY updated_at DESC,id DESC LIMIT 1
		) recovery ON true
		LEFT JOIN LATERAL (
			SELECT sum(actual_sleep_seconds)::bigint AS actual_sleep_seconds,
				sum(time_in_bed_seconds)::bigint AS time_in_bed_seconds,
				avg(sleep_performance_percent)::double precision AS sleep_performance_percent,
				avg(efficiency_percent)::double precision AS efficiency_percent,
				avg(consistency_percent)::double precision AS consistency_percent,
				max(sleep_debt_seconds)::bigint AS sleep_debt_seconds,
				sum(disturbances)::int AS disturbances,
				sum(rem_seconds)::bigint AS rem_seconds,sum(deep_seconds)::bigint AS deep_seconds,
				sum(light_seconds)::bigint AS light_seconds,sum(awake_seconds)::bigint AS awake_seconds
			FROM sleep_entries WHERE owner_id=$1 AND sleep_date=days.day AND NOT is_nap
		) sleep ON true
		LEFT JOIN LATERAL (
			SELECT calories_kcal::double precision,protein_g::double precision,fat_g::double precision,
				carbohydrates_g::double precision,fiber_g::double precision,sugar_g::double precision,
				saturated_fat_g::double precision,sodium_mg::double precision,potassium_mg::double precision,
				water_ml::double precision
			FROM nutrition_days WHERE owner_id=$1 AND entry_date=days.day
			ORDER BY updated_at DESC,id DESC LIMIT 1
		) nutrition ON true
		LEFT JOIN LATERAL (
			SELECT measurement.weight_kg::double precision,measurement.body_fat_percent::double precision,
				measurement.fat_mass_kg::double precision,measurement.lean_mass_kg::double precision,
				measurement.skeletal_muscle_mass_kg::double precision,measurement.waist_cm::double precision,
				measurement.chest_cm::double precision,measurement.biceps_cm::double precision,
				measurement.thigh_cm::double precision,measurement.total_body_water_l::double precision,
				measurement.intracellular_water_l::double precision,measurement.extracellular_water_l::double precision,
				measurement.ecw_tbw_ratio::double precision,measurement.protein_mass_kg::double precision,
				measurement.mineral_mass_kg::double precision,measurement.bmi::double precision,
				measurement.visceral_fat_level::double precision,measurement.visceral_fat_area_cm2::double precision,
				measurement.basal_metabolic_rate_kcal::double precision,measurement.inbody_score::double precision,
				measurement.phase_angle_degrees::double precision,
				COALESCE((SELECT jsonb_agg(jsonb_build_object(
					'segment', segment.segment, 'lean_mass_kg', segment.lean_mass_kg::double precision,
					'lean_percent', segment.lean_percent::double precision,
					'fat_mass_kg', segment.fat_mass_kg::double precision,
					'fat_percent', segment.fat_percent::double precision)
					ORDER BY CASE segment.segment WHEN 'left_arm' THEN 1 WHEN 'right_arm' THEN 2
						WHEN 'trunk' THEN 3 WHEN 'left_leg' THEN 4 ELSE 5 END)
					FROM body_segment_measurements segment
					WHERE segment.body_measurement_id=measurement.id AND segment.owner_id=$1), '[]'::jsonb) AS segments
			FROM body_measurements measurement
			WHERE measurement.owner_id=$1 AND (measurement.measured_at AT TIME ZONE $4)::date=days.day
			ORDER BY measurement.measured_at DESC,measurement.id DESC LIMIT 1
		) body ON true
		LEFT JOIN LATERAL (
			SELECT count(DISTINCT session.id) FILTER (WHERE session.status='finished')::int AS workout_count,
				count(training_set.id) FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop') AND session.status='finished')::int AS working_sets,
				COALESCE(sum(training_set.actual_reps) FILTER (WHERE training_set.completed_at IS NOT NULL
					AND session.status='finished'),0)::int AS repetitions,
				COALESCE(sum(training_set.actual_weight_kg*training_set.actual_reps)
					FILTER (WHERE training_set.completed_at IS NOT NULL AND training_set.type IN ('working','drop')
						AND training_set.actual_weight_kg IS NOT NULL AND session.status='finished'),0)::double precision AS volume_kg,
				avg(training_set.actual_rir) FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop') AND session.status='finished')::double precision AS average_rir,
				COALESCE(sum(training_set.actual_rir) FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop') AND session.status='finished'),0)::double precision AS rir_sum,
				count(training_set.actual_rir) FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop') AND session.status='finished')::int AS rir_samples
			FROM training_sessions session
			LEFT JOIN training_session_exercises exercise ON exercise.session_id=session.id
			LEFT JOIN training_sets training_set ON training_set.session_exercise_id=exercise.id
			WHERE session.owner_id=$1
				AND COALESCE((session.started_at AT TIME ZONE $4)::date, (session.scheduled_at AT TIME ZONE $4)::date)=days.day
		) training ON true
		LEFT JOIN LATERAL (
			SELECT
				count(*) FILTER (WHERE status <> 'excused')::int AS scheduled_sessions,
				count(*) FILTER (WHERE status='finished')::int AS completed_scheduled_sessions
			FROM training_sessions scheduled_session
			WHERE scheduled_session.owner_id=$1
				AND (scheduled_session.scheduled_at AT TIME ZONE $4)::date=days.day
		) schedule ON true
		ORDER BY days.day`, ownerID, dateRange.From.Format("2006-01-02"), dateRange.To.Format("2006-01-02"), loc.String())
	if err != nil {
		return nil, fmt.Errorf("load dashboard daily data: %w", err)
	}
	defer rows.Close()
	points := make([]DailyPoint, 0, dateRange.Days())
	for rows.Next() {
		var point DailyPoint
		var bodySegments json.RawMessage
		if err := rows.Scan(&point.Date, &point.RecoveryScore, &point.HRVMs, &point.RestingHeartRateBPM,
			&point.RespiratoryRate, &point.SpO2Percent, &point.SkinTemperatureC, &point.DailyStrain,
			&point.SleepSeconds, &point.TimeInBedSeconds, &point.SleepPerformancePercent,
			&point.SleepEfficiencyPercent, &point.SleepConsistencyPercent, &point.SleepDebtSeconds, &point.Disturbances,
			&point.REMSeconds, &point.DeepSeconds, &point.LightSeconds, &point.AwakeSeconds,
			&point.CaloriesKcal, &point.ProteinG, &point.FatG, &point.CarbohydratesG, &point.FiberG,
			&point.SugarG, &point.SaturatedFatG, &point.SodiumMG, &point.PotassiumMG, &point.WaterML,
			&point.WeightKG, &point.BodyFatPercent, &point.FatMassKG, &point.LeanMassKG,
			&point.SkeletalMuscleMassKG, &point.WaistCM, &point.ChestCM, &point.BicepsCM, &point.ThighCM,
			&point.TotalBodyWaterL, &point.IntracellularWaterL, &point.ExtracellularWaterL, &point.ECWTBWRatio,
			&point.ProteinMassKG, &point.MineralMassKG, &point.BMI, &point.VisceralFatLevel,
			&point.VisceralFatAreaCM2, &point.BasalMetabolicRateKcal, &point.InBodyScore,
			&point.PhaseAngleDegrees, &bodySegments,
			&point.WorkoutCount, &point.ScheduledSessions, &point.CompletedScheduled,
			&point.WorkingSets, &point.Repetitions, &point.TrainingVolumeKG, &point.AverageRIR,
			&point.RIRSum, &point.RIRSamples); err != nil {
			return nil, fmt.Errorf("scan dashboard daily data: %w", err)
		}
		if len(bodySegments) > 0 {
			if err := json.Unmarshal(bodySegments, &point.BodySegments); err != nil {
				return nil, fmt.Errorf("decode dashboard body segments: %w", err)
			}
		}
		point.SkinTemperatureCelsius = point.SkinTemperatureC
		if point.ScheduledSessions > 0 {
			value := float64(point.CompletedScheduled) / float64(point.ScheduledSessions) * 100
			point.PlanAdherence = &value
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	weights := make([]*float64, len(points))
	hrv := make([]*float64, len(points))
	rhr := make([]*float64, len(points))
	sleep := make([]*float64, len(points))
	for index := range points {
		weights[index] = points[index].WeightKG
		hrv[index] = points[index].HRVMs
		rhr[index] = points[index].RestingHeartRateBPM
		sleep[index] = int64AsFloat(points[index].SleepSeconds)
	}
	weight7 := MovingAverage(weights, 7)
	hrv7, hrv28 := MovingAverage(hrv, 7), MovingAverage(hrv, 28)
	rhr7, rhr28 := MovingAverage(rhr, 7), MovingAverage(rhr, 28)
	sleep7, sleep28 := MovingAverage(sleep, 7), MovingAverage(sleep, 28)
	for index := range points {
		points[index].Weight7DAverage = weight7[index]
		points[index].HRV7DAverage, points[index].HRV28DAverage = hrv7[index], hrv28[index]
		points[index].RHR7DAverage, points[index].RHR28DAverage = rhr7[index], rhr28[index]
		points[index].Sleep7DAverage, points[index].Sleep28DAverage = sleep7[index], sleep28[index]
	}
	return points, nil
}

func (r *PostgresRepository) summarize(ctx context.Context, ownerID int64, dateRange DateRange, daily []DailyPoint, loc *time.Location) (DashboardSummary, error) {
	summary := DashboardSummary{}
	for _, point := range daily {
		summary.Training.Sessions += point.WorkoutCount
		summary.Training.WorkingSets += point.WorkingSets
		summary.Training.Repetitions += point.Repetitions
		summary.Training.VolumeKG += point.TrainingVolumeKG
		if point.CaloriesKcal != nil || point.ProteinG != nil || point.FatG != nil || point.CarbohydratesG != nil {
			summary.Nutrition.DaysLogged++
		}
	}
	summary.Training.AverageRIR = weightedAverageRIR(daily)
	summary.Recovery.Recovery = metricSummary(daily, func(point DailyPoint) *float64 { return point.RecoveryScore })
	summary.Recovery.HRV = metricSummary(daily, func(point DailyPoint) *float64 { return point.HRVMs })
	summary.Recovery.RHR = metricSummary(daily, func(point DailyPoint) *float64 { return point.RestingHeartRateBPM })
	summary.Recovery.Strain = metricSummary(daily, func(point DailyPoint) *float64 { return point.DailyStrain })
	summary.Recovery.Sleep = metricSummary(daily, func(point DailyPoint) *float64 { return int64AsFloat(point.SleepSeconds) })
	summary.Nutrition.Calories = metricSummary(daily, func(point DailyPoint) *float64 { return point.CaloriesKcal })
	summary.Nutrition.Protein = metricSummary(daily, func(point DailyPoint) *float64 { return point.ProteinG })
	summary.Nutrition.Fat = metricSummary(daily, func(point DailyPoint) *float64 { return point.FatG })
	summary.Nutrition.Carbohydrates = metricSummary(daily, func(point DailyPoint) *float64 { return point.CarbohydratesG })
	summary.Body.Weight = metricSummary(daily, func(point DailyPoint) *float64 { return point.WeightKG })
	summary.Body.BodyFat = metricSummary(daily, func(point DailyPoint) *float64 { return point.BodyFatPercent })
	summary.Body.FatMass = metricSummary(daily, func(point DailyPoint) *float64 { return point.FatMassKG })
	summary.Body.LeanMass = metricSummary(daily, func(point DailyPoint) *float64 { return point.LeanMassKG })
	summary.Body.SkeletalMuscleMass = metricSummary(daily, func(point DailyPoint) *float64 { return point.SkeletalMuscleMassKG })
	summary.Body.TotalBodyWater = metricSummary(daily, func(point DailyPoint) *float64 { return point.TotalBodyWaterL })
	summary.Body.IntracellularWater = metricSummary(daily, func(point DailyPoint) *float64 { return point.IntracellularWaterL })
	summary.Body.ExtracellularWater = metricSummary(daily, func(point DailyPoint) *float64 { return point.ExtracellularWaterL })
	summary.Body.ECWTBWRatio = metricSummary(daily, func(point DailyPoint) *float64 { return point.ECWTBWRatio })
	summary.Body.ProteinMass = metricSummary(daily, func(point DailyPoint) *float64 { return point.ProteinMassKG })
	summary.Body.MineralMass = metricSummary(daily, func(point DailyPoint) *float64 { return point.MineralMassKG })
	summary.Body.BMI = metricSummary(daily, func(point DailyPoint) *float64 { return point.BMI })
	summary.Body.VisceralFatLevel = metricSummary(daily, func(point DailyPoint) *float64 { return point.VisceralFatLevel })
	summary.Body.VisceralFatArea = metricSummary(daily, func(point DailyPoint) *float64 { return point.VisceralFatAreaCM2 })
	summary.Body.BasalMetabolicRate = metricSummary(daily, func(point DailyPoint) *float64 { return point.BasalMetabolicRateKcal })
	summary.Body.InBodyScore = metricSummary(daily, func(point DailyPoint) *float64 { return point.InBodyScore })
	summary.Body.PhaseAngle = metricSummary(daily, func(point DailyPoint) *float64 { return point.PhaseAngleDegrees })
	summary.Body.Segments = summarizeBodySegments(daily)
	weights := make([]*float64, len(daily))
	for index := range daily {
		weights[index] = daily[index].WeightKG
	}
	summary.Body.WeightMovingAvg = MovingAverage(weights, 7)

	var duration *float64
	var eligible, finished, prs int
	err := r.pool.QueryRow(ctx, `
		WITH owner_sessions AS (
			SELECT * FROM training_sessions WHERE owner_id=$1
		), candidates AS (
			SELECT session.started_at, exercise.exercise_id, lower(btrim(exercise.name)) AS exercise_name,
				(training_set.actual_weight_kg*(1+training_set.actual_reps::double precision/30)) AS e1rm,
				max(training_set.actual_weight_kg*(1+training_set.actual_reps::double precision/30)) OVER (
					PARTITION BY COALESCE(exercise.exercise_id::text,lower(btrim(exercise.name)))
					ORDER BY session.started_at,training_set.id ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING) AS previous_best
			FROM training_sessions session
			JOIN training_session_exercises exercise ON exercise.session_id=session.id
			JOIN training_sets training_set ON training_set.session_exercise_id=exercise.id
			WHERE session.owner_id=$1 AND session.status='finished' AND training_set.completed_at IS NOT NULL
				AND training_set.type IN ('working','drop') AND training_set.actual_weight_kg IS NOT NULL
				AND training_set.actual_reps BETWEEN 1 AND 12
		)
		SELECT
			(SELECT avg(extract(epoch FROM finished_at-started_at)/60)::double precision FROM owner_sessions
				WHERE status='finished' AND finished_at IS NOT NULL AND started_at IS NOT NULL
					AND (started_at AT TIME ZONE $4)::date BETWEEN $2::date AND $3::date),
			(SELECT count(*)::int FROM owner_sessions WHERE scheduled_at IS NOT NULL AND status <> 'excused'
				AND (scheduled_at AT TIME ZONE $4)::date BETWEEN $2::date AND $3::date),
			(SELECT count(*)::int FROM owner_sessions WHERE scheduled_at IS NOT NULL AND status='finished'
				AND (scheduled_at AT TIME ZONE $4)::date BETWEEN $2::date AND $3::date),
			(SELECT count(*)::int FROM candidates WHERE (started_at AT TIME ZONE $4)::date BETWEEN $2::date AND $3::date
				AND (previous_best IS NULL OR e1rm > previous_best))`, ownerID,
		dateRange.From.Format("2006-01-02"), dateRange.To.Format("2006-01-02"), loc.String()).Scan(&duration, &eligible, &finished, &prs)
	if err != nil {
		return DashboardSummary{}, fmt.Errorf("summarize dashboard training: %w", err)
	}
	summary.Training.AverageMinutes = duration
	if eligible > 0 {
		value := float64(finished) / float64(eligible) * 100
		summary.Training.Adherence = &value
	}
	summary.Training.PersonalRecords = prs
	settings, err := r.Settings(ctx, ownerID, loc.String())
	if err != nil {
		return DashboardSummary{}, err
	}
	if settings.CalorieTargetKcal != nil {
		lower, upper := *settings.CalorieTargetKcal*.9, *settings.CalorieTargetKcal*1.1
		for _, point := range daily {
			if point.CaloriesKcal != nil && *point.CaloriesKcal >= lower && *point.CaloriesKcal <= upper {
				summary.Nutrition.DaysInTarget++
			}
		}
	}
	return summary, nil
}

func summarizeBodySegments(points []DailyPoint) []BodySegmentSummary {
	result := make([]BodySegmentSummary, 0, 5)
	for _, name := range []string{"left_arm", "right_arm", "trunk", "left_leg", "right_leg"} {
		summary := BodySegmentSummary{
			Segment:     name,
			LeanMass:    segmentMetricSummary(points, name, func(segment BodySegmentSnapshot) *float64 { return segment.LeanMassKG }),
			LeanPercent: segmentMetricSummary(points, name, func(segment BodySegmentSnapshot) *float64 { return segment.LeanPercent }),
			FatMass:     segmentMetricSummary(points, name, func(segment BodySegmentSnapshot) *float64 { return segment.FatMassKG }),
			FatPercent:  segmentMetricSummary(points, name, func(segment BodySegmentSnapshot) *float64 { return segment.FatPercent }),
		}
		if summary.LeanMass.Samples > 0 || summary.LeanPercent.Samples > 0 ||
			summary.FatMass.Samples > 0 || summary.FatPercent.Samples > 0 {
			result = append(result, summary)
		}
	}
	return result
}

func segmentMetricSummary(
	points []DailyPoint,
	name string,
	value func(BodySegmentSnapshot) *float64,
) MetricSummary {
	return metricSummary(points, func(point DailyPoint) *float64 {
		for _, segment := range point.BodySegments {
			if segment.Segment == name {
				return value(segment)
			}
		}
		return nil
	})
}

func metricSummary(points []DailyPoint, value func(DailyPoint) *float64) MetricSummary {
	result := MetricSummary{}
	var sum float64
	var first *float64
	for _, point := range points {
		candidate := value(point)
		if candidate == nil || math.IsNaN(*candidate) || math.IsInf(*candidate, 0) {
			continue
		}
		copy := *candidate
		if first == nil {
			first = &copy
			result.Minimum, result.Maximum = &copy, &copy
		}
		if copy < *result.Minimum {
			minimum := copy
			result.Minimum = &minimum
		}
		if copy > *result.Maximum {
			maximum := copy
			result.Maximum = &maximum
		}
		current := copy
		result.Current = &current
		sum += copy
		result.Samples++
	}
	if result.Samples > 0 {
		average := sum / float64(result.Samples)
		result.Average = &average
		if first != nil && result.Current != nil {
			change := *result.Current - *first
			result.Change = &change
		}
	}
	return result
}
