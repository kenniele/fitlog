package fatsecret

import (
	"testing"

	"github.com/stretchr/testify/require"

	"fitlog/internal/domain"
)

func TestParseFoodEntries_ArrayOfTwo(t *testing.T) {
	body := []byte(`{
        "food_entries": {
            "food_entry": [
                {
                    "food_entry_id": "111",
                    "food_entry_name": "Coffee with Sugar",
                    "food_entry_description": "1 cup",
                    "date_int": "20585",
                    "meal": "Breakfast",
                    "number_of_units": "1.000",
                    "calories": "38",
                    "protein": "0.1",
                    "carbohydrate": "9.5",
                    "fat": "0.0",
                    "fiber": null
                },
                {
                    "food_entry_id": "222",
                    "food_entry_name": "Tuna",
                    "food_entry_description": "100 g",
                    "date_int": "20585",
                    "meal": "Lunch",
                    "calories": "120",
                    "protein": "27.17",
                    "fat": "1.0"
                }
            ]
        }
    }`)
	out, err := parseFoodEntries(body)
	require.NoError(t, err)
	require.Len(t, out, 2)

	require.Equal(t, "111", out[0].ExternalID)
	require.Equal(t, domain.MealBreakfast, out[0].Meal)
	require.Equal(t, 20585, out[0].DateInt)
	require.NotNil(t, out[0].Calories)
	require.Equal(t, 38.0, *out[0].Calories)
	require.Nil(t, out[0].Fiber, "null field should map to nil pointer")

	require.Equal(t, domain.MealLunch, out[1].Meal)
	require.NotNil(t, out[1].Protein)
	require.Equal(t, 27.17, *out[1].Protein)
	require.Nil(t, out[1].Fiber, "missing field should map to nil pointer")
}

func TestParseFoodEntries_SingleObject(t *testing.T) {
	body := []byte(`{
        "food_entries": {
            "food_entry": {
                "food_entry_id": "1",
                "food_entry_name": "Egg",
                "date_int": "20585",
                "meal": "Breakfast",
                "calories": "70"
            }
        }
    }`)
	out, err := parseFoodEntries(body)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "Egg", out[0].FoodName)
}

func TestParseFoodEntries_Empty(t *testing.T) {
	for _, body := range []string{
		`{"food_entries":{"food_entry":null}}`,
		`{"food_entries":{"food_entry":""}}`,
		`{"food_entries":{}}`,
	} {
		out, err := parseFoodEntries([]byte(body))
		require.NoError(t, err, "body=%s", body)
		require.Empty(t, out)
	}
}

func TestParseFoodEntries_UnknownMealKindFallsToOther(t *testing.T) {
	body := []byte(`{"food_entries":{"food_entry":{"food_entry_id":"1","meal":"Snack","date_int":"1"}}}`)
	out, err := parseFoodEntries(body)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, domain.MealOther, out[0].Meal)
}

func TestParseMonth_Array(t *testing.T) {
	body := []byte(`{
        "month": {
            "from_date_int": "20580",
            "to_date_int": "20585",
            "day": [
                {"date_int": "20580", "calories": "2000", "protein": "120", "carbohydrate": "200", "fat": "80"},
                {"date_int": "20581", "calories": "1800", "protein": "110", "carbohydrate": "180", "fat": "70"}
            ]
        }
    }`)
	out, err := parseMonth(body)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, 20580, out[0].DateInt)
	require.Equal(t, 2000.0, out[0].Calories)
	require.Equal(t, 120.0, out[0].Protein)
}

func TestParseMonth_SingleDay(t *testing.T) {
	body := []byte(`{"month":{"from_date_int":"1","to_date_int":"1","day":{"date_int":"1","calories":"500"}}}`)
	out, err := parseMonth(body)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, 500.0, out[0].Calories)
}

func TestParseMonth_Empty(t *testing.T) {
	out, err := parseMonth([]byte(`{"month":{"day":null}}`))
	require.NoError(t, err)
	require.Empty(t, out)
}
