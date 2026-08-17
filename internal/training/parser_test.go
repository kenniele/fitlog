package training

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSet(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		reps       int
		weight     float64
		bodyweight bool
	}{
		{name: "kilograms", raw: "12Р 40КГ", reps: 12, weight: 40},
		{name: "lowercase decimal comma", raw: " 12р 58,9кг ", reps: 12, weight: 58.9},
		{name: "bodyweight hyphen", raw: "12Р -", reps: 12, bodyweight: true},
		{name: "bodyweight em dash", raw: "8Р —", reps: 8, bodyweight: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSet(tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.reps, got.Reps)
			if tt.bodyweight {
				require.Nil(t, got.WeightKG)
			} else {
				require.NotNil(t, got.WeightKG)
				require.InDelta(t, tt.weight, *got.WeightKG, 0.001)
			}
		})
	}
}

func TestParseSetRejectsOtherFormats(t *testing.T) {
	for _, raw := range []string{"12 раз", "40КГ 12Р", "12Р", "12Р 0КГ", "0Р -", "12Р собственный вес"} {
		_, err := ParseSet(raw)
		require.Error(t, err, raw)
	}
}

func TestParseProgramsTXT(t *testing.T) {
	raw := "\ufeffПонедельник\r\nТяга блока\r\nЖим гантелей\r\n\r\n\r\nВторник\r\nПодтягивания\r\nОтжимания\r\n"
	programs, err := ParsePrograms("plan.TXT", strings.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, []ProgramInput{
		{Name: "Понедельник", Exercises: []string{"Тяга блока", "Жим гантелей"}},
		{Name: "Вторник", Exercises: []string{"Подтягивания", "Отжимания"}},
	}, programs)
}

func TestParseProgramsCSV(t *testing.T) {
	raw := "program;exercise\nПонедельник;Тяга блока\nПонедельник;Жим гантелей\nВторник;Подтягивания\n"
	programs, err := ParsePrograms("plan.csv", strings.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, []ProgramInput{
		{Name: "Понедельник", Exercises: []string{"Тяга блока", "Жим гантелей"}},
		{Name: "Вторник", Exercises: []string{"Подтягивания"}},
	}, programs)
}

func TestParseProgramsRejectsProgramWithoutExercises(t *testing.T) {
	_, err := ParsePrograms("plan.txt", strings.NewReader("Понедельник\n"))
	require.ErrorContains(t, err, "нет упражнений")
}

func TestParseProgramsYAML(t *testing.T) {
	raw := `version: 1
program:
  name: Test Program
  description: Three days
  days_per_week: 3
defaults:
  target_rir: 2
  rest_between_sets: 2m
  rest_between_exercises: 180s
workouts:
  - id: bench_day
    name: Bench Day
    exercises:
      - exercise: Жим штанги лёжа
        warmup:
          - weight: bar
            reps: 10
          - weight: 40kg
            reps: 6
        sets: 3
        reps: 8-12
        starting_weight: 60kg
        weight_step: 2.5kg
        rest: 3m
        progression: double
      - exercise: Разгибания рук
        sets: 3
        reps: 12
        target_rir: 1
        weight_step: 4.5kg
        after: 0s
        progression: double
`
	programs, err := ParsePrograms("program.yaml", strings.NewReader(raw))
	require.NoError(t, err)
	require.Len(t, programs, 1)
	program := programs[0]
	require.Equal(t, "Test Program", program.PlanName)
	require.Equal(t, "bench_day", program.WorkoutKey)
	require.Equal(t, "Bench Day", program.Name)
	require.Equal(t, []string{"Жим штанги лёжа", "Разгибания рук"}, program.Exercises)
	require.Len(t, program.Prescriptions, 2)
	require.Equal(t, RepRange{Min: 8, Max: 12}, program.Prescriptions[0].Reps)
	require.Equal(t, 180, program.Prescriptions[0].RestSeconds)
	require.Equal(t, 180, program.Prescriptions[0].AfterSeconds)
	require.InDelta(t, 60, *program.Prescriptions[0].StartingWeight, 0.001)
	require.InDelta(t, 2.5, program.Prescriptions[0].WeightStepKG, 0.001)
	require.True(t, program.Prescriptions[0].Warmup[0].Bar)
	require.Equal(t, RepRange{Min: 12, Max: 12}, program.Prescriptions[1].Reps)
	require.Equal(t, 120, program.Prescriptions[1].RestSeconds)
	require.Zero(t, program.Prescriptions[1].AfterSeconds)
	require.InDelta(t, 1, program.Prescriptions[1].TargetRIR, 0.001)
}

func TestParseProgramsYAMLRejectsInvalidValues(t *testing.T) {
	base := `version: 1
program:
  name: Test
defaults:
  target_rir: 2
  rest_between_sets: 120s
  rest_between_exercises: 180s
workouts:
  - id: bench
    name: Bench
    exercises:
      - exercise: Жим
        sets: 3
        reps: %s
        weight_step: 2.5kg
        progression: %s
`
	t.Run("invalid rep range", func(t *testing.T) {
		_, err := ParsePrograms("program.yml", strings.NewReader(fmt.Sprintf(base, "12-8", "double")))
		require.ErrorContains(t, err, "минимум не может быть больше максимума")
	})
	t.Run("unknown progression", func(t *testing.T) {
		_, err := ParsePrograms("program.yml", strings.NewReader(fmt.Sprintf(base, "8-12", "linear")))
		require.ErrorContains(t, err, "поддерживается только double")
	})
	t.Run("duplicate workout id", func(t *testing.T) {
		raw := fmt.Sprintf(base, "8-12", "double") + `  - id: bench
    name: Other
    exercises:
      - exercise: Тяга
        sets: 3
        reps: 8-12
        weight_step: 2.5kg
        progression: double
`
		_, err := ParsePrograms("program.yml", strings.NewReader(raw))
		require.ErrorContains(t, err, "встречается несколько раз")
	})
}

func TestParseProgramsYAMLRejectsNegativeRest(t *testing.T) {
	raw := `version: 1
program: {name: Test}
workouts:
  - id: bench
    name: Bench
    exercises:
      - exercise: Жим
        sets: 3
        reps: 8-12
        target_rir: 2
        weight_step: 2.5kg
        rest: -1s
        after: 0s
        progression: double
`
	_, err := ParsePrograms("program.yml", strings.NewReader(raw))
	require.ErrorContains(t, err, "время отдыха не может быть отрицательным")
}

func TestParseProgramsYAMLMultipleWorkoutDays(t *testing.T) {
	raw := `version: 1
program:
  name: Full Body
defaults:
  target_rir: 2
  rest_between_sets: 2m
  rest_between_exercises: 3m
workouts:
  - id: day_a
    name: Day A
    exercises:
      - exercise: Жим
        sets: 3
        reps: 8-12
        weight_step: 2.5kg
        progression: double
  - id: day_b
    name: Day B
    exercises:
      - exercise: Тяга
        sets: 4
        reps: 10
        weight_step: 5kg
        progression: double
`
	programs, err := ParsePrograms("program.yaml", strings.NewReader(raw))
	require.NoError(t, err)
	require.Len(t, programs, 2)
	require.Equal(t, "day_a", programs[0].WorkoutKey)
	require.Equal(t, "day_b", programs[1].WorkoutKey)
	require.Equal(t, RepRange{Min: 10, Max: 10}, programs[1].Prescriptions[0].Reps)
}
