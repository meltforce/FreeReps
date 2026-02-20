package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDevIdentity verifies that the dev identity middleware sets user_id=1
// for all requests, enabling local development without Tailscale.
func TestDevIdentity(t *testing.T) {
	var gotUserID int
	handler := DevIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = userIDFromContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotUserID != 1 {
		t.Errorf("userID = %d, want 1", gotUserID)
	}
}

// TestUserIDFromContextDefault verifies that userIDFromContext returns 1
// when no identity middleware has set a value (fallback for safety).
func TestUserIDFromContextDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if id := userIDFromContext(req); id != 1 {
		t.Errorf("userIDFromContext without context value = %d, want 1", id)
	}
}

// TestUserIDFromContextSet verifies that userIDFromContext returns the
// value stored by identity middleware.
func TestUserIDFromContextSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), userIDKey, 42)
	req = req.WithContext(ctx)

	if id := userIDFromContext(req); id != 42 {
		t.Errorf("userIDFromContext = %d, want 42", id)
	}
}

// TestUserInfoFromContextDefault verifies the fallback UserInfo when no
// identity middleware has set a value.
func TestUserInfoFromContextDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	info := userInfoFromContext(req)
	if info.Login != "local" {
		t.Errorf("login = %q, want %q", info.Login, "local")
	}
	if info.DisplayName != "Local Dev User" {
		t.Errorf("displayName = %q, want %q", info.DisplayName, "Local Dev User")
	}
}

// TestUserInfoFromContextSet verifies UserInfo is extracted from context when set.
func TestUserInfoFromContextSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), userInfoKey, UserInfo{Login: "alice@example.com", DisplayName: "Alice"})
	req = req.WithContext(ctx)

	info := userInfoFromContext(req)
	if info.Login != "alice@example.com" {
		t.Errorf("login = %q, want %q", info.Login, "alice@example.com")
	}
	if info.DisplayName != "Alice" {
		t.Errorf("displayName = %q, want %q", info.DisplayName, "Alice")
	}
}

// TestDevIdentityUserInfo verifies that DevIdentity middleware stores UserInfo
// alongside the user ID.
func TestDevIdentityUserInfo(t *testing.T) {
	var gotInfo UserInfo
	handler := DevIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInfo = userInfoFromContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotInfo.Login != "local" {
		t.Errorf("login = %q, want %q", gotInfo.Login, "local")
	}
}

// TestRequestLogging verifies that the logging middleware calls the next handler and records status.
func TestRequestLogging(t *testing.T) {
	log := slog.Default()
	handler := RequestLogging(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

// TestCORSHeaders verifies that CORS headers are set on responses.
func TestCORSHeaders(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS origin = %q, want *", got)
	}
}

// TestCORSPreflight verifies that OPTIONS requests get 204 with CORS headers.
func TestCORSPreflight(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for OPTIONS")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}
