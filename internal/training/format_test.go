package training

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFormatActiveAndFinished(t *testing.T) {
	weight := 58.9
	session := Session{
		ID:              1,
		OwnerID:         42,
		ProgramName:     "Понедельник & спина",
		Status:          "active",
		CurrentPosition: 1,
		StartedAt:       time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC),
		Exercises: []SessionExercise{
			{
				Position: 1,
				Name:     "Тяга <блока>",
				Note:     "Локоть & плечо",
				Sets: []WorkoutSet{
					{Position: 1, Reps: 12, WeightKG: &weight},
					{Position: 2, Reps: 8},
				},
			},
			{Position: 2, Name: "Отжимания", Complete: true},
		},
	}

	previousWeight := 55.0
	previous := &PreviousExercise{
		StartedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		Sets: []WorkoutSet{
			{Position: 1, Reps: 10, WeightKG: &previousWeight},
			{Position: 2, Reps: 12},
		},
	}
	active := FormatActiveCard(session, previous, time.UTC, "Отправь: 12Р 40КГ или 12Р -")
	require.Contains(t, active, "Понедельник &amp; спина")
	require.Contains(t, active, "Тяга &lt;блока&gt;")
	require.Contains(t, active, "Прошлый раз · 01.08.2026")
	require.Contains(t, active, "1. 10Р 55КГ")
	require.Contains(t, active, "2. 12Р -")
	require.Contains(t, active, "1. 12Р 58.9КГ")
	require.Contains(t, active, "2. 8Р -")
	require.Contains(t, active, "Локоть &amp; плечо")

	session.Status = "finished"
	finishedAt := session.StartedAt.Add(75 * time.Minute)
	session.FinishedAt = &finishedAt
	finished := FormatFinished(session, time.UTC)
	require.Contains(t, finished, "05.08.2026 · Понедельник &amp; спина")
	require.Contains(t, finished, "Начало: 12:00 · Конец: 13:15")
	require.Contains(t, finished, "Длительность: 1 ч 15 мин")
	require.Contains(t, finished, "пропуск")
	require.Contains(t, finished, "Обычных подходов: 2 · Разминочных: 0")
	require.Contains(t, finished, "Пропусков: 1")
}

func TestFormatActiveCardDoesNotRenderRecommendations(t *testing.T) {
	weight := 62.5
	restUntil := time.Now().Add(3 * time.Minute)
	session := Session{
		ProgramName: "Фуллбади A", Status: "active", CurrentPosition: 2,
		StartedAt: time.Date(2026, 8, 17, 8, 0, 0, 0, time.UTC),
		RestUntil: &restUntil,
		Exercises: []SessionExercise{
			{Position: 1, Name: "Тяга", Complete: true, Plan: Recommendation{AfterSeconds: 180}},
			{
				Position: 2, Name: "Жим штанги лёжа",
				Warmup:         []WarmupSet{{Bar: true, Reps: 10}},
				Recommendation: Recommendation{WeightKG: &weight, MinReps: 8, MaxReps: 12, WorkingSets: 3, TargetRIR: 2, RestSeconds: 180, Reason: "Вес увеличен."},
				Plan:           Recommendation{WeightKG: &weight, MinReps: 8, MaxReps: 12, WorkingSets: 3, TargetRIR: 2, RestSeconds: 180},
			},
		},
	}

	got := FormatActiveCard(session, nil, time.UTC, "")

	require.Contains(t, got, "Жим штанги лёжа")
	require.Contains(t, got, "Упражнение 2 из 2")
	require.Contains(t, got, "Текущие подходы: пока нет.")
	require.Contains(t, got, "Отдых перед упражнением: <b>180 секунд</b>")
	require.NotContains(t, got, "62.5 кг × 8–12")
	require.NotContains(t, got, "Почему:")
	require.NotContains(t, got, "Рекомендация")
}

