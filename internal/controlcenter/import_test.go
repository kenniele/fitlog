package controlcenter

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseImportMappingAndPreviewCounts(t *testing.T) {
	batch, preview, err := parseImport(ImportRequest{
		DataType: "recovery", Format: "csv", Source: "whoop",
		Content: "Day,Score,Remote ID\n2026-08-20,78,row-1\nbad,70,row-2\n",
		Mapping: map[string]string{
			"date":           "day",
			"recovery_score": "score",
			"external_id":    "remote_id",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.TotalRows != 2 || preview.ValidRows != 1 || preview.InvalidRows != 1 {
		t.Fatalf("preview counts = total %d valid %d invalid %d", preview.TotalRows, preview.ValidRows, preview.InvalidRows)
	}
	if !reflect.DeepEqual(preview.RequiredFields, []string{"date"}) {
		t.Fatalf("required fields = %#v", preview.RequiredFields)
	}
	if !reflect.DeepEqual(preview.TargetFields, importSchemas["recovery"]) {
		t.Fatalf("target fields = %#v", preview.TargetFields)
	}
	if len(preview.Rows) != 1 || len(batch.Rows) != 1 || batch.Rows[0].ExternalID != "row-1" {
		t.Fatalf("unexpected parsed rows: preview=%#v batch=%#v", preview.Rows, batch.Rows)
	}
}

func TestParseImportNormalizesCommaDecimalForExecution(t *testing.T) {
	batch, preview, err := parseImport(ImportRequest{
		DataType: "nutrition", Format: "csv",
		Content: "date;protein_g\n2026-08-21;145,5\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ValidRows != 1 || len(batch.Rows) != 1 {
		t.Fatalf("preview=%#v batch=%#v", preview, batch)
	}
	if value := batch.Rows[0].Values["protein_g"]; value != "145.5" {
		t.Fatalf("normalized protein = %q", value)
	}
}

func TestParseImportSupportsInBodyMetricsAndSegments(t *testing.T) {
	batch, preview, err := parseImport(ImportRequest{
		DataType: "body", Format: "csv", Source: "inbody",
		Content: "measured_at,weight,TBW,ICW,ECW,ECW/TBW,bmr,phase_angle,left_arm_lean_mass_kg,left_arm_lean_percent,external_id\n" +
			"2026-08-21T08:00,81.2,48.1,29.8,18.3,0.380,1810,6.8,3.82,104.5,inbody-1\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ValidRows != 1 || preview.InvalidRows != 0 || len(batch.Rows) != 1 {
		t.Fatalf("preview=%#v batch=%#v", preview, batch)
	}
	values := batch.Rows[0].Values
	for field, want := range map[string]string{
		"weight_kg": "81.2", "total_body_water_l": "48.1", "intracellular_water_l": "29.8",
		"extracellular_water_l": "18.3", "ecw_tbw_ratio": "0.38",
		"basal_metabolic_rate_kcal": "1810", "phase_angle_degrees": "6.8",
		"left_arm_lean_mass_kg": "3.82", "left_arm_lean_percent": "104.5",
	} {
		if values[field] != want {
			t.Errorf("%s = %q, want %q", field, values[field], want)
		}
	}
	if batch.Rows[0].ExternalID != "inbody-1" {
		t.Fatalf("external id = %q", batch.Rows[0].ExternalID)
	}
}

func TestParseImportRejectsImpossibleInBodyComposition(t *testing.T) {
	_, preview, err := parseImport(ImportRequest{
		DataType: "body", Format: "json",
		Content: `[{"measured_at":"2026-08-21","total_body_water_l":40,"extracellular_water_l":41,"ecw_tbw_ratio":1.2,"phase_angle_degrees":90}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ValidRows != 0 || preview.InvalidRows != 1 || len(preview.Errors) != 1 {
		t.Fatalf("invalid InBody preview = %#v", preview)
	}
	fields := preview.Errors[0].Fields
	for _, field := range []string{"extracellular_water_l", "ecw_tbw_ratio", "phase_angle_degrees"} {
		if fields[field] == "" {
			t.Errorf("missing validation error for %s: %#v", field, fields)
		}
	}
}

func TestParseImportHonorsExplicitlyUnmappedCanonicalColumn(t *testing.T) {
	batch, preview, err := parseImport(ImportRequest{
		DataType: "nutrition", Format: "csv",
		Content: "date,notes\n2026-08-21,do not import me\n",
		Mapping: map[string]string{"date": "date", "notes": ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ValidRows != 1 || len(batch.Rows) != 1 {
		t.Fatalf("preview=%#v batch=%#v", preview, batch)
	}
	if value := batch.Rows[0].Values["notes"]; value != "" {
		t.Fatalf("explicitly unmapped notes = %q", value)
	}
}

func TestParseImportCapsErrorSamplesWithoutLosingCounts(t *testing.T) {
	content := "date,recovery_score\n" + strings.Repeat("not-a-date,50\n", MaxImportErrorSamples+37)
	batch, preview, err := parseImport(ImportRequest{DataType: "recovery", Format: "csv", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	want := MaxImportErrorSamples + 37
	if preview.InvalidRows != want || batch.FailedRows != want || batch.TotalRows != want {
		t.Fatalf("uncapped counts preview=%d failed=%d total=%d", preview.InvalidRows, batch.FailedRows, batch.TotalRows)
	}
	if len(preview.Errors) != MaxImportErrorSamples || len(batch.Errors) != MaxImportErrorSamples {
		t.Fatalf("error samples preview=%d batch=%d", len(preview.Errors), len(batch.Errors))
	}
}

func TestPreviewSetsUsesParentScopedDuplicateKey(t *testing.T) {
	store := &handlerStore{existingExternalIDsFn: func(_ context.Context, _ int64, dataType, source string, ids []string) (map[string]struct{}, error) {
		if dataType != "sets" || source != "file" {
			t.Fatalf("lookup type=%q source=%q", dataType, source)
		}
		want := "session-1" + compositeExternalSeparator + "set-1"
		if !reflect.DeepEqual(ids, []string{want}) {
			t.Fatalf("duplicate keys = %#v, want %#v", ids, []string{want})
		}
		return map[string]struct{}{want: {}}, nil
	}}
	preview, err := NewService(store, 7, time.UTC).PreviewImport(context.Background(), ImportRequest{
		DataType: "sets", Format: "json", Source: "file",
		Content: `[{"session_external_id":"session-1","exercise_name":"Squat","type":"working","reps":8,"external_id":"set-1"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.DuplicateRows != 1 {
		t.Fatalf("duplicate rows = %d, want 1", preview.DuplicateRows)
	}
}

func TestValidImportDateAcceptsBrowserDateTimeLocal(t *testing.T) {
	for _, value := range []string{"2026-08-21T18:30", "2026-08-21T18:30:45"} {
		if !validImportDate(value) {
			t.Fatalf("datetime-local %q was rejected", value)
		}
	}
}

func TestImportPreviewRejectsValuesThatDatabaseWouldReject(t *testing.T) {
	tests := []ImportRequest{
		{DataType: "recovery", Format: "csv", Content: "date,recovery_score\n2026-08-21,200\n"},
		{DataType: "sleep", Format: "csv", Content: "date,sleep_start,sleep_end\n2026-08-21,2026-08-21T08:00,2026-08-21T07:00\n"},
		{DataType: "body", Format: "json", Content: `[{"measured_at":"2026-08-21T08:00"}]`},
		{DataType: "workouts", Format: "csv", Content: "program_name,started_at,status\nA,2026-08-21T18:00,scheduled\n"},
		{DataType: "sets", Format: "csv", Content: "session_external_id,exercise_name,type,reps\ns-1,Squat,working,8.5\n"},
		{DataType: "sets", Format: "csv", Content: "session_external_id,exercise_name,position,type,reps\ns-1,Squat,0,working,8\n"},
	}
	for _, request := range tests {
		_, preview, err := parseImport(request)
		if err != nil {
			t.Fatalf("parse %s: %v", request.DataType, err)
		}
		if preview.ValidRows != 0 || preview.InvalidRows != 1 {
			t.Fatalf("%s preview accepted invalid row: %#v", request.DataType, preview)
		}
	}
}

func TestPreviewCountsDuplicateIDsWithinFile(t *testing.T) {
	preview, err := NewService(&handlerStore{}, 7, time.UTC).PreviewImport(context.Background(), ImportRequest{
		DataType: "nutrition", Format: "csv", Source: "file",
		Content: "date,calories_kcal,external_id\n2026-08-20,2200,day-1\n2026-08-21,2300,day-1\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.DuplicateRows != 1 {
		t.Fatalf("duplicate rows = %d, want 1", preview.DuplicateRows)
	}
}
