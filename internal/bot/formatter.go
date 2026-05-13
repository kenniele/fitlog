package bot

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"fitlog/internal/domain"
)

// fatSecretCustomMarker captures the size value and (unused) unit code from
// FatSecret's ":custom:<size><code>" token embedded in food_entry_description.
// Examples in the wild:
//
//	"1.3 :custom:130s г Tuna ..." → size=130, code=s
//	"1 :custom:1 котлета Котлета Домашняя" → size=1, code=""
var fatSecretCustomMarker = regexp.MustCompile(`:custom:(\d+(?:\.\d+)?)([a-zA-Z]*)`)

// massVolumeLabels are unit labels for which "size == units" likely means
// the user entered the value as raw mass/volume (e.g. "60 :custom:60s г").
// In that case we collapse the redundant "60 г × 60" into just "60 г".
var massVolumeLabels = map[string]bool{
	"г": true, "g": true, "мл": true, "ml": true, "kg": true, "кг": true,
}

func fmtUnitsCompact(u float64) string {
	s := fmt.Sprintf("%.2f", u)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}

// cleanServing turns FatSecret's noisy food_entry_description into a tidy
// human label by parsing the :custom: marker for the serving size, stripping
// the duplicated food name, and rejoining what's left.
//
// Output by example:
//
//	"1.3 :custom:130s г Fish House Тунец"     name=Fish House Тунец      → "130 г × 1.3"
//	"60 :custom:60s г Сырники"                name=Сырники              → "60 г"      (size==units, mass label)
//	"1 :custom:1 котлета Котлета Домашняя"    name=Котлета Домашняя     → "1 котлета"
//	"2 medium Вареное Яйцо"                   name=Вареное Яйцо         → "2 medium"
//	"1 mug Кофе с Сахаром"                    name=Кофе с Сахаром       → "1 mug"
func cleanServing(desc, name string, units *float64) string {
	if desc == "" {
		if units != nil && *units != 0 {
			return "× " + fmtUnitsCompact(*units)
		}
		return ""
	}

	stripName := func(s string) string {
		if name != "" {
			s = strings.ReplaceAll(s, name, "")
		}
		return strings.Join(strings.Fields(s), " ")
	}

	loc := fatSecretCustomMarker.FindStringSubmatchIndex(desc)
	if loc == nil {
		// Built-in serving description like "2 medium" or "1 mug".
		out := stripName(desc)
		out = strings.Trim(out, " ,")
		if out != "" {
			return out
		}
		if units != nil && *units != 0 {
			return "× " + fmtUnitsCompact(*units)
		}
		return ""
	}

	size := desc[loc[2]:loc[3]] // first capture group
	after := stripName(desc[loc[1]:])

	u := 0.0
	if units != nil {
		u = *units
	}

	// Heuristic: "60 :custom:60s г" means user entered raw grams. Collapse.
	if u > 0 && massVolumeLabels[after] {
		if sf, err := strconv.ParseFloat(size, 64); err == nil && sf == u {
			return strings.TrimSpace(size + " " + after)
		}
	}

	if u == 0 || u == 1 {
		return strings.TrimSpace(size + " " + after)
	}
	return strings.TrimSpace(size + " " + after + " × " + fmtUnitsCompact(u))
}

// telegramMessageLimit is the hard cap on a single MarkdownV2 message.
// We split a hair below to leave room for occasional escape overhead.
const telegramMessageLimit = 4000

