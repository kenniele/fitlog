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
	if set.Type == SetTypeWarmup {
		return formatted
	}
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
	var out strings.Builder
	fmt.Fprintf(&out, "<b>🏋️ %s</b>\n", html.EscapeString(session.ProgramName))
	fmt.Fprintf(&out, "Начало: %s\n", session.StartedAt.In(loc).Format("15:04"))
	fmt.Fprintf(&out, "Упражнение %d из %d\n\n", exercise.Position, len(session.Exercises))
	fmt.Fprintf(&out, "<b>%s</b>\n", html.EscapeString(exercise.Name))
	if previous != nil && len(previous.Sets) > 0 {
		fmt.Fprintf(&out, "\n<b>Прошлый раз · %s</b>\n", previous.StartedAt.In(loc).Format("02.01.2006"))
		for _, set := range previous.Sets {
			fmt.Fprintf(&out, "%d. %s\n", set.Position, formatLoggedSet(set))
		}
	}
	if len(exercise.Sets) == 0 {
		out.WriteString("\nТекущие подходы: пока нет.\n")
	} else {
		out.WriteString("\nТекущие подходы:\n")
		for _, set := range exercise.Sets {
			fmt.Fprintf(&out, "%d. %s\n", set.Position, formatLoggedSet(set))
		}
	}
	if weight, ok := exercise.LastWorkingWeightKG(); ok {
		if weight == nil {
			out.WriteString("\nПоследний вес: собственный вес.\n")
		} else {
			fmt.Fprintf(&out, "\nПоследний вес: %s кг.\n", formatDecimal(*weight))
		}
	}
	if session.RestUntil != nil && session.RestUntil.After(time.Now()) {
		label, seconds := currentRestInstruction(session, *exercise)
		if seconds > 0 {
			fmt.Fprintf(&out, "\n%s: <b>%s</b>\n", label, formatSeconds(seconds))
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

func formatLoggedSet(set WorkoutSet) string {
	switch set.Type {
	case SetTypeWarmup:
		return "разминка · " + FormatSetDetailed(set)
	case SetTypeDrop:
		return "drop · " + FormatSetDetailed(set)
	default:
		return FormatSetDetailed(set)
	}
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
	fmt.Fprintf(&out, "\nУпражнений: %d · Обычных подходов: %d · Разминочных: %d", len(session.Exercises), workingSets, warmupSets)
	if dropSets > 0 {
		fmt.Fprintf(&out, " · Drop: %d", dropSets)
	}
	if skipped > 0 {
		fmt.Fprintf(&out, " · Пропусков: %d", skipped)
	}
	return strings.TrimSpace(out.String())
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
