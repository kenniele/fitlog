package fatsecret

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"fitlog/internal/domain"
)

const MaxBackfillDays = 366

var ErrStoragePermissionRequired = errors.New("persistent FatSecret content storage requires explicit authorization")

// NutritionBackfillSource supplies monthly macros and daily entries for fiber,
// which is not available from the monthly endpoint.
type NutritionBackfillSource interface {
	FoodEntriesMonth(context.Context, time.Time) ([]domain.DailyNutrition, error)
	FoodEntriesForDay(context.Context, time.Time) ([]domain.MealEntry, error)
}

// NutritionBackfillSink atomically upserts a provider batch. Implementations
// must scope every write to ownerID and the FatSecret sync namespace.
type NutritionBackfillSink interface {
	UpsertFatSecretNutritionDays(context.Context, int64, []domain.NutritionDaySnapshot) error
}

type BackfillOptions struct {
	From              time.Time
	To                time.Time
	Location          *time.Location
	DryRun            bool
	StorageAuthorized bool
}

type BackfillResult struct {
	From                string
	To                  string
	RequestedDays       int
	RequestedMonths     int
	LoggedDays          int
	UpsertedDays        int
	FiberDays           int
	LatestAvailableDate string
	DryRun              bool
}

// BackfillNutritionDays fetches the complete provider range before touching
// PostgreSQL. A provider error therefore cannot leave a partially refreshed
// history. Existing rows are upserted, while absent days are deliberately not
// pruned because FatSecret has previously returned lagging monthly snapshots.
func BackfillNutritionDays(
	ctx context.Context,
	source NutritionBackfillSource,
	sink NutritionBackfillSink,
	ownerID int64,
	options BackfillOptions,
) (BackfillResult, error) {
	loc := options.Location
	if loc == nil {
		loc = time.UTC
	}
	from := calendarDay(options.From, loc)
	to := calendarDay(options.To, loc)
	if from.After(to) {
		return BackfillResult{}, fmt.Errorf("invalid FatSecret range: from is after to")
	}
	days := inclusiveDayCount(from, to)
	if days < 1 || days > MaxBackfillDays {
		return BackfillResult{}, fmt.Errorf("invalid FatSecret range: use 1-%d days", MaxBackfillDays)
	}
	if !options.DryRun && !options.StorageAuthorized {
		return BackfillResult{}, ErrStoragePermissionRequired
	}
	if !options.DryRun && sink == nil {
		return BackfillResult{}, errors.New("FatSecret backfill sink is required")
	}

	result := BackfillResult{
		From: from.Format("2006-01-02"), To: to.Format("2006-01-02"),
		RequestedDays: days, DryRun: options.DryRun,
	}
	seen := make(map[int]domain.DailyNutrition)
	for _, month := range monthsInRange(from, to, loc) {
		result.RequestedMonths++
		rows, err := source.FoodEntriesMonth(ctx, month)
		if err != nil {
			return BackfillResult{}, fmt.Errorf("fetch FatSecret month %s: %w", month.Format("2006-01"), err)
		}
		for _, row := range rows {
			if row.DateInt < ToDateInt(from) || row.DateInt > ToDateInt(to) {
				continue
			}
			seen[row.DateInt] = row
		}
	}

	dateInts := make([]int, 0, len(seen))
	for dateInt := range seen {
		dateInts = append(dateInts, dateInt)
	}
	sort.Ints(dateInts)
	snapshots := make([]domain.NutritionDaySnapshot, 0, len(dateInts))
	for _, dateInt := range dateInts {
		row := seen[dateInt]
		date := FromDateInt(dateInt)
		day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
		entries, err := source.FoodEntriesForDay(ctx, day)
		if err != nil {
			return BackfillResult{}, fmt.Errorf("fetch FatSecret day %s: %w", day.Format("2006-01-02"), err)
		}
		for _, entry := range entries {
			if entry.DateInt != dateInt {
				return BackfillResult{}, fmt.Errorf("FatSecret day %s returned entry for date_int %d",
					day.Format("2006-01-02"), entry.DateInt)
			}
		}
		fiber, incomplete := totalFiber(entries)
		if fiber != nil {
			result.FiberDays++
		}
		snapshots = append(snapshots, domain.NutritionDaySnapshot{
			DateInt: dateInt, CaloriesKcal: floatPointer(row.Calories), ProteinG: floatPointer(row.Protein),
			FatG: floatPointer(row.Fat), CarbohydratesG: floatPointer(row.Carbs),
			FiberG: fiber, FiberIncomplete: incomplete,
		})
	}
	result.LoggedDays = len(snapshots)
	if len(dateInts) > 0 {
		result.LatestAvailableDate = FromDateInt(dateInts[len(dateInts)-1]).Format("2006-01-02")
	}
	if options.DryRun {
		return result, nil
	}
	if err := sink.UpsertFatSecretNutritionDays(ctx, ownerID, snapshots); err != nil {
		return BackfillResult{}, fmt.Errorf("persist FatSecret nutrition: %w", err)
	}
	result.UpsertedDays = len(snapshots)
	return result, nil
}

func calendarDay(value time.Time, loc *time.Location) time.Time {
	value = value.In(loc)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc)
}

func inclusiveDayCount(from, to time.Time) int {
	count := 0
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		count++
	}
	return count
}

func monthsInRange(from, to time.Time, loc *time.Location) []time.Time {
	month := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, loc)
	last := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, loc)
	months := make([]time.Time, 0, 1)
	for !month.After(last) {
		months = append(months, month)
		month = month.AddDate(0, 1, 0)
	}
	return months
}

func floatPointer(value float64) *float64 {
	copy := value
	return &copy
}