// mdv2Escape escapes the MarkdownV2 reserved set per Telegram's docs.
func mdv2Escape(s string) string {
	const reserved = "_*[]()~`>#+-=|{}.!"
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if strings.ContainsRune(reserved, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// fmtFloat formats a float with the given decimals, then escapes for MDv2.
// Trailing zeros and a dangling "." are stripped only when a decimal point
// is present — otherwise "0.1" with decimals=0 would render as "" (bug).
func fmtFloat(f float64, decimals int) string {
	s := fmt.Sprintf("%.*f", decimals, f)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return mdv2Escape(s)
}

func recoveryEmoji(score float64) string {
	switch {
	case score >= 67:
		return "🟢"
	case score >= 34:
		return "🟡"
	default:
		return "🔴"
	}
}

func recoveryLabel(score float64) string {
	switch {
	case score >= 67:
		return "хорошее восстановление"
	case score >= 34:
		return "среднее восстановление"
	default:
		return "слабое восстановление"
	}
}

func strainEmoji(strain float64) string {
	switch {
	case strain >= 18:
		return "🟥"
	case strain >= 14:
		return "🟧"
	case strain >= 10:
		return "🟪"
	default:
		return "🟦"
	}
}

func strainLabel(strain float64) string {
	switch {
	case strain >= 18:
		return "очень высокая нагрузка"
	case strain >= 14:
		return "высокая нагрузка"
	case strain >= 10:
		return "заметная нагрузка"
	default:
		return "лёгкий день"
	}
}

func mealEmoji(m domain.MealKind) string {
	switch m {
	case domain.MealBreakfast:
		return "🍳"
	case domain.MealLunch:
		return "🥗"
	case domain.MealDinner:
		return "🍽"
	default:
		return "🥨"
	}
}

func mealLabel(m domain.MealKind) string {
	switch m {
	case domain.MealBreakfast:
		return "Завтрак"
	case domain.MealLunch:
		return "Обед"
	case domain.MealDinner:
		return "Ужин"
	default:
		return "Other"
	}
}

func workoutEmoji(sportName string) string {
	s := strings.ToLower(sportName)
	switch {
	case strings.Contains(s, "running"), strings.Contains(s, "run"):
		return "🏃"
	case strings.Contains(s, "walking"), strings.Contains(s, "walk"), strings.Contains(s, "hik"):
		return "🚶"
	case strings.Contains(s, "powerlift"), strings.Contains(s, "weightlift"), strings.Contains(s, "strength"), strings.Contains(s, "functional"):
		return "💪"
	default:
		return "⚡"
	}
}

// fmtDurationHM renders ms as "Hч Mм".
func fmtDurationHM(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%dч %dм", h, m)
}

// fmtDurationColon renders ms as "H:MM".
func fmtDurationColon(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%d:%02d", h, m)
}

func fmtClock(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("15:04")
}

func fmtDateLong(t time.Time, loc *time.Location) string {
	months := []string{"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря"}
	t = t.In(loc)
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()], t.Year())
}

// fmtDate is the shorter "11 мая" form used inside lists.
func fmtDate(t time.Time, loc *time.Location) string {
	months := []string{"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря"}
	t = t.In(loc)
	return fmt.Sprintf("%d %s", t.Day(), months[t.Month()])
}

func pFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// kjToKcal converts kilojoules → kilocalories.
func kjToKcal(kj float64) float64 { return kj / 4.184 }

// kcalPerKgFat is the conventional energy density of body fat used for rough
// "fat lost / gained" estimates. It's an approximation (real value varies),
// but ±7700 kcal per kg is the number every fitness app uses.
const kcalPerKgFat = 7700.0

