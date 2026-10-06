package controlcenter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWidgetCredentialsArePurposeBoundAndExpire(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	auth := newAuthenticator(7, "owner-secret")
	auth.now = func() time.Time { return now }
	valid, err := auth.widgetValue(now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	expired, err := auth.widgetValue(now)
	if err != nil {
		t.Fatal(err)
	}
	other, rotated := newAuthenticator(8, "owner-secret"), newAuthenticator(7, "rotated-secret")
	for _, test := range []struct {
		name, value string
		auth        *authenticator
		want        bool
	}{
		{"valid", valid, auth, true},
		{"expires at boundary", expired, auth, false},
		{"different owner", valid, other, false},
		{"rotated dashboard token", valid, rotated, false},
		{"tampered", "A" + valid, auth, false},
		{"session credential", auth.sessionValue(now.Add(time.Hour)), auth, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.auth.validWidgetValue(test.value); got != test.want {
				t.Fatalf("accepted = %v, want %v", got, test.want)
			}
		})
	}
}

func TestWidgetTokenGrantRequiresSessionAndReadOnlyBearerCannotWrite(t *testing.T) {
	handler := NewHandler(&handlerStore{}, 7, "owner-secret", time.UTC)
	for _, test := range []struct {
		name, header string
		cookie       *http.Cookie
		want         int
	}{
		{"anonymous", "1", nil, 401},
		{"csrf header required", "", login(t, handler, "owner-secret"), 403},
		{"authenticated", "1", login(t, handler, "owner-secret"), 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := serve(handler, "POST", "/auth/widget-token", "", test.cookie, test.header)
			if response.Code != test.want {
				t.Fatalf("grant status=%d want=%d", response.Code, test.want)
			}
		})
	}
	response := serve(handler, "POST", "/auth/widget-token", "", login(t, handler, "owner-secret"), "1")
	var envelope struct {
		Data widgetGrant `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Scope != "widget:read" || !envelope.Data.ExpiresAt.After(time.Now()) {
		t.Fatal("invalid scope or expiry")
	}
	for _, test := range []struct {
		name, method, path string
		cookie             bool
		want               int
	}{
		{"read snapshot", "GET", "/widget?owner_id=999", false, 200},
		{"dashboard denied", "GET", "/dashboard/overview", false, 401},
		{"write denied", "POST", "/recovery", false, 401},
		{"cannot issue tokens", "POST", "/auth/widget-token", false, 401},
		{"cannot become session", "POST", "/recovery", true, 401},
		{"no widget write method", "POST", "/widget", false, 405},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			request.Header.Set("Authorization", "Bearer "+envelope.Data.AccessToken)
			request.Header.Set(requestHeader, "1")
			if test.cookie {
				request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: envelope.Data.AccessToken})
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d want=%d cache=%s", response.Code, test.want, response.Header().Get("Cache-Control"))
			}
		})
	}
	if response := serve(handler, "GET", "/widget", "", login(t, handler, "owner-secret"), ""); response.Code != 401 {
		t.Fatal("dashboard cookie accepted as widget credential")
	}
}
