package controlcenter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) TrainingAnalytics(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location, filters AnalyticsFilters) (map[string]any, error) {
	allDaily, err := r.FilteredTrainingDaily(ctx, ownerID, dateRange, loc, filters)
	if err != nil {
		return nil, err
	}
	daily := filterDailyByType(allDaily, filters.DayType)
	summary := TrainingSummary{}
	for _, point := range daily {
		summary.Sessions += point.WorkoutCount
		summary.WorkingSets += point.WorkingSets
		summary.Repetitions += point.Repetitions
		summary.VolumeKG += point.TrainingVolumeKG
	}
	summary.AverageRIR = weightedAverageRIR(daily)
	duration, err := r.filteredTrainingDuration(ctx, ownerID, dateRange, loc, filters)
	if err != nil {
		return nil, err
	}
	duration = filterTrainingDurationByDays(duration, daily, filters.DayType)
	summary.AverageMinutes = averageTrainingDuration(duration)
	adherence, err := r.filteredTrainingAdherence(ctx, ownerID, dateRange, loc, filters)
	if err != nil {
		return nil, err
	}
	eligible, completed := adherenceTotals(adherence)
	if eligible > 0 {
		value := float64(completed) / float64(eligible) * 100
		summary.Adherence = &value
	}
	summary.PersonalRecords, err = r.filteredPersonalRecords(ctx, ownerID, dateRange, loc, filters)
	if err != nil {
		return nil, err
	}
	muscleGroups, err := r.filteredTrainingMuscleGroups(ctx, ownerID, dateRange, loc, filters)
	if err != nil {
		return nil, err
	}
	rirDistribution, err := r.filteredTrainingRIRDistribution(ctx, ownerID, dateRange, loc, filters)
	if err != nil {
		return nil, err
	}
	settings, err := r.Settings(ctx, ownerID, loc.String())
	if err != nil {
		return nil, err
	}
	weekly := weeklyTraining(daily, loc, settings.FirstDayOfWeek)
	return map[string]any{
		"range": rangeView(dateRange, loc), "summary": summary,
		"daily": daily, "weekly": weekly,
		"daily_duration": duration, "muscle_groups": muscleGroups,
		"rir_distribution": rirDistribution, "heatmap": trainingHeatmap(daily),
		"adherence": adherence, "streak": trainingStreak(daily, dateRange, loc),
	}, nil
}

func weightedAverageRIR(points []DailyPoint) *float64 {
	var sum float64
	var samples int
	for _, point := range points {
		sum += point.RIRSum
		samples += point.RIRSamples
	}
	if samples == 0 {
		return nil
	}
	value := sum / float64(samples)
	return &value
}

func (r *PostgresRepository) filteredPersonalRecords(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location, filters AnalyticsFilters) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `WITH candidates AS (
		SELECT session.started_at,training_set.id,
			training_set.actual_weight_kg*(1+training_set.actual_reps::double precision/30) AS e1rm,
			max(training_set.actual_weight_kg*(1+training_set.actual_reps::double precision/30)) OVER (
				PARTITION BY COALESCE(exercise.exercise_id::text,lower(btrim(exercise.name)))
				ORDER BY session.started_at,training_set.id ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING) AS previous_best
		FROM training_sessions session
		JOIN training_session_exercises exercise ON exercise.session_id=session.id
		JOIN training_sets training_set ON training_set.session_exercise_id=exercise.id
		WHERE session.owner_id=$1 AND session.started_at IS NOT NULL
			AND (($7::text='' AND session.status='finished') OR ($7::text<>'' AND session.status=$7))
			AND ($5::bigint IS NULL OR session.workout_template_id=$5)
			AND ($6::bigint IS NULL OR exercise.exercise_id=$6)
			AND training_set.completed_at IS NOT NULL AND training_set.type IN ('working','drop')
			AND training_set.actual_weight_kg IS NOT NULL AND training_set.actual_reps BETWEEN 1 AND 12
			AND ($8::bigint IS NULL OR EXISTS (SELECT 1 FROM training_program_revisions filtered_revision
				WHERE filtered_revision.id=session.revision_id AND filtered_revision.program_id=$8))
	)
	SELECT count(*)::int FROM candidates
	WHERE (started_at AT TIME ZONE $4)::date BETWEEN $2::date AND $3::date
		AND (previous_best IS NULL OR e1rm>previous_best)`, ownerID,
		dateRange.From.Format("2006-01-02"), dateRange.To.Format("2006-01-02"), loc.String(),
		filters.TemplateID, filters.ExerciseID, filters.Status, filters.PlanID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count filtered personal records: %w", err)
	}
	return count, nil
}

