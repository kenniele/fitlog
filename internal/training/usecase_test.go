package training

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stateRepository struct {
	Repository
	state            UIState
	deletedSessionID int64
	exercises        []Exercise
	totalExercises   int
	listLimit        int
	listOffset       int
	renameResult     RenameResult
	renamedID        int64
	renamedName      string
	replacedHistory  bool
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

func (r *stateRepository) RenameExercise(
	_ context.Context,
	_ int64,
	exerciseID int64,
	name string,
	replaceHistory bool,
) (RenameResult, error) {
	r.renamedID = exerciseID
	r.renamedName = name
	r.replacedHistory = replaceHistory
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

func TestExerciseRenameRequiresScopeConfirmation(t *testing.T) {
	exerciseID := int64(7)
	repo := &stateRepository{
		state:        UIState{OwnerID: 42, Mode: InputRename, PendingExerciseID: &exerciseID},
		exercises:    []Exercise{{ID: exerciseID, OwnerID: 42, Name: "Старая тяга"}},
		renameResult: RenameResult{Exercise: Exercise{ID: exerciseID, Name: "Новая тяга"}},
	}
	usecase := NewUseCase(repo)

	preview, err := usecase.PrepareExerciseRename(context.Background(), 42, "  Новая тяга  ")

	require.NoError(t, err)
	require.Equal(t, "Старая тяга", preview.Exercise.Name)
	require.Equal(t, "Новая тяга", preview.NewName)
	require.Equal(t, InputRenameOK, repo.state.Mode)
	require.Equal(t, "Новая тяга", repo.state.PendingExerciseName)
	require.Zero(t, repo.renamedID, "rename must wait for the scope button")

	result, err := usecase.ConfirmExerciseRename(context.Background(), 42, false)

	require.NoError(t, err)
	require.Equal(t, exerciseID, repo.renamedID)
	require.Equal(t, "Новая тяга", repo.renamedName)
	require.False(t, repo.replacedHistory)
	require.Equal(t, "Новая тяга", result.Exercise.Name)
	require.Equal(t, InputNone, repo.state.Mode)
	require.Nil(t, repo.state.PendingExerciseID)
	require.Empty(t, repo.state.PendingExerciseName)
}

func TestExerciseRenameCanReplaceHistory(t *testing.T) {
	exerciseID := int64(7)
	repo := &stateRepository{
		state: UIState{
			OwnerID: 42, Mode: InputRenameOK, PendingExerciseID: &exerciseID,
			PendingExerciseName: "Новая тяга",
		},
		renameResult: RenameResult{Exercise: Exercise{ID: exerciseID, Name: "Новая тяга"}},
	}

	_, err := NewUseCase(repo).ConfirmExerciseRename(context.Background(), 42, true)

	require.NoError(t, err)
	require.True(t, repo.replacedHistory)
}
