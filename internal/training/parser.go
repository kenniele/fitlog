package training

import (
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxPrograms            = 50
	maxExercisesPerProgram = 100
	maxNameRunes           = 200
)

var setPattern = regexp.MustCompile(`^\s*([1-9][0-9]*)\s*[Рр]\s*(?:([-–—])|([0-9]+(?:[.,][0-9]+)?)\s*[Кк][Гг])\s*$`)

func ParseSet(raw string) (SetInput, error) {
	matches := setPattern.FindStringSubmatch(raw)
	if matches == nil {
		return SetInput{}, fmt.Errorf("нужен формат «12Р 40КГ» или «12Р -»")
	}
	reps, err := strconv.Atoi(matches[1])
	if err != nil || reps <= 0 || reps > 10000 {
		return SetInput{}, fmt.Errorf("некорректное количество повторений")
	}
	if matches[2] != "" {
		return SetInput{Reps: reps}, nil
	}
	weight, err := strconv.ParseFloat(strings.ReplaceAll(matches[3], ",", "."), 64)
	if err != nil || weight <= 0 || weight > 100000 {
		return SetInput{}, fmt.Errorf("некорректный вес")
	}
	return SetInput{Reps: reps, WeightKG: &weight}, nil
}

func ParsePrograms(filename string, r io.Reader) ([]ProgramInput, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt":
		return parseTXT(r)
	case ".csv":
		return parseCSV(r)
	default:
		return nil, fmt.Errorf("поддерживаются только файлы .txt и .csv")
	}
}

func parseTXT(r io.Reader) ([]ProgramInput, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("прочитать TXT: %w", err)
	}
	text := strings.TrimPrefix(string(raw), "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var programs []ProgramInput
	var block []string
	flush := func() error {
		if len(block) == 0 {
			return nil
		}
		if len(block) < 2 {
			return fmt.Errorf("у программы %q нет упражнений", block[0])
		}
		programs = append(programs, ProgramInput{Name: block[0], Exercises: append([]string(nil), block[1:]...)})
		block = nil
		return nil
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		block = append(block, line)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return validatePrograms(programs)
}

func parseCSV(r io.Reader) ([]ProgramInput, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("прочитать CSV: %w", err)
	}
	data := strings.TrimPrefix(string(raw), "\ufeff")
	comma := ','
	if firstLine, _, _ := strings.Cut(data, "\n"); strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		comma = ';'
	}
	reader := csv.NewReader(strings.NewReader(data))
	reader.Comma = comma
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("разобрать CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("CSV пуст")
	}

	start := 0
	if len(records[0]) >= 2 && isProgramHeader(records[0][0]) && isExerciseHeader(records[0][1]) {
		start = 1
	}
	order := make([]string, 0)
	grouped := make(map[string][]string)
	canonical := make(map[string]string)
	for i, record := range records[start:] {
		line := i + start + 1
		if len(record) < 2 {
			return nil, fmt.Errorf("строка CSV %d: нужны колонки program и exercise", line)
		}
		programName := strings.TrimSpace(record[0])
		exerciseName := strings.TrimSpace(record[1])
		if programName == "" || exerciseName == "" {
			return nil, fmt.Errorf("строка CSV %d: программа и упражнение не могут быть пустыми", line)
		}
		key := strings.ToLower(programName)
		if _, ok := grouped[key]; !ok {
			order = append(order, key)
			canonical[key] = programName
		}
		grouped[key] = append(grouped[key], exerciseName)
	}

	programs := make([]ProgramInput, 0, len(order))
	for _, key := range order {
		programs = append(programs, ProgramInput{Name: canonical[key], Exercises: grouped[key]})
	}
	return validatePrograms(programs)
}

func isProgramHeader(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "program", "программа":
		return true
	default:
		return false
	}
}

func isExerciseHeader(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "exercise", "упражнение":
		return true
	default:
		return false
	}
}

func validatePrograms(programs []ProgramInput) ([]ProgramInput, error) {
	if len(programs) == 0 {
		return nil, fmt.Errorf("в файле не найдено ни одной программы")
	}
	if len(programs) > maxPrograms {
		return nil, fmt.Errorf("в одном файле можно импортировать не больше %d программ", maxPrograms)
	}
	seen := make(map[string]struct{}, len(programs))
	for i := range programs {
		programs[i].Name = strings.TrimSpace(programs[i].Name)
		if utf8.RuneCountInString(programs[i].Name) > maxNameRunes {
			return nil, fmt.Errorf("название программы длиннее %d символов", maxNameRunes)
		}
		key := strings.ToLower(programs[i].Name)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("программа %q встречается несколько раз", programs[i].Name)
		}
		seen[key] = struct{}{}
		if len(programs[i].Exercises) == 0 {
			return nil, fmt.Errorf("у программы %q нет упражнений", programs[i].Name)
		}
		if len(programs[i].Exercises) > maxExercisesPerProgram {
			return nil, fmt.Errorf("в программе %q больше %d упражнений", programs[i].Name, maxExercisesPerProgram)
		}
		for j := range programs[i].Exercises {
			programs[i].Exercises[j] = strings.TrimSpace(programs[i].Exercises[j])
			if programs[i].Exercises[j] == "" {
				return nil, fmt.Errorf("в программе %q есть пустое упражнение", programs[i].Name)
			}
			if utf8.RuneCountInString(programs[i].Exercises[j]) > maxNameRunes {
				return nil, fmt.Errorf("упражнение в программе %q длиннее %d символов", programs[i].Name, maxNameRunes)
			}
		}
	}
	return programs, nil
}
