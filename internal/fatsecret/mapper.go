package fatsecret

import (
	"encoding/json"
	"fmt"

	"fitlog/internal/domain"
)

// mapMealKind maps FatSecret's meal name to the domain enum. Unknown values
// fall back to MealOther rather than failing — FatSecret occasionally tags
// snacks/uncategorized items with off-spec strings.
func mapMealKind(s string) domain.MealKind {
	switch s {
	case "Breakfast":
		return domain.MealBreakfast
	case "Lunch":
		return domain.MealLunch
	case "Dinner":
		return domain.MealDinner
	default:
		return domain.MealOther
	}
}

// toMealEntry converts a single foodEntryDTO to a domain.MealEntry.
func (e *foodEntryDTO) toDomain() domain.MealEntry {
	return domain.MealEntry{
		ExternalID:           e.FoodEntryID,
		DateInt:              int(e.DateInt),
		Meal:                 mapMealKind(e.Meal),
		FoodName:             e.FoodEntryName,
		FoodEntryDescription: e.FoodEntryDescription,
		NumberOfUnits:        e.NumberOfUnits.ptr(),
		Calories:             e.Calories.ptr(),
		Protein:              e.Protein.ptr(),
		Carbs:                e.Carbohydrate.ptr(),
		Fat:                  e.Fat.ptr(),
		Fiber:                e.Fiber.ptr(),
		Sugar:                e.Sugar.ptr(),
		Sodium:               e.Sodium.ptr(),
		Potassium:            e.Potassium.ptr(),
		Cholesterol:          e.Cholesterol.ptr(),
		SaturatedFat:         e.SaturatedFat.ptr(),
		MonounsaturatedFat:   e.MonounsaturatedFat.ptr(),
		PolyunsaturatedFat:   e.PolyunsaturatedFat.ptr(),
		TransFat:             e.TransFat.ptr(),
		Calcium:              e.Calcium.ptr(),
		Iron:                 e.Iron.ptr(),
		VitaminA:             e.VitaminA.ptr(),
		VitaminC:             e.VitaminC.ptr(),
	}
}

// parseFoodEntries decodes a `food_entries.get.v2` response into domain MealEntries.
func parseFoodEntries(body []byte) ([]domain.MealEntry, error) {
	var env foodEntriesResponse
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("unmarshal food_entries: %w", err)
	}
	rows := normalizeList(env.FoodEntries.FoodEntry)
	out := make([]domain.MealEntry, 0, len(rows))
	for i, raw := range rows {
		var dto foodEntryDTO
		if err := json.Unmarshal(raw, &dto); err != nil {
			return nil, fmt.Errorf("unmarshal food_entry[%d]: %w", i, err)
		}
		out = append(out, dto.toDomain())
	}
	return out, nil
}

// parseMonth decodes a `food_entries.get_month` response into domain DailyNutrition rows.
func parseMonth(body []byte) ([]domain.DailyNutrition, error) {
	var env monthResponse
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("unmarshal month: %w", err)
	}
	rows := normalizeList(env.Month.Day)
	out := make([]domain.DailyNutrition, 0, len(rows))
	for i, raw := range rows {
		var dto dayDTO
		if err := json.Unmarshal(raw, &dto); err != nil {
			return nil, fmt.Errorf("unmarshal day[%d]: %w", i, err)
		}
		out = append(out, domain.DailyNutrition{
			DateInt:  int(dto.DateInt),
			Calories: dto.Calories.asFloat(),
			Protein:  dto.Protein.asFloat(),
			Carbs:    dto.Carbohydrate.asFloat(),
			Fat:      dto.Fat.asFloat(),
		})
	}
	return out, nil
}
