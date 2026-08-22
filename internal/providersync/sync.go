package providersync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"fitlog/internal/fatsecret"
	"fitlog/internal/whoop"
)

const defaultTimeout = 5 * time.Minute

// Gate serializes WHOOP operations. WHOOP refresh tokens rotate, so two
// clients must never try to refresh the same stored token concurrently.
type Gate struct {
	permit chan struct{}
}

func NewGate() *Gate {
	permit := make(chan struct{}, 1)
	permit <- struct{}{}
	return &Gate{permit: permit}
}

func (g *Gate) Do(ctx context.Context, operation func() error) error {
	if g == nil {
		return errors.New("provider sync gate is required")
	}
	if operation == nil {
		return errors.New("provider sync operation is required")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.permit:
	}
	defer func() { g.permit <- struct{}{} }()
	return operation()
}

type serializedWhoopReports struct {
	gate *Gate
	next whoop.ReportUseCase
}

// SerializeWhoopReports makes Telegram WHOOP reports share the same refresh
// token gate as the background synchronizer.
func SerializeWhoopReports(gate *Gate, next whoop.ReportUseCase) whoop.ReportUseCase {
	return &serializedWhoopReports{gate: gate, next: next}
}

func (s *serializedWhoopReports) Fetch(
	ctx context.Context,
	request whoop.ReportRequest,
) (fetched whoop.FetchedReport, err error) {
	err = s.gate.Do(ctx, func() error {
		fetched, err = s.next.Fetch(ctx, request)
		return err
	})
	return fetched, err
}

func (s *serializedWhoopReports) Transform(fetched whoop.FetchedReport) whoop.Report {
	return s.next.Transform(fetched)
}

func (s *serializedWhoopReports) Format(report whoop.Report) string {
	return s.next.Format(report)
}

func (s *serializedWhoopReports) Execute(
	ctx context.Context,
	request whoop.ReportRequest,
) (report string, err error) {
	err = s.gate.Do(ctx, func() error {
		report, err = s.next.Execute(ctx, request)
		return err
	})
	return report, err
}

type Config struct {
	OwnerID             int64
	Location            *time.Location
	Interval            time.Duration
	LookbackDays        int
	FatSecretAuthorized bool
	ProviderCallTimeout time.Duration
	Now                 func() time.Time
}

type Sink interface {
	whoop.HealthBackfillSink
	fatsecret.NutritionBackfillSink
}

type Result struct {
	Whoop            whoop.BackfillResult
	WhoopError       error
	FatSecret        fatsecret.BackfillResult
	FatSecretError   error
	FatSecretSkipped bool
}

type Worker struct {
	config          Config
	whoopProvider   whoop.Provider
	fatsecretSource fatsecret.NutritionBackfillSource
	sink            Sink
	whoopGate       *Gate
	logger          *slog.Logger
}

func NewWorker(
	config Config,
	whoopProvider whoop.Provider,
	fatsecretSource fatsecret.NutritionBackfillSource,
	sink Sink,
	whoopGate *Gate,
	logger *slog.Logger,
) (*Worker, error) {
	if config.OwnerID <= 0 {
		return nil, errors.New("provider sync owner ID must be positive")
	}
	if config.Interval < 0 {
		return nil, errors.New("provider sync interval cannot be negative")
	}
	if config.LookbackDays < 1 || config.LookbackDays > whoop.MaxBackfillDays {
		return nil, fmt.Errorf("provider sync lookback must be between 1 and %d days", whoop.MaxBackfillDays)
	}
	if whoopProvider == nil {
		return nil, errors.New("WHOOP provider is required")
	}
	if config.FatSecretAuthorized && fatsecretSource == nil {
		return nil, errors.New("FatSecret source is required when automatic storage is authorized")
	}
	if sink == nil {
		return nil, errors.New("provider sync sink is required")
	}
	if whoopGate == nil {
		return nil, errors.New("WHOOP sync gate is required")
	}
	if config.Location == nil {
		config.Location = time.UTC
	}
	if config.ProviderCallTimeout <= 0 {
		config.ProviderCallTimeout = defaultTimeout
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		config: config, whoopProvider: whoopProvider, fatsecretSource: fatsecretSource,
		sink: sink, whoopGate: whoopGate, logger: logger,
	}, nil
}

// Run synchronizes immediately and then on every configured interval until
// the application context is cancelled.
func (w *Worker) Run(ctx context.Context) {
	if w.config.Interval == 0 {
		w.logger.Info("automatic provider sync disabled")
		return
	}
	if !w.config.FatSecretAuthorized {
		w.logger.Info("automatic FatSecret sync disabled", "reason", "persistent storage not authorized")
	}

	w.syncAndLog(ctx)
	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.syncAndLog(ctx)
		}
	}
}

// SyncOnce refreshes the current local day and a short correction window. A
// failure in one provider does not prevent the other provider from running.
func (w *Worker) SyncOnce(ctx context.Context) Result {
	now := w.config.Now().In(w.config.Location)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, w.config.Location)
	from := to.AddDate(0, 0, -(w.config.LookbackDays - 1))
	result := Result{FatSecretSkipped: !w.config.FatSecretAuthorized}

	whoopCtx, cancelWhoop := context.WithTimeout(ctx, w.config.ProviderCallTimeout)
	result.WhoopError = w.whoopGate.Do(whoopCtx, func() error {
		client, err := w.whoopProvider.Client(whoopCtx)
		if err != nil {
			return err
		}
		result.Whoop, err = whoop.BackfillHealth(whoopCtx, client, w.sink, w.config.OwnerID, whoop.BackfillOptions{
			From: from, To: to, Location: w.config.Location,
		})
		return err
	})
	cancelWhoop()

	if result.FatSecretSkipped {
		return result
	}
	if ctx.Err() != nil {
		result.FatSecretError = ctx.Err()
		return result
	}
	fatsecretCtx, cancelFatSecret := context.WithTimeout(ctx, w.config.ProviderCallTimeout)
	result.FatSecret, result.FatSecretError = fatsecret.BackfillNutritionDays(
		fatsecretCtx, w.fatsecretSource, w.sink, w.config.OwnerID, fatsecret.BackfillOptions{
			From: from, To: to, Location: w.config.Location, StorageAuthorized: true,
		},
	)
	cancelFatSecret()
	return result
}

func (w *Worker) syncAndLog(ctx context.Context) {
	started := time.Now()
	result := w.SyncOnce(ctx)
	if result.WhoopError != nil {
		if !errors.Is(result.WhoopError, context.Canceled) {
			w.logger.Warn("automatic WHOOP sync failed", "err", result.WhoopError)
		}
	} else {
		w.logger.Info("automatic WHOOP sync completed",
			"from", result.Whoop.From, "to", result.Whoop.To,
			"recovery_rows", result.Whoop.RecoveryRows, "sleep_rows", result.Whoop.SleepRows)
	}
	if result.FatSecretError != nil {
		if !errors.Is(result.FatSecretError, context.Canceled) {
			w.logger.Warn("automatic FatSecret sync failed", "err", result.FatSecretError)
		}
	} else if !result.FatSecretSkipped {
		w.logger.Info("automatic FatSecret sync completed",
			"from", result.FatSecret.From, "to", result.FatSecret.To,
			"nutrition_rows", result.FatSecret.UpsertedDays)
	}
	w.logger.Debug("automatic provider sync cycle finished", "duration", time.Since(started))
}
