package controlcenter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) loadBodyFatInputs(ctx context.Context, ownerID int64, now time.Time, loc *time.Location) (bodyFatInputs, error) {
	var baseline bodyFatBaseline
	err := r.pool.QueryRow(ctx, `
		SELECT measured_at, weight_kg::double precision, fat_mass_kg::double precision, body_fat_percent::double precision
		FROM body_measurements
		WHERE owner_id = $1 AND source = 'inbody' AND measured_at <= $2
		ORDER BY measured_at DESC, id DESC LIMIT 1`, ownerID, now,
	).Scan(&baseline.MeasuredAt, &baseline.WeightKG, &baseline.FatMassKG, &baseline.FatPercent)
	if errors.Is(err, pgx.ErrNoRows) {
		return bodyFatInputs{}, nil
	}
	if err != nil {
		return bodyFatInputs{}, fmt.Errorf("load body fat baseline: %w", err)
	}
	inputs := bodyFatInputs{Baseline: &baseline}
	rng := bodyFatDateRange(baseline.MeasuredAt, now, loc)
	if days := rng.Days(); days == 0 || days > MaxRangeDays {
		return inputs, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (entry_date) to_char(entry_date, 'YYYY-MM-DD'), calories_kcal::double precision
		FROM nutrition_days
		WHERE owner_id = $1 AND entry_date BETWEEN $2::date AND $3::date
		ORDER BY entry_date, updated_at DESC, id DESC`, ownerID, rng.From.Format("2006-01-02"), rng.To.Format("2006-01-02"))
	if err != nil {
		return bodyFatInputs{}, fmt.Errorf("load body fat energy balance: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var day DailyPoint
		if err := rows.Scan(&day.Date, &day.CaloriesKcal); err != nil {
			return bodyFatInputs{}, fmt.Errorf("scan body fat energy balance: %w", err)
		}
		inputs.Days = append(inputs.Days, day)
	}
	if err := rows.Err(); err != nil {
		return bodyFatInputs{}, fmt.Errorf("iterate body fat energy balance: %w", err)
	}
	return inputs, nil
}
