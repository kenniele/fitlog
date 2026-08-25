package training

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
)

func FormatSet(set WorkoutSet) string {
	reps := set.Reps
	if set.ActualReps != nil {
		reps = *set.ActualReps
	}
	weight := set.WeightKG
	if set.ActualWeightKG != nil {
		weight = set.ActualWeightKG
	}
	if weight == nil {
		return fmt.Sprintf("%dР -", reps)
	}
	formatted := strconv.FormatFloat(*weight, 'f', -1, 64)
	return fmt.Sprintf("%dР %sКГ", reps, formatted)
}

func FormatSetDetailed(set WorkoutSet) string {
	formatted := FormatSet(set)
	rir := set.RIR
	if set.ActualRIR != nil {
		rir = set.ActualRIR
	}
	if rir != nil {
		formatted += " @ RIR " + formatDecimal(*rir)
	}
	return formatted
}

// FormatActiveCard returns HTML suitable for the one Telegram control card.
func FormatActiveCard(session Session, previous *PreviousExercise, loc *time.Location, prompt string) string {
	exercise := session.CurrentExercise()
	if exercise == nil {
		return "<b>🏋️ " + html.EscapeString(session.ProgramName) + "</b>\n\nНе удалось найти текущее упражнение."
	}
	if exercise.Structured() {
		return formatStructuredActiveCard(session, *exercise, loc, prompt)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "<b>🏋️ %s</b>\n", html.EscapeString(session.ProgramName))
	fmt.Fprintf(&out, "Начало: %s\n", session.StartedAt.In(loc).Format("15:04"))
	fmt.Fprintf(&out, "Упражнение %d из %d\n\n", exercise.Position, len(session.Exercises))
	fmt.Fprintf(&out, "<b>%s</b>\n", html.EscapeString(exercise.Name))
	if previous != nil && len(previous.Sets) > 0 {
		fmt.Fprintf(&out, "\n<b>Прошлый раз · %s</b>\n", previous.StartedAt.In(loc).Format("02.01.2006"))
		for _, set := range previous.Sets {
			fmt.Fprintf(&out, "%d. %s\n", set.Position, FormatSet(set))
		}
	}
	if len(exercise.Sets) == 0 {
		out.WriteString("\nТекущие подходы: пока нет.\n")
	} else {
		out.WriteString("\nТекущие подходы:\n")
		for _, set := range exercise.Sets {
			fmt.Fprintf(&out, "%d. %s\n", set.Position, FormatSet(set))
		}
	}
	if exercise.Note != "" {
		fmt.Fprintf(&out, "\n📝 %s\n", html.EscapeString(exercise.Note))
	}
	if prompt != "" {
		fmt.Fprintf(&out, "\n<b>%s</b>", html.EscapeString(prompt))
	}
	return strings.TrimSpace(out.String())
}

