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
			Start:  day.Add(11 * time.Hour),
			End:    day.Add(12*time.Hour + 14*time.Minute),
			Strain: 12.6, MaxHR: 156, AvgHR: 110,
			Kilojoule: 1500, PercentRecorded: 100,
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

func TestDeltaArrow(t *testing.T) {
	require.Equal(t, "↑8", deltaArrow(8, 0))
	require.Equal(t, "↓4", deltaArrow(-4, 0))
	require.Equal(t, "→", deltaArrow(0, 0))
	require.Equal(t, "→", deltaArrow(0.2, 0))
	// fmtFloat escapes the decimal point for MDv2 inside the arrow output.
	require.Equal(t, "↑0\\.5", deltaArrow(0.5, 1))
	require.Equal(t, "↓2\\.3", deltaArrow(-2.3, 1))
}

func TestFormatNotes(t *testing.T) {
	loc := time.UTC
	ts := time.Date(2026, 5, 11, 8, 30, 0, 0, loc)
	w := 105.2
	waist := 92.5
	bf := 22.4
	ns := []domain.Note{
		{Ts: ts, Kind: domain.NoteKindWeight, Value: &w, Body: "натощак"},
		{Ts: ts.Add(1 * time.Hour), Kind: domain.NoteKindWaist, Value: &waist},
		{Ts: ts.Add(90 * time.Minute), Kind: domain.NoteKindBodyfat, Value: &bf},
		{Ts: ts.Add(2 * time.Hour), Kind: domain.NoteKindSymptom, Body: "горло першит"},
		{Ts: ts.Add(4 * time.Hour), Kind: domain.NoteKindNote, Body: "Усталость в ногах"},
	}
	out := FormatNotes(ns, loc)
	require.Contains(t, out, "*Заметки*")
	require.Contains(t, out, "вес *105\\.2 кг*")
	require.Contains(t, out, "талия *92\\.5 см*")
	require.Contains(t, out, "жир *22\\.4 %*")
	require.Contains(t, out, "натощак")
	require.Contains(t, out, "🩹")
	require.Contains(t, out, "горло першит")
	require.Contains(t, out, "Усталость в ногах")
}

func TestFormatPeriodDigest_Deficit(t *testing.T) {
	consumed := map[int]float64{1: 1800, 2: 1700, 3: 1850}
	burned := map[int]float64{1: 2100, 2: 2200, 3: 2050}
	out := FormatPeriodDigest(consumed, burned, 3)

	require.Contains(t, out, "Дайджест")
	require.Contains(t, out, "Съедено: *5350 kcal*")
	require.Contains(t, out, "Потрачено: *6350 kcal*")
	// 1800+1700+1850=5350; 2100+2200+2050=6350. diff=-1000, avg=-333/day.
	require.Contains(t, out, "Баланс: *\\-1000 kcal*")
	require.Contains(t, out, "дефицит ✅")
	require.Contains(t, out, "За жир: *\\-0\\.13 кг*")
	require.Contains(t, out, "Дни: дефицит 3 · профицит 0 · около нормы 0")
}

func TestFormatPeriodDigest_Empty(t *testing.T) {
	require.Empty(t, FormatPeriodDigest(map[int]float64{}, map[int]float64{}, 7))
}

// Only one side of the data present → no meaningful balance, so the digest is
// suppressed instead of inventing a phantom deficit/surplus.
func TestFormatPeriodDigest_OneSidedSuppressed(t *testing.T) {
	require.Empty(t, FormatPeriodDigest(map[int]float64{}, map[int]float64{1: 2000, 2: 2100}, 7))
	require.Empty(t, FormatPeriodDigest(map[int]float64{1: 1800}, map[int]float64{}, 7))
	// Days that exist in only one map are ignored, not counted as 0.
	require.Empty(t, FormatPeriodDigest(map[int]float64{1: 1800}, map[int]float64{2: 2000}, 7))
}

