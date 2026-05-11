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
	"golang.org/x/oauth2"

	"fitlog/internal/auth"
	"fitlog/internal/bot"
	"fitlog/internal/config"
	"fitlog/internal/fatsecret"
	"fitlog/internal/observability"
	"fitlog/internal/server"
	"fitlog/internal/storage"
	"fitlog/internal/whoop"
)

func main() {
	root := &cobra.Command{
		Use:   "fitlog",
		Short: "Personal fitness bot pulling Whoop + FatSecret on demand",
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
	tokenStore := auth.NewTokenStore(storage.NewTokensRepo(pool), cipher)

	// Whoop OAuth config
	oauthCfg := whoop.NewOAuthConfig(cfg.WhoopClientID, cfg.WhoopClientSecret, cfg.WhoopRedirectURI, nil)

	// FatSecret
	fsClient := fatsecret.NewClient(
		fatsecret.NewSigner(cfg.FatSecretConsumerKey, cfg.FatSecretConsumerSecret,
			cfg.FatSecretAccessToken, cfg.FatSecretAccessSecret),
		fatsecret.Options{},
	)

	// OAuth state store
	states := server.NewStateStore()

	// Bot
	allowlist := bot.NewAllowlist(cfg.TelegramAllowedUserIDs, logger)
	deps := bot.Deps{
		Tokens:      tokenStore,
		OAuthConfig: oauthCfg,
		FatSecret:   fsClient,
		States:      states,
		Location:    loc,
		Logger:      logger,
		NewWhoopClient: func(ctx context.Context, ts oauth2.TokenSource) *whoop.Client {
			return whoop.NewClientWithTokenSource(ctx, ts, whoop.Options{})
		},
	}
	tb, err := bot.New(cfg.TelegramBotToken, allowlist, deps)
	if err != nil {
		return fmt.Errorf("bot: %w", err)
	}

	// HTTP server (OAuth callback + healthz). Bot doubles as Notifier.
	cb := server.NewCallbackHandler(oauthCfg, states, tokenStore, tb, logger)
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Router(cb),
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
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		logger.Warn("http shutdown", "err", err)
	}

	logger.Info("bye")
	return nil
}
