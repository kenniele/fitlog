package controlcenter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) NextCompactWorkout(ctx context.Context, ownerID int64, from time.Time) (*CompactWorkout, error) {
	var workout CompactWorkout
	err := r.pool.QueryRow(ctx, `SELECT session.id,
		COALESCE(NULLIF(template.name,''),NULLIF(session.program_name,''),'Тренировка'),session.scheduled_at
		FROM training_sessions session
		LEFT JOIN workout_templates template ON template.id=session.workout_template_id AND template.owner_id=$1
		WHERE session.owner_id=$1 AND session.status='scheduled' AND session.scheduled_at >= $2
		ORDER BY session.scheduled_at,session.id LIMIT 1`, ownerID, from).Scan(&workout.ID, &workout.Title, &workout.ScheduledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("compact next workout: %w", err)
	}
	return &workout, nil
}
