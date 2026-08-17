package training

import (
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	maxPrograms            = 50
	maxExercisesPerProgram = 100
	maxNameRunes           = 200
)

var setPattern = regexp.MustCompile(`^\s*([1-9][0-9]*)\s*[Рр]\s*(?:([-–—])|([0-9]+(?:[.,][0-9]+)?)\s*[Кк][Гг])\s*$`)
var workoutIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

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
	case ".yaml", ".yml":
		return parseYAML(r)
	default:
		return nil, fmt.Errorf("поддерживаются файлы .yaml, .yml, .txt и .csv")
	}
}

type yamlDuration struct {
	Duration time.Duration
}

func (d *yamlDuration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("время должно быть значением вроде 120s или 2m")
	}
	value := strings.TrimSpace(node.Value)
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed%time.Second != 0 {
		return fmt.Errorf("время %q должно быть целым числом секунд или минут, например 120s или 2m", value)
	}
	d.Duration = parsed
	return nil
}

type yamlWeight struct {
	KG  float64
	Bar bool
}

func (w *yamlWeight) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("вес должен быть значением вроде 60kg или bar")
	}
	value := strings.ToLower(strings.TrimSpace(node.Value))
	if value == "bar" {
		w.Bar = true
		return nil
	}
	value = strings.TrimSpace(strings.TrimSuffix(value, "kg"))
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	if err != nil {
		return fmt.Errorf("вес %q должен иметь формат 60kg или bar", node.Value)
	}
	w.KG = parsed
	return nil
}

type yamlRepRange struct {
	Min int
	Max int
}

func (r *yamlRepRange) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("reps должен быть числом или диапазоном вроде 8-12")
	}
	value := strings.TrimSpace(node.Value)
	parts := strings.Split(value, "-")
	switch len(parts) {
	case 1:
		fixed, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return fmt.Errorf("reps %q должен быть числом или диапазоном вроде 8-12", value)
		}
		r.Min, r.Max = fixed, fixed
	case 2:
		min, minErr := strconv.Atoi(strings.TrimSpace(parts[0]))
		max, maxErr := strconv.Atoi(strings.TrimSpace(parts[1]))
		if minErr != nil || maxErr != nil {
			return fmt.Errorf("reps %q должен быть числом или диапазоном вроде 8-12", value)
		}
		r.Min, r.Max = min, max
	default:
		return fmt.Errorf("reps %q должен быть числом или диапазоном вроде 8-12", value)
	}
	return nil
}

type yamlProgramDocument struct {
	Version int `yaml:"version"`
	Program struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		DaysPerWeek int    `yaml:"days_per_week"`
	} `yaml:"program"`
	Defaults struct {
		TargetRIR            *float64      `yaml:"target_rir"`
		RestBetweenSets      *yamlDuration `yaml:"rest_between_sets"`
		RestBetweenExercises *yamlDuration `yaml:"rest_between_exercises"`
	} `yaml:"defaults"`
	Workouts []struct {
		ID        string `yaml:"id"`
		Name      string `yaml:"name"`
		Exercises []struct {
			Exercise       string        `yaml:"exercise"`
			Sets           int           `yaml:"sets"`
			Reps           yamlRepRange  `yaml:"reps"`
			TargetRIR      *float64      `yaml:"target_rir"`
			WeightStep     *yamlWeight   `yaml:"weight_step"`
			StartingWeight *yamlWeight   `yaml:"starting_weight"`
			Rest           *yamlDuration `yaml:"rest"`
			After          *yamlDuration `yaml:"after"`
			Progression    string        `yaml:"progression"`
			Warmup         []struct {
				Weight yamlWeight `yaml:"weight"`
				Reps   int        `yaml:"reps"`
			} `yaml:"warmup"`
		} `yaml:"exercises"`
	} `yaml:"workouts"`
}

