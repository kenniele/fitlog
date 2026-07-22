package bot

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllowlist(t *testing.T) {
	a := NewAllowlist([]int64{42, 100}, slog.Default())
	require.True(t, a.Allowed(42))
	require.True(t, a.Allowed(100))
	require.False(t, a.Allowed(7))
	require.False(t, a.Allowed(0))
}

func TestBotSurfaceHasTwoButtonsAndOneCommand(t *testing.T) {
	menu := mainMenu()
	require.True(t, menu.IsPersistent)
	require.Len(t, menu.ReplyKeyboard, 1)
	require.Len(t, menu.ReplyKeyboard[0], 2)
	require.Equal(t, HealthButton, menu.ReplyKeyboard[0][0].Text)
	require.Equal(t, NutritionButton, menu.ReplyKeyboard[0][1].Text)

	commands := botCommands()
	require.Len(t, commands, 1)
	require.Equal(t, "health_summary", commands[0].Text)
}
