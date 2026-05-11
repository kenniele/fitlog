package fatsecret

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPercentEncode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"abc", "abc"},
		{"ABC", "ABC"},
		{"012", "012"},
		{"-._~", "-._~"},
		{" ", "%20"},
		{"+", "%2B"},
		{"a b", "a%20b"},
		{"a&b=c", "a%26b%3Dc"},
		{"hello world!", "hello%20world%21"},
		// Per RFC 3986, "/" and ":" are NOT unreserved — must be encoded.
		{"https://example.com/path", "https%3A%2F%2Fexample.com%2Fpath"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, percentEncode(tc.in), "input=%q", tc.in)
	}
}

func fixedSigner() *Signer {
	s := NewSigner("ck", "cs", "at", "asec")
	s.nowFn = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	s.nonceFn = func() (string, error) { return "deadbeefdeadbeef", nil }
	return s
}

func TestSign_Deterministic(t *testing.T) {
	s := fixedSigner()
	got1, err := s.Sign("POST", "https://platform.fatsecret.com/rest/server.api",
		map[string]string{"method": "profile.get", "format": "json"})
	require.NoError(t, err)

	got2, err := s.Sign("POST", "https://platform.fatsecret.com/rest/server.api",
		map[string]string{"method": "profile.get", "format": "json"})
	require.NoError(t, err)

	require.Equal(t, got1["oauth_signature"], got2["oauth_signature"])
	require.Equal(t, "1700000000", got1["oauth_timestamp"])
	require.Equal(t, "deadbeefdeadbeef", got1["oauth_nonce"])
	require.Equal(t, "HMAC-SHA1", got1["oauth_signature_method"])
	require.Equal(t, "1.0", got1["oauth_version"])
	require.Equal(t, "ck", got1["oauth_consumer_key"])
	require.Equal(t, "at", got1["oauth_token"])
}

// Regression vector: pins the exact algorithm. If anything in the base
// string construction, key construction, or HMAC changes, this fails.
func TestComputeSignature_KnownVector(t *testing.T) {
	params := map[string]string{
		"method":                 "profile.get",
		"format":                 "json",
		"oauth_consumer_key":     "ck",
		"oauth_token":            "at",
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_version":          "1.0",
		"oauth_timestamp":        "1700000000",
		"oauth_nonce":            "deadbeefdeadbeef",
	}
	sig := computeSignature("POST", "https://platform.fatsecret.com/rest/server.api",
		params, "cs", "asec")
	// Generated once with this implementation; locks the algorithm.
	// Independently verified against a Python reference implementation.
	require.Equal(t, "+aIBl/XWK9lb3+zH2qAa/fjZUuw=", sig)
}
