package training

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stateRepository struct {
	Repository
	state                     UIState
	deletedSessionID          int64
	exercises                 []Exercise
	totalExercises            int
	listLimit                 int
	listOffset                int
	renameResult              RenameResult
	renamedID                 int64
	renamedName               string
	program                   Program
	programReplacement        ProgramExerciseReplacement
	programReplaceResult      ProgramExerciseReplaceResult
	replacedProgramExerciseID int64
	replacementTargetID       *int64
	replacementTargetName     string
	replacedHistory           bool
	activeSession             Session
	lastSet                   SetInput
	override                  ExerciseOverride
	finishCalled              bool
	prioritizedExerciseID     int64
}

func (r *stateRepository) GetUIState(context.Context, int64) (UIState, error) {
	return r.state, nil
}

func (r *stateRepository) SaveUIState(_ context.Context, state UIState) error {
	r.state = state
	return nil
}

func (r *stateRepository) DeleteSession(_ context.Context, _ int64, sessionID int64) error {
	r.deletedSessionID = sessionID
	return nil
}

func (r *stateRepository) ListExercises(_ context.Context, _ int64, limit, offset int) ([]Exercise, int, error) {
	r.listLimit = limit
	r.listOffset = offset
	return r.exercises, r.totalExercises, nil
}

func (r *stateRepository) SimilarExercises(context.Context, int64, string, int) ([]Exercise, error) {
	return r.exercises, nil
}

func (r *stateRepository) Exercise(_ context.Context, _ int64, exerciseID int64) (Exercise, error) {
	for _, exercise := range r.exercises {
		if exercise.ID == exerciseID {
			return exercise, nil
		}
	}
	return Exercise{}, ErrNotFound
}

func (r *stateRepository) Program(context.Context, int64, int64) (Program, error) {
	return r.program, nil
}

func (r *stateRepository) ProgramExercise(context.Context, int64, int64) (ProgramExerciseReplacement, error) {
	return r.programReplacement, nil
}

func (r *stateRepository) ReplaceProgramExercise(
	_ context.Context,
	_ int64,
	programExerciseID int64,
	targetExerciseID *int64,
	targetName string,
	replaceHistory bool,
) (ProgramExerciseReplaceResult, error) {
	r.replacedProgramExerciseID = programExerciseID
	r.replacementTargetID = targetExerciseID
	r.replacementTargetName = targetName
	r.replacedHistory = replaceHistory
	return r.programReplaceResult, nil
}

func (r *stateRepository) RenameExercise(_ context.Context, _ int64, exerciseID int64, name string) (RenameResult, error) {
	r.renamedID = exerciseID
	r.renamedName = name
	return r.renameResult, nil
}

func (r *stateRepository) ActiveSession(context.Context, int64) (Session, error) {
	return r.activeSession, nil
}

func (r *stateRepository) AddSet(_ context.Context, _ int64, input SetInput) (Session, error) {
	r.lastSet = input
	session := r.activeSession
	exercise := session.CurrentExercise()
	actualReps := input.Reps
	set := WorkoutSet{
		ID: int64(len(exercise.Sets) + 1), Position: len(exercise.Sets) + 1, Type: input.Type,
		ActualWeightKG: cloneFloat(input.WeightKG),
		ActualReps:     &actualReps, ActualRIR: cloneFloat(input.RIR),
		WeightKG: cloneFloat(input.WeightKG), Reps: input.Reps, RIR: cloneFloat(input.RIR),
	}
	exercise.Sets = append(exercise.Sets, set)
	r.activeSession = session
	return session, nil
}

func (r *stateRepository) FinishCurrentExercise(_ context.Context, _ int64, _ time.Time) (Session, error) {
	r.finishCalled = true
	session := r.activeSession
	session.Status = "finished"
	r.activeSession = session
	return session, nil
}

