package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/tailcfg"
)

// TestDevIdentity verifies that the dev identity middleware sets user_id=1
// for all requests, enabling local development without Tailscale.
func TestDevIdentity(t *testing.T) {
	var gotUserID int
	handler := DevIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := userIDFromContext(r)
		if !ok {
			t.Fatal("userIDFromContext returned false inside DevIdentity")
		}
		gotUserID = uid
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

// TestUserIDFromContextMissing verifies that userIDFromContext returns (0, false)
// when no identity middleware has set a value, ensuring loud failure detection.
func TestUserIDFromContextMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	id, ok := userIDFromContext(req)
	if ok {
		t.Error("userIDFromContext returned ok=true without context value")
	}
	if id != 0 {
		t.Errorf("userIDFromContext id = %d, want 0", id)
	}
}

// TestUserIDFromContextSet verifies that userIDFromContext returns the
// value stored by identity middleware.
func TestUserIDFromContextSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), userIDKey, 42)
	req = req.WithContext(ctx)

	id, ok := userIDFromContext(req)
	if !ok {
		t.Error("userIDFromContext returned ok=false with context value set")
	}
	if id != 42 {
		t.Errorf("userIDFromContext = %d, want 42", id)
	}
}

// TestMustUserIDMissing verifies that mustUserID writes a 500 error when
// no identity middleware has set a user ID in the context.
func TestMustUserIDMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	uid, ok := mustUserID(rec, req)
	if ok {
		t.Error("mustUserID returned ok=true without context value")
	}
	if uid != 0 {
		t.Errorf("mustUserID uid = %d, want 0", uid)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// TestMustUserIDPresent verifies that mustUserID returns the user ID when
// identity middleware has set it in the context.
func TestMustUserIDPresent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), userIDKey, 7)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	uid, ok := mustUserID(rec, req)
	if !ok {
		t.Error("mustUserID returned ok=false with context value set")
	}
	if uid != 7 {
		t.Errorf("mustUserID uid = %d, want 7", uid)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (unwritten)", rec.Code)
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

// --- Mocks for TailscaleIdentity tests ---

type mockWhois struct {
	resp *apitype.WhoIsResponse
	err  error
}

func (m *mockWhois) WhoIs(_ context.Context, _ string) (*apitype.WhoIsResponse, error) {
	return m.resp, m.err
}

type mockUserStore struct {
	getOrCreateID  int
	getOrCreateErr error
	primaryID      int
	primaryLogin   string
	primaryErr     error
}

func (m *mockUserStore) GetOrCreateUser(_ context.Context, _, _ string) (int, error) {
	return m.getOrCreateID, m.getOrCreateErr
}

func (m *mockUserStore) GetPrimaryUser(_ context.Context) (int, string, error) {
	return m.primaryID, m.primaryLogin, m.primaryErr
}

// TestTailscaleIdentityPersonalNode verifies that a personal (non-tagged) Tailscale
// node resolves identity from WhoIs, which is the existing flow.
func TestTailscaleIdentityPersonalNode(t *testing.T) {
	wc := &mockWhois{resp: &apitype.WhoIsResponse{
		Node: &tailcfg.Node{
			Name:         "macbook.tail1234.ts.net.",
			ComputedName: "macbook",
		},
		UserProfile: &tailcfg.UserProfile{
			LoginName:   "alice@example.com",
			DisplayName: "Alice",
		},
	}}
	us := &mockUserStore{getOrCreateID: 42}
	log := slog.Default()

	var gotUID int
	var gotInfo UserInfo
	handler := TailscaleIdentity(wc, us, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID, _ = userIDFromContext(r)
		gotInfo = userInfoFromContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotUID != 42 {
		t.Errorf("userID = %d, want 42", gotUID)
	}
	if gotInfo.Login != "alice@example.com" {
		t.Errorf("login = %q, want %q", gotInfo.Login, "alice@example.com")
	}
	if gotInfo.DisplayName != "Alice" {
		t.Errorf("displayName = %q, want %q", gotInfo.DisplayName, "Alice")
	}
	if gotInfo.TailscaleID != "macbook" {
		t.Errorf("tailscaleID = %q, want %q", gotInfo.TailscaleID, "macbook")
	}
	if gotInfo.Tailnet != "tail1234.ts.net" {
		t.Errorf("tailnet = %q, want %q", gotInfo.Tailnet, "tail1234.ts.net")
	}
}

// TestTailscaleIdentityTaggedNodeWithOwner verifies that a tagged device (e.g. an
// MCP proxy) resolves to the primary user from the database instead of being rejected.
func TestTailscaleIdentityTaggedNodeWithOwner(t *testing.T) {
	wc := &mockWhois{resp: &apitype.WhoIsResponse{
		Node: &tailcfg.Node{
			Name:         "tsmcp.tail1234.ts.net.",
			ComputedName: "tsmcp",
			Tags:         []string{"tag:mcp"},
		},
		UserProfile: &tailcfg.UserProfile{
			LoginName: "tagged-devices",
		},
	}}
	us := &mockUserStore{
		primaryID:     1,
		primaryLogin:  "alice@example.com",
		getOrCreateID: 1,
	}
	log := slog.Default()

	var gotUID int
	var gotInfo UserInfo
	handler := TailscaleIdentity(wc, us, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID, _ = userIDFromContext(r)
		gotInfo = userInfoFromContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotUID != 1 {
		t.Errorf("userID = %d, want 1", gotUID)
	}
	if gotInfo.Login != "alice@example.com" {
		t.Errorf("login = %q, want %q", gotInfo.Login, "alice@example.com")
	}
	if gotInfo.TailscaleID != "tsmcp" {
		t.Errorf("tailscaleID = %q, want %q", gotInfo.TailscaleID, "tsmcp")
	}
}

// TestTailscaleIdentityTaggedNodeNoOwner verifies that a tagged device is rejected
// with 403 when no real user (login containing @) has logged in yet.
func TestTailscaleIdentityTaggedNodeNoOwner(t *testing.T) {
	wc := &mockWhois{resp: &apitype.WhoIsResponse{
		Node: &tailcfg.Node{
			Name:         "tsmcp.tail1234.ts.net.",
			ComputedName: "tsmcp",
			Tags:         []string{"tag:mcp"},
		},
		UserProfile: &tailcfg.UserProfile{
			LoginName: "tagged-devices",
		},
	}}
	us := &mockUserStore{
		primaryErr: fmt.Errorf("no rows"),
	}
	log := slog.Default()

	handler := TailscaleIdentity(wc, us, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for rejected tagged device")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}