func formatStructuredActiveCard(session Session, exercise SessionExercise, loc *time.Location, prompt string) string {
	var out strings.Builder
	totalWorking, completedWorking, additionalWorking := 0, 0, 0
	for _, item := range session.Exercises {
		totalWorking += item.Plan.WorkingSets
		completed := len(item.WorkingSets())
		completedWorking += min(completed, item.Plan.WorkingSets)
		if completed > item.Plan.WorkingSets {
			additionalWorking += completed - item.Plan.WorkingSets
		}
	}
	fmt.Fprintf(&out, "<b>🏋️ %s</b>\n", html.EscapeString(session.ProgramName))
	fmt.Fprintf(&out, "%s · %d / %d упражнений\n", session.StartedAt.In(loc).Format("02.01.2006"), exercise.Position, len(session.Exercises))
	fmt.Fprintf(&out, "%d / %d рабочих подходов", completedWorking, totalWorking)
	if additionalWorking > 0 {
		fmt.Fprintf(&out, " · +%d дополнительно", additionalWorking)
	}
	out.WriteString("\n\n")
	fmt.Fprintf(&out, "<b>%s</b>\n", html.EscapeString(exercise.Name))

	warmups := exercise.WarmupSets()
	if len(exercise.Warmup) > 0 || len(warmups) > 0 {
		out.WriteString("\n<b>Разминка:</b>\n")
		for index, planned := range exercise.Warmup {
			switch {
			case index < len(warmups):
				if planned.Bar {
					fmt.Fprintf(&out, "✅ гриф × %d\n", warmups[index].Reps)
				} else {
					fmt.Fprintf(&out, "✅ %s\n", formatPlanSet(
						warmups[index].ActualWeightKG, warmups[index].Reps, warmups[index].Reps,
					))
				}
			case index == len(warmups):
				fmt.Fprintf(&out, "➡️ %s\n", formatWarmup(planned))
			default:
				fmt.Fprintf(&out, "○ %s\n", formatWarmup(planned))
			}
		}
		for index := len(exercise.Warmup); index < len(warmups); index++ {
			fmt.Fprintf(&out, "✅ дополнительно · %s\n", FormatSetDetailed(warmups[index]))
		}
	}

	working := exercise.WorkingSets()
	nextWeight := exercise.NextWorkingWeightKG()
	out.WriteString("\n<b>Рабочие:</b>\n")
	for index := 0; index < exercise.Plan.WorkingSets; index++ {
		switch {
		case index < len(working):
			fmt.Fprintf(&out, "✅ %s\n", FormatSetDetailed(working[index]))
		case index == len(working) && len(warmups) >= len(exercise.Warmup):
			fmt.Fprintf(&out, "➡️ %s\n", formatPlanSet(nextWeight, exercise.Plan.MinReps, exercise.Plan.MaxReps))
		default:
			fmt.Fprintf(&out, "○ %s\n", formatPlanSet(nextWeight, exercise.Plan.MinReps, exercise.Plan.MaxReps))
		}
	}
	for index := exercise.Plan.WorkingSets; index < len(working); index++ {
		fmt.Fprintf(&out, "✅ дополнительно · %s\n", FormatSetDetailed(working[index]))
	}
	if len(working) >= exercise.Plan.WorkingSets {
		out.WriteString("\n✅ План выполнен. Можно добавить подход или завершить упражнение.")
	}
	fmt.Fprintf(&out, "\nЦель: RIR %s · отдых между подходами: %s", formatDecimal(exercise.Plan.TargetRIR), formatSeconds(exercise.Plan.RestSeconds))
	if exercise.Overridden {
		out.WriteString("\n✏️ Рекомендация изменена для этой тренировки.")
	}
	if len(working) > 0 && nextWeight != nil {
		fmt.Fprintf(&out, "\n\n<b>Почему:</b>\nПовторяем последний фактический вес этой тренировки: %s кг.", formatDecimal(*nextWeight))
	} else if exercise.Recommendation.Reason != "" {
		fmt.Fprintf(&out, "\n\n<b>Почему:</b>\n%s", html.EscapeString(exercise.Recommendation.Reason))
	}
	if session.RestUntil != nil && session.RestUntil.After(time.Now()) {
		label, seconds := currentRestInstruction(session, exercise)
		if seconds > 0 {
			fmt.Fprintf(&out, "\n\n%s: <b>%s</b>", label, formatSeconds(seconds))
		}
	}
	if exercise.Note != "" {
		fmt.Fprintf(&out, "\n\n📝 %s", html.EscapeString(exercise.Note))
	}
	if prompt != "" {
		fmt.Fprintf(&out, "\n\n<b>%s</b>", html.EscapeString(prompt))
	}
	return strings.TrimSpace(out.String())
}

func formatWarmup(set WarmupSet) string {
	if set.Bar {
		return fmt.Sprintf("гриф × %d", set.Reps)
	}
	return formatPlanSet(set.WeightKG, set.Reps, set.Reps)
}

func formatPlanSet(weight *float64, minReps, maxReps int) string {
	weightText := "вес вручную"
	if weight != nil {
		weightText = formatDecimal(*weight) + " кг"
	}
	reps := strconv.Itoa(minReps)
	if minReps != maxReps {
		reps += "–" + strconv.Itoa(maxReps)
	}
	return weightText + " × " + reps
}

