package fatsecret

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOAuthClient_RequestAndAccessToken(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/request":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseForm())
			require.Equal(t, "https://fitlog.example/oauth/fatsecret/callback", r.Form.Get("oauth_callback"))
			require.Equal(t, "consumer", r.Form.Get("oauth_consumer_key"))
			require.NotEmpty(t, r.Form.Get("oauth_signature"))
			_, _ = w.Write([]byte("oauth_token=request-token&oauth_token_secret=request-secret&oauth_callback_confirmed=true"))
		case "/access":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "request-token", r.URL.Query().Get("oauth_token"))
			require.Equal(t, "verifier", r.URL.Query().Get("oauth_verifier"))
			require.NotEmpty(t, r.URL.Query().Get("oauth_signature"))
			_, _ = w.Write([]byte("oauth_token=access-token&oauth_token_secret=access-secret"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewOAuthClient("consumer", "consumer-secret", "https://fitlog.example/oauth/fatsecret/callback", server.Client())
	client.requestURL = server.URL + "/request"
	client.authorizeURL = server.URL + "/authorize"
	client.accessURL = server.URL + "/access"

	requestToken, err := client.RequestToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, OAuthToken{Token: "request-token", Secret: "request-secret"}, requestToken)
	require.Equal(t, server.URL+"/authorize?oauth_token=request-token", client.AuthorizationURL(requestToken.Token))

	accessToken, err := client.AccessToken(context.Background(), requestToken.Token, requestToken.Secret, "verifier")
	require.NoError(t, err)
	require.Equal(t, OAuthToken{Token: "access-token", Secret: "access-secret"}, accessToken)
}

func TestParseOAuthTokenRejectsIncompleteResponse(t *testing.T) {
	_, err := parseOAuthToken([]byte("oauth_token=only-token"))
	require.ErrorContains(t, err, "missing")
}