// FormatPeriodDigest builds the "📊 Дайджест" block: consumed vs burned over
// a multi-day window, with deficit/surplus verdict and a rough fat-mass delta.
// Maps are keyed by FatSecret date_int (days since 1970-01-01 UTC) so the
// per-day deficit counter can match nutrition to Whoop without TZ ambiguity.
//
// Returns empty string if there's nothing to show.
func FormatPeriodDigest(consumedByDay, burnedByDay map[int]float64, days int) string {
	var totalConsumed, totalBurned float64
	for _, v := range consumedByDay {
		totalConsumed += v
	}
	for _, v := range burnedByDay {
		totalBurned += v
	}
	if totalConsumed == 0 && totalBurned == 0 {
		return ""
	}

	d := float64(days)
	if d == 0 {
		d = 1
	}
	diff := totalConsumed - totalBurned
	perDay := diff / d

	verdict := "около нормы"
	switch {
	case perDay <= -300:
		verdict = "дефицит ✅"
	case perDay <= -100:
		verdict = "лёгкий дефицит"
	case perDay >= 300:
		verdict = "профицит ⚠️"
	case perDay >= 100:
		verdict = "лёгкий профицит"
	}

	var b strings.Builder
	b.WriteString("📊 *Дайджест*\n")
	fmt.Fprintf(&b, "  Съедено: *%s kcal* \\(avg %s/день\\)\n",
		fmtFloat(totalConsumed, 0), fmtFloat(totalConsumed/d, 0))
	fmt.Fprintf(&b, "  Потрачено: *%s kcal* \\(avg %s/день\\)\n",
		fmtFloat(totalBurned, 0), fmtFloat(totalBurned/d, 0))
	fmt.Fprintf(&b, "  Баланс: *%s kcal* \\(%s/день\\) — %s\n",
		mdv2Escape(fmt.Sprintf("%+.0f", diff)),
		mdv2Escape(fmt.Sprintf("%+.0f", perDay)),
		mdv2Escape(verdict))

	// Fat-mass delta: negative diff (deficit) → fat lost.
	fatKg := -diff / kcalPerKgFat
	if fatKg > 0.01 {
		fmt.Fprintf(&b, "  За жир: *\\-%s кг* \\(1 кг ≈ 7700 kcal\\)\n", fmtFloat(fatKg, 2))
	} else if fatKg < -0.01 {
		fmt.Fprintf(&b, "  За жир: *\\+%s кг* \\(1 кг ≈ 7700 kcal\\)\n", fmtFloat(-fatKg, 2))
	}

	// Per-day deficit/surplus counter — only for days that have BOTH numbers.
	allDates := map[int]struct{}{}
	for di := range consumedByDay {
		allDates[di] = struct{}{}
	}
	for di := range burnedByDay {
		allDates[di] = struct{}{}
	}
	var defDays, surDays, neutralDays int
	for di := range allDates {
		cv, ok1 := consumedByDay[di]
		bv, ok2 := burnedByDay[di]
		if !ok1 || !ok2 || cv == 0 || bv == 0 {
			continue
		}
		switch dv := cv - bv; {
		case dv < -100:
			defDays++
		case dv > 100:
			surDays++
		default:
			neutralDays++
		}
	}
	if defDays+surDays+neutralDays > 0 {
		fmt.Fprintf(&b, "  Дни: дефицит %d · профицит %d · около нормы %d\n",
			defDays, surDays, neutralDays)
	}

	return b.String()
}

// Baseline is the rolling average over the days preceding the target day,
// used to draw "↑8 vs 7d" inline deltas. Nil means "not enough data".
type Baseline struct {
	Days        int
	RecoveryAvg float64
	HRVAvg      float64
	RHRAvg      float64
}

// InfoPayload bundles the data the verbose day report renders from.
type InfoPayload struct {
	Day      time.Time
	Loc      *time.Location
	Cycle    *domain.Cycle
	Recovery *domain.Recovery
	Sleep    *domain.Sleep
	Workouts []domain.Workout
	Meals    []domain.MealEntry
	Baseline *Baseline

	// WhoopStatus describes the outcome of Whoop API calls so the user knows
	// whether "данных нет" means "Whoop вернул пусто" or "не удалось получить".
	WhoopStatus string
}

// deltaArrow renders a signed delta as "↑5", "↓3", or "→" if near zero.
// decimals is how many digits after the dot.
func deltaArrow(delta float64, decimals int) string {
	abs := delta
	if abs < 0 {
		abs = -abs
	}
	if abs < 0.5 && decimals == 0 {
		return "→"
	}
	if abs < 0.05 && decimals > 0 {
		return "→"
	}
	if delta > 0 {
		return "↑" + fmtFloat(delta, decimals)
	}
	return "↓" + fmtFloat(-delta, decimals)
}

