package bot

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v3"

	"fitlog/internal/training"
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
	require.LessOrEqual(t, len("\f"+trainingCallbackRenameExercise+"|"+payload), 64)
	require.LessOrEqual(t, len("\f"+trainingCallbackImportExisting+"|"+maxID), 64)
}

func TestParseTrainingPage(t *testing.T) {
	page, err := parseTrainingPage("")
	require.NoError(t, err)
	require.Equal(t, 1, page)
	page, err = parseTrainingPage("2")
	require.NoError(t, err)
	require.Equal(t, 2, page)
	_, err = parseTrainingPage("0")
	require.Error(t, err)
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

func TestUpdatePublishedTrainingEditsExistingChannelPost(t *testing.T) {
	var payload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/bottest/editMessageText", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		_, err := w.Write([]byte(`{"ok":true,"result":{"message_id":456,"date":1,"chat":{"id":-100123,"type":"channel"}}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	telegram, err := tele.NewBot(tele.Settings{
		URL: server.URL, Token: "test", Offline: true, Client: server.Client(),
	})
	require.NoError(t, err)
	chatID := int64(-100123)
	messageID := 456
	weight := 47.5
	session := training.Session{
		ID: 1, OwnerID: 42, ProgramName: "Фуллбади C", Status: "finished",
		StartedAt:       time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC),
		PublishedChatID: &chatID, PublishedMessageID: &messageID,
		Exercises: []training.SessionExercise{{
			ID: 2, Position: 1, Name: "Тяга", Complete: true,
			Sets: []training.WorkoutSet{{ID: 3, Position: 1, Reps: 8, WeightKG: &weight}},
		}},
	}
	b := &Bot{b: telegram, deps: Deps{Location: time.UTC}}

	updated, err := b.updatePublishedTraining(session)

	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, "-100123", payload["chat_id"])
	require.Equal(t, "456", payload["message_id"])
	require.Contains(t, payload["text"], "8Р 47.5КГ")
}

func TestUpdatePublishedTrainingValidatesStoredMessage(t *testing.T) {
	b := &Bot{}
	updated, err := b.updatePublishedTraining(training.Session{})
	require.NoError(t, err)
	require.False(t, updated)

	chatID := int64(-100123)
	updated, err = b.updatePublishedTraining(training.Session{PublishedChatID: &chatID})
	require.Error(t, err)
	require.False(t, updated)
}
