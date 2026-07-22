package whoop

import (
	"context"

	"golang.org/x/oauth2"
)

// Whoop OAuth endpoints — the auth/token base differs from the API base.
const (
	AuthURL  = "https://api.prod.whoop.com/oauth/oauth2/auth"
	TokenURL = "https://api.prod.whoop.com/oauth/oauth2/token" //nolint:gosec // G101 false positive: public OAuth token endpoint URL, not a credential.
	APIBase  = "https://api.prod.whoop.com/developer"
)

// DefaultScopes are requested by the OAuth button. `offline` is REQUIRED
// to get a refresh token back; without it, the token expires in an hour and
// the bot dies overnight.
var DefaultScopes = []string{
	"read:profile",
	"read:body_measurement",
	"read:cycles",
	"read:recovery",
	"read:sleep",
	"read:workout",
	"offline",
}

// NewOAuthConfig builds an oauth2.Config for Whoop.
//
// Critical: AuthStyleInParams. Whoop's token endpoint rejects HTTP Basic
// auth with `invalid_client`; client_id and client_secret must travel in
// the request body. The default style is InHeader, which silently fails.
func NewOAuthConfig(clientID, clientSecret, redirectURL string, scopes []string) *oauth2.Config {
	if scopes == nil {
		scopes = DefaultScopes
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:   AuthURL,
			TokenURL:  TokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

// PersistingTokenSource wraps a refreshing token source and fires onUpdate
// whenever the access token changes. This is critical for Whoop because
// the refresh token ROTATES on every refresh — if we don't persist the new
// pair immediately, the old refresh token becomes invalid and the user has
// to re-auth.
type PersistingTokenSource struct {
	base     oauth2.TokenSource
	current  *oauth2.Token
	onUpdate func(*oauth2.Token)
}

// NewPersistingTokenSource builds a PersistingTokenSource. initial is the
// last-known token (used to detect "did this refresh change anything"); it
// may be nil, in which case the first successful refresh triggers onUpdate.
func NewPersistingTokenSource(base oauth2.TokenSource, initial *oauth2.Token, onUpdate func(*oauth2.Token)) *PersistingTokenSource {
	return &PersistingTokenSource{base: base, current: initial, onUpdate: onUpdate}
}

// Token implements oauth2.TokenSource.
func (p *PersistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if p.current == nil || tok.AccessToken != p.current.AccessToken {
		p.current = tok
		if p.onUpdate != nil {
			p.onUpdate(tok)
		}
	}
	return tok, nil
}

// MakeTokenSource builds a refreshing, persisting TokenSource ready to be
// fed to an oauth2.Client.
func MakeTokenSource(ctx context.Context, cfg *oauth2.Config, initial *oauth2.Token, onUpdate func(*oauth2.Token)) oauth2.TokenSource {
	base := cfg.TokenSource(ctx, initial)
	return NewPersistingTokenSource(base, initial, onUpdate)
}