// FormatInfo renders a long, narrative day report touching every field we
// pull from Whoop and FatSecret. Result is MarkdownV2 — caller passes it
// to SplitForTelegram before sending.
func FormatInfo(p InfoPayload) string {
	var b strings.Builder

	fmt.Fprintf(&b, "📊 *%s*\n\n", mdv2Escape(fmtDateLong(p.Day, p.Loc)))

	if p.WhoopStatus != "" {
		fmt.Fprintf(&b, "⚠️ %s\n\n", mdv2Escape(p.WhoopStatus))
	}

	if p.Sleep != nil {
		writeSleep(&b, p.Sleep, p.Loc)
	} else {
		b.WriteString("🌙 *Сон* — данных нет\n\n")
	}

	if p.Recovery != nil {
		writeRecovery(&b, p.Recovery, p.Baseline)
	} else {
		b.WriteString("💪 *Recovery* — данных нет\n\n")
	}

	if p.Cycle != nil {
		writeCycle(&b, p.Cycle, p.Loc)
	} else {
		b.WriteString("⚡ *Daily strain* — данных нет\n\n")
	}

	if len(p.Workouts) > 0 {
		writeWorkouts(&b, p.Workouts, p.Loc)
	} else {
		b.WriteString("🏋 *Тренировки* — нет\n\n")
	}

	writeMealsVerbose(&b, p.Meals)

	return strings.TrimRight(b.String(), "\n")
}

func writeSleep(b *strings.Builder, s *domain.Sleep, loc *time.Location) {
	total := s.Stages.LightMs + s.Stages.SWSMs + s.Stages.REMMs
	b.WriteString("🌙 *Сон*\n")
	fmt.Fprintf(b, "  Лёг в %s, проснулся в %s — итого %s\\.\n",
		mdv2Escape(fmtClock(s.Start, loc)),
		mdv2Escape(fmtClock(s.End, loc)),
		mdv2Escape(fmtDurationHM(total)))
	if s.IsNap {
		b.WriteString("  Это nap, не основной сон\\.\n")
	}
	fmt.Fprintf(b, "  Performance %s%%, efficiency %s%%, consistency %s%%\\.\n",
		fmtFloat(s.SleepPerformancePct, 0),
		fmtFloat(s.SleepEfficiencyPct, 0),
		fmtFloat(s.SleepConsistencyPct, 0))
	if s.RespiratoryRate > 0 {
		fmt.Fprintf(b, "  Дыхание во сне: %s в минуту\\.\n", fmtFloat(s.RespiratoryRate, 1))
	}
	fmt.Fprintf(b, "  В постели: %s\\.\n", mdv2Escape(fmtDurationHM(s.Stages.InBedMs)))
	fmt.Fprintf(b, "  Бодрствование: %s, циклов сна %d, нарушений %d\\.\n",
		mdv2Escape(fmtDurationHM(s.Stages.AwakeMs)), s.SleepCycleCount, s.DisturbanceCount)
	fmt.Fprintf(b, "  Фазы: REM %s, deep %s, light %s\\.\n",
		mdv2Escape(fmtDurationHM(s.Stages.REMMs)),
		mdv2Escape(fmtDurationHM(s.Stages.SWSMs)),
		mdv2Escape(fmtDurationHM(s.Stages.LightMs)))
	fmt.Fprintf(b, "  Sleep need: baseline %s, debt %s, strain %s, nap %s\\.\n\n",
		mdv2Escape(fmtDurationHM(s.SleepNeed.BaselineMs)),
		mdv2Escape(fmtDurationHM(s.SleepNeed.FromDebtMs)),
		mdv2Escape(fmtDurationHM(s.SleepNeed.FromStrainMs)),
		mdv2Escape(fmtDurationHM(s.SleepNeed.FromNapMs)))
}

