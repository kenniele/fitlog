package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"fitlog/internal/auth"
	"fitlog/internal/bot"
	"fitlog/internal/config"
	"fitlog/internal/fatsecret"
	"fitlog/internal/observability"
	"fitlog/internal/obsidian"
	"fitlog/internal/server"
	"fitlog/internal/storage"
	"fitlog/internal/training"
	"fitlog/internal/whoop"
)

func main() {
	root := &cobra.Command{
		Use:   "fitlog",
		Short: "Personal Telegram assistant for health, nutrition, and reading",
	}
	root.AddCommand(serverCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Run the Telegram bot + OAuth callback HTTP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context())
		},
	}
}

func run(parent context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := observability.NewLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	loc, err := cfg.Location()
	if err != nil {
		return err
	}
	publicBaseURL, err := cfg.BaseURL()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// DB
	pool, err := storage.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()

	// Crypto + token store
	cipher, err := auth.NewCipherFromBase64(cfg.TokenEncryptionKey)
	if err != nil {
		return fmt.Errorf("cipher: %w", err)
	}
	articleCipher, err := auth.NewDerivedCipherFromBase64(cfg.TokenEncryptionKey, "obsidian-article-links")
	if err != nil {
		return fmt.Errorf("article cipher: %w", err)
	}
	tokenStore := auth.NewTokenStore(storage.NewTokensRepo(pool), cipher)

	// Whoop OAuth config
	oauthCfg := whoop.NewOAuthConfig(cfg.WhoopClientID, cfg.WhoopClientSecret, cfg.WhoopRedirectURI, nil)

	// FatSecret
	fsProvider := fatsecret.NewTokenProvider(tokenStore, cfg.FatSecretConsumerKey, cfg.FatSecretConsumerSecret,
		cfg.FatSecretAccessToken, cfg.FatSecretAccessSecret)
	fsReports := fatsecret.NewUseCase(fsProvider, loc, fatsecret.ReportOptions{EstimatedTDEE: cfg.NutritionEstimatedTDEE})
	articleReports := obsidian.NewUseCase(cfg.ObsidianArticlesPath, articleCipher)

	// OAuth state store
	states := server.NewStateStore()
	fsOAuthClient := fatsecret.NewOAuthClient(cfg.FatSecretConsumerKey, cfg.FatSecretConsumerSecret,
		publicBaseURL+"/oauth/fatsecret/callback", nil)
	fsOAuth := server.NewFatSecretOAuth(fsOAuthClient, tokenStore, logger)

	// Bot
	allowlist := bot.NewAllowlist(cfg.TelegramAllowedUserIDs, logger)
	whoopProvider := whoop.NewOAuthProvider(tokenStore, oauthCfg, logger)
	whoopReports := whoop.NewUseCase(whoopProvider, loc)
	trainingReports := training.NewUseCase(storage.NewTrainingRepo(pool))
	deps := bot.Deps{
		Whoop:            whoopReports,
		FatSecret:        fsReports,
		Articles:         articleReports,
		PublicBaseURL:    publicBaseURL,
		OAuthConfig:      oauthCfg,
		States:           states,
		FatSecretAuth:    fsOAuth,
		Training:         trainingReports,
		WorkoutChannelID: cfg.TelegramWorkoutChannelID,
		Location:         loc,
		Logger:           logger,
	}
	tb, err := bot.New(cfg.TelegramBotToken, allowlist, deps) //nolint:contextcheck // telebot HandlerFunc has no inheritable context.Context; handlers create their own.
	if err != nil {
		return fmt.Errorf("bot: %w", err)
	}
	fsOAuth.SetNotifier(tb)

	// HTTP server (OAuth callback + healthz). Bot doubles as Notifier.
	cb := server.NewCallbackHandler(oauthCfg, states, tokenStore, tb, logger)
	articleHandler := obsidian.NewHandler(articleReports, logger)
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Router(cb, fsOAuth, pool, articleHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start HTTP server.
	httpErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErr <- err
		}
		close(httpErr)
	}()

	// Start bot (blocks until ctx cancel).
	botDone := make(chan struct{})
	go func() {
		defer close(botDone)
		logger.Info("telebot starting")
		tb.Start(ctx)
		logger.Info("telebot stopped")
	}()

	// Wait for shutdown signal or fatal HTTP error.
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-httpErr:
		if err != nil {
			logger.Error("http server failed", "err", err)
		}
		cancel()
	}

	// Bot stops on ctx cancel; wait for it.
	<-botDone

	// HTTP graceful shutdown.
	// Graceful shutdown needs a fresh ctx: the parent is already cancelled here.
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil { //nolint:contextcheck // intentional fresh ctx, see above.
		logger.Warn("http shutdown", "err", err)
	}

	logger.Info("bye")
	return nil
}
