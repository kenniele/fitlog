package fatsecret

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestClient stands up an httptest server, points the client at it,
// and returns both so the test can inspect captured requests.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	signer := NewSigner("ck", "cs", "at", "asec")
	signer.nowFn = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	signer.nonceFn = func() (string, error) { return "deadbeefdeadbeef", nil }

	c := NewClient(signer, Options{Endpoint: srv.URL})
	return c, srv
}

func TestClient_ProfileGet_HappyPath(t *testing.T) {
	var capturedForm url.Values
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		var err error
		capturedForm, err = url.ParseQuery(string(body))
		require.NoError(t, err)
		_, _ = w.Write([]byte(`{"profile":{"user_id":"user-42"}}`))
	})

	id, err := c.ProfileGet(context.Background())
	require.NoError(t, err)
	require.Equal(t, "user-42", id)

	// All OAuth params travel in the BODY, not headers or query.
	require.Equal(t, "profile.get", capturedForm.Get("method"))
	require.Equal(t, "json", capturedForm.Get("format"))
	require.Equal(t, "ck", capturedForm.Get("oauth_consumer_key"))
	require.Equal(t, "at", capturedForm.Get("oauth_token"))
	require.Equal(t, "HMAC-SHA1", capturedForm.Get("oauth_signature_method"))
	require.NotEmpty(t, capturedForm.Get("oauth_signature"))
}

func TestClient_FoodEntriesForDay(t *testing.T) {
	var capturedDate string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		capturedDate = form.Get("date")
		require.Equal(t, "food_entries.get.v2", form.Get("method"))
		_, _ = w.Write([]byte(`{"food_entries":{"food_entry":{"food_entry_id":"1","food_entry_name":"Egg","date_int":"20585","meal":"Breakfast","calories":"70"}}}`))
	})

	day := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	entries, err := c.FoodEntriesForDay(context.Background(), day)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "Egg", entries[0].FoodName)
	require.Equal(t, "20584", capturedDate)
}

func TestClient_FoodEntriesMonth(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		require.Equal(t, "food_entries.get_month", form.Get("method"))
		_, _ = w.Write([]byte(`{"month":{"from_date_int":"20580","to_date_int":"20585","day":[{"date_int":"20580","calories":"2000","protein":"100","carbohydrate":"200","fat":"60"}]}}`))
	})

	days, err := c.FoodEntriesMonth(context.Background(), time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, days, 1)
	require.Equal(t, 20580, days[0].DateInt)
}

func TestClient_ErrorEnvelope(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"code":5,"message":"Invalid signature"}}`))
	})

	_, err := c.ProfileGet(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid signature")
	require.Contains(t, err.Error(), "5")
}

func TestClient_HTTPError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	_, err := c.ProfileGet(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}

func TestClient_ContextCancel(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := c.ProfileGet(ctx)
	require.Error(t, err)
}

func TestTruncate(t *testing.T) {
	require.Equal(t, "abc", truncate("abc", 5))
	require.Equal(t, "abc…", truncate("abcdef", 3))
	require.True(t, strings.HasSuffix(truncate("xxxxxxxxxx", 3), "…"))
}
