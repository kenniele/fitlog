package mcpserver

import (
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"strings"
	"testing"
)

func TestLogWorkoutMapsCompletedSetsAndManualSource(t *testing.T) {
	store := &testStore{}
	cs := connect(t, newTestAdapter(store).server(true))
	r := call(t, cs, "log_workout", map[string]any{"request_id": "workout-2026-09-14", "started_at": "2026-09-14T12:00:00+03:00", "program_name": "A", "exercises": []any{map[string]any{"name": "Подтягивания", "sets": []any{map[string]any{"type": "working", "weight_kg": nil, "reps": 8, "rir": 0}}}}})
	if r.IsError {
		t.Fatal(resultText(r))
	}
	var data struct {
		Status, Source, ExternalID string
		Exercises                  []struct {
			Completed bool
			Sets      []struct {
				Completed bool
				WeightKG  *float64 `json:"weight_kg"`
				RIR       *float64
			}
		}
	}
	if err := json.Unmarshal(store.payload, &data); err != nil {
		t.Fatal(err)
	}
	if data.Status != "finished" || data.Source != "manual" || len(data.Exercises) != 1 || !data.Exercises[0].Completed || !data.Exercises[0].Sets[0].Completed || data.Exercises[0].Sets[0].WeightKG != nil || data.Exercises[0].Sets[0].RIR == nil || *data.Exercises[0].Sets[0].RIR != 0 {
		t.Fatalf("invalid record: %s", store.payload)
	}
	if strings.Contains(string(store.payload), `"request_id"`) || !strings.Contains(string(store.payload), `"external_id":"mcp:workout-2026-09-14"`) {
		t.Fatalf("invalid request marker: %s", store.payload)
	}
}
func TestWriteInputValidation(t *testing.T) {
	for _, tt := range []struct {
		name, tool string
		args       map[string]any
	}{
		{"empty nutrition", "log_nutrition", map[string]any{"request_id": "nutrition-1", "date": "2026-09-14"}},
		{"negative nutrition", "log_nutrition", map[string]any{"request_id": "nutrition-1", "date": "2026-09-14", "calories_kcal": -1}},
		{"source spoofing", "log_nutrition", map[string]any{"request_id": "nutrition-1", "date": "2026-09-14", "calories_kcal": 100, "source": "fatsecret"}},
		{"missing request id", "log_body_measurement", map[string]any{"measured_at": "2026-09-14", "weight_kg": 80}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &testStore{}
			cs := connect(t, newTestAdapter(store).server(true))
			r, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: tt.tool, Arguments: tt.args})
			if err == nil && !r.IsError {
				t.Fatal("invalid write accepted")
			}
			if store.calls != 0 {
				t.Fatal("invalid write reached store")
			}
		})
	}
}
