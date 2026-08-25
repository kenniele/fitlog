package training

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type UseCase struct {
	repo Repository
}

func NewUseCase(repo Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (u *UseCase) State(ctx context.Context, ownerID int64) (UIState, error) {
	state, err := u.repo.GetUIState(ctx, ownerID)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return UIState{}, err
	}
	return UIState{OwnerID: ownerID}, nil
}

func (u *UseCase) SaveControlMessage(ctx context.Context, ownerID, chatID int64, messageID int) error {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return err
	}
	state.ChatID = chatID
	state.MessageID = messageID
	return u.repo.SaveUIState(ctx, state)
}

// OpenControlMessage starts a fresh workout card in the current chat. The old
// message ID is forgotten so opening the section from the persistent reply
// keyboard cannot silently edit a card that is far above in the conversation.
func (u *UseCase) OpenControlMessage(ctx context.Context, ownerID, chatID int64) error {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return err
	}
	state.ChatID = chatID
	state.MessageID = 0
	state.Mode = InputNone
	state.PendingImport = nil
	state.PendingExerciseID = nil
	state.PendingExerciseName = ""
	state.PendingProgramExerciseID = nil
	state.PendingTargetExerciseID = nil
	state.PendingSet = nil
	return u.repo.SaveUIState(ctx, state)
}

func (u *UseCase) Expect(ctx context.Context, ownerID int64, mode InputMode) error {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return err
	}
	state.Mode = mode
	if mode != InputImportOK {
		state.PendingImport = nil
	}
	if mode != InputRename {
		state.PendingExerciseID = nil
	}
	state.PendingExerciseName = ""
	state.PendingProgramExerciseID = nil
	state.PendingTargetExerciseID = nil
	if mode != InputRIR {
		state.PendingSet = nil
	}
	return u.repo.SaveUIState(ctx, state)
}

func (u *UseCase) ClearInput(ctx context.Context, ownerID int64) error {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return err
	}
	state.Mode = InputNone
	state.PendingImport = nil
	state.PendingExerciseID = nil
	state.PendingExerciseName = ""
	state.PendingProgramExerciseID = nil
	state.PendingTargetExerciseID = nil
	state.PendingSet = nil
	return u.repo.SaveUIState(ctx, state)
}

func (u *UseCase) PreviewImport(ctx context.Context, ownerID int64, filename string, r io.Reader) (ImportPreview, error) {
	programs, err := ParsePrograms(filename, r)
	if err != nil {
		return ImportPreview{}, err
	}
	preview := ImportPreview{Filename: filename, Programs: programs}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportPreview{}, err
	}
	state.Mode = InputImportOK
	state.PendingImport = &preview
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportPreview{}, err
	}
	return preview, nil
}

func (u *UseCase) ConfirmImport(ctx context.Context, ownerID int64) (ImportPreview, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportPreview{}, err
	}
	if state.PendingImport == nil || len(state.PendingImport.Programs) == 0 {
		return ImportPreview{}, ErrNoPendingImport
	}
	preview := *state.PendingImport
	if err := u.repo.ReplacePrograms(ctx, ownerID, preview.Programs); err != nil {
		return ImportPreview{}, err
	}
	state.Mode = InputNone
	state.PendingImport = nil
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportPreview{}, err
	}
	return preview, nil
}

func (u *UseCase) BeginImportReview(ctx context.Context, ownerID int64) (ImportExerciseReview, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportExerciseReview{}, err
	}
	if state.PendingImport == nil || len(state.PendingImport.Programs) == 0 {
		return ImportExerciseReview{}, ErrNoPendingImport
	}
	state.Mode = InputImportOK
	state.PendingImport.ReviewStarted = true
	state.PendingImport.ReviewProgram = 0
	state.PendingImport.ReviewExercise = 0
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportExerciseReview{}, err
	}
	return u.importReview(ctx, ownerID, state)
}

func (u *UseCase) ImportReview(ctx context.Context, ownerID int64) (ImportExerciseReview, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportExerciseReview{}, err
	}
	return u.importReview(ctx, ownerID, state)
}

