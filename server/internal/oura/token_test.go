package oura

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAuthorizeURL is covered by integration tests (requires DB for per-user creds).
// The postToken tests below verify the token endpoint interaction.

// TestExchangeCodeSuccess verifies the token exchange flow: the manager sends
// client credentials with the authorization code and receives tokens.
func TestExchangeCodeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q, want authorization_code", r.FormValue("grant_type"))
		}
		if r.FormValue("code") != "auth-code-123" {
			t.Errorf("code = %q, want auth-code-123", r.FormValue("code"))
		}
		if r.FormValue("client_id") != "cid" {
			t.Errorf("client_id = %q, want cid", r.FormValue("client_id"))
		}
		if r.FormValue("client_secret") != "csecret" {
			t.Errorf("client_secret = %q, want csecret", r.FormValue("client_secret"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResponse{
			AccessToken:  "access-tok",
			RefreshToken: "refresh-tok",
			TokenType:    "Bearer",
			ExpiresIn:    86400,
		})
	}))
	defer srv.Close()

	tm := &TokenManager{
		httpClient: srv.Client(),
		tokenURL:   srv.URL,
	}

	// ExchangeCode needs a real DB, so we just test the HTTP interaction by
	// verifying the postToken method directly.
	tok, err := tm.postToken(context.Background(), "cid", "csecret", map[string][]string{
		"grant_type":   {"authorization_code"},
		"code":         {"auth-code-123"},
		"redirect_uri": {"https://example.com/callback"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.AccessToken != "access-tok" {
		t.Errorf("access_token = %q, want access-tok", tok.AccessToken)
	}
	if tok.RefreshToken != "refresh-tok" {
		t.Errorf("refresh_token = %q, want refresh-tok", tok.RefreshToken)
	}
	if tok.ExpiresIn != 86400 {
		t.Errorf("expires_in = %d, want 86400", tok.ExpiresIn)
	}
}

// TestTokenEndpointError verifies that a failed token request returns an error
// with the status code and response body for debugging.
func TestTokenEndpointError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	tm := &TokenManager{
		httpClient: srv.Client(),
		tokenURL:   srv.URL,
	}

	_, err := tm.postToken(context.Background(), "cid", "csecret", map[string][]string{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"bad-refresh"},
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

