package controlcenter

import (
	"context"
	"fmt"
	"time"
)

type trainingDurationPoint struct {
	Date                   string   `json:"date"`
	DurationMinutes        *float64 `json:"duration_minutes"`
	AverageDurationMinutes *float64 `json:"average_duration_minutes"`
	Sessions               int      `json:"sessions"`
}

type trainingMuscleGroupPoint struct {
	MuscleGroup string  `json:"muscle_group"`
	WorkingSets int     `json:"working_sets"`
	VolumeKG    float64 `json:"volume_kg"`
}

type trainingRIRPoint struct {
	RIR  string `json:"rir"`
	Sets int    `json:"sets"`
}

type trainingAdherencePoint struct {
	Date       string   `json:"date"`
	Planned    int      `json:"planned"`
	Completed  int      `json:"completed"`
	Percentage *float64 `json:"adherence_percent"`
}

type trainingHeatmapPoint struct {
	Date        string  `json:"date"`
	Sessions    int     `json:"sessions"`
	WorkingSets int     `json:"working_sets"`
	VolumeKG    float64 `json:"volume_kg"`
}

type trainingStreakSummary struct {
	CurrentDays       int `json:"current_days"`
	LongestLast30Days int `json:"longest_last_30_days"`
	ActiveLast30Days  int `json:"active_days_last_30"`
}

