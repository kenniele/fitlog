package fatsecret

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // OAuth 1.0a mandates HMAC-SHA1.
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// percentEncode encodes per RFC 3986 §2.1 as required by OAuth 1.0a §3.6.
// url.QueryEscape is NOT a substitute: it encodes space as "+" and leaves
// "+" unencoded. Here we keep only unreserved (ALPHA / DIGIT / "-._~")
// and percent-encode everything else, including space → %20.
func percentEncode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'A' <= c && c <= 'Z',
			'a' <= c && c <= 'z',
			'0' <= c && c <= '9',
			c == '-', c == '.', c == '_', c == '~':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteString(strings.ToUpper(hex.EncodeToString([]byte{c})))
		}
	}
	return b.String()
}

// nonce returns 16 random bytes hex-encoded.
func nonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// SignedParams takes user-supplied form params (e.g. method, format, date),
// injects the OAuth 1.0a fields, computes the signature, and returns the
// full params map ready to be encoded into the request body.
//
// httpMethod is uppercase ("POST"). endpoint is the full URL without query.
type SignedParams = map[string]string

// Signer holds the long-lived FatSecret credentials.
type Signer struct {
	ConsumerKey    string
	ConsumerSecret string
	AccessToken    string
	AccessSecret   string

	// nowFn and nonceFn are overridable in tests; nil → real values.
	nowFn   func() time.Time
	nonceFn func() (string, error)
}

// NewSigner constructs a Signer with real time and crypto/rand.
func NewSigner(consumerKey, consumerSecret, accessToken, accessSecret string) *Signer {
	return &Signer{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
		AccessToken:    accessToken,
		AccessSecret:   accessSecret,
	}
}

// Sign returns a fresh map containing the caller's params plus all required
// oauth_* fields, including the final oauth_signature.
func (s *Signer) Sign(httpMethod, endpoint string, params map[string]string) (SignedParams, error) {
	now := time.Now
	if s.nowFn != nil {
		now = s.nowFn
	}
	gen := nonce
	if s.nonceFn != nil {
		gen = s.nonceFn
	}

	n, err := gen()
	if err != nil {
		return nil, err
	}

	out := make(map[string]string, len(params)+6)
	for k, v := range params {
		out[k] = v
	}
	out["oauth_consumer_key"] = s.ConsumerKey
	out["oauth_token"] = s.AccessToken
	out["oauth_signature_method"] = "HMAC-SHA1"
	out["oauth_version"] = "1.0"
	out["oauth_timestamp"] = strconv.FormatInt(now().Unix(), 10)
	out["oauth_nonce"] = n

	out["oauth_signature"] = computeSignature(httpMethod, endpoint, out, s.ConsumerSecret, s.AccessSecret)
	return out, nil
}

// computeSignature builds the OAuth 1.0a HMAC-SHA1 signature.
func computeSignature(httpMethod, endpoint string, params map[string]string, consumerSecret, tokenSecret string) string {
	// Sort keys; build "k1=v1&k2=v2..." with each pair percent-encoded.
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pb strings.Builder
	for i, k := range keys {
		if i > 0 {
			pb.WriteByte('&')
		}
		pb.WriteString(percentEncode(k))
		pb.WriteByte('=')
		pb.WriteString(percentEncode(params[k]))
	}

	base := strings.ToUpper(httpMethod) + "&" + percentEncode(endpoint) + "&" + percentEncode(pb.String())
	key := percentEncode(consumerSecret) + "&" + percentEncode(tokenSecret)

	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(base))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
