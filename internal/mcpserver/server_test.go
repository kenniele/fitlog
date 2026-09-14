package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fitlog/internal/controlcenter"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type testStore struct {
	controlcenter.Store
	owner    int64
	loc      *time.Location
	options  controlcenter.Pagination
	resource string
	payload  json.RawMessage
	failure  error
	calls    int
}

func (s *testStore) Settings(_ context.Context, owner int64, _ string) (controlcenter.Settings, error) {
	s.owner = owner
	return controlcenter.Settings{Timezone: "Europe/Moscow", Units: "metric"}, nil
}
func (s *testStore) Sources(_ context.Context, owner int64) ([]controlcenter.SourceStatus, error) {
	s.owner = owner
	return []controlcenter.SourceStatus{{Source: "whoop", Status: "connected"}}, nil
}
func (s *testStore) List(_ context.Context, owner int64, resource string, p controlcenter.Pagination, loc *time.Location) (controlcenter.ListResult, error) {
	s.owner, s.loc, s.options, s.resource = owner, loc, p, resource
	s.calls++
	return controlcenter.ListResult{Items: []json.RawMessage{json.RawMessage(`{"id":7,"weight_kg":null,"rir":0}`)}, Total: 26, Page: p.Page, PageSize: p.PageSize}, s.failure
}
func (s *testStore) Get(_ context.Context, owner int64, resource string, id int64, loc *time.Location) (json.RawMessage, error) {
	s.owner, s.loc, s.resource = owner, loc, resource
	s.calls++
	return json.RawMessage(`{"id":7}`), s.failure
}
func (s *testStore) Create(_ context.Context, owner int64, resource string, raw json.RawMessage, loc *time.Location) (json.RawMessage, error) {
	s.owner, s.loc, s.resource, s.payload = owner, loc, resource, raw
	s.calls++
	return json.RawMessage(`{"id":7}`), s.failure
}

func newTestAdapter(store *testStore) *adapter {
	return &adapter{store: store, options: Options{OwnerID: 42, Location: time.UTC, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
}
func connect(t *testing.T, s *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	a, b := mcp.NewInMemoryTransports()
	session, err := s.Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}
func call(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func resultText(r *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range r.Content {
		if v, ok := c.(*mcp.TextContent); ok {
			b.WriteString(v.Text)
		}
	}
	return b.String()
}

func TestToolDiscoveryAndScopes(t *testing.T) {
	for _, writes := range []bool{false, true} {
		name := "read"
		want := 12
		if writes {
			name = "write"
			want = 16
		}
		t.Run(name, func(t *testing.T) {
			cs := connect(t, newTestAdapter(&testStore{}).server(writes))
			list, err := cs.ListTools(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Tools) != want {
				t.Fatalf("got %d tools, want %d", len(list.Tools), want)
			}
			for _, tool := range list.Tools {
				if tool.Annotations == nil || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
					t.Fatalf("missing annotations: %s", tool.Name)
				}
				if !writes && !tool.Annotations.ReadOnlyHint {
					t.Fatalf("write tool leaked: %s", tool.Name)
				}
			}
			if !writes {
				_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "log_body_measurement", Arguments: map[string]any{}})
				if err == nil {
					t.Fatal("read-only server accepted write tool")
				}
			}
		})
	}
}
func TestReadToolsPreserveOwnerTimezoneAndNull(t *testing.T) {
	store := &testStore{}
	cs := connect(t, newTestAdapter(store).server(false))
	r := call(t, cs, "list_workouts", map[string]any{"from": "2026-09-01", "to": "2026-09-14", "page": 2, "page_size": 1, "exercise_id": 9, "status": "finished"})
	text := resultText(r)
	if r.IsError || !strings.Contains(text, `"weight_kg":null`) || !strings.Contains(text, `"rir":0`) || r.StructuredContent == nil {
		t.Fatalf("result=%+v", r)
	}
	if store.owner != 42 || store.loc.String() != "Europe/Moscow" || store.options.Page != 2 || store.options.Filters["exercise_id"] != "9" || store.options.From.Location() != store.loc {
		t.Fatalf("wrong scope/options: %+v", store)
	}
}
func TestInvalidInputsAndErrorRedaction(t *testing.T) {
	tests := []struct {
		name, tool string
		args       map[string]any
	}{
		{"owner injection", "list_workouts", map[string]any{"owner_id": 77}},
		{"huge page", "list_workouts", map[string]any{"page": 1e15}},
		{"large page size", "list_workouts", map[string]any{"page_size": 101}},
		{"partial dates", "list_workouts", map[string]any{"from": "2026-09-01"}},
		{"wide dates", "list_workouts", map[string]any{"from": "2020-01-01", "to": "2026-09-01"}},
		{"negative id", "get_workout", map[string]any{"id": -1}},
		{"ignored filter", "list_sleep", map[string]any{"search": "anything"}},
		{"bad analytics", "get_analytics", map[string]any{"kind": "sql"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &testStore{}
			cs := connect(t, newTestAdapter(store).server(false))
			r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tt.tool, Arguments: tt.args})
			if err == nil && !r.IsError {
				t.Fatal("invalid input accepted")
			}
			if store.calls != 0 {
				t.Fatal("repository called for invalid input")
			}
		})
	}
	t.Run("error redaction", func(t *testing.T) {
		store := &testStore{failure: errors.New("postgres password=super-secret")}
		cs := connect(t, newTestAdapter(store).server(false))
		r := call(t, cs, "get_workout", map[string]any{"id": 1})
		if !r.IsError || strings.Contains(resultText(r), "super-secret") {
			t.Fatalf("unsafe result: %+v", r)
		}
	})
}
func TestStreamableHTTP(t *testing.T) {
	handler := NewHandler(&testStore{}, Options{OwnerID: 42})
	for _, body := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_context","arguments":{}}}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"result"`) {
			t.Fatalf("%d: %s", w.Code, w.Body.String())
		}
	}
}
