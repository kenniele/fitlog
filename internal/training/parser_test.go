package training

import (
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
		{name: "external weight", raw: "12Р 40КГ", reps: 12, weight: 40},
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
