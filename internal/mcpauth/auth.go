// Package mcpauth implements single-owner OAuth for web and native MCP clients.
// It is intentionally separate from provider OAuth tokens and dashboard cookies.
package mcpauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ReadScope   = "fitlog:read"
	WriteScope  = "fitlog:write"
	loginCookie = "__Host-fitlog-mcp-csrf"
	maxBody     = 1 << 20
	consentCSP  = "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'"
)

type Config struct {
	BaseURL      string
	OwnerID      int64
	ClientID     string
	ClientSecret string
	LoginToken   string
	RedirectURIs []string
	AllowWrites  bool
}

type Server struct {
	cfg     Config
	store   grantStore
	binding string
	logger  *slog.Logger
}

type scopeKey struct{}

// NewServer requires HTTPS and an exact web redirect allowlist. The two secrets
// are separate: LoginToken is entered only on FitLog, ClientSecret in ChatGPT.
func NewServer(cfg Config, pool *pgxpool.Pool, logger *slog.Logger) (*Server, error) {
	return newServer(cfg, &postgresStore{pool: pool}, logger)
}
func newServer(cfg Config, store grantStore, logger *slog.Logger) (*Server, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("MCP requires an HTTPS PUBLIC_BASE_URL origin")
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.OwnerID <= 0 || cfg.ClientID == "" || len(cfg.ClientSecret) < 32 || len(cfg.LoginToken) < 32 || cfg.ClientSecret == cfg.LoginToken {
		return nil, errors.New("MCP requires an owner, client ID and distinct client/login secrets of at least 32 characters")
	}
	if len(cfg.RedirectURIs) == 0 {
		return nil, errors.New("MCP requires at least one exact OAuth redirect URI")
	}
	for _, raw := range cfg.RedirectURIs {
		redirect, err := url.Parse(raw)
		if err != nil || redirect.Scheme != "https" || redirect.Host == "" || redirect.User != nil || redirect.Fragment != "" {
			return nil, errors.New("MCP redirect URIs must be exact HTTPS URLs without fragments or userinfo")
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	// Rotating either secret, changing owner, resource, client or redirect URIs
	// revokes outstanding grants without depending on a process-local cache.
	bindingBytes, _ := json.Marshal([]any{cfg.OwnerID, cfg.BaseURL, cfg.ClientID, cfg.ClientSecret, cfg.LoginToken, cfg.RedirectURIs})
	return &Server{cfg: cfg, store: store, binding: digest(string(bindingBytes)), logger: logger}, nil
}

// CanWrite is consumed by the MCP adapter to select the granted tool surface.
func CanWrite(ctx context.Context) bool {
	scope, _ := ctx.Value(scopeKey{}).(string)
	return hasScope(scope, WriteScope)
}
func (s *Server) resource() string { return s.cfg.BaseURL + "/mcp" }
func (s *Server) scopes() []string {
	scopes := []string{ReadScope, "offline_access"}
	if s.cfg.AllowWrites {
		scopes = append(scopes, WriteScope)
	}
	return scopes
}

// Routes publishes discovery, consent, token exchange and token revocation.
// Mount at /oauth/mcp/ and both /.well-known/oauth-* paths.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.resourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", s.resourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.serverMetadata)
	mux.HandleFunc("GET /oauth/mcp/authorize", s.authorize)
	mux.HandleFunc("POST /oauth/mcp/authorize", s.authorize)
	mux.HandleFunc("POST /oauth/mcp/token", s.token)
	mux.HandleFunc("POST /oauth/mcp/revoke", s.revoke)
	mux.HandleFunc("POST /oauth/mcp/register", s.register)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func headers(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", consentCSP+"; form-action 'self'")
	w.Header().Set("X-Frame-Options", "DENY")
}
func (s *Server) resourceMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"resource": s.resource(), "authorization_servers": []string{s.cfg.BaseURL}, "scopes_supported": s.scopes(), "bearer_methods_supported": []string{"header"}, "resource_name": "FitLog"})
}
func (s *Server) serverMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"issuer": s.cfg.BaseURL, "authorization_endpoint": s.cfg.BaseURL + "/oauth/mcp/authorize",
		"token_endpoint": s.cfg.BaseURL + "/oauth/mcp/token", "revocation_endpoint": s.cfg.BaseURL + "/oauth/mcp/revoke",
		"registration_endpoint":    s.cfg.BaseURL + "/oauth/mcp/register",
		"response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
		"code_challenge_methods_supported":      []string{"S256"}, "scopes_supported": s.scopes(), "authorization_response_iss_parameter_supported": true})
}

