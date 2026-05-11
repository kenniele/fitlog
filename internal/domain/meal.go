package domain

// MealKind identifies which meal slot an entry belongs to.
type MealKind string

const (
	MealBreakfast MealKind = "Breakfast"
	MealLunch     MealKind = "Lunch"
	MealDinner    MealKind = "Dinner"
	MealOther     MealKind = "Other"
)

// MealEntry is a single FatSecret food_entry. FatSecret returns sparse
// nutrition fields per entry, so every macro/micro is an optional pointer.
type MealEntry struct {
	ExternalID           string
	DateInt              int // days since 1970-01-01 UTC
	Meal                 MealKind
	FoodName             string
	FoodEntryDescription string
	NumberOfUnits        *float64
	Calories             *float64
	Protein              *float64
	Carbs                *float64
	Fat                  *float64
	Fiber                *float64
	Sugar                *float64
	Sodium               *float64
	Potassium            *float64
	Cholesterol          *float64
	SaturatedFat         *float64
	MonounsaturatedFat   *float64
	PolyunsaturatedFat   *float64
	TransFat             *float64
	Calcium              *float64
	Iron                 *float64
	VitaminA             *float64
	VitaminC             *float64
}