func formatSeconds(seconds int) string {
	if seconds <= 0 {
		return "0 секунд"
	}
	return fmt.Sprintf("%d секунд", seconds)
}

func currentRestInstruction(session Session, current SessionExercise) (string, int) {
	if len(current.Sets) == 0 {
		for index := len(session.Exercises) - 1; index >= 0; index-- {
			previous := session.Exercises[index]
			if previous.Position < current.Position && previous.Complete {
				return "Отдых перед упражнением", previous.Plan.AfterSeconds
			}
		}
	}
	return "Отдых после подхода", current.Plan.RestSeconds
}

func formatDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func FormatFinished(session Session, loc *time.Location) string {
	day := session.StartedAt.In(loc).Format("02.01.2006")
	var out strings.Builder
	out.WriteString("<b>🏋️ Эффект Гоггинса</b>\n")
	fmt.Fprintf(&out, "%s · %s\n", day, html.EscapeString(session.ProgramName))
	if session.FinishedAt != nil {
		fmt.Fprintf(&out, "Начало: %s · Конец: %s\n",
			session.StartedAt.In(loc).Format("15:04"), session.FinishedAt.In(loc).Format("15:04"),
		)
		fmt.Fprintf(&out, "Длительность: %s\n", FormatSessionDuration(session))
	}
	for _, exercise := range session.Exercises {
		out.WriteString("\n<b>" + html.EscapeString(exercise.Name) + "</b>\n")
		if len(exercise.Sets) == 0 {
			out.WriteString("пропуск\n")
		} else {
			warmupIndex := 0
			for _, set := range exercise.Sets {
				prefix := ""
				formatted := FormatSetDetailed(set)
				if set.Type == SetTypeWarmup {
					prefix = "разминка · "
					if warmupIndex < len(exercise.Warmup) && exercise.Warmup[warmupIndex].Bar && set.ActualWeightKG == nil && set.WeightKG == nil {
						formatted = fmt.Sprintf("гриф × %d", set.Reps)
						if set.ActualReps != nil {
							formatted = fmt.Sprintf("гриф × %d", *set.ActualReps)
						}
					}
					warmupIndex++
				} else if set.Type == SetTypeDrop {
					prefix = "drop · "
				}
				out.WriteString(prefix + formatted + "\n")
			}
		}
		if exercise.Note != "" {
			out.WriteString("Заметка: " + html.EscapeString(exercise.Note) + "\n")
		}
	}

	workingSets, warmupSets := session.SetCounts()
	dropSets := session.DropSetCount()
	skipped := 0
	for _, exercise := range session.Exercises {
		if len(exercise.Sets) == 0 {
			skipped++
		}
	}
	if warmupSets > 0 || dropSets > 0 || sessionHasStructuredExercises(session) {
		fmt.Fprintf(&out, "\nУпражнений: %d · Рабочих подходов: %d · Разминочных: %d", len(session.Exercises), workingSets, warmupSets)
		if dropSets > 0 {
			fmt.Fprintf(&out, " · Drop: %d", dropSets)
		}
	} else {
		fmt.Fprintf(&out, "\nУпражнений: %d · Подходов: %d", len(session.Exercises), workingSets)
	}
	if skipped > 0 {
		fmt.Fprintf(&out, " · Пропусков: %d", skipped)
	}
	return strings.TrimSpace(out.String())
}

func sessionHasStructuredExercises(session Session) bool {
	for _, exercise := range session.Exercises {
		if exercise.Structured() {
			return true
		}
	}
	return false
}

func FormatSessionDuration(session Session) string {
	if session.FinishedAt == nil || session.FinishedAt.Before(session.StartedAt) {
		return "—"
	}
	duration := session.FinishedAt.Sub(session.StartedAt).Round(time.Minute)
	if duration < time.Minute {
		return "менее минуты"
	}
	hours := int(duration / time.Hour)
	minutes := int(duration%time.Hour) / int(time.Minute)
	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%d ч %d мин", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%d ч", hours)
	default:
		return fmt.Sprintf("%d мин", minutes)
	}
}
