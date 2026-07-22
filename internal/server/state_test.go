package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type healthChecker struct{ err error }

func (h healthChecker) Ping(context.Context) error { return h.err }

func TestHealthzChecksDatabase(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	ok := httptest.NewRecorder()
	Router(nil, healthChecker{}, nil).ServeHTTP(ok, request)
	require.Equal(t, http.StatusOK, ok.Code)

	unavailable := httptest.NewRecorder()
	Router(nil, healthChecker{err: errors.New("down")}, nil).ServeHTTP(unavailable, request)
	require.Equal(t, http.StatusServiceUnavailable, unavailable.Code)
}

func TestRouterMountsArticles(t *testing.T) {
	var gotPath string
	articles := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/articles/article-id", nil)
	Router(nil, healthChecker{}, articles).ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "/article-id", gotPath)
}

func TestStateStore_IssueConsume(t *testing.T) {
	s := NewStateStore()
	st, err := s.Issue(42)
	require.NoError(t, err)
	require.NotEmpty(t, st)

	cid, ok := s.Consume(st)
	require.True(t, ok)
	require.Equal(t, int64(42), cid)

	// Single-use.
	_, ok = s.Consume(st)
	require.False(t, ok)
}

func TestStateStore_Expires(t *testing.T) {
	s := NewStateStore()
	frozen := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return frozen }

	st, err := s.Issue(99)
	require.NoError(t, err)

	// Advance past TTL.
	s.now = func() time.Time { return frozen.Add(stateTTL + time.Second) }

	_, ok := s.Consume(st)
	require.False(t, ok, "state should be GC'd after TTL")
}

func TestStateStore_UnknownState(t *testing.T) {
	s := NewStateStore()
	_, ok := s.Consume("never-issued")
	require.False(t, ok)
}
