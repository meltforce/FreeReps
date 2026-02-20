package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleMeDefault verifies the /api/v1/me endpoint returns the dev user
// identity when no Tailscale middleware is active.
func TestHandleMeDefault(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	ctx := context.WithValue(req.Context(), userInfoKey, UserInfo{Login: "local", DisplayName: "Local Dev User"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	s.handleMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var info UserInfo
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if info.Login != "local" {
		t.Errorf("login = %q, want %q", info.Login, "local")
	}
	if info.DisplayName != "Local Dev User" {
		t.Errorf("display_name = %q, want %q", info.DisplayName, "Local Dev User")
	}
}

// TestHandleMeTailscaleUser verifies the /api/v1/me endpoint returns the
// Tailscale user identity when set in context.
func TestHandleMeTailscaleUser(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	ctx := context.WithValue(req.Context(), userInfoKey, UserInfo{Login: "alice@example.com", DisplayName: "Alice"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	s.handleMe(rec, req)

	var info UserInfo
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if info.Login != "alice@example.com" {
		t.Errorf("login = %q, want %q", info.Login, "alice@example.com")
	}
	if info.DisplayName != "Alice" {
		t.Errorf("display_name = %q, want %q", info.DisplayName, "Alice")
	}
}
