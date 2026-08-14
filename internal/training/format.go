package training

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
)

func FormatSet(set WorkoutSet) string {
	if set.WeightKG == nil {
		return fmt.Sprintf("%dР -", set.Reps)
	}
	weight := strconv.FormatFloat(*set.WeightKG, 'f', -1, 64)
	return fmt.Sprintf("%dР %sКГ", set.Reps, weight)
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
			for _, set := range exercise.Sets {
				out.WriteString(FormatSet(set) + "\n")
			}
		}
		if exercise.Note != "" {
			out.WriteString("Заметка: " + html.EscapeString(exercise.Note) + "\n")
		}
	}

	sets := 0
	skipped := 0
	for _, exercise := range session.Exercises {
		sets += len(exercise.Sets)
		if len(exercise.Sets) == 0 {
			skipped++
		}
	}
	fmt.Fprintf(&out, "\nУпражнений: %d · Подходов: %d", len(session.Exercises), sets)
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
