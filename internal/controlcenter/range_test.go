package controlcenter

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDateRangeDefaultsToThirtyInclusiveLocalDays(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	request := httptest.NewRequest("GET", "/dashboard/overview", nil)

	dateRange, err := ParseDateRange(request, loc, time.Date(2026, 8, 21, 23, 30, 0, 0, loc))
	require.NoError(t, err)
	require.Equal(t, "2026-07-23", dateRange.From.Format("2006-01-02"))
	require.Equal(t, "2026-08-21", dateRange.To.Format("2006-01-02"))
	require.Equal(t, 30, dateRange.Days())
}

func TestParseDateRangeUsesCalendarDaysAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	request := httptest.NewRequest("GET", "/analytics/recovery?from=2026-03-07&to=2026-03-10&compare=true", nil)

	dateRange, err := ParseDateRange(request, loc, time.Now())
	require.NoError(t, err)
	require.Equal(t, 4, dateRange.Days())
	require.True(t, dateRange.Compare)
	require.Equal(t, "2026-03-03", dateRange.Previous().From.Format("2006-01-02"))
	require.Equal(t, "2026-03-06", dateRange.Previous().To.Format("2006-01-02"))
}

func TestParseDateRangeRejectsInvalidAndUnboundedInputs(t *testing.T) {
	for _, path := range []string{
		"/?from=2026-08-22&to=2026-08-21",
		"/?from=2025-01-01&to=2026-01-02",
		"/?from=not-a-date",
		"/?compare=sometimes",
	} {
		_, err := ParseDateRange(httptest.NewRequest("GET", path, nil), time.UTC, time.Now())
		require.Error(t, err, path)
	}
}

func TestParsePaginationAndAnalyticsFilters(t *testing.T) {
	request := httptest.NewRequest("GET", "/workout-sessions?page=2&page_size=100&status=finished&exercise_id=7&date_basis=calendar&from=2026-08-01&to=2026-08-21", nil)
	pagination, err := ParsePagination(request, time.UTC)
	require.NoError(t, err)
	require.Equal(t, 2, pagination.Page)
	require.Equal(t, 100, pagination.PageSize)
	require.Equal(t, "finished", pagination.Filters["status"])
	require.Equal(t, "7", pagination.Filters["exercise_id"])
	require.Equal(t, "calendar", pagination.Filters["date_basis"])
	_, err = ParsePagination(httptest.NewRequest("GET", "/workout-sessions?date_basis=sideways", nil), time.UTC)
	require.Error(t, err)

	analyticsRequest := httptest.NewRequest("GET", "/analytics/correlations?exercise_id=7&plan_id=8&template_id=9&status=finished&day_type=training", nil)
	filters, err := ParseAnalyticsFilters(analyticsRequest)
	require.NoError(t, err)
	require.Equal(t, int64(7), *filters.ExerciseID)
	require.Equal(t, int64(8), *filters.PlanID)
	require.Equal(t, int64(9), *filters.TemplateID)
	require.Equal(t, "finished", filters.Status)
	require.Equal(t, "training", filters.DayType)

	_, err = ParsePagination(httptest.NewRequest("GET", "/?page_size=101", nil), time.UTC)
	require.Error(t, err)
	_, err = ParsePagination(httptest.NewRequest("GET", "/?status=unknown", nil), time.UTC)
	require.Error(t, err)
	for _, path := range []string{
		"/?from=2026-08-01",
		"/?from=2026-08-22&to=2026-08-21",
		"/?from=2025-01-01&to=2026-01-02",
	} {
		_, err = ParsePagination(httptest.NewRequest("GET", path, nil), time.UTC)
		require.Error(t, err, path)
	}
	_, err = ParseAnalyticsFilters(httptest.NewRequest("GET", "/?day_type=weekend", nil))
	require.Error(t, err)
}
