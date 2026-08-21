package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"fitlog/internal/auth"
	"fitlog/internal/config"
	"fitlog/internal/controlcenter"
	"fitlog/internal/observability"
	"fitlog/internal/storage"
	"fitlog/internal/whoop"
)

func whoopBackfillCmd() *cobra.Command {
	var (
		days    int
		fromRaw string
		toRaw   string
		dryRun  bool
	)
	command := &cobra.Command{
		Use:   "whoop-backfill",
		Short: "Backfill WHOOP recovery and sleep into Control Center",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			loc, err := cfg.Location()
			if err != nil {
				return err
			}
			from, to, err := resolveWhoopBackfillRange(time.Now(), loc, days, fromRaw, toRaw)
			if err != nil {
				return err
			}
			ownerID, err := cfg.DashboardOwner()
			if err != nil {
				return err
			}
			pool, err := storage.NewPool(cmd.Context(), cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("db: %w", err)
			}
			defer pool.Close()

			cipher, err := auth.NewCipherFromBase64(cfg.TokenEncryptionKey)
			if err != nil {
				return fmt.Errorf("cipher: %w", err)
			}
			logger := observability.NewLogger(cfg.LogLevel)
			tokens := auth.NewTokenStore(storage.NewTokensRepo(pool), cipher)
			oauthConfig := whoop.NewOAuthConfig(
				cfg.WhoopClientID, cfg.WhoopClientSecret, cfg.WhoopRedirectURI, nil,
			)
			provider := whoop.NewOAuthProvider(tokens, oauthConfig, logger)
			client, err := provider.Client(cmd.Context())
			if err != nil {
				if errors.Is(err, whoop.ErrNotConnected) {
					return errors.New("WHOOP is not connected: open the Telegram bot, press Здоровье🫀, and complete OAuth")
				}
				return fmt.Errorf("WHOOP client: %w", err)
			}
			result, err := whoop.BackfillHealth(
				cmd.Context(), client, controlcenter.NewRepository(pool), ownerID,
				whoop.BackfillOptions{From: from, To: to, Location: loc, DryRun: dryRun},
			)
			if err != nil {
				return fmt.Errorf("WHOOP backfill: %w", err)
			}
			verb := "upserted"
			if result.DryRun {
				verb = "would upsert"
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"WHOOP %s..%s: %d days; fetched %d cycles, %d recoveries, %d sleeps; %s %d recovery and %d sleep rows; unmatched recoveries %d\n",
				result.From, result.To, result.RequestedDays, result.FetchedCycles,
				result.FetchedRecoveries, result.FetchedSleeps, verb,
				result.RecoveryRows, result.SleepRows, result.UnmatchedRecovery,
			)
			return nil
		},
	}
	command.Flags().IntVar(&days, "days", 100, "number of local calendar days ending today (1-366)")
	command.Flags().StringVar(&fromRaw, "from", "", "inclusive start date in YYYY-MM-DD; requires --to")
	command.Flags().StringVar(&toRaw, "to", "", "inclusive end date in YYYY-MM-DD; requires --from")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "fetch and report without writing recovery/sleep rows")
	return command
}

func resolveWhoopBackfillRange(
	now time.Time,
	loc *time.Location,
	days int,
	fromRaw, toRaw string,
) (time.Time, time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	fromRaw = strings.TrimSpace(fromRaw)
	toRaw = strings.TrimSpace(toRaw)
	if (fromRaw == "") != (toRaw == "") {
		return time.Time{}, time.Time{}, errors.New("--from and --to must be provided together")
	}
	if fromRaw == "" {
		if days < 1 || days > whoop.MaxBackfillDays {
			return time.Time{}, time.Time{}, fmt.Errorf("--days must be between 1 and %d", whoop.MaxBackfillDays)
		}
		now = now.In(loc)
		to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		return to.AddDate(0, 0, -(days - 1)), to, nil
	}
	from, err := time.ParseInLocation("2006-01-02", fromRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid --from: use YYYY-MM-DD")
	}
	to, err := time.ParseInLocation("2006-01-02", toRaw, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid --to: use YYYY-MM-DD")
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, errors.New("--from must not be after --to")
	}
	count := 0
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		count++
	}
	if count > whoop.MaxBackfillDays {
		return time.Time{}, time.Time{}, fmt.Errorf("explicit range must not exceed %d days", whoop.MaxBackfillDays)
	}
	return from, to, nil
}
