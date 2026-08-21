package controlcenter

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookieName = "fitlog_dashboard_session"
	sessionLifetime   = 30 * 24 * time.Hour
	requestHeader     = "X-Fitlog-Request"
)

type authenticator struct {
	ownerID int64
	token   string
	now     func() time.Time
}

func newAuthenticator(ownerID int64, token string) *authenticator {
	return &authenticator{ownerID: ownerID, token: strings.TrimSpace(token), now: time.Now}
}

func (a *authenticator) enabled() bool { return a.ownerID != 0 && a.token != "" }

func (a *authenticator) matchesToken(candidate string) bool {
	want := sha256.Sum256([]byte(a.token))
	got := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func (a *authenticator) sessionValue(expires time.Time) string {
	payload := fmt.Sprintf("v1.%d.%d", a.ownerID, expires.Unix())
	signature := a.sign(payload)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (a *authenticator) sign(payload string) []byte {
	key := sha256.Sum256([]byte("fitlog:dashboard-session\x00" + a.token))
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func (a *authenticator) authenticate(r *http.Request) bool {
	if !a.enabled() {
		return false
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, a.sign(string(payloadBytes))) {
		return false
	}
	payload := strings.Split(string(payloadBytes), ".")
	if len(payload) != 3 || payload[0] != "v1" {
		return false
	}
	ownerID, err := strconv.ParseInt(payload[1], 10, 64)
	if err != nil || ownerID != a.ownerID {
		return false
	}
	expiresUnix, err := strconv.ParseInt(payload[2], 10, 64)
	return err == nil && a.now().Before(time.Unix(expiresUnix, 0))
}

func sessionCookie(r *http.Request, value string, expires time.Time) *http.Cookie {
	secure := r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
	maxAge := int(time.Until(expires).Seconds())
	if value == "" {
		maxAge = -1
	}
	return &http.Cookie{
		Name: sessionCookieName, Value: value, Path: "/", HttpOnly: true,
		Secure: secure, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: maxAge,
	}
}
