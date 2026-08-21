package controlcenter

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepository_ControlCenterRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)
	const ownerID int64 = 9_101_023_920
	const otherOwnerID int64 = ownerID + 1
	cleanup := func() {
		if err := repository.DeleteAll(ctx, ownerID); err != nil {
			t.Fatalf("cleanup owner: %v", err)
		}
		if err := repository.DeleteAll(ctx, otherOwnerID); err != nil {
			t.Fatalf("cleanup other owner: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	created, err := repository.Create(ctx, ownerID, "recovery", json.RawMessage(`{
		"date":"2026-08-21","recovery_score":81,"skin_temperature_celsius":36.4
	}`), loc)
	if err != nil {
		t.Fatalf("create recovery: %v", err)
	}
	if len(created) == 0 {
		t.Fatal("created recovery is empty")
	}
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, loc)
	to := time.Date(2026, 8, 31, 0, 0, 0, 0, loc)
	listed, err := repository.List(ctx, ownerID, "recovery", Pagination{
		Page: 1, PageSize: 25, From: &from, To: &to, Filters: map[string]string{},
	}, loc)
	if err != nil {
		t.Fatalf("list recovery: %v", err)
	}
	if listed.Total != 1 || len(listed.Items) != 1 {
		t.Fatalf("owner recovery list = total %d, items %d", listed.Total, len(listed.Items))
	}
	other, err := repository.List(ctx, otherOwnerID, "recovery", Pagination{
		Page: 1, PageSize: 25, From: &from, To: &to, Filters: map[string]string{},
	}, loc)
	if err != nil {
		t.Fatalf("list other owner recovery: %v", err)
	}
	if other.Total != 0 {
		t.Fatalf("other owner sees %d recovery rows", other.Total)
	}

	exerciseRaw, err := repository.Create(ctx, ownerID, "exercises", json.RawMessage(`{
		"name":"Integration Squat","muscle_groups":"legs, core","notes":"owner scoped"
	}`), loc)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	var exerciseRecord struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(exerciseRaw, &exerciseRecord); err != nil || exerciseRecord.ID == 0 {
		t.Fatalf("decode exercise: id=%d err=%v raw=%s", exerciseRecord.ID, err, exerciseRaw)
	}
	planPayload := json.RawMessage(`{
		"name":"Integration A/B/C","description":"versioned snapshot","days_per_week":3,
		"templates":[{"name":"A","position":1,"exercises":[{
			"exercise_id":` + strconv.FormatInt(exerciseRecord.ID, 10) + `,"position":1,
			"working_sets":3,"min_reps":6,"max_reps":10,"target_rir":2,
			"weight_step_kg":2.5,"starting_weight_kg":40,"progression_type":"double",
			"warmup_sets":[{"position":1,"weight_mode":"bar","bar":true,"reps":10},
				{"position":2,"weight_mode":"kg","weight_kg":30,"reps":6}],
			"rest_seconds":120,"rest_after_exercise_seconds":180
		}]}]
	}`)
	planRaw, err := repository.Create(ctx, ownerID, "workout-plans", planPayload, loc)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	var planRecord struct {
		ID        int64 `json:"id"`
		Templates []struct {
			ID        int64 `json:"id"`
			Exercises []struct {
				WeightStepKG     float64 `json:"weight_step_kg"`
				StartingWeightKG float64 `json:"starting_weight_kg"`
				ProgressionType  string  `json:"progression_type"`
				WarmupSets       []struct {
					WeightMode string   `json:"weight_mode"`
					WeightKG   *float64 `json:"weight_kg"`
					Reps       int      `json:"reps"`
				} `json:"warmup_sets"`
			} `json:"exercises"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(planRaw, &planRecord); err != nil || planRecord.ID == 0 || len(planRecord.Templates) != 1 {
		t.Fatalf("decode plan: err=%v raw=%s", err, planRaw)
	}
	if len(planRecord.Templates[0].Exercises) != 1 || planRecord.Templates[0].Exercises[0].WeightStepKG != 2.5 ||
		planRecord.Templates[0].Exercises[0].StartingWeightKG != 40 ||
		planRecord.Templates[0].Exercises[0].ProgressionType != "double" ||
		len(planRecord.Templates[0].Exercises[0].WarmupSets) != 2 {
		t.Fatalf("plan prescription did not round-trip: raw=%s", planRaw)
	}
	updatedPlanRaw, err := repository.Update(ctx, ownerID, "workout-plans", planRecord.ID, planPayload, loc)
	if err != nil {
		t.Fatalf("update plan prescription: %v", err)
	}
	if err := json.Unmarshal(updatedPlanRaw, &planRecord); err != nil || len(planRecord.Templates) != 1 ||
		len(planRecord.Templates[0].Exercises) != 1 || len(planRecord.Templates[0].Exercises[0].WarmupSets) != 2 {
		t.Fatalf("updated plan prescription did not round-trip: err=%v raw=%s", err, updatedPlanRaw)
	}

	materializedPayload, err := json.Marshal(map[string]any{
		"date":         "2026-08-22",
		"scheduled_at": "2026-08-22T09:00",
		"status":       "scheduled",
		"plan_id":      planRecord.ID,
		"template_id":  planRecord.Templates[0].ID,
		"source":       "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	materializedRaw, err := repository.Create(ctx, ownerID, "workout-sessions", materializedPayload, loc)
	if err != nil {
		t.Fatalf("materialize scheduled template session: %v", err)
	}
	var materialized struct {
		ID                     int64 `json:"id"`
		HasProgressionSnapshot bool  `json:"has_progression_snapshot"`
		Exercises              []struct {
			ID   int64 `json:"id"`
			Sets []struct {
				ID              int64    `json:"id"`
				Type            string   `json:"type"`
				PlannedWeightKG *float64 `json:"planned_weight_kg"`
				PlannedMinReps  *int     `json:"planned_min_reps"`
				ActualWeightKG  *float64 `json:"actual_weight_kg"`
				ActualReps      *int     `json:"actual_reps"`
				Completed       bool     `json:"completed"`
			} `json:"sets"`
		} `json:"exercises"`
	}
	if err := json.Unmarshal(materializedRaw, &materialized); err != nil || materialized.ID == 0 ||
		!materialized.HasProgressionSnapshot || len(materialized.Exercises) != 1 ||
		len(materialized.Exercises[0].Sets) != 5 {
		t.Fatalf("scheduled template snapshot is incomplete: record=%+v err=%v raw=%s", materialized, err, materializedRaw)
	}
	var workingSetID int64
	for _, set := range materialized.Exercises[0].Sets {
		if set.Type == "working" && workingSetID == 0 {
			workingSetID = set.ID
			if set.PlannedWeightKG == nil || *set.PlannedWeightKG != 40 || set.PlannedMinReps == nil || *set.PlannedMinReps != 6 ||
				set.ActualWeightKG != nil || set.ActualReps != nil || set.Completed {
				t.Fatalf("materialized working set has wrong plan/actual fields: %+v", set)
			}
		}
	}
	if workingSetID == 0 {
		t.Fatalf("materialized session has no working set: %s", materializedRaw)
	}
	var materializedWarmupPlan string
	if err := pool.QueryRow(ctx, `SELECT warmup_plan::text FROM training_session_exercises WHERE id=$1`, materialized.Exercises[0].ID).Scan(&materializedWarmupPlan); err != nil {
		t.Fatalf("read materialized warmup plan: %v", err)
	}
	if !strings.Contains(materializedWarmupPlan, `"weight_kg": 30`) {
		t.Fatalf("materialized warmup plan is not compatible with training.WarmupSet: %s", materializedWarmupPlan)
	}

	completedPayload, err := json.Marshal(map[string]any{
		"date":         "2026-08-22",
		"scheduled_at": "2026-08-22T09:00",
		"started_at":   "2026-08-22T18:00",
		"finished_at":  "2026-08-22T19:00",
		"status":       "finished",
		"plan_id":      planRecord.ID,
		"template_id":  planRecord.Templates[0].ID,
		"source":       "manual",
		"exercises": []any{map[string]any{
			"id": materialized.Exercises[0].ID, "completed": true,
			"sets": []any{map[string]any{
				"id": workingSetID, "position": 3, "type": "working", "weight_kg": 42.5,
				"reps": 8, "rir": 2, "completed": true, "rest_seconds": 120,
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	completedRaw, err := repository.Update(ctx, ownerID, "workout-sessions", materialized.ID, completedPayload, loc)
	if err != nil {
		t.Fatalf("record actual result against progression snapshot: %v", err)
	}
	if err := json.Unmarshal(completedRaw, &materialized); err != nil {
		t.Fatalf("decode completed progression session: %v raw=%s", err, completedRaw)
	}
	if len(materialized.Exercises) != 1 || len(materialized.Exercises[0].Sets) != 5 {
		t.Fatalf("actual patch replaced prescription children: %s", completedRaw)
	}
	foundCompleted := false
	for _, set := range materialized.Exercises[0].Sets {
		if set.ID == workingSetID {
			foundCompleted = set.Completed && set.ActualWeightKG != nil && *set.ActualWeightKG == 42.5 &&
				set.ActualReps != nil && *set.ActualReps == 8 && set.PlannedWeightKG != nil && *set.PlannedWeightKG == 40
		}
	}
	if !foundCompleted {
		t.Fatalf("actual patch did not preserve planned fields and store actual fields: %s", completedRaw)
	}

	emptyFinishedPayload, err := json.Marshal(map[string]any{
		"date": "2026-08-23", "started_at": "2026-08-23T18:00", "finished_at": "2026-08-23T19:00",
		"status": "finished", "template_id": planRecord.Templates[0].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.Create(ctx, ownerID, "workout-sessions", emptyFinishedPayload, loc)
	var emptyValidation *ValidationError
	if !errors.As(err, &emptyValidation) || emptyValidation.Fields["exercises"] == "" {
		t.Fatalf("empty finished template session error = %#v", err)
	}

	sessionPayload := json.RawMessage(`{
		"date":"2026-08-21","started_at":"2026-08-21T18:30","finished_at":"2026-08-21T19:30",
		"plan_id":` + strconv.FormatInt(planRecord.ID, 10) + `,"notes":"integration workout",
		"exercises":[{"exercise_id":` + strconv.FormatInt(exerciseRecord.ID, 10) + `,"position":1,"rest_after_exercise_seconds":180,"sets":[
			{"position":1,"type":"warmup","weight_kg":20,"reps":10,"completed":true,"rest_seconds":60},
			{"position":2,"type":"working","weight_kg":80,"reps":8,"rir":2,"completed":true,"rest_seconds":120,"comment":"clean"}
		]}]
	}`)
	sessionRaw, err := repository.Create(ctx, ownerID, "workout-sessions", sessionPayload, loc)
	if err != nil {
		t.Fatalf("create workout session: %v", err)
	}
	var sessionRecord struct {
		ID                     int64   `json:"id"`
		VolumeKG               float64 `json:"volume_kg"`
		WorkingSets            int     `json:"working_sets"`
		HasProgressionSnapshot bool    `json:"has_progression_snapshot"`
		Exercises              []struct {
			RestAfterExerciseSeconds *int `json:"rest_after_exercise_seconds"`
		} `json:"exercises"`
	}
	if err := json.Unmarshal(sessionRaw, &sessionRecord); err != nil || sessionRecord.ID == 0 || sessionRecord.VolumeKG != 640 || sessionRecord.WorkingSets != 1 || sessionRecord.HasProgressionSnapshot ||
		len(sessionRecord.Exercises) != 1 || sessionRecord.Exercises[0].RestAfterExerciseSeconds == nil || *sessionRecord.Exercises[0].RestAfterExerciseSeconds != 180 {
		t.Fatalf("decode workout session: record=%+v err=%v raw=%s", sessionRecord, err, sessionRaw)
	}
	updatedSessionPayload := json.RawMessage(strings.Replace(string(sessionPayload), `"rest_after_exercise_seconds":180`, `"rest_after_exercise_seconds":240`, 1))
	sessionRaw, err = repository.Update(ctx, ownerID, "workout-sessions", sessionRecord.ID, updatedSessionPayload, loc)
	if err != nil {
		t.Fatalf("update workout session rest after exercise: %v", err)
	}
	if err := json.Unmarshal(sessionRaw, &sessionRecord); err != nil || sessionRecord.HasProgressionSnapshot ||
		len(sessionRecord.Exercises) != 1 || sessionRecord.Exercises[0].RestAfterExerciseSeconds == nil || *sessionRecord.Exercises[0].RestAfterExerciseSeconds != 240 {
		t.Fatalf("updated workout rest did not round-trip: record=%+v err=%v raw=%s", sessionRecord, err, sessionRaw)
	}
	var sessionExerciseID int64
	if err := pool.QueryRow(ctx, `UPDATE training_session_exercises
		SET working_sets=3, min_reps=6, max_reps=10, warmup_plan='[{"reps":10}]'::jsonb,
			recommendation='{"weight_kg":80}'::jsonb, planned_weight_kg=80
		WHERE session_id=$1 RETURNING id`, sessionRecord.ID).Scan(&sessionExerciseID); err != nil {
		t.Fatalf("mark progression snapshot: %v", err)
	}
	snapshotRaw, err := repository.Get(ctx, ownerID, "workout-sessions", sessionRecord.ID, loc)
	if err != nil {
		t.Fatalf("get progression snapshot: %v", err)
	}
	if err := json.Unmarshal(snapshotRaw, &sessionRecord); err != nil || !sessionRecord.HasProgressionSnapshot {
		t.Fatalf("progression snapshot flag: record=%+v err=%v raw=%s", sessionRecord, err, snapshotRaw)
	}
	metadataUpdate := json.RawMessage(`{
		"date":"2026-08-21","started_at":"2026-08-21T18:30","finished_at":"2026-08-21T19:30",
		"template_id":` + strconv.FormatInt(planRecord.Templates[0].ID, 10) + `,"status":"finished",
		"program_name":"Integration A/B/C","notes":"metadata only","source":"manual"
	}`)
	if _, err := repository.Update(ctx, ownerID, "workout-sessions", sessionRecord.ID, metadataUpdate, loc); err != nil {
		t.Fatalf("metadata-only snapshot update: %v", err)
	}
	var exerciseCount, workingSets int
	var warmupPlan string
	if err := pool.QueryRow(ctx, `SELECT count(*), max(working_sets), max(warmup_plan::text)
		FROM training_session_exercises WHERE session_id=$1`, sessionRecord.ID).Scan(&exerciseCount, &workingSets, &warmupPlan); err != nil {
		t.Fatalf("read preserved snapshot: %v", err)
	}
	if exerciseCount != 1 || workingSets != 3 || !strings.Contains(warmupPlan, `"reps": 10`) {
		t.Fatalf("metadata update replaced progression snapshot: exercises=%d working_sets=%d warmup=%s", exerciseCount, workingSets, warmupPlan)
	}
	_, err = repository.Update(ctx, ownerID, "workout-sessions", sessionRecord.ID, sessionPayload, loc)
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Fields["exercises.0.id"] == "" {
		t.Fatalf("progression snapshot child replacement error = %#v", err)
	}
	exportRange := DateRange{From: from, To: to}
	filteredCSV, err := repository.ExportSessionsCSV(ctx, ownerID, exportRange, Pagination{
		Search: "Integration Squat", Filters: map[string]string{
			"status": "finished", "exercise_id": strconv.FormatInt(exerciseRecord.ID, 10),
		},
	}, loc)
	if err != nil {
		t.Fatalf("filtered session export: %v", err)
	}
	if !strings.Contains(string(filteredCSV), "Integration A/B/C") || !strings.Contains(string(filteredCSV), "Integration Squat") {
		t.Fatalf("filtered session export misses selected workout: %s", filteredCSV)
	}
	emptyCSV, err := repository.ExportSessionsCSV(ctx, ownerID, exportRange, Pagination{
		Search: "does-not-exist", Filters: map[string]string{},
	}, loc)
	if err != nil {
		t.Fatalf("empty filtered session export: %v", err)
	}
	if lines := strings.Count(strings.TrimSpace(string(emptyCSV)), "\n"); lines != 0 {
		t.Fatalf("empty filtered export contains data rows: %s", emptyCSV)
	}

	service := NewService(repository, ownerID, loc)
	request := ImportRequest{
		DataType: "recovery", Filename: "integration.csv", Format: "csv", Source: "integration",
		Content: "date,recovery_score,external_id\n2026-08-20,77,integration-recovery-1\n",
	}
	first, err := service.ExecuteImport(ctx, request)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	second, err := service.ExecuteImport(ctx, request)
	if err != nil {
		t.Fatalf("duplicate import: %v", err)
	}
	if first.Imported != 1 || second.Imported != 0 || second.Skipped != 1 {
		t.Fatalf("import counts first=%+v second=%+v", first, second)
	}
	workoutImport := ImportRequest{
		DataType: "workouts", Filename: "sets.json", Format: "json", Source: "integration",
		Content: `[{"program_name":"Imported workout","started_at":"2026-08-20T18:00","finished_at":"2026-08-20T19:00","status":"finished","external_id":"sets-session-1"}]`,
	}
	if result, err := service.ExecuteImport(ctx, workoutImport); err != nil || result.Imported != 1 {
		t.Fatalf("workout parent import: result=%+v err=%v", result, err)
	}
	firstSetImport := ImportRequest{
		DataType: "sets", Filename: "sets.json", Format: "json", Source: "integration",
		Content: `[{"session_external_id":"sets-session-1","exercise_name":"Imported squat","position":1,"type":"working","reps":8,"external_id":"stable-set-1"}]`,
	}
	if result, err := service.ExecuteImport(ctx, firstSetImport); err != nil || result.Imported != 1 {
		t.Fatalf("first set import: result=%+v err=%v", result, err)
	}
	renamedSetImport := firstSetImport
	renamedSetImport.Content = `[{"session_external_id":"sets-session-1","exercise_name":"Renamed imported squat","position":1,"type":"working","reps":8,"external_id":"stable-set-1"}]`
	if result, err := service.ExecuteImport(ctx, renamedSetImport); err != nil || result.Imported != 0 || result.Skipped != 1 {
		t.Fatalf("renamed duplicate set import: result=%+v err=%v", result, err)
	}
	var importedSets, importedExercises int
	if err := pool.QueryRow(ctx, `SELECT
		count(DISTINCT training_set.id), count(DISTINCT exercise.id)
		FROM training_sessions session
		JOIN training_session_exercises exercise ON exercise.session_id=session.id
		JOIN training_sets training_set ON training_set.session_exercise_id=exercise.id
		WHERE session.owner_id=$1 AND session.source='integration' AND session.external_id='sets-session-1'`, ownerID).
		Scan(&importedSets, &importedExercises); err != nil {
		t.Fatalf("count imported set children: %v", err)
	}
	if importedSets != 1 || importedExercises != 1 {
		t.Fatalf("session-scoped set idempotency failed: sets=%d exercises=%d", importedSets, importedExercises)
	}

	now := time.Date(2026, 8, 21, 12, 0, 0, 0, loc)
	firstSeed, err := repository.SeedDemo(ctx, ownerID, now, loc)
	if err != nil {
		t.Fatalf("first demo seed: %v", err)
	}
	if firstSeed.WorkoutSessions == 0 || firstSeed.RecoveryEntries == 0 || firstSeed.SleepEntries == 0 || firstSeed.NutritionEntries == 0 || firstSeed.BodyMeasurements == 0 {
		t.Fatalf("demo seed is incomplete: %+v", firstSeed)
	}
	var beforeSessions, beforeSets int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM training_sessions WHERE owner_id=$1 AND source='demo'),
		(SELECT count(*) FROM training_sets training_set
		 JOIN training_session_exercises exercise ON exercise.id=training_set.session_exercise_id
		 JOIN training_sessions session ON session.id=exercise.session_id
		 WHERE session.owner_id=$1 AND training_set.source='demo')`, ownerID).Scan(&beforeSessions, &beforeSets); err != nil {
		t.Fatal(err)
	}
	secondSeed, err := repository.SeedDemo(ctx, ownerID, now, loc)
	if err != nil {
		t.Fatalf("second demo seed: %v", err)
	}
	var afterSessions, afterSets int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM training_sessions WHERE owner_id=$1 AND source='demo'),
		(SELECT count(*) FROM training_sets training_set
		 JOIN training_session_exercises exercise ON exercise.id=training_set.session_exercise_id
		 JOIN training_sessions session ON session.id=exercise.session_id
		 WHERE session.owner_id=$1 AND training_set.source='demo')`, ownerID).Scan(&afterSessions, &afterSets); err != nil {
		t.Fatal(err)
	}
	if secondSeed.WorkoutSessions != 0 || beforeSessions != afterSessions || beforeSets != afterSets {
		t.Fatalf("seed is not idempotent: second=%+v before=(%d,%d) after=(%d,%d)", secondSeed, beforeSessions, beforeSets, afterSessions, afterSets)
	}

	range90 := DateRange{From: now.AddDate(0, 0, -89), To: now}
	overview, err := repository.Overview(ctx, ownerID, range90, loc)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if len(overview.Daily) != 90 || overview.Summary.Training.Sessions == 0 {
		t.Fatalf("unexpected overview: daily=%d sessions=%d", len(overview.Daily), overview.Summary.Training.Sessions)
	}
	var exerciseID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM training_exercises WHERE owner_id=$1 ORDER BY id LIMIT 1`, ownerID).Scan(&exerciseID); err != nil {
		t.Fatal(err)
	}
	analytics, err := repository.TrainingAnalytics(ctx, ownerID, range90, loc, AnalyticsFilters{ExerciseID: &exerciseID})
	if err != nil {
		t.Fatalf("training analytics: %v", err)
	}
	if analytics["daily"] == nil || analytics["weekly"] == nil || analytics["daily_duration"] == nil ||
		analytics["muscle_groups"] == nil || analytics["rir_distribution"] == nil || analytics["heatmap"] == nil ||
		analytics["adherence"] == nil || analytics["streak"] == nil {
		t.Fatalf("training analytics lacks real series: %#v", analytics)
	}
	muscleGroups, ok := analytics["muscle_groups"].([]trainingMuscleGroupPoint)
	if !ok || len(muscleGroups) != 1 || muscleGroups[0].MuscleGroup != "legs" || muscleGroups[0].WorkingSets != 1 {
		t.Fatalf("exercise filter did not constrain muscle groups: %#v", analytics["muscle_groups"])
	}
	rir, ok := analytics["rir_distribution"].([]trainingRIRPoint)
	if !ok || len(rir) != 1 || rir[0].RIR != "2" || rir[0].Sets != 1 {
		t.Fatalf("exercise filter did not constrain RIR: %#v", analytics["rir_distribution"])
	}
	duration, ok := analytics["daily_duration"].([]trainingDurationPoint)
	if !ok || len(duration) != 90 || duration[len(duration)-1].DurationMinutes == nil || *duration[len(duration)-1].DurationMinutes != 60 {
		t.Fatalf("filtered duration series is unexpected: %#v", analytics["daily_duration"])
	}
}

func TestPostgresRepository_InBodyRoundTripImportAndAnalytics(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)
	const ownerID int64 = 9_101_023_925
	const otherOwnerID int64 = ownerID + 1
	t.Cleanup(func() {
		if err := repository.DeleteAll(ctx, ownerID); err != nil {
			t.Errorf("cleanup owner: %v", err)
		}
		if err := repository.DeleteAll(ctx, otherOwnerID); err != nil {
			t.Errorf("cleanup other owner: %v", err)
		}
	})
	if err := repository.DeleteAll(ctx, ownerID); err != nil {
		t.Fatal(err)
	}
	if err := repository.DeleteAll(ctx, otherOwnerID); err != nil {
		t.Fatal(err)
	}

	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{
		"measured_at":"2026-08-21T08:00:00+03:00","weight_kg":81.2,"body_fat_percent":18.4,
		"skeletal_muscle_mass_kg":37.8,"total_body_water_l":48.1,"intracellular_water_l":29.8,
		"extracellular_water_l":18.3,"ecw_tbw_ratio":0.38046,"protein_mass_kg":12.9,
		"mineral_mass_kg":4.2,"bmi":24.8,"visceral_fat_level":7,"visceral_fat_area_cm2":72.4,
		"basal_metabolic_rate_kcal":1810,"inbody_score":82,"phase_angle_degrees":6.8,
		"source":"inbody","external_id":"spirit-2026-08-21",
		"segments":[
			{"segment":"left_arm","lean_mass_kg":3.82,"lean_percent":104.5,"fat_mass_kg":0.71,"fat_percent":82},
			{"segment":"right_arm","lean_mass_kg":3.91,"lean_percent":106.1,"fat_mass_kg":0.69,"fat_percent":80},
			{"segment":"trunk","lean_mass_kg":29.4,"lean_percent":102.2,"fat_mass_kg":7.4,"fat_percent":93},
			{"segment":"left_leg","lean_mass_kg":10.11,"lean_percent":99.8,"fat_mass_kg":2.51,"fat_percent":88},
			{"segment":"right_leg","lean_mass_kg":10.18,"lean_percent":100.4,"fat_mass_kg":2.48,"fat_percent":87}
		]
	}`)
	created, err := repository.Create(ctx, ownerID, "body-measurements", payload, loc)
	if err != nil {
		t.Fatalf("create InBody measurement: %v", err)
	}
	var record struct {
		ID                int64                 `json:"id"`
		TotalBodyWaterL   float64               `json:"total_body_water_l"`
		ECWTBWRatio       float64               `json:"ecw_tbw_ratio"`
		PhaseAngleDegrees float64               `json:"phase_angle_degrees"`
		Segments          []BodySegmentSnapshot `json:"segments"`
	}
	if err := json.Unmarshal(created, &record); err != nil {
		t.Fatalf("decode InBody record: %v; raw=%s", err, created)
	}
	if record.ID == 0 || record.TotalBodyWaterL != 48.1 || record.ECWTBWRatio != 0.38046 ||
		record.PhaseAngleDegrees != 6.8 || len(record.Segments) != 5 {
		t.Fatalf("InBody values did not round-trip: %+v; raw=%s", record, created)
	}
	if _, err := repository.Get(ctx, otherOwnerID, "body-measurements", record.ID, loc); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other owner get error = %v, want not found", err)
	}

	// Omitting segments on an update preserves the imported snapshot. Sending an
	// explicit empty array is the opt-in operation for clearing it.
	withoutSegments := json.RawMessage(`{
		"measured_at":"2026-08-21T08:00:00+03:00","weight_kg":81.0,"total_body_water_l":48.1,
		"intracellular_water_l":29.8,"extracellular_water_l":18.3,"ecw_tbw_ratio":0.38046,
		"source":"inbody","external_id":"spirit-2026-08-21"
	}`)
	updated, err := repository.Update(ctx, ownerID, "body-measurements", record.ID, withoutSegments, loc)
	if err != nil {
		t.Fatalf("update InBody measurement: %v", err)
	}
	if err := json.Unmarshal(updated, &record); err != nil || len(record.Segments) != 5 {
		t.Fatalf("omitted segments were not preserved: err=%v raw=%s", err, updated)
	}

	day := time.Date(2026, 8, 21, 0, 0, 0, 0, loc)
	overview, err := repository.Overview(ctx, ownerID, DateRange{From: day, To: day}, loc)
	if err != nil {
		t.Fatalf("InBody overview: %v", err)
	}
	if len(overview.Daily) != 1 || overview.Daily[0].TotalBodyWaterL == nil ||
		len(overview.Daily[0].BodySegments) != 5 || overview.Summary.Body.ECWTBWRatio.Samples != 1 ||
		len(overview.Summary.Body.Segments) != 5 {
		t.Fatalf("InBody analytics incomplete: daily=%+v summary=%+v", overview.Daily, overview.Summary.Body)
	}

	service := NewService(repository, ownerID, loc)
	importRequest := ImportRequest{
		DataType: "body", Format: "csv", Source: "inbody", Filename: "inbody.csv",
		Content: "measured_at,weight_kg,total_body_water_l,ecw_tbw_ratio,left_arm_lean_mass_kg,left_arm_lean_percent,external_id\n" +
			"2026-08-20T08:00,81.4,48.0,0.381,3.79,103.7,spirit-2026-08-20\n",
	}
	first, err := service.ExecuteImport(ctx, importRequest)
	if err != nil || first.Imported != 1 {
		t.Fatalf("first InBody import: result=%+v err=%v", first, err)
	}
	second, err := service.ExecuteImport(ctx, importRequest)
	if err != nil || second.Imported != 0 || second.Skipped != 1 {
		t.Fatalf("duplicate InBody import: result=%+v err=%v", second, err)
	}
	var importedSegments int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM body_segment_measurements segment
		JOIN body_measurements measurement ON measurement.id=segment.body_measurement_id
			AND measurement.owner_id=segment.owner_id
		WHERE measurement.owner_id=$1 AND measurement.external_id='spirit-2026-08-20'`, ownerID).Scan(&importedSegments); err != nil {
		t.Fatal(err)
	}
	if importedSegments != 1 {
		t.Fatalf("imported segment rows = %d, want 1", importedSegments)
	}
}

func TestTrainingAnalyticsKeepsScheduleAndActualDatesSeparate(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)
	const ownerID int64 = 9_101_023_921
	const otherOwnerID int64 = ownerID + 100
	cleanup := func() {
		if err := repository.DeleteAll(ctx, ownerID); err != nil {
			t.Fatalf("cleanup owner: %v", err)
		}
		if err := repository.DeleteAll(ctx, otherOwnerID); err != nil {
			t.Fatalf("cleanup other owner: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	scheduled := time.Date(2026, 8, 20, 23, 30, 0, 0, loc)
	started := time.Date(2026, 8, 21, 0, 10, 0, 0, loc)
	finished := started.Add(50 * time.Minute)
	if _, err := pool.Exec(ctx, `INSERT INTO training_sessions
		(owner_id,program_name,status,current_position,scheduled_at,started_at,finished_at,source)
		VALUES ($1,'Cross-midnight','finished',1,$2,$3,$4,'manual'),
			($1,'Missed','cancelled',1,$2,NULL,NULL,'manual'),
			($1,'Excused','excused',1,$2,NULL,NULL,'manual'),
			($5,'Other owner','finished',1,$2,$3,$4,'manual')`,
		ownerID, scheduled, started, finished, otherOwnerID); err != nil {
		t.Fatalf("insert schedule regression rows: %v", err)
	}

	scheduleRange := DateRange{From: time.Date(2026, 8, 20, 0, 0, 0, 0, loc), To: time.Date(2026, 8, 20, 0, 0, 0, 0, loc)}
	analytics, err := repository.TrainingAnalytics(ctx, ownerID, scheduleRange, loc, AnalyticsFilters{})
	if err != nil {
		t.Fatalf("scheduled-day analytics: %v", err)
	}
	adherence, ok := analytics["adherence"].([]trainingAdherencePoint)
	if !ok || len(adherence) != 1 {
		t.Fatalf("adherence series type/length: %#v", analytics["adherence"])
	}
	if adherence[0].Date != "2026-08-20" || adherence[0].Planned != 2 || adherence[0].Completed != 1 || adherence[0].Percentage == nil || *adherence[0].Percentage != 50 {
		t.Fatalf("scheduled-day adherence = %#v", adherence[0])
	}
	summary, ok := analytics["summary"].(TrainingSummary)
	if !ok || summary.Adherence == nil || *summary.Adherence != 50 {
		t.Fatalf("scheduled-day summary = %#v", analytics["summary"])
	}
	daily, ok := analytics["daily"].([]DailyPoint)
	if !ok || len(daily) != 1 || daily[0].WorkoutCount != 0 {
		t.Fatalf("workout leaked onto scheduled date: %#v", analytics["daily"])
	}

	actualRange := DateRange{From: time.Date(2026, 8, 21, 0, 0, 0, 0, loc), To: time.Date(2026, 8, 21, 0, 0, 0, 0, loc)}
	analytics, err = repository.TrainingAnalytics(ctx, ownerID, actualRange, loc, AnalyticsFilters{})
	if err != nil {
		t.Fatalf("actual-day analytics: %v", err)
	}
	daily = analytics["daily"].([]DailyPoint)
	if daily[0].WorkoutCount != 1 {
		t.Fatalf("actual workout missing from started date: %#v", daily[0])
	}
	adherence = analytics["adherence"].([]trainingAdherencePoint)
	if adherence[0].Planned != 0 || adherence[0].Completed != 0 || adherence[0].Percentage != nil {
		t.Fatalf("schedule leaked onto actual date: %#v", adherence[0])
	}
}

func TestPostgresRepository_SourceFreshnessAndImportExportAreOwnerScoped(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)
	const ownerID int64 = 9_101_023_922
	const otherOwnerID int64 = ownerID + 1
	cleanup := func() {
		if err := repository.DeleteAll(ctx, ownerID); err != nil {
			t.Fatalf("cleanup owner: %v", err)
		}
		if err := repository.DeleteAll(ctx, otherOwnerID); err != nil {
			t.Fatalf("cleanup other owner: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	insertTrainingSources := func(owner int64, sessionUpdated, setUpdated time.Time) {
		t.Helper()
		var sessionID int64
		if err := pool.QueryRow(ctx, `INSERT INTO training_sessions (
			owner_id,program_name,status,current_position,started_at,finished_at,source,updated_at)
			VALUES ($1,'Provider workout','finished',1,$2,$2,'whoop',$3) RETURNING id`,
			owner, sessionUpdated.Add(-time.Hour), sessionUpdated).Scan(&sessionID); err != nil {
			t.Fatalf("insert provider session: %v", err)
		}
		var exerciseID int64
		if err := pool.QueryRow(ctx, `INSERT INTO training_session_exercises (session_id,position,name)
			VALUES ($1,1,'Provider exercise') RETURNING id`, sessionID).Scan(&exerciseID); err != nil {
			t.Fatalf("insert provider session exercise: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO training_sets (
			session_exercise_id,position,type,actual_reps,completed_at,source,updated_at)
			VALUES ($1,1,'working',8,$2,'fatsecret',$2)`, exerciseID, setUpdated); err != nil {
			t.Fatalf("insert provider set: %v", err)
		}
	}

	ownerSessionUpdated := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	ownerSetUpdated := time.Date(2026, 8, 19, 11, 0, 0, 0, time.UTC)
	insertTrainingSources(ownerID, ownerSessionUpdated, ownerSetUpdated)
	insertTrainingSources(otherOwnerID, ownerSessionUpdated.Add(72*time.Hour), ownerSetUpdated.Add(72*time.Hour))

	statuses, err := repository.Sources(ctx, ownerID)
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	bySource := make(map[string]SourceStatus, len(statuses))
	for _, status := range statuses {
		bySource[status.Source] = status
		if status.Connected || status.Status != "file_import_only" {
			t.Fatalf("source %q falsely presented as connected: %+v", status.Source, status)
		}
	}
	if got := bySource["whoop"].LastSyncedAt; got == nil || !got.Equal(ownerSessionUpdated) {
		t.Fatalf("WHOOP freshness = %v, want owner session timestamp %v", got, ownerSessionUpdated)
	}
	if got := bySource["fatsecret"].LastSyncedAt; got == nil || !got.Equal(ownerSetUpdated) {
		t.Fatalf("FatSecret freshness = %v, want owner set timestamp %v", got, ownerSetUpdated)
	}

	errorSummary := json.RawMessage(`[{"row":7,"message":"invalid calories","fields":{"calories_kcal":"must be non-negative"}}]`)
	var importID int64
	if err := pool.QueryRow(ctx, `INSERT INTO data_imports (
		owner_id,source,data_type,filename,format,status,total_rows,failed_rows,error_summary,completed_at)
		VALUES ($1,'fatsecret','nutrition','owner.csv','csv','failed',1,1,$2::jsonb,now()) RETURNING id`,
		ownerID, errorSummary).Scan(&importID); err != nil {
		t.Fatalf("insert owner import: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO data_imports (
		owner_id,source,data_type,filename,format,status,total_rows,failed_rows,error_summary,completed_at)
		VALUES ($1,'fatsecret','nutrition','other.csv','csv','failed',1,1,
			'[{"row":99,"message":"other owner"}]'::jsonb,now())`, otherOwnerID); err != nil {
		t.Fatalf("insert other owner import: %v", err)
	}

	list, err := repository.List(ctx, ownerID, "imports", Pagination{Page: 1, PageSize: 25}, time.UTC)
	if err != nil {
		t.Fatalf("list imports: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("owner import list = total %d, items %d", list.Total, len(list.Items))
	}
	var listProjection map[string]json.RawMessage
	if err := json.Unmarshal(list.Items[0], &listProjection); err != nil {
		t.Fatalf("decode import list projection: %v", err)
	}
	if _, exists := listProjection["errors"]; exists {
		t.Fatal("compact import list unexpectedly contains error details")
	}

	exported, err := repository.ExportAll(ctx, ownerID, time.UTC)
	if err != nil {
		t.Fatalf("export all: %v", err)
	}
	var document struct {
		Imports []struct {
			ID     int64            `json:"id"`
			Errors []ImportRowError `json:"errors"`
		} `json:"imports"`
	}
	if err := json.Unmarshal(exported, &document); err != nil {
		t.Fatalf("decode full export: %v", err)
	}
	if len(document.Imports) != 1 || document.Imports[0].ID != importID {
		t.Fatalf("full export leaked or lost an import: %+v", document.Imports)
	}
	if len(document.Imports[0].Errors) != 1 || document.Imports[0].Errors[0].Row != 7 ||
		document.Imports[0].Errors[0].Message != "invalid calories" {
		t.Fatalf("full export lost import error_summary: %+v", document.Imports[0].Errors)
	}
}
