package fatsecret

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"fitlog/internal/domain"
)

const (
	defaultEndpoint            = "https://platform.fatsecret.com/rest/server.api"
	defaultFoodEntriesEndpoint = "https://platform.fatsecret.com/rest/food-entries/v2"
)

// Client speaks the FatSecret REST API over OAuth 1.0a.
//
// All requests are POST with form-encoded params (OAuth params live in the
// body, not the header — verified to be the only style the platform accepts).
type Client struct {
	endpoint            string
	foodEntriesEndpoint string
	signer              *Signer
	http                *http.Client
	logger              *slog.Logger
}

// Options configure a Client. Zero values pick reasonable defaults.
type Options struct {
	Endpoint            string        // default: https://platform.fatsecret.com/rest/server.api
	FoodEntriesEndpoint string        // default: https://platform.fatsecret.com/rest/food-entries/v2
	HTTPClient          *http.Client  // default: &http.Client{Timeout: 10s}
	Timeout             time.Duration // applied to default HTTPClient if HTTPClient is nil
	Logger              *slog.Logger  // optional structured diagnostics; secrets are never logged
}

// NewClient wires a signer and HTTP client.
func NewClient(signer *Signer, opts Options) *Client {
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	foodEntriesEndpoint := opts.FoodEntriesEndpoint
	if foodEntriesEndpoint == "" {
		if opts.Endpoint == "" {
			foodEntriesEndpoint = defaultFoodEntriesEndpoint
		} else {
			// Custom endpoints are primarily used by local tests/proxies.
			foodEntriesEndpoint = opts.Endpoint
		}
	}
	hc := opts.HTTPClient
	if hc == nil {
		t := opts.Timeout
		if t == 0 {
			t = 10 * time.Second
		}
		hc = &http.Client{Timeout: t}
	}
	return &Client{endpoint: endpoint, foodEntriesEndpoint: foodEntriesEndpoint, signer: signer, http: hc, logger: opts.Logger}
}

// do signs and executes a request, returning the response body or a typed error.
func (c *Client) do(ctx context.Context, params map[string]string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, c.endpoint, params, false)
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, params map[string]string, query bool) ([]byte, error) {
	started := time.Now()
	operation := params["method"]
	if operation == "" {
		operation = "food_entries.get.v2"
	}
	if c.logger != nil {
		c.logger.Info("fatsecret api request started",
			"operation", operation, "http_method", method, "endpoint", endpoint,
			"date_int", params["date"], "transport", map[bool]string{true: "query", false: "form"}[query])
	}
	params["format"] = "json"

	signed, err := c.signer.Sign(method, endpoint, params)
	if err != nil {
		c.logFailure("sign", operation, method, endpoint, started, err)
		return nil, fmt.Errorf("sign: %w", err)
	}

	form := url.Values{}
	for k, v := range signed {
		form.Set(k, v)
	}

	var requestBody io.Reader
	requestURL := endpoint
	if query {
		requestURL += "?" + form.Encode()
	} else {
		requestBody = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	if !query {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		c.logFailure("http", operation, method, endpoint, started, err)
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logFailure("read_body", operation, method, endpoint, started, err)
		return nil, fmt.Errorf("read body: %w", err)
	}
	if c.logger != nil {
		keys, validJSON := jsonTopLevelKeys(body)
		digest := sha256.Sum256(body)
		c.logger.Info("fatsecret api response received",
			"operation", operation, "http_method", method, "endpoint", endpoint,
			"status", resp.StatusCode, "duration", time.Since(started), "body_bytes", len(body),
			"content_type", resp.Header.Get("Content-Type"), "json_valid", validJSON,
			"top_level_keys", keys, "body_sha256_prefix", fmt.Sprintf("%x", digest[:6]))
	}

	if resp.StatusCode/100 != 2 {
		if c.logger != nil {
			c.logger.Warn("fatsecret api returned non-2xx", "operation", operation, "status", resp.StatusCode)
		}
		return nil, fmt.Errorf("fatsecret http %d: %s", resp.StatusCode, truncate(string(body), 256))
	}

	// FatSecret returns 200 with an `error` envelope on failure.
	var errEnv errorResponse
	if json.Unmarshal(body, &errEnv) == nil && errEnv.Error != nil {
		if c.logger != nil {
			c.logger.Warn("fatsecret api returned error envelope", "operation", operation,
				"error_code", errEnv.Error.Code, "error_message", errEnv.Error.Message)
		}
		return nil, fmt.Errorf("fatsecret error %d: %s", errEnv.Error.Code, errEnv.Error.Message)
	}
	return body, nil
}

func (c *Client) logFailure(stage, operation, method, endpoint string, started time.Time, err error) {
	if c.logger != nil {
		c.logger.Warn("fatsecret api request failed", "stage", stage, "operation", operation,
			"http_method", method, "endpoint", endpoint, "duration", time.Since(started), "err", err)
	}
}

func jsonTopLevelKeys(body []byte) ([]string, bool) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, false
	}
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ProfileGet pings `profile.get`; useful as a credential sanity check.
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

// FoodEntriesForDay calls the latest v2 diary endpoint. V2 guarantees a JSON
// array for food_entry, including the single-entry case.
func (c *Client) FoodEntriesForDay(ctx context.Context, day time.Time) ([]domain.MealEntry, error) {
	body, err := c.doRequest(ctx, http.MethodGet, c.foodEntriesEndpoint, map[string]string{
		"date": strconv.Itoa(ToDateInt(day)),
	}, true)
	if err != nil {
		return nil, err
	}
	entries, err := parseFoodEntries(body)
	if c.logger != nil {
		c.logger.Info("fatsecret daily response parsed", "requested_date", day.Format("2006-01-02"),
			"requested_date_int", ToDateInt(day), "entries", len(entries), "parse_error", err)
	}
	return entries, err
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
	days, err := parseMonth(body)
	if c.logger != nil {
		first, last := 0, 0
		if len(days) > 0 {
			first, last = days[0].DateInt, days[len(days)-1].DateInt
		}
		c.logger.Info("fatsecret monthly response parsed", "requested_month", day.Format("2006-01"),
			"days", len(days), "first_date_int", first, "last_date_int", last, "parse_error", err)
	}
	return days, err
}
