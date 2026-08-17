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
	require.Contains(t, finished, "Подходов: 2")
	require.Contains(t, finished, "Пропусков: 1")
}

func TestFormatActiveCardShowsOnlyCurrentStructuredAction(t *testing.T) {
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
	require.Contains(t, got, "2 / 2 упражнений")
	require.Contains(t, got, "➡️ гриф × 10")
	require.Contains(t, got, "○ 62.5 кг × 8–12")
	require.Contains(t, got, "RIR 2 · отдых между подходами: 180 секунд")
	require.Contains(t, got, "Отдых перед упражнением: <b>180 секунд</b>")
	require.NotContains(t, got, "Отдых до")
	require.Contains(t, got, "Почему:")
}

func TestFormatSessionDuration(t *testing.T) {
	started := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	finished := started.Add(42*time.Minute + 20*time.Second)
	require.Equal(t, "42 мин", FormatSessionDuration(Session{StartedAt: started, FinishedAt: &finished}))
	require.Equal(t, "—", FormatSessionDuration(Session{StartedAt: started}))
}
