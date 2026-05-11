package config

import (
	"fmt"
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

	FatSecretConsumerKey    string `env:"FATSECRET_CONSUMER_KEY,required"`
	FatSecretConsumerSecret string `env:"FATSECRET_CONSUMER_SECRET,required"`
	FatSecretAccessToken    string `env:"FATSECRET_ACCESS_TOKEN,required"`
	FatSecretAccessSecret   string `env:"FATSECRET_ACCESS_SECRET,required"`

	TelegramBotToken       string  `env:"TELEGRAM_BOT_TOKEN,required"`
	TelegramAllowedUserIDs []int64 `env:"TELEGRAM_ALLOWED_USER_IDS,required" envSeparator:","`

	LogLevel   string `env:"LOG_LEVEL" envDefault:"info"`
	HTTPAddr   string `env:"HTTP_ADDR" envDefault:":8080"`
	TZLocation string `env:"TZ_LOCATION" envDefault:"Europe/Moscow"`
}

// Location returns the parsed time.Location for the configured zone.
func (c *Config) Location() (*time.Location, error) {
	loc, err := time.LoadLocation(c.TZLocation)
	if err != nil {
		return nil, fmt.Errorf("load tz %q: %w", c.TZLocation, err)
	}
	return loc, nil
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
