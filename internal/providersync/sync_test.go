package providersync

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"fitlog/internal/domain"
	"fitlog/internal/fatsecret"
	"fitlog/internal/whoop"
)

type fakeWhoopProvider struct {
	api   whoop.API
	err   error
	calls int
}

func (p *fakeWhoopProvider) Client(context.Context) (whoop.API, error) {
	p.calls++
	return p.api, p.err
}

type fakeWhoopAPI struct {
	mu     sync.Mutex
	ranges []domain.TimeRange
}

func (a *fakeWhoopAPI) record(rng domain.TimeRange) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ranges = append(a.ranges, rng)
}

func (a *fakeWhoopAPI) Cycles(_ context.Context, rng domain.TimeRange, _ int) ([]domain.Cycle, error) {
	a.record(rng)
	return nil, nil
}

func (a *fakeWhoopAPI) Recoveries(_ context.Context, rng domain.TimeRange, _ int) ([]domain.Recovery, error) {
	a.record(rng)
	return nil, nil
}

func (a *fakeWhoopAPI) Sleeps(_ context.Context, rng domain.TimeRange, _ int) ([]domain.Sleep, error) {
	a.record(rng)
	return nil, nil
}

func (a *fakeWhoopAPI) Workouts(_ context.Context, rng domain.TimeRange, _ int) ([]domain.Workout, error) {
	a.record(rng)
	return nil, nil
}

type fakeFatSecretSource struct {
	calls int
	rows  []domain.DailyNutrition
	err   error
}

func (s *fakeFatSecretSource) FoodEntriesMonth(context.Context, time.Time) ([]domain.DailyNutrition, error) {
	s.calls++
	return s.rows, s.err
}

type fakeSink struct {
	whoopCalls     int
	fatSecretCalls int
	ownerID        int64
	nutritionRows  []domain.NutritionDaySnapshot
	onWhoop        func()
}

func (s *fakeSink) UpsertWhoopHealth(
	_ context.Context,
	ownerID int64,
	_ []domain.WhoopRecoverySnapshot,
	_ []domain.WhoopSleepSnapshot,
) error {
	s.whoopCalls++
	s.ownerID = ownerID
	if s.onWhoop != nil {
		s.onWhoop()
	}
	return nil
}

