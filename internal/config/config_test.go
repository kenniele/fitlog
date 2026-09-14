package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadProviderSyncConfig(t *testing.T) {
	for key, value := range map[string]string{
		"DATABASE_URL":                       "postgres://fitlog:test@localhost/fitlog",
		"FITLOG_TOKEN_ENCRYPTION_KEY":        "test-key",
		"WHOOP_CLIENT_ID":                    "whoop-id",
		"WHOOP_CLIENT_SECRET":                "whoop-secret",
		"WHOOP_REDIRECT_URI":                 "https://fitlog.example/oauth/whoop/callback",
		"FATSECRET_CONSUMER_KEY":             "fatsecret-key",
		"FATSECRET_CONSUMER_SECRET":          "fatsecret-secret",
		"TELEGRAM_BOT_TOKEN":                 "telegram-token",
		"TELEGRAM_ALLOWED_USER_IDS":          "42",
		"FITLOG_PROVIDER_SYNC_INTERVAL":      "45m",
		"FITLOG_PROVIDER_SYNC_LOOKBACK_DAYS": "5",
		"FATSECRET_STORAGE_AUTHORIZED":       "true",
	} {
		t.Setenv(key, value)
	}

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 45*time.Minute, cfg.ProviderSyncInterval)
	require.Equal(t, 5, cfg.ProviderSyncLookbackDays)
	require.True(t, cfg.FatSecretStorageAuthorized)
}

func TestBaseURL(t *testing.T) {
	cfg := Config{PublicBaseURL: "https://fitlog.example/path/", WhoopRedirectURI: "https://oauth.example/callback"}
	base, err := cfg.BaseURL()
	require.NoError(t, err)
	require.Equal(t, "https://fitlog.example", base)
}

func TestBaseURLFallsBackToWhoopOrigin(t *testing.T) {
	cfg := Config{WhoopRedirectURI: "https://fitlog.example/oauth/whoop/callback"}
	base, err := cfg.BaseURL()
	require.NoError(t, err)
	require.Equal(t, "https://fitlog.example", base)
}

func TestBaseURLRejectsUnsupportedScheme(t *testing.T) {
	_, err := (&Config{PublicBaseURL: "file:///tmp/vault"}).BaseURL()
	require.Error(t, err)
}

func TestWorkoutChannelsSupportsLegacyAndList(t *testing.T) {
	cfg := Config{
		TelegramWorkoutChannelID:  -1001,
		TelegramWorkoutChannelIDs: []int64{-1002, -1001, 0, -1003},
	}
	require.Equal(t, []int64{-1001, -1002, -1003}, cfg.WorkoutChannels())
}

func TestDashboardOwner(t *testing.T) {
	t.Run("explicit owner", func(t *testing.T) {
		cfg := Config{DashboardOwnerID: 77, TelegramAllowedUserIDs: []int64{42}}
		ownerID, err := cfg.DashboardOwner()
		require.NoError(t, err)
		require.Equal(t, int64(77), ownerID)
	})

	t.Run("first allowlisted owner", func(t *testing.T) {
		cfg := Config{TelegramAllowedUserIDs: []int64{42, 77}}
		ownerID, err := cfg.DashboardOwner()
		require.NoError(t, err)
		require.Equal(t, int64(42), ownerID)
	})

	t.Run("missing owner", func(t *testing.T) {
		_, err := (&Config{}).DashboardOwner()
		require.Error(t, err)
	})
}

func TestMCPConfig(t *testing.T) {
	for key, value := range map[string]string{"DATABASE_URL": "unused", "FITLOG_TOKEN_ENCRYPTION_KEY": "unused", "WHOOP_CLIENT_ID": "unused", "WHOOP_CLIENT_SECRET": "unused", "WHOOP_REDIRECT_URI": "https://fitlog.example/callback", "FATSECRET_CONSUMER_KEY": "unused", "FATSECRET_CONSUMER_SECRET": "unused", "TELEGRAM_BOT_TOKEN": "unused", "TELEGRAM_ALLOWED_USER_IDS": "42", "FITLOG_MCP_ENABLED": "false", "FITLOG_MCP_ALLOW_WRITES": "false", "FITLOG_MCP_CLIENT_ID": "fitlog-chatgpt", "FITLOG_MCP_CLIENT_SECRET": "", "FITLOG_MCP_LOGIN_TOKEN": "", "FITLOG_MCP_REDIRECT_URIS": ""} {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	require.NoError(t, err)
	require.False(t, cfg.MCPEnabled)
	require.False(t, cfg.MCPAllowWrites)
	t.Setenv("FITLOG_MCP_ENABLED", "true")
	t.Setenv("FITLOG_MCP_ALLOW_WRITES", "true")
	t.Setenv("FITLOG_MCP_REDIRECT_URIS", "https://chatgpt.com/cb,https://chatgpt.com/cb2")
	cfg, err = Load()
	require.NoError(t, err)
	require.True(t, cfg.MCPEnabled)
	require.True(t, cfg.MCPAllowWrites)
	require.Len(t, cfg.MCPRedirectURIs, 2)
}
