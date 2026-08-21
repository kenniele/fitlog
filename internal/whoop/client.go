package whoop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/oauth2"

	"fitlog/internal/domain"
)

// whoopTimeFormat matches the format Whoop expects in start/end query params:
// RFC3339 with millisecond precision and a trailing Z. time.RFC3339Nano gives
// nanos and is also accepted, but the millis form is what the docs show.
const whoopTimeFormat = "2006-01-02T15:04:05.000Z"

// maxPages caps the number of pagination round trips. WHOOP returns up to 25
// records per page. A 366-day backfill (plus naps) needs more than ten pages;
// 100 still guards against a broken or looping pagination token.
const maxPages = 100

// Client speaks the Whoop v2 REST API.
type Client struct {
	baseURL string
	http    *http.Client
}

// Options configure a Client. Zero values are fine.
type Options struct {
	BaseURL    string        // default: APIBase
	HTTPClient *http.Client  // if nil, one is built from the token source
	Timeout    time.Duration // default: 15s; applied to the built client
}

// NewClient wires an oauth2.Client (or any *http.Client) and a base URL.
//
// The most common construction is to pass an oauth2-backed http.Client, e.g.:
//
//	hc := oauth2.NewClient(ctx, persistingTokenSource)
//	c := whoop.NewClient(hc, whoop.Options{})
func NewClient(hc *http.Client, opts Options) *Client {
	base := opts.BaseURL
	if base == "" {
		base = APIBase
	}
	if hc == nil {
		t := opts.Timeout
		if t == 0 {
			t = 15 * time.Second
		}
		hc = &http.Client{Timeout: t}
	}
	return &Client{baseURL: base, http: hc}
}

// NewClientWithTokenSource is a convenience wrapper that builds the underlying
// oauth2-aware http.Client from a TokenSource.
func NewClientWithTokenSource(ctx context.Context, ts oauth2.TokenSource, opts Options) *Client {
	hc := oauth2.NewClient(ctx, ts)
	if opts.Timeout > 0 {
		hc.Timeout = opts.Timeout
	} else {
		hc.Timeout = 15 * time.Second
	}
	return NewClient(hc, opts)
}

// formatWhoopTime renders a time as Whoop expects.
func formatWhoopTime(t time.Time) string {
	return t.UTC().Format(whoopTimeFormat)
}

// rangedQuery builds the start/end/limit query string used by all range endpoints.
func rangedQuery(rng domain.TimeRange, limit int) url.Values {
	q := url.Values{}
	if !rng.From.IsZero() {
		q.Set("start", formatWhoopTime(rng.From))
	}
	if !rng.To.IsZero() {
		q.Set("end", formatWhoopTime(rng.To))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return q
}

// fetchPaged loops through a paged endpoint, accumulating records into out.
// path is the API path (e.g. "/v2/cycle"); the function appends query params
// including pagination token automatically.
func fetchPaged[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, error) {
	var all []T
	nextToken := ""
	for page := 0; page < maxPages; page++ {
		if nextToken != "" {
			q.Set("nextToken", nextToken)
		}
		u := c.baseURL + path
		if encoded := q.Encode(); encoded != "" {
			u += "?" + encoded
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("new request %s: %w", path, err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http do %s: %w", path, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", path, readErr)
		}
		if resp.StatusCode/100 != 2 {
			return nil, fmt.Errorf("whoop %s http %d: %s", path, resp.StatusCode, truncate(string(body), 256))
		}

		var env pageEnvelope[T]
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %w", path, err)
		}
		all = append(all, env.Records...)

		if env.NextToken == "" {
			return all, nil
		}
		nextToken = env.NextToken
	}
	return nil, fmt.Errorf("whoop %s pagination exceeded %d pages", path, maxPages)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Cycles returns Whoop physiological cycles in the given range.
func (c *Client) Cycles(ctx context.Context, rng domain.TimeRange, limit int) ([]domain.Cycle, error) {
	dtos, err := fetchPaged[cycleDTO](ctx, c, "/v2/cycle", rangedQuery(rng, limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Cycle, len(dtos))
	for i := range dtos {
		out[i] = dtos[i].toDomain()
	}
	return out, nil
}

// Recoveries returns recovery scores in the given range.
func (c *Client) Recoveries(ctx context.Context, rng domain.TimeRange, limit int) ([]domain.Recovery, error) {
	dtos, err := fetchPaged[recoveryDTO](ctx, c, "/v2/recovery", rangedQuery(rng, limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Recovery, len(dtos))
	for i := range dtos {
		out[i] = dtos[i].toDomain()
	}
	return out, nil
}

// Sleeps returns sleep activities in the given range.
func (c *Client) Sleeps(ctx context.Context, rng domain.TimeRange, limit int) ([]domain.Sleep, error) {
	dtos, err := fetchPaged[sleepDTO](ctx, c, "/v2/activity/sleep", rangedQuery(rng, limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Sleep, len(dtos))
	for i := range dtos {
		out[i] = dtos[i].toDomain()
	}
	return out, nil
}

// Workouts returns workouts in the given range.
func (c *Client) Workouts(ctx context.Context, rng domain.TimeRange, limit int) ([]domain.Workout, error) {
	dtos, err := fetchPaged[workoutDTO](ctx, c, "/v2/activity/workout", rangedQuery(rng, limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Workout, len(dtos))
	for i := range dtos {
		out[i] = dtos[i].toDomain()
	}
	return out, nil
}
