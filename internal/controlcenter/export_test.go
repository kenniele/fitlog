package controlcenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSessionJSONExportIncludesEveryPageAndPreservesFilters(t *testing.T) {
	loc := time.FixedZone("export-zone", 3*60*60)
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, loc)
	to := from.AddDate(0, 1, -1)
	var pages int
	service := NewService(&handlerStore{listFn: func(
		_ context.Context, ownerID int64, resource string, filters Pagination, gotLoc *time.Location,
	) (ListResult, error) {
		pages++
		if ownerID != 42 || resource != "workout-sessions" || gotLoc != loc {
			t.Fatalf("wrong scope: %d %s %v", ownerID, resource, gotLoc)
		}
		if filters.Page != pages || filters.PageSize != MaxPageSize || filters.From != &from || filters.To != &to ||
			filters.Search != "жим" || filters.Filters["status"] != "finished" || filters.Filters["date_basis"] != "calendar" {
			t.Fatalf("filters lost: %+v", filters)
		}
		items := make([]json.RawMessage, 0)
		for id := (pages-1)*MaxPageSize + 1; id <= min(pages*MaxPageSize, MaxPageSize+1); id++ {
			items = append(items, json.RawMessage(fmt.Sprintf(`{"id":%d,"exercises":[{"sets":[{"type":"warmup","weight_kg":null,"rir":0}]}]}`, id)))
		}
		return ListResult{Items: items, Total: MaxPageSize + 1}, nil
	}}, 42, loc)
	content, err := service.ExportSessions(context.Background(), Pagination{
		Page: 9, PageSize: 1, From: &from, To: &to, Search: "жим",
		Filters: map[string]string{"status": "finished", "date_basis": "calendar"},
	}, "json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Version         int               `json:"version"`
		Timezone        string            `json:"timezone"`
		WorkoutSessions []json.RawMessage `json:"workout_sessions"`
	}
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	if pages != 2 || len(document.WorkoutSessions) != MaxPageSize+1 || document.Version != 1 || document.Timezone != loc.String() {
		t.Fatalf("incomplete export: pages=%d document=%+v", pages, document)
	}
	if !strings.Contains(string(document.WorkoutSessions[0]), `"weight_kg": null`) ||
		!strings.Contains(string(document.WorkoutSessions[0]), `"rir": 0`) {
		t.Fatalf("null/zero values changed: %s", document.WorkoutSessions[0])
	}
}

func TestSessionExportEmptyAndFailures(t *testing.T) {
	service := NewService(&handlerStore{}, 42, time.UTC)
	content, err := service.ExportSessions(context.Background(), Pagination{}, "json")
	if err != nil || !strings.Contains(string(content), `"workout_sessions": []`) {
		t.Fatalf("empty JSON export = %s, %v", content, err)
	}
	if _, err := service.ExportSessions(context.Background(), Pagination{}, "xml"); err == nil {
		t.Fatal("invalid format accepted")
	}
	failure := errors.New("storage unavailable")
	service.store = &handlerStore{listFn: func(context.Context, int64, string, Pagination, *time.Location) (ListResult, error) {
		return ListResult{}, failure
	}}
	if content, err := service.ExportSessions(context.Background(), Pagination{}, "json"); content != nil || !errors.Is(err, failure) {
		t.Fatalf("storage error: content=%s err=%v", content, err)
	}
}

func TestHandlerSessionExports(t *testing.T) {
	for _, format := range []string{"csv", "json"} {
		t.Run(format, func(t *testing.T) {
			calls := 0
			check := func(ownerID int64, filters Pagination) {
				calls++
				if ownerID != 7 || filters.From != nil || filters.To != nil || filters.Filters["status"] != "finished" {
					t.Fatalf("all-time scope: owner=%d filters=%+v", ownerID, filters)
				}
			}
			store := &handlerStore{
				listFn: func(_ context.Context, owner int64, _ string, filters Pagination, _ *time.Location) (ListResult, error) {
					check(owner, filters)
					return ListResult{Items: []json.RawMessage{}}, nil
				},
				exportSessionsCSVFn: func(_ context.Context, owner int64, dates DateRange, filters Pagination, _ *time.Location) ([]byte, error) {
					check(owner, filters)
					if !dates.From.IsZero() || !dates.To.IsZero() {
						t.Fatal("CSV export has date bounds")
					}
					return []byte("session_id\n"), nil
				},
			}
			handler := NewHandler(store, 7, "token", time.UTC)
			cookie := login(t, handler, "token")
			path := "/workout-sessions/export." + format + "?scope=all&status=finished"
			if response := serve(handler, http.MethodGet, path, "", nil, ""); response.Code != http.StatusUnauthorized {
				t.Fatalf("unprotected export: %d", response.Code)
			}
			for _, path := range []string{path, "/export?type=training&format=" + format + "&scope=all&status=finished"} {
				response := serve(handler, http.MethodGet, path, "", cookie, "")
				if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Disposition"), "."+format+`"`) {
					t.Fatalf("export failed: %d %s", response.Code, response.Body.String())
				}
			}
			if calls != 2 {
				t.Fatalf("export calls = %d", calls)
			}
			for _, query := range []string{"scope=invalid", "from=wrong", "from=2026-09-09&to=2026-09-01"} {
				response := serve(handler, http.MethodGet, "/workout-sessions/export."+format+"?"+query, "", cookie, "")
				if response.Code != http.StatusUnprocessableEntity {
					t.Fatalf("invalid %s accepted: %d", query, response.Code)
				}
			}
		})
	}
}
