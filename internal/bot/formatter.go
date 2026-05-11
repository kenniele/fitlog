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

// InfoPayload bundles the data the verbose day report renders from.
type InfoPayload struct {
	Day      time.Time
	Loc      *time.Location
	Cycle    *domain.Cycle
	Recovery *domain.Recovery
	Sleep    *domain.Sleep
	Workouts []domain.Workout
	Meals    []domain.MealEntry

	// WhoopStatus describes the outcome of Whoop API calls so the user knows
	// whether "данных нет" means "Whoop вернул пусто" or "не удалось получить".
	WhoopStatus string
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
		writeRecovery(&b, p.Recovery)
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

func writeRecovery(b *strings.Builder, r *domain.Recovery) {
	fmt.Fprintf(b, "💪 *Recovery* %s%% %s — %s\\.\n",
		fmtFloat(r.Score, 0), recoveryEmoji(r.Score), mdv2Escape(recoveryLabel(r.Score)))
	fmt.Fprintf(b, "  HRV %s ms, RHR %s bpm\\.\n",
		fmtFloat(r.HRVMilli, 0), fmtFloat(r.RestingHR, 0))
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