func (r *stateRepository) PrioritizeExercise(_ context.Context, _ int64, exerciseID int64) (Session, error) {
	r.prioritizedExerciseID = exerciseID
	session := r.activeSession
	current := session.CurrentExercise()
	var target *SessionExercise
	for index := range session.Exercises {
		if session.Exercises[index].ID == exerciseID {
			target = &session.Exercises[index]
			break
		}
	}
	if current == nil || target == nil || target.Complete || target.Position <= current.Position {
		return Session{}, ErrNotEditable
	}
	currentPosition := current.Position
	targetPosition := target.Position
	for index := range session.Exercises {
		position := session.Exercises[index].Position
		if position >= currentPosition && position < targetPosition {
			session.Exercises[index].Position++
		}
	}
	target.Position = currentPosition
	r.activeSession = session
	return session, nil
}

func (r *stateRepository) OverrideCurrentExercise(_ context.Context, _ int64, override ExerciseOverride) (Session, error) {
	r.override = override
	session := r.activeSession
	exercise := session.CurrentExercise()
	exercise.Plan.WeightKG = cloneFloat(override.WeightKG)
	exercise.Plan.MinReps = override.Reps.Min
	exercise.Plan.MaxReps = override.Reps.Max
	exercise.Plan.WorkingSets = override.WorkingSets
	exercise.Plan.TargetRIR = override.TargetRIR
	exercise.Plan.RestSeconds = override.RestSeconds
	exercise.Overridden = true
	r.activeSession = session
	return session, nil
}

func TestOpenControlMessageStartsFreshCard(t *testing.T) {
	repo := &stateRepository{state: UIState{
		OwnerID:       42,
		ChatID:        100,
		MessageID:     777,
		Mode:          InputImportOK,
		PendingImport: &ImportPreview{Filename: "program.txt"},
	}}

	err := NewUseCase(repo).OpenControlMessage(context.Background(), 42, 200)

	require.NoError(t, err)
	require.Equal(t, int64(42), repo.state.OwnerID)
	require.Equal(t, int64(200), repo.state.ChatID)
	require.Zero(t, repo.state.MessageID)
	require.Equal(t, InputNone, repo.state.Mode)
	require.Nil(t, repo.state.PendingImport)
}

func TestDeleteSessionClearsPendingInput(t *testing.T) {
	repo := &stateRepository{state: UIState{
		OwnerID:       42,
		ChatID:        100,
		MessageID:     777,
		Mode:          InputNote,
		PendingImport: &ImportPreview{Filename: "program.txt"},
	}}

	err := NewUseCase(repo).DeleteSession(context.Background(), 42, 123)

	require.NoError(t, err)
	require.Equal(t, int64(123), repo.deletedSessionID)
	require.Equal(t, InputNone, repo.state.Mode)
	require.Nil(t, repo.state.PendingImport)
}

func TestExercisePaginationUsesFiveItemOffsets(t *testing.T) {
	repo := &stateRepository{
		exercises:      []Exercise{{ID: 6, Name: "Шестое"}, {ID: 7, Name: "Седьмое"}},
		totalExercises: 7,
	}

	page, err := NewUseCase(repo).Exercises(context.Background(), 42, 2, 5)

	require.NoError(t, err)
	require.Equal(t, 5, repo.listLimit)
	require.Equal(t, 5, repo.listOffset)
	require.Equal(t, 2, page.Page)
	require.Equal(t, 2, page.TotalPages)
	require.Len(t, page.Items, 2)
}

func TestImportReviewReplacesOnlySelectedExercise(t *testing.T) {
	repo := &stateRepository{
		state: UIState{OwnerID: 42, Mode: InputImportOK, PendingImport: &ImportPreview{
			Filename: "plan.txt",
			Programs: []ProgramInput{
				{Name: "A", Exercises: []string{"Тяга гантели", "Присед"}},
				{Name: "B", Exercises: []string{"Жим"}},
			},
		}},
		exercises: []Exercise{{ID: 9, OwnerID: 42, Name: "Тяга верхнего блока"}},
	}
	usecase := NewUseCase(repo)

	review, err := usecase.BeginImportReview(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 1, review.Current)
	require.Equal(t, 3, review.Total)

	review, done, err := usecase.UseExistingImportExercise(context.Background(), 42, 9)
	require.NoError(t, err)
	require.False(t, done)
	require.Equal(t, "Присед", review.ProposedName)
	require.Equal(t, "Тяга верхнего блока", repo.state.PendingImport.Programs[0].Exercises[0])

	review, done, err = usecase.KeepNewImportExercise(context.Background(), 42)
	require.NoError(t, err)
	require.False(t, done)
	require.Equal(t, "Жим", review.ProposedName)
	_, done, err = usecase.KeepNewImportExercise(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, done)
}

