package fatsecret

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fitlog/internal/domain"
)

const defaultEndpoint = "https://platform.fatsecret.com/rest/server.api"

// Client speaks the FatSecret REST API over OAuth 1.0a.
//
// All requests are POST with form-encoded params (OAuth params live in the
// body, not the header — verified to be the only style the platform accepts).
type Client struct {
	endpoint string
	signer   *Signer
	http     *http.Client
}

// Options configure a Client. Zero values pick reasonable defaults.
type Options struct {
	Endpoint   string        // default: https://platform.fatsecret.com/rest/server.api
	HTTPClient *http.Client  // default: &http.Client{Timeout: 10s}
	Timeout    time.Duration // applied to default HTTPClient if HTTPClient is nil
}

// NewClient wires a signer and HTTP client.
func NewClient(signer *Signer, opts Options) *Client {
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	hc := opts.HTTPClient
	if hc == nil {
		t := opts.Timeout
		if t == 0 {
			t = 10 * time.Second
		}
		hc = &http.Client{Timeout: t}
	}
	return &Client{endpoint: endpoint, signer: signer, http: hc}
}

// do signs and executes a request, returning the response body or a typed error.
func (c *Client) do(ctx context.Context, params map[string]string) ([]byte, error) {
	params["format"] = "json"

	signed, err := c.signer.Sign("POST", c.endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	form := url.Values{}
	for k, v := range signed {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("fatsecret http %d: %s", resp.StatusCode, truncate(string(body), 256))
	}

	// FatSecret returns 200 with an `error` envelope on failure.
	var errEnv errorResponse
	if json.Unmarshal(body, &errEnv) == nil && errEnv.Error != nil {
		return nil, fmt.Errorf("fatsecret error %d: %s", errEnv.Error.Code, errEnv.Error.Message)
	}
	return body, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ProfileGet pings `profile.get`; used by /status and as a sanity check.
func (c *Client) ProfileGet(ctx context.Context) (string, error) {
	body, err := c.do(ctx, map[string]string{"method": "profile.get"})
	if err != nil {
		return "", err
	}
	var p profileResponse
	if err := json.Unmarshal(body, &p); err != nil {
		return "", fmt.Errorf("unmarshal profile: %w", err)
	}
	return p.Profile.UserID, nil
}

// FoodEntriesForDay calls `food_entries.get` for a single day.
func (c *Client) FoodEntriesForDay(ctx context.Context, day time.Time) ([]domain.MealEntry, error) {
	body, err := c.do(ctx, map[string]string{
		"method": "food_entries.get",
		"date":   strconv.Itoa(ToDateInt(day)),
	})
	if err != nil {
		return nil, err
	}
	return parseFoodEntries(body)
}

// FoodEntriesMonth calls `food_entries.get_month` for the month containing day.
// FatSecret keys the call to "first of that month": pass any date in the month
// and the API returns the full month plus the from/to bounds.
func (c *Client) FoodEntriesMonth(ctx context.Context, day time.Time) ([]domain.DailyNutrition, error) {
	body, err := c.do(ctx, map[string]string{
		"method": "food_entries.get_month",
		"date":   strconv.Itoa(ToDateInt(day)),
	})
	if err != nil {
		return nil, err
	}
	return parseMonth(body)
}
