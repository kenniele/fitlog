package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"fitlog/internal/domain"
)

// NotesRepo persists user log entries (weight, notes, symptoms).
type NotesRepo struct {
	pool *pgxpool.Pool
}

// NewNotesRepo wires a pgxpool.
func NewNotesRepo(pool *pgxpool.Pool) *NotesRepo {
	return &NotesRepo{pool: pool}
}

// Insert records a new note. ts defaults to now() if zero.
func (r *NotesRepo) Insert(ctx context.Context, n domain.Note) (int64, error) {
	const q = `
        INSERT INTO notes (ts, kind, value, body)
        VALUES (COALESCE(NULLIF($1, 'epoch'::timestamptz), now()), $2, $3, $4)
        RETURNING id
    `
	ts := n.Ts
	if ts.IsZero() {
		ts = time.Unix(0, 0) // sentinel; SQL falls back to now()
	}
	var id int64
	if err := r.pool.QueryRow(ctx, q, ts, n.Kind, n.Value, n.Body).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert note: %w", err)
	}
	return id, nil
}

// ListBetween returns notes with ts in [from, to), newest first.
func (r *NotesRepo) ListBetween(ctx context.Context, from, to time.Time) ([]domain.Note, error) {
	const q = `
        SELECT id, ts, kind, value, body
        FROM notes
        WHERE ts >= $1 AND ts < $2
        ORDER BY ts DESC
    `
	rows, err := r.pool.Query(ctx, q, from, to)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var out []domain.Note
	for rows.Next() {
		var n domain.Note
		var v *float64
		if err := rows.Scan(&n.ID, &n.Ts, &n.Kind, &v, &n.Body); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		n.Value = v
		out = append(out, n)
	}
	return out, rows.Err()
}