func (u *UseCase) UseExistingImportExercise(ctx context.Context, ownerID, exerciseID int64) (ImportExerciseReview, bool, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportExerciseReview{}, false, err
	}
	review, err := u.importReview(ctx, ownerID, state)
	if err != nil {
		return ImportExerciseReview{}, false, err
	}
	exercise, err := u.repo.Exercise(ctx, ownerID, exerciseID)
	if err != nil {
		return ImportExerciseReview{}, false, err
	}
	state.PendingImport.Programs[review.ProgramIndex].Exercises[review.ExerciseIndex] = exercise.Name
	u.advanceImportReview(state.PendingImport)
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportExerciseReview{}, false, err
	}
	if state.PendingImport.ReviewProgram >= len(state.PendingImport.Programs) {
		return ImportExerciseReview{}, true, nil
	}
	next, err := u.importReview(ctx, ownerID, state)
	return next, false, err
}

func (u *UseCase) KeepNewImportExercise(ctx context.Context, ownerID int64) (ImportExerciseReview, bool, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ImportExerciseReview{}, false, err
	}
	if _, err := u.importReview(ctx, ownerID, state); err != nil {
		return ImportExerciseReview{}, false, err
	}
	u.advanceImportReview(state.PendingImport)
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ImportExerciseReview{}, false, err
	}
	if state.PendingImport.ReviewProgram >= len(state.PendingImport.Programs) {
		return ImportExerciseReview{}, true, nil
	}
	next, err := u.importReview(ctx, ownerID, state)
	return next, false, err
}

func (u *UseCase) importReview(ctx context.Context, ownerID int64, state UIState) (ImportExerciseReview, error) {
	preview := state.PendingImport
	if preview == nil || !preview.ReviewStarted || preview.ReviewProgram >= len(preview.Programs) {
		return ImportExerciseReview{}, ErrNoPendingImport
	}
	program := preview.Programs[preview.ReviewProgram]
	if preview.ReviewExercise >= len(program.Exercises) {
		return ImportExerciseReview{}, ErrNoPendingImport
	}
	proposed := program.Exercises[preview.ReviewExercise]
	similar, err := u.repo.SimilarExercises(ctx, ownerID, proposed, 5)
	if err != nil {
		return ImportExerciseReview{}, err
	}
	current := 0
	total := 0
	for i, item := range preview.Programs {
		total += len(item.Exercises)
		if i < preview.ReviewProgram {
			current += len(item.Exercises)
		}
	}
	current += preview.ReviewExercise + 1
	return ImportExerciseReview{
		ProgramIndex: preview.ReviewProgram, ExerciseIndex: preview.ReviewExercise,
		Current: current, Total: total, ProgramName: program.Name,
		ProposedName: proposed, Similar: similar,
	}, nil
}

func (u *UseCase) advanceImportReview(preview *ImportPreview) {
	preview.ReviewExercise++
	for preview.ReviewProgram < len(preview.Programs) && preview.ReviewExercise >= len(preview.Programs[preview.ReviewProgram].Exercises) {
		preview.ReviewProgram++
		preview.ReviewExercise = 0
	}
}

func (u *UseCase) Programs(ctx context.Context, ownerID int64) ([]Program, error) {
	return u.repo.ListPrograms(ctx, ownerID)
}

func (u *UseCase) Program(ctx context.Context, ownerID, programID int64) (Program, error) {
	return u.repo.Program(ctx, ownerID, programID)
}

func (u *UseCase) Exercises(ctx context.Context, ownerID int64, page, pageSize int) (ExercisePage, error) {
	if page < 1 || pageSize < 1 || pageSize > 20 {
		return ExercisePage{}, ErrInvalidPage
	}
	items, total, err := u.repo.ListExercises(ctx, ownerID, pageSize, (page-1)*pageSize)
	if err != nil {
		return ExercisePage{}, err
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		return ExercisePage{}, ErrInvalidPage
	}
	return ExercisePage{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages}, nil
}

func (u *UseCase) Exercise(ctx context.Context, ownerID, exerciseID int64) (Exercise, error) {
	return u.repo.Exercise(ctx, ownerID, exerciseID)
}