func writeRecovery(b *strings.Builder, r *domain.Recovery, base *Baseline) {
	fmt.Fprintf(b, "💪 *Recovery* %s%% %s", fmtFloat(r.Score, 0), recoveryEmoji(r.Score))
	if base != nil {
		fmt.Fprintf(b, " \\(%s vs %dd avg\\)",
			mdv2Escape(deltaArrow(r.Score-base.RecoveryAvg, 0)), base.Days)
	}
	fmt.Fprintf(b, " — %s\\.\n", mdv2Escape(recoveryLabel(r.Score)))

	fmt.Fprintf(b, "  HRV %s ms", fmtFloat(r.HRVMilli, 0))
	if base != nil {
		fmt.Fprintf(b, " \\(%s vs %dd\\)",
			mdv2Escape(deltaArrow(r.HRVMilli-base.HRVAvg, 0)), base.Days)
	}
	fmt.Fprintf(b, ", RHR %s bpm", fmtFloat(r.RestingHR, 0))
	if base != nil {
		fmt.Fprintf(b, " \\(%s vs %dd\\)",
			mdv2Escape(deltaArrow(r.RestingHR-base.RHRAvg, 0)), base.Days)
	}
	b.WriteString("\\.\n")
	if r.SpO2Pct != nil {
		fmt.Fprintf(b, "  SpO2 %s%%\\.\n", fmtFloat(*r.SpO2Pct, 1))
	}
	if r.SkinTempC != nil {
		fmt.Fprintf(b, "  Кожная температура %s°C\\.\n", fmtFloat(*r.SkinTempC, 2))
	}
	if r.UserCalibrating {
		b.WriteString("  Whoop ещё калибруется, score может быть неточным\\.\n")
	}
	if r.CycleID != 0 {
		fmt.Fprintf(b, "  Cycle id: %d\\.\n", r.CycleID)
	}
	b.WriteString("\n")
}

func writeCycle(b *strings.Builder, c *domain.Cycle, loc *time.Location) {
	fmt.Fprintf(b, "⚡ *Daily strain* %s %s — %s\\.\n",
		fmtFloat(c.Strain, 1), strainEmoji(c.Strain), mdv2Escape(strainLabel(c.Strain)))
	if c.End != nil {
		fmt.Fprintf(b, "  Цикл: %s → %s\\.\n",
			mdv2Escape(fmtClock(c.Start, loc)), mdv2Escape(fmtClock(*c.End, loc)))
	} else {
		fmt.Fprintf(b, "  Цикл стартовал в %s, ещё идёт\\.\n", mdv2Escape(fmtClock(c.Start, loc)))
	}
	fmt.Fprintf(b, "  HR avg %s bpm, max %s bpm\\.\n", fmtFloat(c.AvgHR, 0), fmtFloat(c.MaxHR, 0))
	fmt.Fprintf(b, "  Энергозатраты: %s kJ \\(\\~%s kcal\\)\\.\n",
		fmtFloat(c.Kilojoule, 0), fmtFloat(kjToKcal(c.Kilojoule), 0))
	if c.ScoreState != "" {
		fmt.Fprintf(b, "  Score state: %s\\.\n", mdv2Escape(c.ScoreState))
	}
	if c.TZOffset != "" {
		fmt.Fprintf(b, "  TZ offset: %s\\.\n", mdv2Escape(c.TZOffset))
	}
	b.WriteString("\n")
}