func (r *PostgresRepository) FilteredTrainingDaily(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location, filters AnalyticsFilters) ([]DailyPoint, error) {
	if err := r.validateAnalyticsOwnership(ctx, ownerID, filters); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		WITH days AS (SELECT generate_series($2::date,$3::date,'1 day'::interval)::date AS day)
		SELECT to_char(days.day,'YYYY-MM-DD'),
			training.workout_count,training.working_sets,training.repetitions,
			training.volume_kg,training.average_rir,training.rir_sum,training.rir_samples,training.estimated_1rm
		FROM days
		LEFT JOIN LATERAL (
			SELECT count(DISTINCT session.id)::int AS workout_count,
				count(training_set.id) FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop'))::int AS working_sets,
				COALESCE(sum(training_set.actual_reps) FILTER (WHERE training_set.completed_at IS NOT NULL),0)::int AS repetitions,
				COALESCE(sum(training_set.actual_weight_kg*training_set.actual_reps)
					FILTER (WHERE training_set.completed_at IS NOT NULL AND training_set.type IN ('working','drop')
						AND training_set.actual_weight_kg IS NOT NULL),0)::double precision AS volume_kg,
				avg(training_set.actual_rir) FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop'))::double precision AS average_rir,
				COALESCE(sum(training_set.actual_rir) FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop')),0)::double precision AS rir_sum,
				count(training_set.actual_rir) FILTER (WHERE training_set.completed_at IS NOT NULL
					AND training_set.type IN ('working','drop'))::int AS rir_samples,
				CASE WHEN $5::bigint IS NOT NULL THEN max(training_set.actual_weight_kg*(1+training_set.actual_reps::double precision/30))
					FILTER (WHERE training_set.completed_at IS NOT NULL AND training_set.type IN ('working','drop')
						AND training_set.actual_weight_kg IS NOT NULL AND training_set.actual_reps BETWEEN 1 AND 12)
					::double precision END AS estimated_1rm
			FROM training_sessions session
			LEFT JOIN training_session_exercises exercise ON exercise.session_id=session.id
				AND ($5::bigint IS NULL OR exercise.exercise_id=$5)
			LEFT JOIN training_sets training_set ON training_set.session_exercise_id=exercise.id
			WHERE session.owner_id=$1
				AND COALESCE((session.started_at AT TIME ZONE $4)::date,(session.scheduled_at AT TIME ZONE $4)::date)=days.day
				AND (($7::text='' AND session.status='finished') OR ($7::text<>'' AND session.status=$7))
				AND ($6::bigint IS NULL OR session.workout_template_id=$6)
				AND ($8::bigint IS NULL OR EXISTS (SELECT 1 FROM training_program_revisions filtered_revision
					WHERE filtered_revision.id=session.revision_id AND filtered_revision.program_id=$8))
				AND ($5::bigint IS NULL OR EXISTS (SELECT 1 FROM training_session_exercises required_exercise
					WHERE required_exercise.session_id=session.id AND required_exercise.exercise_id=$5))
		) training ON true
		ORDER BY days.day`, ownerID, dateRange.From.Format("2006-01-02"), dateRange.To.Format("2006-01-02"),
		loc.String(), filters.ExerciseID, filters.TemplateID, filters.Status, filters.PlanID)
	if err != nil {
		return nil, fmt.Errorf("load filtered training analytics: %w", err)
	}
	defer rows.Close()
	points := make([]DailyPoint, 0, dateRange.Days())
	for rows.Next() {
		var point DailyPoint
		if err := rows.Scan(&point.Date, &point.WorkoutCount, &point.WorkingSets, &point.Repetitions,
			&point.TrainingVolumeKG, &point.AverageRIR, &point.RIRSum, &point.RIRSamples, &point.Estimated1RM); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func (r *PostgresRepository) validateAnalyticsOwnership(ctx context.Context, ownerID int64, filters AnalyticsFilters) error {
	if filters.ExerciseID != nil {
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM training_exercises WHERE owner_id=$1 AND id=$2)`, ownerID, *filters.ExerciseID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return &ValidationError{Message: "invalid analytics filters", Fields: map[string]string{"exercise_id": "does not belong to the dashboard owner"}}
		}
	}
	if filters.TemplateID != nil {
		var exists bool
		err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM workout_templates WHERE owner_id=$1 AND id=$2)`, ownerID, *filters.TemplateID).Scan(&exists)
		if errors.Is(err, pgx.ErrNoRows) {
			exists = false
		} else if err != nil {
			return err
		}
		if !exists {
			return &ValidationError{Message: "invalid analytics filters", Fields: map[string]string{"template_id": "does not belong to the dashboard owner"}}
		}
	}
	if filters.PlanID != nil {
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM training_programs WHERE owner_id=$1 AND id=$2)`, ownerID, *filters.PlanID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return &ValidationError{Message: "invalid analytics filters", Fields: map[string]string{"plan_id": "does not belong to the dashboard owner"}}
		}
	}
	return nil
}

func weeklyTraining(points []DailyPoint, loc *time.Location, firstDayOfWeek int) []map[string]any {
	type aggregate struct {
		date        string
		sessions    int
		sets        int
		repetitions int
		volume      float64
	}
	order := make([]string, 0)
	byWeek := map[string]*aggregate{}
	for _, point := range points {
		day, err := time.ParseInLocation("2006-01-02", point.Date, loc)
		if err != nil {
			continue
		}
		weekStart := startOfWeek(day, firstDayOfWeek).Format("2006-01-02")
		current := byWeek[weekStart]
		if current == nil {
			current = &aggregate{date: weekStart}
			byWeek[weekStart] = current
			order = append(order, weekStart)
		}
		current.sessions += point.WorkoutCount
		current.sets += point.WorkingSets
		current.repetitions += point.Repetitions
		current.volume += point.TrainingVolumeKG
	}
	result := make([]map[string]any, 0, len(order))
	for _, key := range order {
		value := byWeek[key]
		result = append(result, map[string]any{
			"date": value.date, "sessions": value.sessions, "working_sets": value.sets,
			"repetitions": value.repetitions, "volume_kg": value.volume,
		})
	}
	return result
}
