package domain

// DailyNutrition is the per-day macro rollup returned by FatSecret's
// `food_entries.get_month`.
type DailyNutrition struct {
	DateInt  int // days since 1970-01-01 UTC
	Calories float64
	Protein  float64
	Carbs    float64
	Fat      float64
}
