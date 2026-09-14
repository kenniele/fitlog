//go:build integration

package mcpauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"fitlog/internal/controlcenter"
	"fitlog/internal/mcpserver"
	"fitlog/internal/server"
)

func integrationServer(t *testing.T) (*Server, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	cfg := testConfig()
	cfg.OwnerID = 9_140_130_001
	s, err := NewServer(cfg, pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(ctx, `DELETE FROM mcp_oauth_grants WHERE binding=$1`, s.binding); err != nil {
			t.Error(err)
		}
	})
	return s, pool
}
func TestPostgresOAuthAtomicExchange(t *testing.T) {
	s, pool := integrationServer(t)
	code := approvedCode(t, s)
	f := codeForm(s, code)
	var mu sync.Mutex
	var replies []*httptest.ResponseRecorder
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); r := tokenRequest(s, f); mu.Lock(); replies = append(replies, r); mu.Unlock() }()
	}
	wg.Wait()
	successes := 0
	var token string
	for _, reply := range replies {
		if reply.Code == 200 {
			successes++
			token = tokens(t, reply)["access_token"].(string)
		} else if reply.Code != 400 {
			t.Fatalf("unexpected response %d: %s", reply.Code, reply.Body.String())
		}
	}
	if successes != 1 {
		t.Fatalf("code exchanged %d times", successes)
	}
	reloaded, err := NewServer(s.cfg, pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reloaded.store.Validate(t.Context(), digest(token), reloaded.binding); err != nil {
		t.Fatal("grant lost on restart", err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE mcp_oauth_grants SET access_expires_at=now()-interval '1 second' WHERE binding=$1`, s.binding); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.Validate(t.Context(), digest(token), s.binding); err == nil {
		t.Fatal("expired access token accepted")
	}
}
func TestPostgresMCPJournal(t *testing.T) {
	s, pool := integrationServer(t)
	ctx := t.Context()
	owner := s.cfg.OwnerID
	cleanup := func() {
		for _, table := range []string{"training_sessions", "training_programs", "training_exercises", "nutrition_days", "body_measurements", "dashboard_settings"} {
			if _, err := pool.Exec(ctx, "DELETE FROM "+table+" WHERE owner_id=$1", owner); err != nil {
				t.Error(err)
			}
		}
	}
	cleanup()
	defer cleanup()
	repo := controlcenter.NewRepository(pool)
	if _, err := repo.SaveSettings(ctx, owner, controlcenter.Settings{Timezone: "Europe/Moscow", Units: "metric", Theme: "dark", FirstDayOfWeek: 1}); err != nil {
		t.Fatal(err)
	}
	tok := tokens(t, tokenRequest(s, codeForm(s, approvedCode(t, s))))["access_token"].(string)
	handler := server.RouterWithMCP(nil, nil, pool, nil, nil, s.Protect(mcpserver.NewHandler(repo, mcpserver.Options{OwnerID: owner, CanWrite: CanWrite})), s.Routes())
	call := func(t *testing.T, name string, args any) (json.RawMessage, bool) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		r := httptest.NewRequest("POST", s.resource(), strings.NewReader(string(body)))
		r.Header.Set("Authorization", "Bearer "+tok)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("MCP-Protocol-Version", "2025-11-25")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s HTTP %d: %s", name, w.Code, w.Body.String())
		}
		var response struct {
			Error  any `json:"error"`
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
				Structured struct {
					Data json.RawMessage `json:"data"`
				} `json:"structuredContent"`
			} `json:"result"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Error != nil {
			t.Fatalf("protocol error: %s", w.Body.String())
		}
		if response.Result.IsError {
			raw, _ := json.Marshal(response.Result.Content)
			return raw, true
		}
		return response.Result.Structured.Data, false
	}
	for _, tt := range []struct {
		name     string
		args     map[string]any
		resource string
	}{
		{"log_nutrition", map[string]any{"request_id": "integration-food-1", "date": "2026-09-14", "calories_kcal": 2000, "protein_g": 140}, "nutrition"},
		{"log_body_measurement", map[string]any{"request_id": "integration-body-1", "measured_at": "2026-09-14T09:00:00+03:00", "weight_kg": 80}, "body-measurements"},
		{"create_workout_plan", map[string]any{"request_id": "integration-plan-1", "name": "MCP Plan", "templates": []any{map[string]any{"name": "A", "exercises": []any{map[string]any{"name": "Pull-up", "working_sets": 3, "min_reps": 6, "max_reps": 10, "weight_step_kg": 1}}}}}, "workout-plans"},
		{"log_workout", map[string]any{"request_id": "integration-workout-1", "started_at": "2026-09-14T20:00:00Z", "program_name": "MCP Workout", "exercises": []any{map[string]any{"name": "Pull-up", "sets": []any{map[string]any{"type": "working", "weight_kg": nil, "reps": 8, "rir": 0}}}}}, "workout-sessions"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, failed := call(t, tt.name, tt.args)
			if failed {
				t.Fatal(string(data))
			}
			var record struct {
				ID     int64  `json:"id"`
				Source string `json:"source"`
			}
			if err := json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			if record.ID <= 0 || record.Source != "manual" {
				t.Fatalf("bad saved record: %s", data)
			}
			loc, _ := time.LoadLocation("Europe/Moscow")
			service := controlcenter.NewService(repo, owner, loc)
			stored, err := service.Get(ctx, tt.resource, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(stored) {
				t.Fatal("record not visible through shared service")
			}
			if _, err := controlcenter.NewService(repo, owner+1, loc).Get(ctx, tt.resource, record.ID); err == nil {
				t.Fatal("record leaked to another owner")
			}
			data, failed = call(t, tt.name, tt.args)
			if !failed {
				t.Fatalf("retry did not conflict: %s", data)
			}
			if tt.resource == "workout-sessions" {
				var completed int
				err := pool.QueryRow(ctx, `SELECT count(*) FROM training_sets ts JOIN training_session_exercises se ON se.id=ts.session_exercise_id
     WHERE se.session_id=$1 AND se.complete AND ts.completed_at IS NOT NULL AND ts.actual_weight_kg IS NULL AND ts.actual_rir=0`, record.ID).Scan(&completed)
				if err != nil || completed != 1 {
					t.Fatalf("completed sets lost: %d %v", completed, err)
				}
			}
		})
	}
	data, failed := call(t, "get_overview", map[string]any{"from": "2026-09-14", "to": "2026-09-14"})
	if failed {
		t.Fatal(string(data))
	}
	for _, path := range []string{"/.well-known/oauth-authorization-server", "/.well-known/oauth-protected-resource/mcp"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, s.cfg.BaseURL+path, nil))
		if w.Code != 200 {
			t.Fatal(fmt.Sprintf("route %s: %d", path, w.Code))
		}
	}
}
