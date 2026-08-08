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
	finished := FormatFinished(session, time.UTC)
	require.Contains(t, finished, "05.08.2026 · Понедельник &amp; спина")
	require.Contains(t, finished, "пропуск")
	require.Contains(t, finished, "Подходов: 2")
	require.Contains(t, finished, "Пропусков: 1")
}