func (u *UseCase) ExpectExerciseRename(ctx context.Context, ownerID, exerciseID int64) (Exercise, error) {
	exercise, err := u.repo.Exercise(ctx, ownerID, exerciseID)
	if err != nil {
		return Exercise{}, err
	}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return Exercise{}, err
	}
	state.Mode = InputRename
	state.PendingImport = nil
	state.PendingExerciseID = &exerciseID
	state.PendingExerciseName = ""
	state.PendingProgramExerciseID = nil
	state.PendingTargetExerciseID = nil
	state.PendingSet = nil
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return Exercise{}, err
	}
	return exercise, nil
}

func (u *UseCase) RenameExercise(ctx context.Context, ownerID int64, raw string) (RenameResult, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return RenameResult{}, fmt.Errorf("название пустое")
	}
	if len([]rune(name)) > 200 {
		return RenameResult{}, fmt.Errorf("название длиннее 200 символов")
	}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return RenameResult{}, err
	}
	if state.Mode != InputRename || state.PendingExerciseID == nil {
		return RenameResult{}, ErrNotEditable
	}
	result, err := u.repo.RenameExercise(ctx, ownerID, *state.PendingExerciseID, name)
	if err != nil {
		return RenameResult{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return RenameResult{}, err
	}
	return result, nil
}

func (u *UseCase) BeginProgramExerciseReplacement(
	ctx context.Context,
	ownerID, programExerciseID int64,
) (ProgramExerciseReplacement, error) {
	replacement, err := u.repo.ProgramExercise(ctx, ownerID, programExerciseID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	state.Mode = InputProgramExerciseChoice
	state.PendingImport = nil
	state.PendingExerciseID = nil
	state.PendingExerciseName = ""
	state.PendingProgramExerciseID = &programExerciseID
	state.PendingTargetExerciseID = nil
	state.PendingSet = nil
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ProgramExerciseReplacement{}, err
	}
	return replacement, nil
}

func (u *UseCase) PendingProgramExerciseReplacement(
	ctx context.Context,
	ownerID int64,
) (ProgramExerciseReplacement, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	if state.PendingProgramExerciseID == nil {
		return ProgramExerciseReplacement{}, ErrNotEditable
	}
	replacement, err := u.repo.ProgramExercise(ctx, ownerID, *state.PendingProgramExerciseID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	if state.PendingTargetExerciseID != nil {
		replacement.Target, err = u.repo.Exercise(ctx, ownerID, *state.PendingTargetExerciseID)
		if err != nil {
			return ProgramExerciseReplacement{}, err
		}
	} else if state.PendingExerciseName != "" {
		replacement.Target = Exercise{OwnerID: ownerID, Name: state.PendingExerciseName}
	}
	return replacement, nil
}

func (u *UseCase) ChooseExistingProgramExercise(
	ctx context.Context,
	ownerID, targetExerciseID int64,
) (ProgramExerciseReplacement, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	if state.Mode != InputProgramExerciseChoice || state.PendingProgramExerciseID == nil {
		return ProgramExerciseReplacement{}, ErrNotEditable
	}
	target, err := u.repo.Exercise(ctx, ownerID, targetExerciseID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	state.Mode = InputProgramExerciseConfirm
	state.PendingTargetExerciseID = &targetExerciseID
	state.PendingExerciseName = ""
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ProgramExerciseReplacement{}, err
	}
	replacement, err := u.repo.ProgramExercise(ctx, ownerID, *state.PendingProgramExerciseID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	replacement.Target = target
	return replacement, nil
}

func (u *UseCase) ExpectNewProgramExercise(
	ctx context.Context,
	ownerID int64,
) (ProgramExerciseReplacement, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	if state.Mode != InputProgramExerciseChoice || state.PendingProgramExerciseID == nil {
		return ProgramExerciseReplacement{}, ErrNotEditable
	}
	state.Mode = InputProgramExerciseNew
	state.PendingTargetExerciseID = nil
	state.PendingExerciseName = ""
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ProgramExerciseReplacement{}, err
	}
	return u.repo.ProgramExercise(ctx, ownerID, *state.PendingProgramExerciseID)
}

func (u *UseCase) PrepareNewProgramExercise(
	ctx context.Context,
	ownerID int64,
	raw string,
) (ProgramExerciseReplacement, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ProgramExerciseReplacement{}, fmt.Errorf("название пустое")
	}
	if len([]rune(name)) > 200 {
		return ProgramExerciseReplacement{}, fmt.Errorf("название длиннее 200 символов")
	}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	if state.Mode != InputProgramExerciseNew || state.PendingProgramExerciseID == nil {
		return ProgramExerciseReplacement{}, ErrNotEditable
	}
	state.Mode = InputProgramExerciseConfirm
	state.PendingTargetExerciseID = nil
	state.PendingExerciseName = name
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return ProgramExerciseReplacement{}, err
	}
	replacement, err := u.repo.ProgramExercise(ctx, ownerID, *state.PendingProgramExerciseID)
	if err != nil {
		return ProgramExerciseReplacement{}, err
	}
	replacement.Target = Exercise{OwnerID: ownerID, Name: name}
	return replacement, nil
}