func writeWorkouts(b *strings.Builder, ws []domain.Workout, loc *time.Location) {
	fmt.Fprintf(b, "🏋 *Тренировки* \\(%d\\)\n\n", len(ws))
	for i, w := range ws {
		dur := w.End.Sub(w.Start)
		fmt.Fprintf(b, "%d\\. %s *%s* в %s–%s \\(%s\\)\n",
			i+1, workoutEmoji(w.SportName), mdv2Escape(w.SportName),
			mdv2Escape(fmtClock(w.Start, loc)), mdv2Escape(fmtClock(w.End, loc)),
			mdv2Escape(fmtDurationHM(dur.Milliseconds())))
		fmt.Fprintf(b, "   Strain %s %s\\.\n", fmtFloat(w.Strain, 1), strainEmoji(w.Strain))
		fmt.Fprintf(b, "   HR avg %s, max %s\\.\n", fmtFloat(w.AvgHR, 0), fmtFloat(w.MaxHR, 0))
		fmt.Fprintf(b, "   %s kJ \\(\\~%s kcal\\)\\.\n",
			fmtFloat(w.Kilojoule, 0), fmtFloat(kjToKcal(w.Kilojoule), 0))
		fmt.Fprintf(b, "   Записано: %s%%\\.\n", fmtFloat(w.PercentRecorded, 0))
		if w.DistanceM != nil && *w.DistanceM > 0 {
			fmt.Fprintf(b, "   Дистанция: %s м\\.\n", fmtFloat(*w.DistanceM, 0))
		}
		if w.AltitudeGainM != nil && *w.AltitudeGainM > 0 {
			fmt.Fprintf(b, "   Набор высоты: %s м\\.\n", fmtFloat(*w.AltitudeGainM, 0))
		}
		fmt.Fprintf(b, "   Зоны HR \\(время\\): Z0 %s · Z1 %s · Z2 %s · Z3 %s · Z4 %s · Z5 %s\\.\n",
			mdv2Escape(fmtDurationColon(w.ZoneDurations.Zone0)),
			mdv2Escape(fmtDurationColon(w.ZoneDurations.Zone1)),
			mdv2Escape(fmtDurationColon(w.ZoneDurations.Zone2)),
			mdv2Escape(fmtDurationColon(w.ZoneDurations.Zone3)),
			mdv2Escape(fmtDurationColon(w.ZoneDurations.Zone4)),
			mdv2Escape(fmtDurationColon(w.ZoneDurations.Zone5)))
		b.WriteString("\n")
	}
}

// dayMicros aggregates everything-but-macros across all entries of a day.
// Each pointer remains nil if none of the entries reported the field, so we
// can omit absent micros instead of printing "Fiber 0".
type dayMicros struct {
	Fiber, Sugar                                       *float64
	Cholesterol                                        *float64
	Sat, Mono, Poly, Trans                             *float64
	Na, K, Ca, Fe                                      *float64
	VitA, VitC                                         *float64
}

// add folds a single MealEntry's micros into the accumulator.
func (m *dayMicros) add(e domain.MealEntry) {
	accum(&m.Fiber, e.Fiber)
	accum(&m.Sugar, e.Sugar)
	accum(&m.Cholesterol, e.Cholesterol)
	accum(&m.Sat, e.SaturatedFat)
	accum(&m.Mono, e.MonounsaturatedFat)
	accum(&m.Poly, e.PolyunsaturatedFat)
	accum(&m.Trans, e.TransFat)
	accum(&m.Na, e.Sodium)
	accum(&m.K, e.Potassium)
	accum(&m.Ca, e.Calcium)
	accum(&m.Fe, e.Iron)
	accum(&m.VitA, e.VitaminA)
	accum(&m.VitC, e.VitaminC)
}

// accum adds *v into *acc, allocating *acc on first non-nil v so absent
// fields stay nil in the final output.
func accum(acc **float64, v *float64) {
	if v == nil {
		return
	}
	if *acc == nil {
		x := *v
		*acc = &x
		return
	}
	**acc += *v
}

