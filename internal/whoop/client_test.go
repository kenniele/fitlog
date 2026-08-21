package whoop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fitlog/internal/domain"
)

func TestFormatWhoopTime(t *testing.T) {
	in := time.Date(2026, 5, 11, 3, 14, 46, 595000000, time.UTC)
	require.Equal(t, "2026-05-11T03:14:46.595Z", formatWhoopTime(in))
}

func TestRangedQuery(t *testing.T) {
	rng := domain.TimeRange{
		From: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
	}
	q := rangedQuery(rng, 25)
	require.Equal(t, "2026-05-01T00:00:00.000Z", q.Get("start"))
	require.Equal(t, "2026-05-11T00:00:00.000Z", q.Get("end"))
	require.Equal(t, "25", q.Get("limit"))
}

func TestClient_Cycles_SinglePage(t *testing.T) {
	var capturedQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/cycle", r.URL.Path)
		capturedQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{
            "records": [
                {"id": 1, "start": "2026-05-11T00:00:00.000Z", "end": "2026-05-12T00:00:00.000Z",
                 "score_state": "SCORED",
                 "score": {"strain": 13.6, "kilojoule": 9000, "average_heart_rate": 66, "max_heart_rate": 156}}
            ]
        }`))
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), Options{BaseURL: srv.URL})
	rng := domain.TimeRange{
		From: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC),
	}
	out, err := c.Cycles(context.Background(), rng, 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, int64(1), out[0].ID)
	require.Equal(t, 13.6, out[0].Strain)

	require.Equal(t, "2026-05-11T00:00:00.000Z", capturedQuery.Get("start"))
	require.Equal(t, "10", capturedQuery.Get("limit"))
}

func TestClient_Sleeps_Paginates(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		switch n {
		case 1:
			require.Empty(t, r.URL.Query().Get("nextToken"))
			_, _ = w.Write([]byte(`{
                "records": [{"id":"s1","start":"2026-05-10T00:00:00.000Z","end":"2026-05-10T07:00:00.000Z"}],
                "next_token": "tok-2"
            }`))
		case 2:
			require.Equal(t, "tok-2", r.URL.Query().Get("nextToken"))
			_, _ = w.Write([]byte(`{
                "records": [{"id":"s2","start":"2026-05-11T00:00:00.000Z","end":"2026-05-11T07:00:00.000Z"}]
            }`))
		default:
			t.Fatalf("unexpected extra page call: %d", n)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), Options{BaseURL: srv.URL})
	out, err := c.Sleeps(context.Background(), domain.TimeRange{}, 0)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, "s1", out[0].ExternalID)
	require.Equal(t, "s2", out[1].ExternalID)
	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestClient_Pagination_HardCap(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		// Always return a next_token → would loop forever without the cap.
		_, _ = fmt.Fprintf(w, `{"records":[{"id":%d}],"next_token":"t-%d"}`, n, n)
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), Options{BaseURL: srv.URL})
	out, err := c.Cycles(context.Background(), domain.TimeRange{}, 0)
	require.ErrorContains(t, err, "pagination exceeded")
	require.Equal(t, int32(maxPages), atomic.LoadInt32(&calls), "must stop at maxPages")
	require.Nil(t, out, "a truncated provider result must never look successful")
}

func TestClient_Workouts_Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), Options{BaseURL: srv.URL})
	_, err := c.Workouts(context.Background(), domain.TimeRange{}, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestClient_Recoveries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/recovery", r.URL.Path)
		_, _ = w.Write([]byte(`{"records":[{"cycle_id":12345,"sleep_id":"s","score_state":"SCORED","score":{"recovery_score":72,"hrv_rmssd_milli":73,"resting_heart_rate":53}}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.Client(), Options{BaseURL: srv.URL})
	out, err := c.Recoveries(context.Background(), domain.TimeRange{}, 0)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, 72.0, out[0].Score)
	require.Equal(t, "SCORED", out[0].ScoreState)
}
