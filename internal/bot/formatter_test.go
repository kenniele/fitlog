package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fitlog/internal/domain"
)

func TestMDV2Escape(t *testing.T) {
	require.Equal(t, `hello`, mdv2Escape("hello"))
	require.Equal(t, `1\.5`, mdv2Escape("1.5"))
	require.Equal(t, `a\_b\*c`, mdv2Escape("a_b*c"))
	require.Equal(t, `\(\)\[\]\{\}`, mdv2Escape("()[]{}"))
	require.Equal(t, `\!\#\+\-\=\|`, mdv2Escape("!#+-=|"))
}

func TestRecoveryAndStrainEmoji(t *testing.T) {
	require.Equal(t, "🟢", recoveryEmoji(67))
	require.Equal(t, "🟡", recoveryEmoji(50))
	require.Equal(t, "🔴", recoveryEmoji(10))
	require.Equal(t, "🟦", strainEmoji(5))
	require.Equal(t, "🟪", strainEmoji(10))
	require.Equal(t, "🟧", strainEmoji(15))
	require.Equal(t, "🟥", strainEmoji(20))
}

func TestWorkoutEmoji(t *testing.T) {
	require.Equal(t, "💪", workoutEmoji("Powerlifting"))
	require.Equal(t, "🏃", workoutEmoji("Running"))
	require.Equal(t, "🚶", workoutEmoji("Walking"))
	require.Equal(t, "⚡", workoutEmoji("Activity"))
}

func TestFmtDurationColon(t *testing.T) {
	require.Equal(t, "2:29", fmtDurationColon(int64((2*time.Hour+29*time.Minute)/time.Millisecond)))
	require.Equal(t, "0:01", fmtDurationColon(int64(time.Minute/time.Millisecond)))
	require.Equal(t, "0:00", fmtDurationColon(0))
}

func TestFormatInfo_FullPayload(t *testing.T) {
	loc := time.UTC
	day := time.Date(2026, 5, 11, 0, 0, 0, 0, loc)
	wokAt := day.Add(11 * time.Hour)

	skin, spo2 := 33.5, 97.1
	rec := &domain.Recovery{CycleID: 12345, Score: 72, HRVMilli: 73, RestingHR: 53, SkinTempC: &skin, SpO2Pct: &spo2}
	end := day.Add(24 * time.Hour)
	cyc := &domain.Cycle{ID: 12345, Start: wokAt, End: &end, Strain: 13.6, AvgHR: 66, MaxHR: 156, Kilojoule: 9000}
	sl := &domain.Sleep{
		Start: day.Add(-1 * time.Hour), End: wokAt,
		SleepPerformancePct: 92, SleepEfficiencyPct: 92, SleepConsistencyPct: 85,
		RespiratoryRate: 14.5,
		Stages: domain.SleepStages{
			InBedMs: int64(9*time.Hour) / int64(time.Millisecond),
			LightMs: int64(4*time.Hour+11*time.Minute) / int64(time.Millisecond),
			SWSMs:   int64(2*time.Hour+18*time.Minute) / int64(time.Millisecond),
			REMMs:   int64(2*time.Hour+29*time.Minute) / int64(time.Millisecond),
		},
		SleepCycleCount: 5, DisturbanceCount: 3,
	}

	cal, prot, fat, carbs, fiber := 38.0, 0.1, 0.0, 9.5, 0.0
	meals := []domain.MealEntry{
		{Meal: domain.MealBreakfast, FoodName: "Кофе с Сахаром",
			Calories: &cal, Protein: &prot, Fat: &fat, Carbs: &carbs, Fiber: &fiber},
	}
	wos := []domain.Workout{
		{SportID: 59, SportName: "Powerlifting",
			Start:           day.Add(11 * time.Hour),
			End:             day.Add(12*time.Hour + 14*time.Minute),
			Strain:          12.6, MaxHR: 156, AvgHR: 110,
			Kilojoule:       1500, PercentRecorded: 100,
			ZoneDurations: domain.ZoneDurations{Zone1: 600_000, Zone2: 1_200_000}},
	}

	out := FormatInfo(InfoPayload{
		Day: day, Loc: loc, Cycle: cyc, Recovery: rec, Sleep: sl, Workouts: wos, Meals: meals,
	})

	require.True(t, strings.HasPrefix(out, "📊 *11 мая 2026*"))
	require.Contains(t, out, "*Recovery* 72% 🟢")
	require.Contains(t, out, "HRV 73 ms")
	require.Contains(t, out, "SpO2 97\\.1%")
	require.Contains(t, out, "Кожная температура 33\\.5°C")
	require.Contains(t, out, "*Daily strain* 13\\.6 🟪")
	require.Contains(t, out, "Powerlifting")
	require.Contains(t, out, "Strain 12\\.6")
	require.Contains(t, out, "Зоны HR")
	require.Contains(t, out, "Кофе с Сахаром")
	require.Contains(t, out, "Микро за день")
	require.Contains(t, out, "REM 2ч 29м")
	require.Contains(t, out, "Sleep need")
}

func TestFormatInfo_EmptyShowsPlaceholders(t *testing.T) {
	out := FormatInfo(InfoPayload{
		Day: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
		Loc: time.UTC,
	})
	require.Contains(t, out, "Сон* — данных нет")
	require.Contains(t, out, "Recovery* — данных нет")
	require.Contains(t, out, "Daily strain* — данных нет")
	require.Contains(t, out, "Тренировки* — нет")
	require.Contains(t, out, "Питание* — записей нет")
}

func TestSplitForTelegram_Short(t *testing.T) {
	chunks := SplitForTelegram("hello")
	require.Equal(t, []string{"hello"}, chunks)
}

func TestSplitForTelegram_BreaksOnBlankLine(t *testing.T) {
	a := strings.Repeat("a", telegramMessageLimit-200)
	b := strings.Repeat("b", 500)
	full := a + "\n\n" + b
	chunks := SplitForTelegram(full)
	require.Len(t, chunks, 2)
	require.Equal(t, a, chunks[0])
	require.Equal(t, b, chunks[1])
}

func TestSplitForTelegram_MidLineFallback(t *testing.T) {
	// A single line longer than the limit must still be split.
	full := strings.Repeat("x", telegramMessageLimit+100)
	chunks := SplitForTelegram(full)
	require.Len(t, chunks, 2)
	for _, c := range chunks {
		require.LessOrEqual(t, len(c), telegramMessageLimit)
	}
}