func (s *fakeSink) UpsertFatSecretNutritionDays(
	_ context.Context,
	ownerID int64,
	rows []domain.NutritionDaySnapshot,
) error {
	s.fatSecretCalls++
	s.ownerID = ownerID
	s.nutritionRows = append([]domain.NutritionDaySnapshot(nil), rows...)
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestWorkerSyncOnceRefreshesCorrectionWindow(t *testing.T) {
	loc := time.FixedZone("test", 3*60*60)
	now := time.Date(2026, 8, 22, 8, 40, 0, 0, loc)
	api := &fakeWhoopAPI{}
	whoopProvider := &fakeWhoopProvider{api: api}
	fatSource := &fakeFatSecretSource{rows: []domain.DailyNutrition{{
		DateInt: fatsecret.ToDateInt(now), Calories: 2_500, Protein: 180,
	}}}
	sink := &fakeSink{}
	worker, err := NewWorker(Config{
		OwnerID: 42, Location: loc, Interval: time.Hour, LookbackDays: 3,
		FatSecretAuthorized: true, Now: func() time.Time { return now },
	}, whoopProvider, fatSource, sink, NewGate(), testLogger())
	if err != nil {
		t.Fatal(err)
	}

	result := worker.SyncOnce(context.Background())
	if result.WhoopError != nil || result.FatSecretError != nil {
		t.Fatalf("sync errors: WHOOP=%v FatSecret=%v", result.WhoopError, result.FatSecretError)
	}
	if result.Whoop.From != "2026-08-20" || result.Whoop.To != "2026-08-22" || result.Whoop.RequestedDays != 3 {
		t.Fatalf("WHOOP range = %+v", result.Whoop)
	}
	if result.FatSecret.From != "2026-08-20" || result.FatSecret.To != "2026-08-22" || result.FatSecret.RequestedDays != 3 {
		t.Fatalf("FatSecret range = %+v", result.FatSecret)
	}
	if whoopProvider.calls != 1 || sink.whoopCalls != 1 || fatSource.calls != 1 || sink.fatSecretCalls != 1 {
		t.Fatalf("calls: provider=%d WHOOP sink=%d FatSecret source=%d sink=%d",
			whoopProvider.calls, sink.whoopCalls, fatSource.calls, sink.fatSecretCalls)
	}
	if sink.ownerID != 42 || len(sink.nutritionRows) != 1 || result.FatSecret.UpsertedDays != 1 {
		t.Fatalf("persisted owner=%d rows=%d result=%+v", sink.ownerID, len(sink.nutritionRows), result.FatSecret)
	}
	if len(api.ranges) != 3 {
		t.Fatalf("WHOOP collection calls = %d, want 3", len(api.ranges))
	}
	wantFrom := time.Date(2026, 8, 19, 0, 0, 0, 0, loc)
	wantTo := time.Date(2026, 8, 23, 0, 0, 0, 0, loc)
	for _, rng := range api.ranges {
		if !rng.From.Equal(wantFrom) || !rng.To.Equal(wantTo) {
			t.Fatalf("WHOOP provider range = %s..%s, want %s..%s", rng.From, rng.To, wantFrom, wantTo)
		}
	}
}

func TestWorkerSkipsFatSecretWithoutStorageAuthorization(t *testing.T) {
	sink := &fakeSink{}
	provider := &fakeWhoopProvider{api: &fakeWhoopAPI{}}
	worker, err := NewWorker(Config{
		OwnerID: 42, Interval: time.Hour, LookbackDays: 3,
	}, provider, nil, sink, NewGate(), testLogger())
	if err != nil {
		t.Fatal(err)
	}

	result := worker.SyncOnce(context.Background())
	if !result.FatSecretSkipped || sink.fatSecretCalls != 0 || sink.whoopCalls != 1 {
		t.Fatalf("unexpected result=%+v WHOOP calls=%d FatSecret calls=%d", result, sink.whoopCalls, sink.fatSecretCalls)
	}
}

func TestWorkerContinuesWithFatSecretAfterWhoopFailure(t *testing.T) {
	whoopFailure := errors.New("WHOOP unavailable")
	provider := &fakeWhoopProvider{err: whoopFailure}
	fatSource := &fakeFatSecretSource{}
	sink := &fakeSink{}
	worker, err := NewWorker(Config{
		OwnerID: 42, Interval: time.Hour, LookbackDays: 3, FatSecretAuthorized: true,
	}, provider, fatSource, sink, NewGate(), testLogger())
	if err != nil {
		t.Fatal(err)
	}

	result := worker.SyncOnce(context.Background())
	if !errors.Is(result.WhoopError, whoopFailure) {
		t.Fatalf("WHOOP error = %v", result.WhoopError)
	}
	if result.FatSecretError != nil || fatSource.calls != 1 || sink.fatSecretCalls != 1 {
		t.Fatalf("FatSecret result=%+v source calls=%d sink calls=%d", result.FatSecret, fatSource.calls, sink.fatSecretCalls)
	}
}

func TestWorkerRunSynchronizesImmediatelyAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sink := &fakeSink{onWhoop: cancel}
	worker, err := NewWorker(Config{
		OwnerID: 42, Interval: time.Hour, LookbackDays: 1,
	}, &fakeWhoopProvider{api: &fakeWhoopAPI{}}, nil, sink, NewGate(), testLogger())
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	if sink.whoopCalls != 1 {
		t.Fatalf("immediate WHOOP sync calls = %d, want 1", sink.whoopCalls)
	}
}

type fakeWhoopReports struct {
	executeCalls int
}

func (r *fakeWhoopReports) Fetch(context.Context, whoop.ReportRequest) (whoop.FetchedReport, error) {
	return whoop.FetchedReport{}, nil
}

func (r *fakeWhoopReports) Transform(whoop.FetchedReport) whoop.Report { return whoop.Report{} }
func (r *fakeWhoopReports) Format(whoop.Report) string                 { return "report" }

func (r *fakeWhoopReports) Execute(context.Context, whoop.ReportRequest) (string, error) {
	r.executeCalls++
	return "report", nil
}

func TestSerializeWhoopReportsUsesSharedGate(t *testing.T) {
	gate := NewGate()
	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = gate.Do(context.Background(), func() error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held

	next := &fakeWhoopReports{}
	serialized := SerializeWhoopReports(gate, next)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := serialized.Execute(cancelled, whoop.ReportRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want context canceled", err)
	}
	if next.executeCalls != 0 {
		t.Fatalf("underlying report executed while gate was held: %d", next.executeCalls)
	}
	close(release)
	<-done

	if _, err := serialized.Execute(context.Background(), whoop.ReportRequest{}); err != nil {
		t.Fatal(err)
	}
	if next.executeCalls != 1 {
		t.Fatalf("underlying report calls = %d, want 1", next.executeCalls)
	}
}
