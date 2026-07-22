package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
