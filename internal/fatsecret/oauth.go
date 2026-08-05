package fatsecret

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	requestTokenURL = "https://authentication.fatsecret.com/oauth/request_token"
	authorizeURL    = "https://authentication.fatsecret.com/oauth/authorize"
	accessTokenURL  = "https://authentication.fatsecret.com/oauth/access_token"
)

type OAuthToken struct {
	Token  string
	Secret string
}

type OAuthClient struct {
	consumerKey    string
	consumerSecret string
	callbackURL    string
	http           *http.Client
	requestURL     string
	authorizeURL   string
	accessURL      string
}

func NewOAuthClient(consumerKey, consumerSecret, callbackURL string, httpClient *http.Client) *OAuthClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &OAuthClient{
		consumerKey: consumerKey, consumerSecret: consumerSecret, callbackURL: callbackURL, http: httpClient,
		requestURL: requestTokenURL, authorizeURL: authorizeURL, accessURL: accessTokenURL,
	}
}

func (c *OAuthClient) RequestToken(ctx context.Context) (OAuthToken, error) {
	values, err := c.signedParams(http.MethodPost, c.requestURL, "", "", map[string]string{"oauth_callback": c.callbackURL})
	if err != nil {
		return OAuthToken{}, err
	}
	body, err := c.do(ctx, http.MethodPost, c.requestURL, values)
	if err != nil {
		return OAuthToken{}, fmt.Errorf("request token: %w", err)
	}
	return parseOAuthToken(body)
}

func (c *OAuthClient) AuthorizationURL(token string) string {
	return c.authorizeURL + "?oauth_token=" + url.QueryEscape(token)
}

func (c *OAuthClient) AccessToken(ctx context.Context, requestToken, requestSecret, verifier string) (OAuthToken, error) {
	values, err := c.signedParams(http.MethodGet, c.accessURL, requestToken, requestSecret, map[string]string{"oauth_verifier": verifier})
	if err != nil {
		return OAuthToken{}, err
	}
	body, err := c.do(ctx, http.MethodGet, c.accessURL, values)
	if err != nil {
		return OAuthToken{}, fmt.Errorf("access token: %w", err)
	}
	return parseOAuthToken(body)
}

func (c *OAuthClient) signedParams(method, endpoint, token, tokenSecret string, extra map[string]string) (url.Values, error) {
	n, err := nonce()
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"oauth_consumer_key": c.consumerKey, "oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp": strconv.FormatInt(time.Now().Unix(), 10), "oauth_nonce": n, "oauth_version": "1.0",
	}
	if token != "" {
		params["oauth_token"] = token
	}
	for k, v := range extra {
		params[k] = v
	}
	params["oauth_signature"] = computeSignature(method, endpoint, params, c.consumerSecret, tokenSecret)
	values := make(url.Values, len(params))
	for k, v := range params {
		values.Set(k, v)
	}
	return values, nil
}

func (c *OAuthClient) do(ctx context.Context, method, endpoint string, values url.Values) ([]byte, error) {
	var body io.Reader
	if method == http.MethodGet {
		endpoint += "?" + values.Encode()
	} else {
		body = strings.NewReader(values.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(data), 256))
	}
	return data, nil
}

func parseOAuthToken(body []byte) (OAuthToken, error) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return OAuthToken{}, fmt.Errorf("parse response: %w", err)
	}
	token := OAuthToken{Token: values.Get("oauth_token"), Secret: values.Get("oauth_token_secret")}
	if token.Token == "" || token.Secret == "" {
		return OAuthToken{}, fmt.Errorf("response missing oauth token or secret")
	}
	return token, nil
}
