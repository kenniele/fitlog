package bot

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrainingPairRoundTrip(t *testing.T) {
	payload := trainingPair(math.MaxInt64, math.MinInt64)
	first, second, err := parseTrainingPair(payload)
	require.NoError(t, err)
	require.Equal(t, int64(math.MaxInt64), first)
	require.Equal(t, int64(math.MinInt64), second)
	const maxID = "9223372036854775807"
	require.LessOrEqual(t, len("\f"+trainingCallbackPublishChannel+"|"+payload), 64)
	require.LessOrEqual(t, len("\f"+trainingCallbackConfirmDelete+"|"+maxID), 64)
	require.LessOrEqual(t, len("\f"+trainingCallbackDeleteLocal+"|"+maxID), 64)
}

func TestParseTrainingPairRejectsInvalidPayload(t *testing.T) {
	for _, payload := range []string{"", "1", "0:2", "1:0", "one:two", "1:2:3"} {
		_, _, err := parseTrainingPair(payload)
		require.Error(t, err, payload)
	}
}

func TestTruncateTrainingButtonKeepsUnicodeBoundaries(t *testing.T) {
	value := "Очень длинное название упражнения с гантелями и дополнительным текстом"
	truncated := truncateTrainingButton(value)
	require.LessOrEqual(t, len([]rune(truncated)), 48)
	require.Contains(t, truncated, "…")
}

func TestIsMessageAlreadyDeleted(t *testing.T) {
	require.True(t, isMessageAlreadyDeleted(errors.New("Bad Request: message to delete not found")))
	require.False(t, isMessageAlreadyDeleted(errors.New("Bad Request: message can't be deleted")))
}