func writeMealsVerbose(b *strings.Builder, entries []domain.MealEntry) {
	if len(entries) == 0 {
		b.WriteString("🥗 *Питание* — записей нет\n")
		return
	}
	var cal, prot, fat, carbs float64
	micros := dayMicros{}
	for _, e := range entries {
		cal += pFloat(e.Calories)
		prot += pFloat(e.Protein)
		fat += pFloat(e.Fat)
		carbs += pFloat(e.Carbs)
		micros.add(e)
	}
	fmt.Fprintf(b, "🥗 *Питание* — %s kcal · %s P · %s F · %s C\n\n",
		fmtFloat(cal, 0), fmtFloat(prot, 0), fmtFloat(fat, 0), fmtFloat(carbs, 0))

	groups := map[domain.MealKind][]domain.MealEntry{}
	for _, e := range entries {
		groups[e.Meal] = append(groups[e.Meal], e)
	}
	for _, k := range []domain.MealKind{domain.MealBreakfast, domain.MealLunch, domain.MealDinner, domain.MealOther} {
		es := groups[k]
		if len(es) == 0 {
			continue
		}
		sort.SliceStable(es, func(i, j int) bool { return es[i].FoodName < es[j].FoodName })

		var mc, mp, mf, mca float64
		for _, e := range es {
			mc += pFloat(e.Calories)
			mp += pFloat(e.Protein)
			mf += pFloat(e.Fat)
			mca += pFloat(e.Carbs)
		}
		fmt.Fprintf(b, "%s *%s* — %s kcal · %s P · %s F · %s C\n",
			mealEmoji(k), mdv2Escape(mealLabel(k)),
			fmtFloat(mc, 0), fmtFloat(mp, 0), fmtFloat(mf, 0), fmtFloat(mca, 0))

		for _, e := range es {
			writeFoodLine(b, e)
		}
		b.WriteString("\n")
	}

	writeDayMicros(b, micros)
}

// writeFoodLine renders one food entry as a single line:
//
//	• Name (serving) — KCAL · P · F · C
func writeFoodLine(b *strings.Builder, e domain.MealEntry) {
	serving := cleanServing(e.FoodEntryDescription, e.FoodName, e.NumberOfUnits)

	b.WriteString("  • *")
	b.WriteString(mdv2Escape(e.FoodName))
	b.WriteString("*")
	if serving != "" {
		fmt.Fprintf(b, " \\(%s\\)", mdv2Escape(serving))
	}

	parts := []string{}
	if e.Calories != nil {
		parts = append(parts, fmt.Sprintf("%s kcal", fmtFloat(*e.Calories, 0)))
	}
	if e.Protein != nil {
		parts = append(parts, fmt.Sprintf("%s P", fmtFloat(*e.Protein, 1)))
	}
	if e.Fat != nil {
		parts = append(parts, fmt.Sprintf("%s F", fmtFloat(*e.Fat, 1)))
	}
	if e.Carbs != nil {
		parts = append(parts, fmt.Sprintf("%s C", fmtFloat(*e.Carbs, 1)))
	}
	if len(parts) > 0 {
		fmt.Fprintf(b, " — %s", strings.Join(parts, " · "))
	}
	b.WriteString("\n")
}

// writeDayMicros prints aggregate micros for the day, grouped into logical
// sections. Empty groups are skipped entirely.
func writeDayMicros(b *strings.Builder, m dayMicros) {
	var lines []string

	if l := microLine(map[string]*float64{"Fiber": m.Fiber, "Sugar": m.Sugar}, "", 1); l != "" {
		lines = append(lines, l)
	}
	if l := microLine(map[string]*float64{"Cholesterol": m.Cholesterol}, "mg", 0); l != "" {
		lines = append(lines, l)
	}
	if l := microLineOrdered([]microSpec{
		{"Sat", m.Sat, "", 1}, {"Mono", m.Mono, "", 1},
		{"Poly", m.Poly, "", 1}, {"Trans", m.Trans, "", 1},
	}); l != "" {
		lines = append(lines, l)
	}
	if l := microLineOrdered([]microSpec{
		{"Na", m.Na, "mg", 0}, {"K", m.K, "mg", 0},
		{"Ca", m.Ca, "mg", 0}, {"Fe", m.Fe, "mg", 1},
	}); l != "" {
		lines = append(lines, l)
	}
	if l := microLineOrdered([]microSpec{
		{"Vit A", m.VitA, "", 0}, {"Vit C", m.VitC, "", 0},
	}); l != "" {
		lines = append(lines, l)
	}

	if len(lines) == 0 {
		return
	}
	b.WriteString("📊 *Микро за день*\n")
	for _, l := range lines {
		fmt.Fprintf(b, "  %s\n", l)
	}
}