func TestRenameExerciseKeepsGlobalCatalogBehavior(t *testing.T) {
	exerciseID := int64(7)
	repo := &stateRepository{
		state:        UIState{OwnerID: 42, Mode: InputRename, PendingExerciseID: &exerciseID},
		renameResult: RenameResult{Exercise: Exercise{ID: exerciseID, Name: "Новая тяга"}},
	}

	result, err := NewUseCase(repo).RenameExercise(context.Background(), 42, "  Новая тяга  ")

	require.NoError(t, err)
	require.Equal(t, exerciseID, repo.renamedID)
	require.Equal(t, "Новая тяга", repo.renamedName)
	require.Equal(t, "Новая тяга", result.Exercise.Name)
	require.Equal(t, InputNone, repo.state.Mode)
	require.Nil(t, repo.state.PendingExerciseID)
}

func TestProgramExerciseCanBeReplacedWithExistingOnlyInProgram(t *testing.T) {
	programExerciseID := int64(17)
	targetExerciseID := int64(9)
	program := Program{ID: 3, OwnerID: 42, Name: "Fullbody A"}
	repo := &stateRepository{
		state:     UIState{OwnerID: 42},
		exercises: []Exercise{{ID: targetExerciseID, OwnerID: 42, Name: "Тяга верхнего блока"}},
		programReplacement: ProgramExerciseReplacement{
			Program: program,
			Current: ProgramExercise{ID: programExerciseID, Position: 2, Name: "Тяга гантели"},
		},
		programReplaceResult: ProgramExerciseReplaceResult{Program: program},
	}
	usecase := NewUseCase(repo)

	_, err := usecase.BeginProgramExerciseReplacement(context.Background(), 42, programExerciseID)
	require.NoError(t, err)
	require.Equal(t, InputProgramExerciseChoice, repo.state.Mode)

	preview, err := usecase.ChooseExistingProgramExercise(context.Background(), 42, targetExerciseID)
	require.NoError(t, err)
	require.Equal(t, "Тяга верхнего блока", preview.Target.Name)
	require.Equal(t, InputProgramExerciseConfirm, repo.state.Mode)

	_, err = usecase.ConfirmProgramExerciseReplacement(context.Background(), 42, false)
	require.NoError(t, err)
	require.Equal(t, programExerciseID, repo.replacedProgramExerciseID)
	require.Equal(t, targetExerciseID, *repo.replacementTargetID)
	require.Empty(t, repo.replacementTargetName)
	require.False(t, repo.replacedHistory)
	require.Equal(t, InputNone, repo.state.Mode)
}

func TestProgramExerciseCanBeReplacedWithNewAndHistory(t *testing.T) {
	programExerciseID := int64(17)
	program := Program{ID: 3, OwnerID: 42, Name: "Fullbody A"}
	repo := &stateRepository{
		state: UIState{OwnerID: 42},
		programReplacement: ProgramExerciseReplacement{
			Program: program,
			Current: ProgramExercise{ID: programExerciseID, Position: 2, Name: "Тяга гантели"},
		},
		programReplaceResult: ProgramExerciseReplaceResult{Program: program},
	}
	usecase := NewUseCase(repo)

	_, err := usecase.BeginProgramExerciseReplacement(context.Background(), 42, programExerciseID)
	require.NoError(t, err)
	_, err = usecase.ExpectNewProgramExercise(context.Background(), 42)
	require.NoError(t, err)
	preview, err := usecase.PrepareNewProgramExercise(context.Background(), 42, "  Тяга Т-грифа  ")
	require.NoError(t, err)
	require.Equal(t, "Тяга Т-грифа", preview.Target.Name)

	_, err = usecase.ConfirmProgramExerciseReplacement(context.Background(), 42, true)

	require.NoError(t, err)
	require.Nil(t, repo.replacementTargetID)
	require.Equal(t, "Тяга Т-грифа", repo.replacementTargetName)
	require.True(t, repo.replacedHistory)
}

