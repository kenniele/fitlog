package fatsecret

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
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
		require.Equal(t, http.MethodGet, r.Method)
		capturedDate = r.URL.Query().Get("date")
		require.Empty(t, r.URL.Query().Get("method"))
		require.NotEmpty(t, r.URL.Query().Get("oauth_signature"))
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
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"error":{"code":5,"message":"Invalid signature"}}`))
	})

	_, err := c.ProfileGet(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid signature")
	require.Contains(t, err.Error(), "5")
	require.Equal(t, 1, calls)
}

func TestClient_HTTPError(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	_, err := c.ProfileGet(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
	require.Equal(t, 1, calls)
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClient_RetriesTimeoutWithFreshSignature(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query bool
	}{
		{name: "monthly form"},
		{name: "daily query", query: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var requests []url.Values
				signer := NewSigner("ck", "cs", "at", "asec")
				client := NewClient(signer, Options{HTTPClient: &http.Client{
					Timeout: 10 * time.Second,
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						require.NoError(t, req.ParseForm())
						params := req.PostForm
						if tc.query {
							require.Equal(t, http.MethodGet, req.Method)
							params = req.URL.Query()
						} else {
							require.Equal(t, http.MethodPost, req.Method)
							require.Equal(t, "food_entries.get_month", params.Get("method"))
						}
						require.NotEmpty(t, params.Get("date"))
						requests = append(requests, params)
						if len(requests) == 1 {
							<-req.Context().Done()
							return nil, req.Context().Err()
						}
						return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
					}),
				}})
				if tc.query {
					_, err := client.FoodEntriesForDay(t.Context(), time.Now())
					require.NoError(t, err)
				} else {
					_, err := client.FoodEntriesMonth(t.Context(), time.Now())
					require.NoError(t, err)
				}
				require.Len(t, requests, 2)
				require.NotEqual(t, requests[0].Get("oauth_nonce"), requests[1].Get("oauth_nonce"))
				require.NotEqual(t, requests[0].Get("oauth_signature"), requests[1].Get("oauth_signature"))
			})
		})
	}
}

func TestClient_TimeoutRetryIsBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		client := NewClient(NewSigner("ck", "cs", "at", "asec"), Options{HTTPClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, context.DeadlineExceeded
			}),
		}})
		_, err := client.FoodEntriesMonth(t.Context(), time.Now())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 2, calls)
	})
}

func TestClient_RetryRespectsContext(t *testing.T) {
	for _, tc := range []struct {
		name            string
		cancelInRequest bool
	}{
		{name: "canceled request", cancelInRequest: true},
		{name: "canceled during backoff"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if !tc.cancelInRequest {
					time.AfterFunc(100*time.Millisecond, cancel)
				}
				calls := 0
				client := NewClient(NewSigner("ck", "cs", "at", "asec"), Options{HTTPClient: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						calls++
						if tc.cancelInRequest {
							cancel()
							return nil, context.Canceled
						}
						return nil, context.DeadlineExceeded
					}),
				}})
				_, err := client.FoodEntriesMonth(ctx, time.Now())
				require.ErrorIs(t, err, context.Canceled)
				require.Equal(t, 1, calls)
			})
		})
	}
}

func TestTruncate(t *testing.T) {
	require.Equal(t, "abc", truncate("abc", 5))
	require.Equal(t, "abc…", truncate("abcdef", 3))
	require.True(t, strings.HasSuffix(truncate("xxxxxxxxxx", 3), "…"))
}
