package bot

import (
	"fitlog/internal/training"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTrainingPlansCardGroupsInterleavedWorkoutsByPlanID(t *testing.T) {
	workouts := []training.Program{
		{ID: 4, PlanID: 10, PlanName: "Фуллбади", Name: "А"},
		{ID: 7, PlanID: 20, PlanName: "Фуллбади", Name: "ГП А"},
		{ID: 5, PlanID: 10, PlanName: "Фуллбади", Name: "Б"},
	}
	_, markup := trainingPlansCard(workouts, false)
	require.Len(t, markup.InlineKeyboard, 3)
	require.Equal(t, trainingCallbackStartPlan, markup.InlineKeyboard[0][0].Unique)
	require.Equal(t, "10", markup.InlineKeyboard[0][0].Data)
	require.Equal(t, "20", markup.InlineKeyboard[1][0].Data)
	text, days, err := trainingPlanWorkoutsCard(workouts, 10, false)
	require.NoError(t, err)
	require.Contains(t, text, "Фуллбади")
	require.Len(t, days.InlineKeyboard, 3)
	require.Equal(t, "А", days.InlineKeyboard[0][0].Text)
	require.Equal(t, "4", days.InlineKeyboard[0][0].Data)
	require.Equal(t, trainingCallbackProgram, days.InlineKeyboard[0][0].Unique)
	require.Equal(t, "Б", days.InlineKeyboard[1][0].Text)
	require.Equal(t, trainingCallbackStart, days.InlineKeyboard[2][0].Unique)
}

func TestTrainingPlanEditingAndMissingPlan(t *testing.T) {
	workouts := []training.Program{{ID: 7, PlanID: 20, PlanName: "Грудь <A>", Name: "ГП А"}}
	_, plans := trainingPlansCard(workouts, true)
	require.Equal(t, trainingCallbackEditPlan, plans.InlineKeyboard[0][0].Unique)
	require.Equal(t, trainingCallbackImport, plans.InlineKeyboard[1][0].Unique)
	text, days, err := trainingPlanWorkoutsCard(workouts, 20, true)
	require.NoError(t, err)
	require.Contains(t, text, "Грудь &lt;A&gt;")
	require.Equal(t, trainingCallbackProgramView, days.InlineKeyboard[0][0].Unique)
	require.Equal(t, trainingCallbackPrograms, days.InlineKeyboard[1][0].Unique)
	_, _, err = trainingPlanWorkoutsCard(workouts, 99, false)
	require.ErrorIs(t, err, training.ErrNotFound)
	text, plans = trainingPlansCard(nil, true)
	require.Contains(t, text, "нет сохранённых программ")
	require.Len(t, plans.InlineKeyboard, 2)
}
