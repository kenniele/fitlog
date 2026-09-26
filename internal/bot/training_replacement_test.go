package bot

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"fitlog/internal/training"
)

func TestTrainingReplacementCard(t *testing.T) {
	catalogID := int64(10)
	text, markup := trainingReplacementCard(training.SessionExercise{ID: 99, ExerciseID: &catalogID, Name: "Жим <лёжа>"}, training.ExercisePage{
		Page: 2, TotalPages: 3, Items: []training.Exercise{{ID: catalogID, Name: "Жим <лёжа>"}, {ID: 20, Name: "Отжимания"}},
	})
	require.Contains(t, text, "Жим &lt;лёжа&gt;")
	require.Contains(t, text, "Программа и прошлые тренировки сохранятся")
	require.Len(t, markup.InlineKeyboard, 3)
	require.Len(t, markup.InlineKeyboard[0], 1)
	require.Equal(t, "Отжимания", markup.InlineKeyboard[0][0].Text)
	require.Contains(t, markup.InlineKeyboard[0][0].Data, "99:20")
	require.Len(t, markup.InlineKeyboard[1], 3)
	require.Contains(t, markup.InlineKeyboard[1][0].Data, "99:1")
	require.Contains(t, markup.InlineKeyboard[1][2].Data, "99:3")
	require.Equal(t, "‹ Отмена", markup.InlineKeyboard[2][0].Text)
}

func TestTrainingReplacementCardEmptyCatalog(t *testing.T) {
	text, markup := trainingReplacementCard(training.SessionExercise{ID: 99}, training.ExercisePage{Page: 1, TotalPages: 1})
	require.Contains(t, text, "нет других упражнений")
	require.Len(t, markup.InlineKeyboard, 1)
}

func TestTrainingReplacementCallbackFitsTelegramLimit(t *testing.T) {
	payload := trainingPair(math.MaxInt64, math.MaxInt64)
	for _, callback := range []string{trainingCallbackReplaceExercise, trainingCallbackReplaceExercisePage} {
		require.LessOrEqual(t, len("\f"+callback+"|"+payload), 64)
	}
}
