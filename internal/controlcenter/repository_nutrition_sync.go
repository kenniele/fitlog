package controlcenter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"fitlog/internal/domain"
)

// UpsertFatSecretNutritionDays persists one fully fetched range atomically.
// It only owns source=fatsecret rows under the sync:day namespace and cannot
// overwrite manual/file imports or another dashboard owner.
func (r *PostgresRepository) UpsertFatSecretNutritionDays(
	ctx context.Context,
	ownerID int64,
	days []domain.NutritionDaySnapshot,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin FatSecret nutrition sync: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lockKey := "fatsecret-nutrition:" + strconv.FormatInt(ownerID, 10)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return fmt.Errorf("lock FatSecret nutrition sync: %w", err)
	}
	for _, day := range days {
		if day.DateInt < 0 {
			return fmt.Errorf("invalid FatSecret date_int %d", day.DateInt)
		}
		entryDate := domainDate(day.DateInt)
		externalID := "sync:day:" + strconv.Itoa(day.DateInt)
		if _, err := tx.Exec(ctx, `INSERT INTO nutrition_days (
			owner_id,entry_date,calories_kcal,protein_g,fat_g,carbohydrates_g,
			fiber_g,sugar_g,saturated_fat_g,sodium_mg,potassium_mg,water_ml,
			source,external_id,notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'fatsecret',$13,'')
			ON CONFLICT (owner_id,source,external_id) WHERE external_id IS NOT NULL
			DO UPDATE SET entry_date=EXCLUDED.entry_date,calories_kcal=EXCLUDED.calories_kcal,
				protein_g=EXCLUDED.protein_g,fat_g=EXCLUDED.fat_g,
				carbohydrates_g=EXCLUDED.carbohydrates_g,fiber_g=EXCLUDED.fiber_g,
				sugar_g=EXCLUDED.sugar_g,saturated_fat_g=EXCLUDED.saturated_fat_g,
				sodium_mg=EXCLUDED.sodium_mg,potassium_mg=EXCLUDED.potassium_mg,
				water_ml=EXCLUDED.water_ml,updated_at=now()`,
			ownerID, entryDate, day.CaloriesKcal, day.ProteinG, day.FatG, day.CarbohydratesG,
			day.FiberG, day.SugarG, day.SaturatedFatG, day.SodiumMg, day.PotassiumMg,
			day.WaterML, externalID,
		); err != nil {
			return fmt.Errorf("upsert FatSecret day %s: %w", entryDate, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit FatSecret nutrition sync: %w", err)
	}
	return nil
}

func domainDate(dateInt int) string {
	// FatSecret date_int is a calendar-day index; formatting the UTC inverse
	// yields the same DATE independently of the dashboard timezone.
	return time.Unix(int64(dateInt)*24*60*60, 0).UTC().Format("2006-01-02")
}