func (u *UseCase) ConfirmProgramExerciseReplacement(
	ctx context.Context,
	ownerID int64,
	replaceHistory bool,
) (ProgramExerciseReplaceResult, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return ProgramExerciseReplaceResult{}, err
	}
	if state.Mode != InputProgramExerciseConfirm || state.PendingProgramExerciseID == nil ||
		(state.PendingTargetExerciseID == nil && state.PendingExerciseName == "") {
		return ProgramExerciseReplaceResult{}, ErrNotEditable
	}
	result, err := u.repo.ReplaceProgramExercise(
		ctx, ownerID, *state.PendingProgramExerciseID,
		state.PendingTargetExerciseID, state.PendingExerciseName, replaceHistory,
	)
	if err != nil {
		return ProgramExerciseReplaceResult{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return ProgramExerciseReplaceResult{}, err
	}
	return result, nil
}

func (u *UseCase) Start(ctx context.Context, ownerID, programID int64, now time.Time) (Session, error) {
	if _, err := u.repo.ActiveSession(ctx, ownerID); err == nil {
		return Session{}, ErrActiveSession
	} else if !errors.Is(err, ErrNoActiveSession) {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return u.repo.StartSession(ctx, ownerID, programID, now)
}

func (u *UseCase) Active(ctx context.Context, ownerID int64) (Session, error) {
	return u.repo.ActiveSession(ctx, ownerID)
}

func (u *UseCase) AddSet(ctx context.Context, ownerID int64, raw string) (Session, error) {
	input, err := ParseSet(raw)
	if err != nil {
		return Session{}, err
	}
	input.Type = SetTypeWorking
	input.CompletedAt = time.Now()
	session, err := u.repo.AddSet(ctx, ownerID, input)
	if err != nil {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *UseCase) CompleteWarmup(ctx context.Context, ownerID int64, now time.Time) (Session, error) {
	session, err := u.repo.ActiveSession(ctx, ownerID)
	if err != nil {
		return Session{}, err
	}
	exercise := session.CurrentExercise()
	if exercise == nil || !exercise.Structured() {
		return Session{}, ErrNotEditable
	}
	completed := len(exercise.WarmupSets())
	if completed >= len(exercise.Warmup) {
		return Session{}, ErrNotEditable
	}
	warmup := exercise.Warmup[completed]
	updated, err := u.repo.AddSet(ctx, ownerID, SetInput{
		Type: SetTypeWarmup, WeightKG: cloneFloat(warmup.WeightKG), Reps: warmup.Reps, CompletedAt: now,
	})
	if err != nil {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return updated, nil
}

func (u *UseCase) BeginWarmup(ctx context.Context, ownerID int64) (Session, error) {
	session, err := u.repo.ActiveSession(ctx, ownerID)
	if err != nil {
		return Session{}, err
	}
	exercise := session.CurrentExercise()
	if exercise == nil || !exercise.Structured() || len(exercise.WarmupSets()) < len(exercise.Warmup) || len(exercise.WorkingSets()) > 0 {
		return Session{}, ErrNotEditable
	}
	if err := u.Expect(ctx, ownerID, InputWarmup); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *UseCase) AddWarmup(ctx context.Context, ownerID int64, raw string, now time.Time) (Session, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return Session{}, err
	}
	if state.Mode != InputWarmup {
		return Session{}, ErrNotEditable
	}
	input, err := ParseSet(raw)
	if err != nil {
		return Session{}, err
	}
	session, err := u.repo.ActiveSession(ctx, ownerID)
	if err != nil {
		return Session{}, err
	}
	exercise := session.CurrentExercise()
	if exercise == nil || !exercise.Structured() || len(exercise.WarmupSets()) < len(exercise.Warmup) || len(exercise.WorkingSets()) > 0 {
		return Session{}, ErrNotEditable
	}
	input.Type = SetTypeWarmup
	input.CompletedAt = now
	session, err = u.repo.AddSet(ctx, ownerID, input)
	if err != nil {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *UseCase) PrepareWorkingSet(ctx context.Context, ownerID int64, reps int) (Session, error) {
	return u.prepareWorkingSet(ctx, ownerID, reps, nil, false)
}

func (u *UseCase) PrepareWorkingSetWithWeight(
	ctx context.Context,
	ownerID int64,
	reps int,
	weightKG *float64,
) (Session, error) {
	return u.prepareWorkingSet(ctx, ownerID, reps, weightKG, true)
}

func (u *UseCase) prepareWorkingSet(
	ctx context.Context,
	ownerID int64,
	reps int,
	actualWeightKG *float64,
	weightProvided bool,
) (Session, error) {
	if reps <= 0 || reps > 1000 {
		return Session{}, fmt.Errorf("повторения должны быть от 1 до 1000")
	}
	session, err := u.repo.ActiveSession(ctx, ownerID)
	if err != nil {
		return Session{}, err
	}
	exercise := session.CurrentExercise()
	if exercise == nil || !exercise.Structured() || len(exercise.WarmupSets()) < len(exercise.Warmup) {
		return Session{}, ErrNotEditable
	}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return Session{}, err
	}
	state.Mode = InputRIR
	weight := exercise.NextWorkingWeightKG()
	if weightProvided {
		weight = actualWeightKG
	}
	state.PendingSet = &PendingSet{Type: SetTypeWorking, WeightKG: cloneFloat(weight), Reps: reps}
	if err := u.repo.SaveUIState(ctx, state); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *UseCase) CompletePendingSet(
	ctx context.Context,
	ownerID int64,
	rir *float64,
	now time.Time,
) (Session, error) {
	if rir != nil && (*rir < 0 || *rir > 10) {
		return Session{}, fmt.Errorf("RIR должен быть от 0 до 10")
	}
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return Session{}, err
	}
	if state.Mode != InputRIR || state.PendingSet == nil {
		return Session{}, ErrNoPendingSet
	}
	pending := *state.PendingSet
	session, err := u.repo.AddSet(ctx, ownerID, SetInput{
		Type: pending.Type, WeightKG: cloneFloat(pending.WeightKG),
		Reps: pending.Reps, RIR: cloneFloat(rir), CompletedAt: now,
	})
	if err != nil {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *UseCase) BeginOverride(ctx context.Context, ownerID int64) (Session, error) {
	session, err := u.repo.ActiveSession(ctx, ownerID)
	if err != nil {
		return Session{}, err
	}
	exercise := session.CurrentExercise()
	if exercise == nil || !exercise.Structured() {
		return Session{}, ErrNotEditable
	}
	if err := u.Expect(ctx, ownerID, InputOverride); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *UseCase) OverrideCurrentExercise(ctx context.Context, ownerID int64, raw string) (Session, error) {
	state, err := u.State(ctx, ownerID)
	if err != nil {
		return Session{}, err
	}
	if state.Mode != InputOverride {
		return Session{}, ErrNotEditable
	}
	override, err := ParseExerciseOverride(raw)
	if err != nil {
		return Session{}, err
	}
	session, err := u.repo.OverrideCurrentExercise(ctx, ownerID, override)
	if err != nil {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return session, nil
}

func ParseExerciseOverride(raw string) (ExerciseOverride, error) {
	parts := strings.Split(raw, ";")
	if len(parts) != 5 {
		return ExerciseOverride{}, fmt.Errorf("нужен формат: 60;3;8-12;2;180s")
	}
	var weight *float64
	weightRaw := strings.TrimSpace(strings.TrimSuffix(strings.ToLower(parts[0]), "kg"))
	if weightRaw != "-" {
		value, err := strconv.ParseFloat(strings.ReplaceAll(weightRaw, ",", "."), 64)
		if err != nil || value <= 0 {
			return ExerciseOverride{}, fmt.Errorf("вес должен быть положительным числом или -")
		}
		weight = &value
	}
	sets, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || sets <= 0 || sets > 20 {
		return ExerciseOverride{}, fmt.Errorf("подходы должны быть от 1 до 20")
	}
	reps, err := parseOverrideRepRange(parts[2])
	if err != nil {
		return ExerciseOverride{}, err
	}
	targetRIR, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(parts[3]), ",", "."), 64)
	if err != nil || targetRIR < 0 || targetRIR > 10 {
		return ExerciseOverride{}, fmt.Errorf("RIR должен быть от 0 до 10")
	}
	restRaw := strings.TrimSpace(parts[4])
	if _, err := strconv.Atoi(restRaw); err == nil {
		restRaw += "s"
	}
	rest, err := time.ParseDuration(restRaw)
	if err != nil || rest < 0 || rest%time.Second != 0 {
		return ExerciseOverride{}, fmt.Errorf("отдых должен быть задан как 180s или 3m")
	}
	return ExerciseOverride{
		WeightKG: weight, Reps: reps, WorkingSets: sets,
		TargetRIR: targetRIR, RestSeconds: int(rest / time.Second),
	}, nil
}

func parseOverrideRepRange(raw string) (RepRange, error) {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) > 2 {
		return RepRange{}, fmt.Errorf("повторения должны быть числом или диапазоном 8-12")
	}
	min, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || min <= 0 {
		return RepRange{}, fmt.Errorf("повторения должны быть числом или диапазоном 8-12")
	}
	max := min
	if len(parts) == 2 {
		max, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || max < min {
			return RepRange{}, fmt.Errorf("минимум повторений не может быть больше максимума")
		}
	}
	return RepRange{Min: min, Max: max}, nil
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (u *UseCase) AddNote(ctx context.Context, ownerID int64, raw string) (Session, error) {
	note := strings.TrimSpace(raw)
	if note == "" {
		return Session{}, fmt.Errorf("заметка пустая")
	}
	if len([]rune(note)) > 1000 {
		return Session{}, fmt.Errorf("заметка длиннее 1000 символов")
	}
	session, err := u.repo.SetCurrentExerciseNote(ctx, ownerID, note)
	if err != nil {
		return Session{}, err
	}
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (u *UseCase) FinishExercise(ctx context.Context, ownerID int64, now time.Time) (Session, error) {
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return u.repo.FinishCurrentExercise(ctx, ownerID, now)
}

func (u *UseCase) PrioritizeExercise(ctx context.Context, ownerID, exerciseID int64) (Session, error) {
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return u.repo.PrioritizeExercise(ctx, ownerID, exerciseID)
}

func (u *UseCase) ReopenExercise(ctx context.Context, ownerID, sessionID, exerciseID int64) (Session, error) {
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return Session{}, err
	}
	return u.repo.ReopenExercise(ctx, ownerID, sessionID, exerciseID)
}

func (u *UseCase) PreviousExercise(
	ctx context.Context,
	ownerID, sessionID int64,
	exerciseName string,
) (*PreviousExercise, error) {
	return u.repo.PreviousExercise(ctx, ownerID, sessionID, exerciseName)
}

func (u *UseCase) Session(ctx context.Context, ownerID, sessionID int64) (Session, error) {
	return u.repo.Session(ctx, ownerID, sessionID)
}

func (u *UseCase) History(ctx context.Context, ownerID int64, page, pageSize int) (SessionPage, error) {
	if page < 1 || pageSize < 1 || pageSize > 20 {
		return SessionPage{}, ErrInvalidPage
	}
	items, total, err := u.repo.RecentSessions(ctx, ownerID, pageSize, (page-1)*pageSize)
	if err != nil {
		return SessionPage{}, err
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		return SessionPage{}, ErrInvalidPage
	}
	return SessionPage{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages}, nil
}

func (u *UseCase) MarkPublished(ctx context.Context, ownerID, sessionID, chatID int64, messageID int) error {
	return u.repo.MarkPublished(ctx, ownerID, sessionID, chatID, messageID)
}

func (u *UseCase) DeleteSession(ctx context.Context, ownerID, sessionID int64) error {
	if err := u.ClearInput(ctx, ownerID); err != nil {
		return err
	}
	return u.repo.DeleteSession(ctx, ownerID, sessionID)
}
