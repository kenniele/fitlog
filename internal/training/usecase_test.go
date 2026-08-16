package training

import (
	"context"
	"testing"

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
