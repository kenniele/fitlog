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

func TestBotSurfaceHasThreeButtonsAndFourCommands(t *testing.T) {
	menu := mainMenu()
	require.True(t, menu.IsPersistent)
	require.Len(t, menu.ReplyKeyboard, 2)
	require.Len(t, menu.ReplyKeyboard[0], 2)
	require.Equal(t, HealthButton, menu.ReplyKeyboard[0][0].Text)
	require.Equal(t, NutritionButton, menu.ReplyKeyboard[0][1].Text)
	require.Len(t, menu.ReplyKeyboard[1], 1)
	require.Equal(t, ArticleButton, menu.ReplyKeyboard[1][0].Text)

	commands := botCommands()
	require.Len(t, commands, 4)
	require.Equal(t, "health_summary", commands[0].Text)
	require.Equal(t, "nutrition_analysis", commands[1].Text)
	require.Equal(t, "info", commands[2].Text)
	require.Equal(t, "connect_fatsecret", commands[3].Text)
}
