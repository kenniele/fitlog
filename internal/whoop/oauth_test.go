package whoop

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// stubSource is an oauth2.TokenSource that hands out a scripted sequence.
type stubSource struct {
	tokens []*oauth2.Token
	errs   []error
	i      int
}

func (s *stubSource) Token() (*oauth2.Token, error) {
	if s.i >= len(s.tokens) {
		return nil, errors.New("stub exhausted")
	}
	t, e := s.tokens[s.i], s.errs[s.i]
	s.i++
	return t, e
}

func TestPersistingTokenSource_FiresOnChange(t *testing.T) {
	initial := &oauth2.Token{AccessToken: "a0", RefreshToken: "r0", Expiry: time.Now()}
	stub := &stubSource{
		tokens: []*oauth2.Token{
			{AccessToken: "a1", RefreshToken: "r1", Expiry: time.Now().Add(time.Hour)},
		},
		errs: []error{nil},
	}

	var updates []*oauth2.Token
	p := NewPersistingTokenSource(stub, initial, func(tok *oauth2.Token) {
		updates = append(updates, tok)
	})

	got, err := p.Token()
	require.NoError(t, err)
	require.Equal(t, "a1", got.AccessToken)
	require.Len(t, updates, 1, "onUpdate should fire when access token changes")
	require.Equal(t, "r1", updates[0].RefreshToken)
}

func TestPersistingTokenSource_NoFireWhenUnchanged(t *testing.T) {
	initial := &oauth2.Token{AccessToken: "a0", RefreshToken: "r0"}
	stub := &stubSource{
		tokens: []*oauth2.Token{
			{AccessToken: "a0", RefreshToken: "r0"},
		},
		errs: []error{nil},
	}

	var calls int
	p := NewPersistingTokenSource(stub, initial, func(*oauth2.Token) { calls++ })

	_, err := p.Token()
	require.NoError(t, err)
	require.Equal(t, 0, calls, "no onUpdate when token unchanged")
}

func TestPersistingTokenSource_FiresFromNilInitial(t *testing.T) {
	stub := &stubSource{
		tokens: []*oauth2.Token{{AccessToken: "a1", RefreshToken: "r1"}},
		errs:   []error{nil},
	}

	var calls int
	p := NewPersistingTokenSource(stub, nil, func(*oauth2.Token) { calls++ })
	_, err := p.Token()
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestPersistingTokenSource_PropagatesError(t *testing.T) {
	stub := &stubSource{tokens: []*oauth2.Token{nil}, errs: []error{errors.New("boom")}}
	var calls int
	p := NewPersistingTokenSource(stub, nil, func(*oauth2.Token) { calls++ })

	_, err := p.Token()
	require.Error(t, err)
	require.Equal(t, 0, calls, "must not persist on error")
}

func TestNewOAuthConfig(t *testing.T) {
	cfg := NewOAuthConfig("cid", "cs", "http://x/cb", nil)
	require.Equal(t, "cid", cfg.ClientID)
	require.Equal(t, AuthURL, cfg.Endpoint.AuthURL)
	require.Equal(t, TokenURL, cfg.Endpoint.TokenURL)
	require.Equal(t, oauth2.AuthStyleInParams, cfg.Endpoint.AuthStyle,
		"Whoop returns invalid_client unless client creds go in body")
	require.Contains(t, cfg.Scopes, "offline", "offline scope required for refresh token")
}