func parseYAML(r io.Reader) ([]ProgramInput, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("прочитать YAML: %w", err)
	}
	source := strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff"))
	if source == "" {
		return nil, fmt.Errorf("YAML пуст")
	}
	var document yamlProgramDocument
	decoder := yaml.NewDecoder(strings.NewReader(source))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("не удалось разобрать YAML: %s", friendlyYAMLError(err))
	}
	if document.Version != 1 {
		return nil, fmt.Errorf("неподдерживаемая версия формата %d; ожидается version: 1", document.Version)
	}
	document.Program.Name = strings.TrimSpace(document.Program.Name)
	if document.Program.Name == "" {
		return nil, fmt.Errorf("поле program.name обязательно")
	}
	if len(document.Workouts) == 0 {
		return nil, fmt.Errorf("программа %q не содержит ни одной тренировки", document.Program.Name)
	}
	if document.Program.DaysPerWeek < 0 || document.Program.DaysPerWeek > 7 {
		return nil, fmt.Errorf("program.days_per_week должен быть от 1 до 7 или отсутствовать")
	}

	seenIDs := make(map[string]struct{}, len(document.Workouts))
	programs := make([]ProgramInput, 0, len(document.Workouts))
	for workoutIndex, workout := range document.Workouts {
		workout.ID = strings.TrimSpace(workout.ID)
		workout.Name = strings.TrimSpace(workout.Name)
		if workout.ID == "" || !workoutIDPattern.MatchString(workout.ID) {
			return nil, fmt.Errorf("тренировка %d: id обязателен и может содержать только a-z, 0-9, _ и -", workoutIndex+1)
		}
		if _, exists := seenIDs[workout.ID]; exists {
			return nil, fmt.Errorf("workout id %q встречается несколько раз", workout.ID)
		}
		seenIDs[workout.ID] = struct{}{}
		if workout.Name == "" {
			return nil, fmt.Errorf("тренировка %q: поле name обязательно", workout.ID)
		}
		if len(workout.Exercises) == 0 {
			return nil, fmt.Errorf("%s: нет ни одного упражнения", workout.Name)
		}
		program := ProgramInput{
			Name: workout.Name, PlanName: document.Program.Name,
			PlanDescription: strings.TrimSpace(document.Program.Description), DaysPerWeek: document.Program.DaysPerWeek,
			Version: document.Version, WorkoutKey: workout.ID,
		}
		if workoutIndex == 0 {
			program.RawSource = source
		}
		for _, exercise := range workout.Exercises {
			name := strings.TrimSpace(exercise.Exercise)
			prefix := workout.Name
			if name != "" {
				prefix += " → " + name
			}
			if name == "" {
				return nil, fmt.Errorf("%s: упражнение не имеет имени", workout.Name)
			}
			if exercise.Sets <= 0 {
				return nil, fmt.Errorf("%s: sets должен быть больше нуля", prefix)
			}
			if exercise.Reps.Min <= 0 || exercise.Reps.Max <= 0 {
				return nil, fmt.Errorf("%s: reps должен быть положительным числом или диапазоном", prefix)
			}
			if exercise.Reps.Min > exercise.Reps.Max {
				return nil, fmt.Errorf("%s: поле reps содержит недопустимый диапазон %d-%d: минимум не может быть больше максимума", prefix, exercise.Reps.Min, exercise.Reps.Max)
			}
			targetRIR := document.Defaults.TargetRIR
			if exercise.TargetRIR != nil {
				targetRIR = exercise.TargetRIR
			}
			if targetRIR == nil {
				return nil, fmt.Errorf("%s: target_rir не задан ни у упражнения, ни в defaults", prefix)
			}
			if *targetRIR < 0 {
				return nil, fmt.Errorf("%s: target_rir не может быть отрицательным", prefix)
			}
			if exercise.WeightStep == nil || exercise.WeightStep.Bar || exercise.WeightStep.KG <= 0 {
				return nil, fmt.Errorf("%s: weight_step должен быть больше нуля и содержать единицу kg, например 2.5kg", prefix)
			}
			if exercise.StartingWeight != nil && (exercise.StartingWeight.Bar || exercise.StartingWeight.KG <= 0) {
				return nil, fmt.Errorf("%s: starting_weight должен быть больше нуля и задаваться в kg", prefix)
			}
			rest := document.Defaults.RestBetweenSets
			if exercise.Rest != nil {
				rest = exercise.Rest
			}
			after := document.Defaults.RestBetweenExercises
			if exercise.After != nil {
				after = exercise.After
			}
			if rest == nil {
				return nil, fmt.Errorf("%s: rest не задан ни у упражнения, ни в defaults", prefix)
			}
			if after == nil {
				return nil, fmt.Errorf("%s: after не задан ни у упражнения, ни в defaults", prefix)
			}
			if rest.Duration < 0 || after.Duration < 0 {
				return nil, fmt.Errorf("%s: время отдыха не может быть отрицательным", prefix)
			}
			progressionType := strings.TrimSpace(exercise.Progression)
			if progressionType != ProgressionDouble {
				return nil, fmt.Errorf("%s: неизвестный progression %q; в version 1 поддерживается только double", prefix, progressionType)
			}
			prescription := ExercisePrescription{
				WorkingSets: exercise.Sets,
				Reps:        RepRange{Min: exercise.Reps.Min, Max: exercise.Reps.Max},
				TargetRIR:   *targetRIR, WeightStepKG: exercise.WeightStep.KG,
				RestSeconds: int(rest.Duration / time.Second), AfterSeconds: int(after.Duration / time.Second),
				Progression: progressionType,
			}
			if exercise.StartingWeight != nil {
				value := exercise.StartingWeight.KG
				prescription.StartingWeight = &value
			}
			for warmupIndex, warmup := range exercise.Warmup {
				if warmup.Reps <= 0 {
					return nil, fmt.Errorf("%s → разминка %d: reps должен быть больше нуля", prefix, warmupIndex+1)
				}
				planned := WarmupSet{Bar: warmup.Weight.Bar, Reps: warmup.Reps}
				if !warmup.Weight.Bar {
					if warmup.Weight.KG <= 0 {
						return nil, fmt.Errorf("%s → разминка %d: weight должен быть bar или положительным весом в kg", prefix, warmupIndex+1)
					}
					value := warmup.Weight.KG
					planned.WeightKG = &value
				}
				prescription.Warmup = append(prescription.Warmup, planned)
			}
			program.Exercises = append(program.Exercises, name)
			program.Prescriptions = append(program.Prescriptions, prescription)
		}
		programs = append(programs, program)
	}
	return validatePrograms(programs)
}

func friendlyYAMLError(err error) string {
	message := strings.ReplaceAll(err.Error(), "yaml: unmarshal errors:\n  ", "")
	message = strings.ReplaceAll(message, "yaml: ", "")
	return message
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
		if len(programs[i].Prescriptions) != 0 && len(programs[i].Prescriptions) != len(programs[i].Exercises) {
			return nil, fmt.Errorf("в тренировке %q число назначений не совпадает с числом упражнений", programs[i].Name)
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
