package controlcenter

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestCompactSummaryRequiresSessionAndDoesNotCacheHealthData(t *testing.T) {
	handler := NewHandler(&handlerStore{}, 7, "owner-secret", time.UTC)
	response := serve(handler, "GET", "/dashboard/summary", "", nil, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous summary: %d", response.Code)
	}
	cookie := login(t, handler, "owner-secret")
	response = serve(handler, "GET", "/dashboard/summary?owner_id=999", "", cookie, "")
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("summary: %d, cache=%q", response.Code, response.Header().Get("Cache-Control"))
	}
}

func TestCompactSnapshotKeepsIndependentDatesMissingValuesAndWeeklyTotals(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Date(2026, 10, 5, 22, 30, 0, 0, time.UTC) // Oct 6 locally
	sleepSeconds, zeroSeconds := int64(27000), int64(0)
	store := &handlerStore{
		overviewFn: func(_ context.Context, ownerID int64, rng DateRange, zone *time.Location) (Overview, error) {
			if ownerID != 7 || zone.String() != "Europe/Moscow" || rng.To.Format("2006-01-02") != "2026-10-06" || rng.Days() != 30 {
				t.Fatalf("wrong owner/range: %d %#v %s", ownerID, rng, zone)
			}
			return Overview{Daily: []DailyPoint{
				{Date: "2026-09-29", WorkoutCount: 9, TrainingVolumeKG: 999},
				{Date: "2026-10-01", WorkoutCount: 1, WorkingSets: 3, TrainingVolumeKG: 600, RIRSum: 6, RIRSamples: 3},
				{Date: "2026-10-05", RecoveryScore: pointer(76.0), HRVMs: pointer(64.0), SleepSeconds: &sleepSeconds, CaloriesKcal: pointer(0.0), WorkoutCount: 1, WorkingSets: 1, RIRSum: 4, RIRSamples: 1},
				{Date: "2026-10-06", RecoveryScore: pointer(70.0)},
				{Date: "2026-10-07", RecoveryScore: pointer(100.0), SleepSeconds: &zeroSeconds, WorkoutCount: 9},
			}}, nil
		},
		listFn: func(_ context.Context, ownerID int64, resource string, p Pagination, zone *time.Location) (ListResult, error) {
			if ownerID != 7 || resource != "body-measurements" || p.PageSize != 1 || p.From != nil || p.To.Format("2006-01-02") != "2026-10-06" {
				t.Fatal("wrong latest-body scope")
			}
			return ListResult{Items: []json.RawMessage{json.RawMessage(`{"measured_at":"2026-08-01T10:00:00Z","weight_kg":80}`)}}, nil
		},
	}
	service := NewService(store, 7, loc)
	service.now = func() time.Time { return now }
	snapshot, err := service.CompactSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Date != "2026-10-06" || snapshot.Recovery.Date != "2026-10-06" || snapshot.Recovery.HRVMs != nil || snapshot.Sleep.Date != "2026-10-05" || snapshot.Nutrition.Date != "2026-10-05" || *snapshot.Nutrition.CaloriesKcal != 0 {
		t.Fatalf("mixed dates/missing: %#v", snapshot)
	}
	if snapshot.Training.Sessions != 2 || snapshot.Training.VolumeKG != 600 || *snapshot.Training.AverageRIR != 2.5 {
		t.Fatalf("weekly totals: %#v", snapshot.Training)
	}
	if snapshot.NextWorkout != nil || snapshot.Sources == nil {
		t.Fatal("invalid empty state")
	}
	if snapshot.Body == nil || *snapshot.Body.WeightKG != 80 || snapshot.Body.MeasuredAt.Month() != time.August {
		t.Fatalf("older body measurement missing: %#v", snapshot.Body)
	}
}

type compactWorkoutTestStore struct {
	handlerStore
	lookup func(int64, time.Time) (*CompactWorkout, error)
}

func (s *compactWorkoutTestStore) NextCompactWorkout(_ context.Context, ownerID int64, from time.Time) (*CompactWorkout, error) {
	return s.lookup(ownerID, from)
}

func TestCompactSnapshotRequestsUpcomingWorkoutFromCurrentInstant(t *testing.T) {
	now := time.Date(2026, 10, 6, 15, 0, 0, 0, time.UTC)
	scheduled := now.Add(time.Hour)
	store := &compactWorkoutTestStore{lookup: func(ownerID int64, from time.Time) (*CompactWorkout, error) {
		if ownerID != 7 || !from.Equal(now) {
			t.Fatalf("wrong upcoming scope: owner=%d from=%s", ownerID, from)
		}
		return &CompactWorkout{ID: 1, Title: "Upper", ScheduledAt: &scheduled}, nil
	}}
	service := NewService(store, 7, time.UTC)
	service.now = func() time.Time { return now }
	snapshot, err := service.CompactSnapshot(context.Background())
	if err != nil || snapshot.NextWorkout == nil || snapshot.NextWorkout.Title != "Upper" {
		t.Fatalf("upcoming: %#v %v", snapshot.NextWorkout, err)
	}
}
