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
	"fitlog/internal/fatsecret"
	"fitlog/internal/observability"
	"fitlog/internal/storage"
)

const fatSecretStorageGuide = "https://platform.fatsecret.com/docs/guides/storable-data"

func fatSecretBackfillCmd() *cobra.Command {
	var (
		days              int
		fromRaw           string
		toRaw             string
		dryRun            bool
		storageAuthorized bool
	)
	command := &cobra.Command{
		Use:   "fatsecret-backfill",
		Short: "Backfill daily FatSecret nutrition totals into Control Center",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !dryRun && !storageAuthorized {
				return fmt.Errorf("FatSecret allows long-term storage of diary nutrients only with separate authorization; use --dry-run or pass --storage-authorized only when that right is confirmed (%s)", fatSecretStorageGuide)
			}
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			loc, err := cfg.Location()
			if err != nil {
				return err
			}
			from, to, err := resolveFatSecretBackfillRange(time.Now(), loc, days, fromRaw, toRaw)
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
			provider := fatsecret.NewTokenProvider(
				tokens, cfg.FatSecretConsumerKey, cfg.FatSecretConsumerSecret,
				cfg.FatSecretAccessToken, cfg.FatSecretAccessSecret, logger,
			)
			result, err := fatsecret.BackfillNutritionDays(
				cmd.Context(), provider, controlcenter.NewRepository(pool), ownerID,
				fatsecret.BackfillOptions{
					From: from, To: to, Location: loc, DryRun: dryRun,
					StorageAuthorized: storageAuthorized,
				},
			)
			if err != nil {
				if errors.Is(err, fatsecret.ErrNotConnected) {
					return errors.New("FatSecret is not connected: authorize it with /connect_fatsecret or configure the legacy access token and secret")
				}
				return fmt.Errorf("FatSecret backfill: %w", err)
			}
			latest := result.LatestAvailableDate
			if latest == "" {
				latest = "none"
			}
			verb := "upserted"
			count := result.UpsertedDays
			if result.DryRun {
				verb = "would upsert"
				count = result.LoggedDays
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"FatSecret %s..%s: fetched %d months for %d completed days; %d logged days; latest %s; %s %d rows\n",
				result.From, result.To, result.RequestedMonths, result.RequestedDays,
				result.LoggedDays, latest, verb, count,
			)
			return nil
		},
	}
	command.Flags().IntVar(&days, "days", 100, "number of completed local calendar days (1-366)")
	command.Flags().StringVar(&fromRaw, "from", "", "inclusive start date in YYYY-MM-DD; requires --to")
	command.Flags().StringVar(&toRaw, "to", "", "inclusive end date in YYYY-MM-DD; requires --from")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "fetch and report without writing nutrition_days")
	command.Flags().BoolVar(&storageAuthorized, "storage-authorized", false, "confirm separate FatSecret authorization for persistent nutrient storage")
	return command
}

func resolveFatSecretBackfillRange(
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
		if days < 1 || days > fatsecret.MaxBackfillDays {
			return time.Time{}, time.Time{}, fmt.Errorf("--days must be between 1 and %d", fatsecret.MaxBackfillDays)
		}
		now = now.In(loc)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		to := today.AddDate(0, 0, -1)
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
	if count > fatsecret.MaxBackfillDays {
		return time.Time{}, time.Time{}, fmt.Errorf("explicit range must not exceed %d days", fatsecret.MaxBackfillDays)
	}
	return from, to, nil
}