func (s *Server) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		// Reject browser origins explicitly, including GET; proxies must preserve Host.
		if origins := r.Header.Values("Origin"); len(origins) > 0 && (len(origins) != 1 || origins[0] != s.cfg.BaseURL) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(r.Header.Values("Authorization")) != 1 || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 256 {
			s.challenge(w)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
		defer cancel()
		scope, err := s.store.Validate(ctx, digest(parts[1]), s.binding)
		if err != nil {
			if !errors.Is(err, errInvalidGrant) {
				s.internalError(w, r, err)
				return
			}
			s.challenge(w)
			return
		}
		if !hasScope(scope, ReadScope) {
			s.challenge(w)
			return
		}
		if !s.cfg.AllowWrites {
			scope = ReadScope
		}
		ctx = context.WithValue(ctx, scopeKey{}, scope)
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (s *Server) challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource/mcp", scope="%s"`, s.cfg.BaseURL, strings.Join(s.scopes(), " ")))
	writeJSON(w, 401, map[string]string{"error": "unauthorized"})
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// No redirects occur until the client and its registered callback are verified.
	client, valid := s.resolveClient(q.Get("client_id"))
	if !valid || !client.allowsRedirect(q.Get("redirect_uri")) {
		oauthError(w, 400, "invalid_request")
		return
	}
	scope := q.Get("scope")
	if scope == "" {
		scope = strings.Join(s.scopes(), " ")
		if !client.refresh {
			scope = strings.ReplaceAll(scope, " offline_access", "")
		}
	}
	for _, values := range q {
		if len(values) != 1 {
			oauthError(w, 400, "invalid_request")
			return
		}
	}
	if q.Get("response_type") != "code" || q.Get("resource") != s.resource() || q.Get("code_challenge_method") != "S256" || !validChallenge(q.Get("code_challenge")) || len(q.Get("state")) > 2048 {
		oauthError(w, 400, "invalid_request")
		return
	}
	for _, v := range strings.Fields(scope) {
		if !slices.Contains(s.scopes(), v) || (v == "offline_access" && !client.refresh) {
			oauthError(w, 400, "invalid_scope")
			return
		}
	}
	if !hasScope(scope, ReadScope) {
		oauthError(w, 400, "invalid_scope")
		return
	}
	if r.Method == http.MethodGet {
		// Chromium also applies form-action to the POST's redirect chain.
		// The form action is fixed to FitLog, and the callback is validated above.
		// Keep scripts/frames disabled, but allow navigation back to the client.
		w.Header().Set("Content-Security-Policy", consentCSP)
		csrf := randomToken()
		http.SetCookie(w, &http.Cookie{Name: loginCookie, Value: csrf, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: 600})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = consentPage.Execute(w, struct{ Action, CSRF, Client, Scope, Redirect string }{r.URL.RequestURI(), s.consentCSRF(csrf, q), client.name, scope, q.Get("redirect_uri")})
		return
	}
	if r.Header.Get("Origin") != s.cfg.BaseURL {
		oauthError(w, 403, "invalid_request")
		return
	}
	if err := r.ParseForm(); err != nil {
		oauthError(w, 400, "invalid_request")
		return
	}
	cookie, err := r.Cookie(loginCookie)
	if err != nil || cookie.Value == "" || !equal(s.consentCSRF(cookie.Value, q), r.PostForm.Get("csrf")) {
		oauthError(w, 403, "invalid_request")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: loginCookie, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	redirect, _ := url.Parse(q.Get("redirect_uri"))
	response := redirect.Query()
	response.Set("state", q.Get("state"))
	response.Set("iss", s.cfg.BaseURL)
	if r.PostForm.Get("decision") != "allow" {
		response.Set("error", "access_denied")
	} else {
		if !equal(s.cfg.LoginToken, r.PostForm.Get("login_token")) {
			oauthError(w, 401, "access_denied")
			return
		}
		code := randomToken()
		err := s.store.SaveCode(r.Context(), grant{Binding: s.binding, ClientHash: client.hash, Scope: scope, RedirectURI: q.Get("redirect_uri"), Challenge: q.Get("code_challenge"), CodeHash: digest(code), CodeExpiresAt: time.Now().Add(5 * time.Minute)})
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		response.Set("code", code)
	}
	redirect.RawQuery = response.Encode()
	// The redirect is verified against the client's registered callbacks above.
	http.Redirect(w, r, redirect.String(), http.StatusSeeOther) //nolint:gosec // G710: only the verified callback receives OAuth code/state.
}

func (s *Server) authenticateClient(w http.ResponseWriter, r *http.Request) (oauthClient, bool) {
	if err := r.ParseForm(); err != nil {
		oauthError(w, 400, "invalid_request")
		return oauthClient{}, false
	}
	for _, values := range r.PostForm {
		if len(values) != 1 {
			oauthError(w, 400, "invalid_request")
			return oauthClient{}, false
		}
	}
	if len(r.Header.Values("Authorization")) > 1 {
		oauthError(w, 400, "invalid_request")
		return oauthClient{}, false
	}
	id, secret, ok := r.BasicAuth()
	if ok {
		if r.PostForm.Has("client_secret") {
			oauthError(w, 400, "invalid_request")
			return oauthClient{}, false
		}
		var err error
		id, err = url.QueryUnescape(id)
		if err != nil {
			oauthError(w, 401, "invalid_client")
			return oauthClient{}, false
		}
		secret, err = url.QueryUnescape(secret)
		if err != nil {
			oauthError(w, 401, "invalid_client")
			return oauthClient{}, false
		}
		if r.PostForm.Has("client_id") && r.PostForm.Get("client_id") != id {
			oauthError(w, 400, "invalid_request")
			return oauthClient{}, false
		}
	} else {
		if len(r.Header.Values("Authorization")) != 0 {
			oauthError(w, 401, "invalid_client")
			return oauthClient{}, false
		}
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	client, valid := s.resolveClient(id)
	if valid && client.public {
		if ok || r.PostForm.Has("client_secret") {
			oauthError(w, 401, "invalid_client")
			return oauthClient{}, false
		}
		return client, true
	}
	if !valid || !equal(secret, s.cfg.ClientSecret) {
		w.Header().Set("WWW-Authenticate", `Basic realm="FitLog MCP"`)
		oauthError(w, 401, "invalid_client")
		return oauthClient{}, false
	}
	return client, true
}
func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	client, ok := s.authenticateClient(w, r)
	if !ok {
		return
	}
	f := r.PostForm
	if f.Get("resource") != s.resource() {
		oauthError(w, 400, "invalid_target")
		return
	}
	// A token exchange may narrow consented scopes, never enlarge them.
	if requested := f.Get("scope"); requested != "" {
		if !hasScope(requested, ReadScope) {
			oauthError(w, 400, "invalid_scope")
			return
		}
		for _, scope := range strings.Fields(requested) {
			if !slices.Contains(s.scopes(), scope) {
				oauthError(w, 400, "invalid_scope")
				return
			}
		}
	}
	kind := f.Get("grant_type")
	var hash, challenge string
	switch kind {
	case "authorization_code":
		verifier := f.Get("code_verifier")
		if !validVerifier(verifier) {
			oauthError(w, 400, "invalid_grant")
			return
		}
		sum := sha256.Sum256([]byte(verifier))
		challenge = base64.RawURLEncoding.EncodeToString(sum[:])
		hash = digest(f.Get("code"))
	case "refresh_token":
		if !client.refresh {
			oauthError(w, 400, "unauthorized_client")
			return
		}
		hash = digest(f.Get("refresh_token"))
	default:
		oauthError(w, 400, "unsupported_grant_type")
		return
	}
	access, refresh := randomToken(), randomToken()
	scope, err := s.store.Exchange(r.Context(), exchange{Kind: kind, Hash: hash, Binding: s.binding, ClientHash: client.hash, RequestedScope: f.Get("scope"), RedirectURI: f.Get("redirect_uri"), Challenge: challenge,
		AccessHash: digest(access), RefreshHash: digest(refresh), AccessExpiresAt: time.Now().Add(time.Hour), RefreshExpiresAt: time.Now().Add(30 * 24 * time.Hour)})
	if err != nil {
		if errors.Is(err, errInvalidGrant) {
			oauthError(w, 400, "invalid_grant")
			return
		}
		s.internalError(w, r, err)
		return
	}
	result := map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 3600, "scope": scope}
	if hasScope(scope, "offline_access") {
		result["refresh_token"] = refresh
	}
	writeJSON(w, 200, result)
}
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	client, ok := s.authenticateClient(w, r)
	if !ok {
		return
	}
	if err := s.store.Revoke(r.Context(), digest(r.PostForm.Get("token")), s.binding, client.hash); err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(200)
}
func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.ErrorContext(r.Context(), "MCP authorization failed", "path", r.URL.Path, "err", err)
	oauthError(w, 500, "server_error")
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func oauthError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func equal(a, b string) bool {
	aa, bb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(aa[:], bb[:]) == 1
}
func randomToken() string                { return rand.Text() }
func hasScope(scope, wanted string) bool { return slices.Contains(strings.Fields(scope), wanted) }
func validChallenge(v string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(v)
	return err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == v
}
func validVerifier(v string) bool {
	if len(v) < 43 || len(v) > 128 {
		return false
	}
	for _, c := range v {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~", c) {
			return false
		}
	}
	return true
}