// When the two maps overlap on only some days, totals/averages/verdict are
// computed over the overlapping days only and divided by that count — not by
// the full requested window — and the header says so.
func TestFormatPeriodDigest_PartialOverlap(t *testing.T) {
	consumed := map[int]float64{1: 9999, 2: 1700, 3: 1900} // day 1 has no burn → excluded
	burned := map[int]float64{2: 2200, 3: 2100, 4: 8888}   // day 4 has no food → excluded
	out := FormatPeriodDigest(consumed, burned, 7)

	require.Contains(t, out, "по 2 дня с данными")
	require.Contains(t, out, "Съедено: *3600 kcal*")   // 1700+1900, day 1 excluded
	require.Contains(t, out, "Потрачено: *4300 kcal*") // 2200+2100, day 4 excluded
	require.Contains(t, out, "Баланс: *\\-700 kcal*")
	require.Contains(t, out, "дефицит ✅")
	require.Contains(t, out, "Дни: дефицит 2 · профицит 0 · около нормы 0")
}

// Regression: a numeric note with a free-text comment containing MarkdownV2
// reserved chars (or a literal '*') must be escaped exactly once — never
// double-escaped (which Telegram rejects).
func TestFormatNotes_BodyEscaping(t *testing.T) {
	loc := time.UTC
	ts := time.Date(2026, 6, 12, 8, 0, 0, 0, loc)
	w := 80.0
	out := FormatNotes([]domain.Note{
		{Ts: ts, Kind: domain.NoteKindWeight, Value: &w, Body: "после еды (-0.5)"},
	}, loc)

	require.Contains(t, out, "после еды \\(\\-0\\.5\\)") // single escape
	require.NotContains(t, out, `\\(`)                   // no double-backslash
	require.Contains(t, out, "*80 кг*")                  // bold value intact

	// A literal '*' in the body must not break the bold parity.
	out2 := FormatNotes([]domain.Note{
		{Ts: ts, Kind: domain.NoteKindWeight, Value: &w, Body: "сет 3*8"},
	}, loc)
	require.Contains(t, out2, "*80 кг*")
	require.Contains(t, out2, "сет 3\\*8")
}

func TestFormatPeriodDigest_Surplus(t *testing.T) {
	consumed := map[int]float64{1: 3000}
	burned := map[int]float64{1: 2000}
	out := FormatPeriodDigest(consumed, burned, 1)

	require.Contains(t, out, "Баланс: *\\+1000 kcal*")
	require.Contains(t, out, "профицит")
	require.Contains(t, out, "За жир: *\\+0\\.13 кг*")
}

func TestCleanServing(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	cases := []struct {
		desc, name string
		units      *float64
		want       string
	}{
		// :custom: tag with mass label & multiplier > 1 → show "size label × units"
		{"1.3 :custom:130s  г Fish House Тунец", "Fish House Тунец", f(1.3), "130 г × 1.3"},
		{"2 1/2 :custom:250s  г Йогурт", "Йогурт", f(2.5), "250 г × 2.5"},
		// units == size && mass label → user typed raw grams; collapse.
		{"60 :custom:60s  г Сырники", "Сырники", f(60), "60 г"},
		// :custom: with non-numeric label, units == 1 → drop "× 1".
		{"1 :custom:1 котлета Котлета Домашняя", "Котлета Домашняя", f(1), "1 котлета"},
		{"1 :custom:1 обычный ломтик Хлеб Черный", "Хлеб Черный", f(1), "1 обычный ломтик"},
		// No :custom: marker — built-in serving description.
		{"2 medium Вареное Яйцо", "Вареное Яйцо", f(2), "2 medium"},
		{"1 mug Кофе с Сахаром", "Кофе с Сахаром", f(1), "1 mug"},
		// Empty description with units → at least show the units.
		{"", "X", f(1.5), "× 1.5"},
	}
	for _, tc := range cases {
		got := cleanServing(tc.desc, tc.name, tc.units)
		require.Equal(t, tc.want, got, "desc=%q name=%q", tc.desc, tc.name)
	}
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
