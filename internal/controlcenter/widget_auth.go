package controlcenter

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const widgetTokenLifetime = 90 * 24 * time.Hour

type widgetGrant struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	Scope       string    `json:"scope"`
}

// Widget credentials have a separate signed purpose and cannot become sessions.
func (a *authenticator) widgetValue(expires time.Time) (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("widget token nonce: %w", err)
	}
	payload := fmt.Sprintf("w1.%d.%d.%s", a.ownerID, expires.Unix(), base64.RawURLEncoding.EncodeToString(nonce[:]))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(a.sign("fitlog:widget\x00"+payload)), nil
}

func (a *authenticator) validWidgetValue(value string) bool {
	if !a.enabled() || len(value) > 512 {
		return false
	}
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, a.sign("fitlog:widget\x00"+string(payload))) {
		return false
	}
	fields := strings.Split(string(payload), ".")
	if len(fields) != 4 || fields[0] != "w1" {
		return false
	}
	ownerID, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || ownerID != a.ownerID {
		return false
	}
	expires, err := strconv.ParseInt(fields[2], 10, 64)
	return err == nil && a.now().Before(time.Unix(expires, 0))
}

func (h *apiHandler) createWidgetToken(w http.ResponseWriter, _ *http.Request) {
	expires := h.auth.now().Add(widgetTokenLifetime)
	token, err := h.auth.widgetValue(expires)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, widgetGrant{AccessToken: token, ExpiresAt: expires.UTC(), Scope: "widget:read"})
}

func (h *apiHandler) requireWidgetToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.auth.enabled() {
			writeAPIError(w, http.StatusServiceUnavailable, "dashboard_disabled", "dashboard is disabled", nil)
			return
		}
		header := strings.Fields(r.Header.Get("Authorization"))
		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") || !h.auth.validWidgetValue(header[1]) {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "widget authentication required", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