var consentPage = template.Must(template.New("consent").Parse(`<!doctype html><html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Подключить FitLog</title>
<style>body{font:18px system-ui;max-width:640px;margin:8vh auto;padding:24px;color:#192b26;background:#f2f7f4}main{padding:32px;background:white;border-radius:20px}input{box-sizing:border-box;width:100%;padding:12px;margin:12px 0}button{padding:12px 18px;margin:10px 8px 0 0}code{overflow-wrap:anywhere}</style>
<main><h1>Подключить FitLog</h1><p>Приложение <strong>{{.Client}}</strong> запрашивает доступ к твоему журналу тренировок и здоровья.</p>
<p>Разрешения: <code>{{.Scope}}</code></p><p><code>fitlog:read</code> — чтение данных; <code>fitlog:write</code> — добавление записей; <code>offline_access</code> — сохранение подключения на срок до 30 дней.</p>
<p>После входа ты вернёшься на <code>{{.Redirect}}</code>.</p>
<form method="post" action="{{.Action}}"><input type="hidden" name="csrf" value="{{.CSRF}}"><label for="login">Ключ подключения FitLog</label><input id="login" name="login_token" type="password" autocomplete="off"><p>Введи ключ только на этой странице FitLog. Не отправляй его в чат.</p><button name="decision" value="allow">Разрешить доступ</button><button name="decision" value="deny">Отмена</button></form></main></html>`))

func (s *Server) consentCSRF(nonce string, query url.Values) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.LoginToken))
	_, _ = mac.Write([]byte(nonce + "\x00" + query.Encode()))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