func (r *PostgresRepository) filteredTrainingDuration(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location, filters AnalyticsFilters) ([]trainingDurationPoint, error) {
	rows, err := r.pool.Query(ctx, `
		WITH days AS (SELECT generate_series($2::date,$3::date,'1 day'::interval)::date AS day)
		SELECT to_char(days.day,'YYYY-MM-DD'), duration.total_minutes,
			duration.average_minutes, duration.sessions
		FROM days
		LEFT JOIN LATERAL (
			SELECT sum(extract(epoch FROM session.finished_at-session.started_at)/60)::double precision AS total_minutes,
				avg(extract(epoch FROM session.finished_at-session.started_at)/60)::double precision AS average_minutes,
				count(*)::int AS sessions
			FROM training_sessions session
			WHERE session.owner_id=$1
				AND (session.started_at AT TIME ZONE $4)::date=days.day
				AND session.finished_at IS NOT NULL AND session.finished_at>=session.started_at
				AND (($7::text='' AND session.status='finished') OR ($7::text<>'' AND session.status=$7))
				AND ($6::bigint IS NULL OR session.workout_template_id=$6)
				AND ($5::bigint IS NULL OR EXISTS (SELECT 1 FROM training_session_exercises filtered
					WHERE filtered.session_id=session.id AND filtered.exercise_id=$5))
				AND ($8::bigint IS NULL OR EXISTS (SELECT 1 FROM training_program_revisions filtered_revision
					WHERE filtered_revision.id=session.revision_id AND filtered_revision.program_id=$8))
		) duration ON true
		ORDER BY days.day`, ownerID, dateRange.From.Format("2006-01-02"), dateRange.To.Format("2006-01-02"),
		loc.String(), filters.ExerciseID, filters.TemplateID, filters.Status, filters.PlanID)
	if err != nil {
		return nil, fmt.Errorf("load filtered training duration: %w", err)
	}
	defer rows.Close()
	points := make([]trainingDurationPoint, 0, dateRange.Days())
	for rows.Next() {
		var point trainingDurationPoint
		if err := rows.Scan(&point.Date, &point.DurationMinutes, &point.AverageDurationMinutes, &point.Sessions); err != nil {
			return nil, fmt.Errorf("scan filtered training duration: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate filtered training duration: %w", err)
	}
	return points, nil
}

func (r *PostgresRepository) filteredTrainingMuscleGroups(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location, filters AnalyticsFilters) ([]trainingMuscleGroupPoint, error) {
	if filters.DayType == "rest" {
		return []trainingMuscleGroupPoint{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(btrim(catalog.primary_muscle_group),''),'Без группы') AS muscle_group,
			count(training_set.id)::int AS working_sets,
			COALESCE(sum(training_set.actual_weight_kg*training_set.actual_reps)
				FILTER (WHERE training_set.actual_weight_kg IS NOT NULL),0)::double precision AS volume_kg
		FROM training_sessions session
		JOIN training_session_exercises exercise ON exercise.session_id=session.id
		LEFT JOIN training_exercises catalog ON catalog.id=exercise.exercise_id AND catalog.owner_id=session.owner_id
		JOIN training_sets training_set ON training_set.session_exercise_id=exercise.id
		WHERE session.owner_id=$1
			AND (session.started_at AT TIME ZONE $4)::date BETWEEN $2::date AND $3::date
			AND (($7::text='' AND session.status='finished') OR ($7::text<>'' AND session.status=$7))
			AND ($6::bigint IS NULL OR session.workout_template_id=$6)
			AND ($5::bigint IS NULL OR exercise.exercise_id=$5)
			AND ($8::bigint IS NULL OR EXISTS (SELECT 1 FROM training_program_revisions filtered_revision
				WHERE filtered_revision.id=session.revision_id AND filtered_revision.program_id=$8))
			AND training_set.completed_at IS NOT NULL AND training_set.type IN ('working','drop')
		GROUP BY muscle_group
		ORDER BY working_sets DESC,muscle_group`, ownerID, dateRange.From.Format("2006-01-02"),
		dateRange.To.Format("2006-01-02"), loc.String(), filters.ExerciseID, filters.TemplateID,
		filters.Status, filters.PlanID)
	if err != nil {
		return nil, fmt.Errorf("load filtered training muscle groups: %w", err)
	}
	defer rows.Close()
	points := make([]trainingMuscleGroupPoint, 0)
	for rows.Next() {
		var point trainingMuscleGroupPoint
		if err := rows.Scan(&point.MuscleGroup, &point.WorkingSets, &point.VolumeKG); err != nil {
			return nil, fmt.Errorf("scan filtered training muscle groups: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate filtered training muscle groups: %w", err)
	}
	return points, nil
}

func (r *PostgresRepository) filteredTrainingRIRDistribution(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location, filters AnalyticsFilters) ([]trainingRIRPoint, error) {
	if filters.DayType == "rest" {
		return []trainingRIRPoint{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		WITH buckets AS (
			SELECT LEAST(round(training_set.actual_rir)::int,5) AS bucket, count(*)::int AS sets
			FROM training_sessions session
			JOIN training_session_exercises exercise ON exercise.session_id=session.id
			JOIN training_sets training_set ON training_set.session_exercise_id=exercise.id
			WHERE session.owner_id=$1
				AND (session.started_at AT TIME ZONE $4)::date BETWEEN $2::date AND $3::date
				AND (($7::text='' AND session.status='finished') OR ($7::text<>'' AND session.status=$7))
				AND ($6::bigint IS NULL OR session.workout_template_id=$6)
				AND ($5::bigint IS NULL OR exercise.exercise_id=$5)
				AND ($8::bigint IS NULL OR EXISTS (SELECT 1 FROM training_program_revisions filtered_revision
					WHERE filtered_revision.id=session.revision_id AND filtered_revision.program_id=$8))
				AND training_set.completed_at IS NOT NULL AND training_set.type IN ('working','drop')
				AND training_set.actual_rir IS NOT NULL
			GROUP BY bucket
		)
		SELECT CASE WHEN bucket=5 THEN '5+' ELSE bucket::text END,sets
		FROM buckets ORDER BY bucket`, ownerID, dateRange.From.Format("2006-01-02"),
		dateRange.To.Format("2006-01-02"), loc.String(), filters.ExerciseID, filters.TemplateID,
		filters.Status, filters.PlanID)
	if err != nil {
		return nil, fmt.Errorf("load filtered training RIR distribution: %w", err)
	}
	defer rows.Close()
	points := make([]trainingRIRPoint, 0, 6)
	for rows.Next() {
		var point trainingRIRPoint
		if err := rows.Scan(&point.RIR, &point.Sets); err != nil {
			return nil, fmt.Errorf("scan filtered training RIR distribution: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate filtered training RIR distribution: %w", err)
	}
	return points, nil
}

func (r *PostgresRepository) filteredTrainingAdherence(ctx context.Context, ownerID int64, dateRange DateRange, loc *time.Location, filters AnalyticsFilters) ([]trainingAdherencePoint, error) {
	rows, err := r.pool.Query(ctx, `
		WITH days AS (SELECT generate_series($2::date,$3::date,'1 day'::interval)::date AS day)
		SELECT to_char(days.day,'YYYY-MM-DD'), schedule.planned, schedule.completed
		FROM days
		LEFT JOIN LATERAL (
			SELECT count(*) FILTER (WHERE session.status<>'excused')::int AS planned,
				count(*) FILTER (WHERE session.status='finished')::int AS completed
			FROM training_sessions session
			WHERE session.owner_id=$1
				AND (session.scheduled_at AT TIME ZONE $4)::date=days.day
				AND ($7::text='' OR session.status=$7)
				AND ($6::bigint IS NULL OR session.workout_template_id=$6)
				AND ($5::bigint IS NULL OR EXISTS (SELECT 1 FROM training_session_exercises filtered
					WHERE filtered.session_id=session.id AND filtered.exercise_id=$5))
				AND ($8::bigint IS NULL OR EXISTS (SELECT 1 FROM training_program_revisions filtered_revision
					WHERE filtered_revision.id=session.revision_id AND filtered_revision.program_id=$8))
		) schedule ON true
		ORDER BY days.day`, ownerID, dateRange.From.Format("2006-01-02"), dateRange.To.Format("2006-01-02"),
		loc.String(), filters.ExerciseID, filters.TemplateID, filters.Status, filters.PlanID)
	if err != nil {
		return nil, fmt.Errorf("load filtered training adherence: %w", err)
	}
	defer rows.Close()
	points := make([]trainingAdherencePoint, 0, dateRange.Days())
	for rows.Next() {
		var point trainingAdherencePoint
		if err := rows.Scan(&point.Date, &point.Planned, &point.Completed); err != nil {
			return nil, fmt.Errorf("scan filtered training adherence: %w", err)
		}
		if point.Planned > 0 {
			value := float64(point.Completed) / float64(point.Planned) * 100
			point.Percentage = &value
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate filtered training adherence: %w", err)
	}
	return points, nil
}

func filterTrainingDurationByDays(points []trainingDurationPoint, days []DailyPoint, dayType string) []trainingDurationPoint {
	if dayType == "" {
		return points
	}
	allowed := make(map[string]struct{}, len(days))
	for _, day := range days {
		allowed[day.Date] = struct{}{}
	}
	filtered := make([]trainingDurationPoint, 0, len(allowed))
	for _, point := range points {
		if _, ok := allowed[point.Date]; ok {
			filtered = append(filtered, point)
		}
	}
	return filtered
}

func averageTrainingDuration(points []trainingDurationPoint) *float64 {
	var total float64
	var sessions int
	for _, point := range points {
		if point.DurationMinutes == nil || point.Sessions == 0 {
			continue
		}
		total += *point.DurationMinutes
		sessions += point.Sessions
	}
	if sessions == 0 {
		return nil
	}
	value := total / float64(sessions)
	return &value
}

func adherenceTotals(points []trainingAdherencePoint) (planned, completed int) {
	for _, point := range points {
		planned += point.Planned
		completed += point.Completed
	}
	return planned, completed
}

func trainingHeatmap(points []DailyPoint) []trainingHeatmapPoint {
	result := make([]trainingHeatmapPoint, 0, len(points))
	for _, point := range points {
		result = append(result, trainingHeatmapPoint{
			Date: point.Date, Sessions: point.WorkoutCount, WorkingSets: point.WorkingSets,
			VolumeKG: point.TrainingVolumeKG,
		})
	}
	return result
}

func trainingStreak(points []DailyPoint, dateRange DateRange, loc *time.Location) trainingStreakSummary {
	active := make(map[string]bool, len(points))
	for _, point := range points {
		active[point.Date] = point.WorkoutCount > 0
	}
	current := 0
	for day := dateRange.To; !day.Before(dateRange.From); day = day.AddDate(0, 0, -1) {
		if !active[day.In(loc).Format("2006-01-02")] {
			break
		}
		current++
	}
	windowFrom := dateRange.To.AddDate(0, 0, -29)
	if windowFrom.Before(dateRange.From) {
		windowFrom = dateRange.From
	}
	longest, running, activeDays := 0, 0, 0
	for day := windowFrom; !day.After(dateRange.To); day = day.AddDate(0, 0, 1) {
		if active[day.In(loc).Format("2006-01-02")] {
			activeDays++
			running++
			if running > longest {
				longest = running
			}
		} else {
			running = 0
		}
	}
	return trainingStreakSummary{CurrentDays: current, LongestLast30Days: longest, ActiveLast30Days: activeDays}
}