func TestOrdinarySetsRequireRIRAndExerciseWaitsForExplicitFinish(t *testing.T) {
	weight := 60.0
	rir := 2.0
	repo := &stateRepository{
		state: UIState{OwnerID: 42, Mode: InputSet},
		activeSession: Session{ID: 1, OwnerID: 42, Status: "active", CurrentPosition: 1, Exercises: []SessionExercise{{
			ID: 2, Position: 1, Name: "Жим",
		}}},
	}
	usecase := NewUseCase(repo)

	_, err := usecase.PrepareWorkingSetInput(context.Background(), 42, "12Р 60КГ")
	require.NoError(t, err)
	require.Equal(t, InputRIR, repo.state.Mode)
	require.Equal(t, 12, repo.state.PendingSet.Reps)

	session, err := usecase.CompletePendingSet(context.Background(), 42, nil, time.Now())
	require.ErrorContains(t, err, "укажи RIR")
	require.Empty(t, session)
	require.Empty(t, repo.activeSession.CurrentExercise().Sets)

	session, err = usecase.CompletePendingSet(context.Background(), 42, &rir, time.Now())
	require.NoError(t, err)
	require.False(t, repo.finishCalled)
	require.True(t, session.Active())
	require.InDelta(t, 2, *repo.lastSet.RIR, 0.001)
	require.Equal(t, SetTypeWorking, repo.lastSet.Type)
	require.Equal(t, InputNone, repo.state.Mode)

	repo.state.Mode = InputSet
	_, err = usecase.PrepareWorkingSetInput(context.Background(), 42, "10")
	require.NoError(t, err, "the latest working weight must be reusable for another set")
	require.InDelta(t, weight, *repo.state.PendingSet.WeightKG, 0.001)
	session, err = usecase.CompletePendingSet(context.Background(), 42, &rir, time.Now())
	require.NoError(t, err)
	require.Len(t, session.CurrentExercise().WorkingSets(), 2)
	require.False(t, repo.finishCalled)

	session, err = usecase.FinishExercise(context.Background(), 42, time.Now())
	require.NoError(t, err)
	require.True(t, repo.finishCalled)
	require.False(t, session.Active())
}

func TestPrioritizeExerciseMovesItBeforeCurrentAndClearsPendingInput(t *testing.T) {
	repo := &stateRepository{
		state: UIState{OwnerID: 42, Mode: InputNote},
		activeSession: Session{ID: 1, OwnerID: 42, Status: "active", CurrentPosition: 1, Exercises: []SessionExercise{
			{ID: 10, Position: 1, Name: "Жим"},
			{ID: 11, Position: 2, Name: "Тяга"},
			{ID: 12, Position: 3, Name: "Присед"},
		}},
	}

	session, err := NewUseCase(repo).PrioritizeExercise(context.Background(), 42, 12)

	require.NoError(t, err)
	require.Equal(t, int64(12), repo.prioritizedExerciseID)
	require.Equal(t, InputNone, repo.state.Mode)
	require.Equal(t, int64(12), session.CurrentExercise().ID)
	require.Equal(t, []int64{10, 11, 12}, []int64{
		session.Exercises[0].ID, session.Exercises[1].ID, session.Exercises[2].ID,
	}, "the loaded slice keeps identity order in the fake")
	require.Equal(t, []int{2, 3, 1}, []int{
		session.Exercises[0].Position, session.Exercises[1].Position, session.Exercises[2].Position,
	})
}

func TestWarmupButtonExplicitlyStoresWarmupWithoutRIR(t *testing.T) {
	workingWeight := 40.0
	workingReps := 8
	repo := &stateRepository{
		state: UIState{OwnerID: 42, Mode: InputSet},
		activeSession: Session{ID: 1, OwnerID: 42, Status: "active", CurrentPosition: 1, Exercises: []SessionExercise{{
			ID: 2, Position: 1, Name: "Жим",
			Sets: []WorkoutSet{{
				ID: 3, Position: 1, Type: SetTypeWorking, ActualWeightKG: &workingWeight, ActualReps: &workingReps, Reps: workingReps,
			}},
		}}},
	}
	usecase := NewUseCase(repo)

	_, err := usecase.BeginWarmup(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, InputWarmup, repo.state.Mode)

	completedAt := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	session, err := usecase.AddWarmup(context.Background(), 42, "5Р 40КГ", completedAt)
	require.NoError(t, err)
	require.Equal(t, SetTypeWarmup, repo.lastSet.Type)
	require.Equal(t, 5, repo.lastSet.Reps)
	require.InDelta(t, 40, *repo.lastSet.WeightKG, 0.001)
	require.Nil(t, repo.lastSet.RIR)
	require.Equal(t, completedAt, repo.lastSet.CompletedAt)
	require.Len(t, session.CurrentExercise().WarmupSets(), 1)
	require.Equal(t, InputNone, repo.state.Mode)
}

