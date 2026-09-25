package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleVersion verifies the /api/v1/version endpoint returns the
// server version as a public (no-auth) JSON response.
func TestHandleVersion(t *testing.T) {
	origVersion := Version
	Version = "1.2.3"
	defer func() { Version = origVersion }()

	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()

	s.handleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["version"] != "1.2.3" {
		t.Errorf("version = %q, want %q", resp["version"], "1.2.3")
	}
}

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

// TestIngestLogSource covers the only signal that tells the iOS app from
// Health Auto Export in import_logs: both post the same payload to the same
// endpoint, so a wrong mapping files one client's calls under the other.
func TestIngestLogSource(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", "hae_rest"},
		{ClientIOSApp, "freereps_ios"},
		{"something-else", "hae_rest"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", nil)
		if c.header != "" {
			r.Header.Set(ClientHeader, c.header)
		}
		if got := ingestLogSource(r); got != c.want {
			t.Errorf("header %q: got %q, want %q", c.header, got, c.want)
		}
	}
}
