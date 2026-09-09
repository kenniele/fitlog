package controlcenter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ExportSessions exports every matching session, regardless of the list page.
// Nil date bounds include the owner's entire training history.
func (s *Service) ExportSessions(ctx context.Context, filters Pagination, format string) ([]byte, error) {
	if (filters.From == nil) != (filters.To == nil) {
		return nil, &ValidationError{
			Message: "invalid export range", Fields: map[string]string{"range": "use from and to together"},
		}
	}
	switch format {
	case "csv":
		dateRange := DateRange{}
		if filters.From != nil && filters.To != nil {
			dateRange.From, dateRange.To = *filters.From, *filters.To
		}
		return s.ExportSessionsCSV(ctx, dateRange, filters)
	case "json":
		return s.exportSessionsJSON(ctx, filters)
	default:
		return nil, &ValidationError{
			Message: "invalid export format", Fields: map[string]string{"format": "use csv or json"},
		}
	}
}

func (s *Service) exportSessionsJSON(ctx context.Context, filters Pagination) ([]byte, error) {
	loc := s.location(ctx)
	items := make([]json.RawMessage, 0)
	filters.PageSize = MaxPageSize
	for filters.Page = 1; ; filters.Page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, err := s.store.List(ctx, s.ownerID, "workout-sessions", filters, loc)
		if err != nil {
			return nil, fmt.Errorf("export sessions: %w", err)
		}
		items = append(items, result.Items...)
		if len(items) >= result.Total || len(result.Items) == 0 {
			break
		}
	}
	return json.MarshalIndent(struct {
		Version         int               `json:"version"`
		ExportedAt      time.Time         `json:"exported_at"`
		Timezone        string            `json:"timezone"`
		WorkoutSessions []json.RawMessage `json:"workout_sessions"`
	}{
		Version: 1, ExportedAt: s.now().UTC(), Timezone: loc.String(), WorkoutSessions: items,
	}, "", "  ")
}