func TestWorkingSetUsesLatestActualWeightFromCurrentWorkout(t *testing.T) {
	actualWeight := 40.8
	repo := &stateRepository{
		state: UIState{OwnerID: 42, Mode: InputSet},
		activeSession: Session{ID: 1, OwnerID: 42, Status: "active", CurrentPosition: 1, Exercises: []SessionExercise{{
			ID: 2, Position: 1, Name: "Жим",
			Sets: []WorkoutSet{{
				ID: 3, Position: 1, Type: SetTypeWorking, ActualWeightKG: &actualWeight, Reps: 12,
			}},
		}}},
	}

	_, err := NewUseCase(repo).PrepareWorkingSetInput(context.Background(), 42, "10")

	require.NoError(t, err)
	require.Equal(t, InputRIR, repo.state.Mode)
	require.NotNil(t, repo.state.PendingSet)
	require.InDelta(t, 40.8, *repo.state.PendingSet.WeightKG, 0.001)
}

func TestFirstOrdinarySetRequiresExplicitWeight(t *testing.T) {
	repo := &stateRepository{
		state: UIState{OwnerID: 42, Mode: InputSet},
		activeSession: Session{ID: 1, OwnerID: 42, Status: "active", CurrentPosition: 1, Exercises: []SessionExercise{{
			ID: 2, Position: 1, Name: "Подтягивания",
		}}},
	}
	usecase := NewUseCase(repo)

	_, err := usecase.PrepareWorkingSetInput(context.Background(), 42, "10")
	require.ErrorContains(t, err, "для первого обычного подхода укажи вес полностью")

	_, err = usecase.PrepareWorkingSetInput(context.Background(), 42, "10Р -")
	require.NoError(t, err)
	require.Equal(t, InputRIR, repo.state.Mode)
	require.Nil(t, repo.state.PendingSet.WeightKG)

	rir := 3.0
	_, err = usecase.CompletePendingSet(context.Background(), 42, &rir, time.Now())
	require.NoError(t, err)
	repo.state.Mode = InputSet
	_, err = usecase.PrepareWorkingSetInput(context.Background(), 42, "8")
	require.NoError(t, err)
	require.Nil(t, repo.state.PendingSet.WeightKG)
}

func TestOverrideDoesNotModifyOriginalRecommendation(t *testing.T) {
	originalWeight := 62.5
	repo := &stateRepository{
		state: UIState{OwnerID: 42},
		activeSession: Session{ID: 1, OwnerID: 42, Status: "active", CurrentPosition: 1, Exercises: []SessionExercise{{
			ID: 2, Position: 1, Name: "Жим",
			Recommendation: Recommendation{WeightKG: &originalWeight, MinReps: 8, MaxReps: 12, WorkingSets: 3, TargetRIR: 2},
			Plan:           Recommendation{WeightKG: &originalWeight, MinReps: 8, MaxReps: 12, WorkingSets: 3, TargetRIR: 2},
		}}},
	}
	usecase := NewUseCase(repo)
	_, err := usecase.BeginOverride(context.Background(), 42)
	require.NoError(t, err)

	session, err := usecase.OverrideCurrentExercise(context.Background(), 42, "60;2;10-12;3;2m")
	require.NoError(t, err)
	require.InDelta(t, 60, *session.CurrentExercise().Plan.WeightKG, 0.001)
	require.Equal(t, 2, session.CurrentExercise().Plan.WorkingSets)
	require.InDelta(t, 62.5, *session.CurrentExercise().Recommendation.WeightKG, 0.001)
	require.InDelta(t, 60, *repo.override.WeightKG, 0.001)
	require.Equal(t, 120, repo.override.RestSeconds)
}
