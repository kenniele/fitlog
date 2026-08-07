package training

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stateRepository struct {
	Repository
	state UIState
}

func (r *stateRepository) GetUIState(context.Context, int64) (UIState, error) {
	return r.state, nil
}

func (r *stateRepository) SaveUIState(_ context.Context, state UIState) error {
	r.state = state
	return nil
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
