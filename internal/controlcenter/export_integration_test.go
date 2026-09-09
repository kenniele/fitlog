package controlcenter

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresSessionExports(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	const owner = 9_109_090_001
	const other = owner + 1
	cleanup := func() {
		if _, err := pool.Exec(ctx, "DELETE FROM training_sessions WHERE owner_id IN ($1,$2)", owner, other); err != nil {
			t.Fatal(err)
		}
	}
	cleanup()
	defer cleanup()
	var firstID int64
	err = pool.QueryRow(ctx, `WITH inserted AS (
		INSERT INTO training_sessions (owner_id,program_name,status,started_at)
		SELECT $1,'Экспорт','finished','2020-01-01 21:30:00+00'::timestamptz + (n-1)*interval '1 day'
		FROM generate_series(1,101) n RETURNING id
	) SELECT min(id) FROM inserted`, owner).Scan(&firstID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO training_sessions (owner_id,program_name,status,scheduled_at)
		VALUES ($1,'Будущая','scheduled','2099-01-01'),($2,'Чужая','scheduled','2099-01-01')`, owner, other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE training_sessions SET scheduled_at='2019-12-31 12:00:00+00' WHERE id=$1`, firstID); err != nil {
		t.Fatal(err)
	}
	const name = "=Тяга, \"гантели\""
	const note = "Первая строка\nВторая строка"
	var exerciseID int64
	err = pool.QueryRow(ctx, `INSERT INTO training_session_exercises (session_id,position,name,note)
		VALUES ($1,1,$2,$3) RETURNING id`, firstID, name, note).Scan(&exerciseID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO training_session_exercises (session_id,position,name)
		VALUES ($1,2,'Без подходов')`, firstID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO training_sets
		(session_exercise_id,position,type,actual_weight_kg,actual_reps,actual_rir,rest_seconds,notes,completed_at)
		VALUES ($1,1,'warmup',NULL,12,NULL,0,$2,now()),
		       ($1,2,'working',40,10,0,120,'',now()),($1,3,'drop',30,8,1,60,'',now())`, exerciseID, note); err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(pool), owner, loc)
	content, err := service.ExportSessions(ctx, Pagination{Page: 3, PageSize: 1}, "json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Sessions []struct {
			ID        int64  `json:"id"`
			Date      string `json:"date"`
			Exercises []struct {
				Name string `json:"name"`
				Sets []struct {
					Type   string   `json:"type"`
					Weight *float64 `json:"weight_kg"`
					RIR    *float64 `json:"rir"`
				} `json:"sets"`
			} `json:"exercises"`
		} `json:"workout_sessions"`
	}
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Sessions) != 102 || strings.Contains(string(content), "Чужая") {
		t.Fatalf("JSON history lost or leaked records: %d", len(document.Sessions))
	}
	for _, session := range document.Sessions {
		if session.ID != firstID {
			continue
		}
		if session.Date != "2020-01-02" || len(session.Exercises) != 2 || session.Exercises[0].Name != name {
			t.Fatalf("session details changed: %+v", session)
		}
		sets := session.Exercises[0].Sets
		if len(sets) != 3 || sets[0].Type != "warmup" || sets[0].Weight != nil ||
			sets[1].RIR == nil || *sets[1].RIR != 0 || sets[2].Type != "drop" {
			t.Fatalf("sets changed: %+v", sets)
		}
	}
	content, err = service.ExportSessions(ctx, Pagination{}, "csv")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(content))).ReadAll()
	if err != nil || len(rows) != 106 || strings.Contains(string(content), "Чужая") {
		t.Fatalf("CSV history lost or leaked rows: count=%d err=%v", len(rows), err)
	}
	first := rows[1]
	if first[7] != "'"+name || first[8] != note || first[10] != "warmup" || first[11] != "" || first[13] != "" || first[14] != "0" {
		t.Fatalf("CSV quoting, nulls or types changed: %#v", first)
	}
	date := time.Date(2019, 12, 31, 0, 0, 0, 0, loc)
	filters := Pagination{From: &date, To: &date, Filters: map[string]string{"date_basis": "calendar"}}
	for _, format := range []string{"csv", "json"} {
		content, err = service.ExportSessions(ctx, filters, format)
		if err != nil || !strings.Contains(string(content), "Экспорт") || strings.Contains(string(content), "Будущая") {
			t.Fatalf("calendar range %s failed: %s %v", format, content, err)
		}
	}
}