func TestCardsSeparateWarmupFromOrdinaryAndShowLastWeight(t *testing.T) {
	recommendedWeight := 35.0
	actualWeight := 40.8
	warmupWeight := 20.0
	warmupRIR := 9.0
	workingRIR := 2.0
	actualWarmupReps := 10
	actualWorkingReps := 12
	session := Session{
		ProgramName: "Фуллбади A", Status: "active", CurrentPosition: 1,
		StartedAt: time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC),
		Exercises: []SessionExercise{{
			Position: 1, Name: "Жим штанги лёжа",
			Warmup:         []WarmupSet{{Bar: true, Reps: 10}},
			Recommendation: Recommendation{WeightKG: &recommendedWeight, MinReps: 8, MaxReps: 12, WorkingSets: 3, Reason: "Вес из истории."},
			Plan:           Recommendation{WeightKG: &recommendedWeight, MinReps: 8, MaxReps: 12, WorkingSets: 3},
			Sets: []WorkoutSet{
				{Position: 1, Type: SetTypeWarmup, ActualReps: &actualWarmupReps, ActualRIR: &warmupRIR, Reps: actualWarmupReps},
				{Position: 2, Type: SetTypeWarmup, ActualWeightKG: &warmupWeight, ActualReps: &actualWarmupReps, Reps: actualWarmupReps},
				{Position: 3, Type: SetTypeWorking, ActualWeightKG: &actualWeight, ActualReps: &actualWorkingReps, ActualRIR: &workingRIR, Reps: actualWorkingReps},
			},
		}},
	}

	active := FormatActiveCard(session, nil, time.UTC, "")
	require.Contains(t, active, "1. разминка · 10Р -")
	require.Contains(t, active, "2. разминка · 10Р 20КГ")
	require.Contains(t, active, "3. 12Р 40.8КГ @ RIR 2")
	require.Contains(t, active, "Последний вес: 40.8 кг.")
	require.NotContains(t, active, "разминка · 10Р - @ RIR")
	require.NotContains(t, active, "Вес из истории.")

	finishedAt := session.StartedAt.Add(time.Hour)
	session.Status = "finished"
	session.FinishedAt = &finishedAt
	finished := FormatFinished(session, time.UTC)
	require.Contains(t, finished, "разминка · гриф × 10")
	require.Contains(t, finished, "разминка · 10Р 20КГ")
	require.Contains(t, finished, "12Р 40.8КГ @ RIR 2")
	require.NotContains(t, finished, "разминка · гриф × 10 @ RIR")
	require.Contains(t, finished, "Обычных подходов: 1 · Разминочных: 2")

	working, warmup := session.SetCounts()
	require.Equal(t, 1, working)
	require.Equal(t, 2, warmup)
}

func TestActiveCardKeepsEveryManuallyAddedSetVisible(t *testing.T) {
	weight := 60.0
	reps := 10
	rir := 2.0
	session := Session{
		ProgramName: "Фуллбади A", Status: "active", CurrentPosition: 1,
		StartedAt: time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC),
		Exercises: []SessionExercise{{
			Position: 1, Name: "Жим",
			Sets: []WorkoutSet{
				{Position: 1, Type: SetTypeWorking, ActualWeightKG: &weight, ActualReps: &reps, ActualRIR: &rir, Reps: reps},
				{Position: 2, Type: SetTypeWorking, ActualWeightKG: &weight, ActualReps: &reps, ActualRIR: &rir, Reps: reps},
			},
		}},
	}

	got := FormatActiveCard(session, nil, time.UTC, "")

	require.Contains(t, got, "1. 10Р 60КГ @ RIR 2")
	require.Contains(t, got, "2. 10Р 60КГ @ RIR 2")
	require.NotContains(t, got, "План выполнен")
}

func TestFormatFinishedMarksDropSetsWithoutCountingThemAsWorking(t *testing.T) {
	weight := 30.0
	reps := 12
	finishedAt := time.Date(2026, 8, 21, 9, 30, 0, 0, time.UTC)
	session := Session{
		ProgramName: "Фуллбади A", Status: "finished",
		StartedAt: finishedAt.Add(-time.Hour), FinishedAt: &finishedAt,
		Exercises: []SessionExercise{{Name: "Разгибания рук", Sets: []WorkoutSet{
			{Type: SetTypeWorking, ActualWeightKG: &weight, ActualReps: &reps, Reps: reps},
			{Type: SetTypeDrop, ActualWeightKG: &weight, ActualReps: &reps, Reps: reps},
		}}},
	}

	got := FormatFinished(session, time.UTC)
	require.Contains(t, got, "drop · 12Р 30КГ")
	require.Contains(t, got, "Обычных подходов: 1")
	require.Contains(t, got, "Drop: 1")
}

func TestFormatSessionDuration(t *testing.T) {
	started := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	finished := started.Add(42*time.Minute + 20*time.Second)
	require.Equal(t, "42 мин", FormatSessionDuration(Session{StartedAt: started, FinishedAt: &finished}))
	require.Equal(t, "—", FormatSessionDuration(Session{StartedAt: started}))
}
