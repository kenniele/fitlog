package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config holds all runtime configuration. Anything secret comes from env;
// nothing is hard-coded.
type Config struct {
	DatabaseURL        string `env:"DATABASE_URL,required"`
	TokenEncryptionKey string `env:"FITLOG_TOKEN_ENCRYPTION_KEY,required"`

	WhoopClientID     string `env:"WHOOP_CLIENT_ID,required"`
	WhoopClientSecret string `env:"WHOOP_CLIENT_SECRET,required"`
	WhoopRedirectURI  string `env:"WHOOP_REDIRECT_URI,required"`

	FatSecretConsumerKey    string  `env:"FATSECRET_CONSUMER_KEY,required"`
	FatSecretConsumerSecret string  `env:"FATSECRET_CONSUMER_SECRET,required"`
	FatSecretAccessToken    string  `env:"FATSECRET_ACCESS_TOKEN"`
	FatSecretAccessSecret   string  `env:"FATSECRET_ACCESS_SECRET"`
	NutritionEstimatedTDEE  float64 `env:"NUTRITION_ESTIMATED_TDEE"`

	TelegramBotToken          string  `env:"TELEGRAM_BOT_TOKEN,required"`
	TelegramAllowedUserIDs    []int64 `env:"TELEGRAM_ALLOWED_USER_IDS,required" envSeparator:","`
	TelegramWorkoutChannelID  int64   `env:"TELEGRAM_WORKOUT_CHANNEL_ID"`
	TelegramWorkoutChannelIDs []int64 `env:"TELEGRAM_WORKOUT_CHANNEL_IDS" envSeparator:","`

	LogLevel   string `env:"LOG_LEVEL" envDefault:"info"`
	HTTPAddr   string `env:"HTTP_ADDR" envDefault:":8080"`
	TZLocation string `env:"TZ_LOCATION" envDefault:"Europe/Moscow"`

	ObsidianArticlesPath string `env:"OBSIDIAN_ARTICLES_PATH"`
	PublicBaseURL        string `env:"PUBLIC_BASE_URL"`
}

// BaseURL returns the public origin used in links sent to Telegram. When an
// explicit value is absent, the already-required Whoop redirect URI provides
// the same externally reachable scheme and host.
func (c *Config) BaseURL() (string, error) {
	raw := strings.TrimSpace(c.PublicBaseURL)
	if raw == "" {
		raw = c.WhoopRedirectURI
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid public base URL %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("public base URL must use http or https")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

// Location returns the parsed time.Location for the configured zone.
func (c *Config) Location() (*time.Location, error) {
	loc, err := time.LoadLocation(c.TZLocation)
	if err != nil {
		return nil, fmt.Errorf("load tz %q: %w", c.TZLocation, err)
	}
	return loc, nil
}

// WorkoutChannels returns the configured publishing destinations without
// duplicates. The singular value remains supported for existing deployments.
func (c *Config) WorkoutChannels() []int64 {
	seen := make(map[int64]struct{}, len(c.TelegramWorkoutChannelIDs)+1)
	channels := make([]int64, 0, len(c.TelegramWorkoutChannelIDs)+1)
	appendChannel := func(id int64) {
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		channels = append(channels, id)
	}
	appendChannel(c.TelegramWorkoutChannelID)
	for _, id := range c.TelegramWorkoutChannelIDs {
		appendChannel(id)
	}
	return channels
}

// Load parses environment variables into Config. If a .env file exists in
// the working directory it is loaded first (dev convenience; production
// should use real env vars).
func Load() (*Config, error) {
	_ = godotenv.Load() // silent: missing .env is fine in production

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	return &cfg, nil
}