type microSpec struct {
	label    string
	value    *float64
	unit     string
	decimals int
}

func microLineOrdered(specs []microSpec) string {
	parts := []string{}
	for _, s := range specs {
		if s.value == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s%s", s.label, fmtFloat(*s.value, s.decimals), s.unit))
	}
	return strings.Join(parts, " · ")
}

// microLine is the unordered convenience used for tiny groups; map order
// is alphabetised so output is stable.
func microLine(fields map[string]*float64, unit string, decimals int) string {
	keys := make([]string, 0, len(fields))
	for k, v := range fields {
		if v != nil {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %s%s", k, fmtFloat(*fields[k], decimals), unit))
	}
	return strings.Join(parts, " · ")
}

// FormatNotes renders a section of user log entries (weight, notes, symptoms)
// newest first. Returns empty string if the slice is empty.
func FormatNotes(ns []domain.Note, loc *time.Location) string {
	if len(ns) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n📝 *Заметки*\n")
	for _, n := range ns {
		date := fmtDate(n.Ts, loc)
		clock := fmtClock(n.Ts, loc)
		switch n.Kind {
		case domain.NoteKindWeight:
			b.WriteString(numericNoteLine(date, "вес", n.Value, "кг", n.Body))
		case domain.NoteKindWaist:
			b.WriteString(numericNoteLine(date, "талия", n.Value, "см", n.Body))
		case domain.NoteKindBodyfat:
			b.WriteString(numericNoteLine(date, "жир", n.Value, "%", n.Body))
		case domain.NoteKindSymptom:
			b.WriteString(fmt.Sprintf("  • %s %s · 🩹 %s\n",
				mdv2Escape(date), mdv2Escape(clock), mdv2Escape(n.Body)))
		default:
			b.WriteString(fmt.Sprintf("  • %s %s · %s\n",
				mdv2Escape(date), mdv2Escape(clock), mdv2Escape(n.Body)))
		}
	}
	return b.String()
}

// numericNoteLine renders one row for a measurement-style note (weight, waist).
// The value goes in bold; the date and any trailing comment are plain text.
func numericNoteLine(date, label string, value *float64, unit, body string) string {
	val := "—"
	if value != nil {
		val = fmtFloat(*value, 2)
	}
	line := fmt.Sprintf("  • %s · %s *%s %s*", date, label, val, unit)
	if body != "" {
		line += " · " + mdv2Escape(body)
	}
	return mdv2EscapePrefix(line) + "\n"
}

// mdv2EscapePrefix escapes the parts of a pre-built line that we DIDN'T
// already write as MarkdownV2 syntax (the * blocks). Kept tiny on purpose —
// only used for the weight line where we mix bold and escaped text.
func mdv2EscapePrefix(s string) string {
	// Hack: split by "*", odd-indexed segments are bold content (already raw),
	// even-indexed are surrounding text that needs escaping.
	parts := strings.Split(s, "*")
	for i := range parts {
		if i%2 == 0 {
			parts[i] = mdv2Escape(parts[i])
		}
	}
	return strings.Join(parts, "*")
}

// SplitForTelegram breaks s into chunks no longer than telegramMessageLimit,
// preferring to split on blank lines, then plain newlines, then mid-line as
// a last resort. Each chunk is suitable for sending as one message.
func SplitForTelegram(s string) []string {
	if len(s) <= telegramMessageLimit {
		return []string{s}
	}
	var out []string
	remaining := s
	for len(remaining) > telegramMessageLimit {
		cut := strings.LastIndex(remaining[:telegramMessageLimit], "\n\n")
		if cut < 0 {
			cut = strings.LastIndex(remaining[:telegramMessageLimit], "\n")
		}
		if cut < 0 {
			cut = telegramMessageLimit
		}
		out = append(out, strings.TrimRight(remaining[:cut], "\n"))
		remaining = strings.TrimLeft(remaining[cut:], "\n")
	}
	if remaining != "" {
		out = append(out, remaining)
	}
	return out
}
