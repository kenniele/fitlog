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

// NutritionDaySnapshot is a provider-neutral daily rollup ready for durable
// persistence. Optional fields stay nil when the source does not expose them;
// missing micronutrients must never be turned into zeroes by a sync.
type NutritionDaySnapshot struct {
	DateInt        int
	CaloriesKcal   *float64
	ProteinG       *float64
	FatG           *float64
	CarbohydratesG *float64
	FiberG         *float64
	SugarG         *float64
	SaturatedFatG  *float64
	SodiumMg       *float64
	PotassiumMg    *float64
	WaterML        *float64
}

// NutritionAnalysis is an aggregate for a completed calendar period. Deficit
// is positive when intake is below the configured estimated TDEE.
type NutritionAnalysis struct {
	Calories      float64
	Protein       float64
	Fat           float64
	Carbs         float64
	EstimatedTDEE float64
	Deficit       float64
}
