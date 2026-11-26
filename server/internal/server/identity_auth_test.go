package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/claude/freereps/internal/storage"
	mkmw "github.com/meltforce/meltkit/pkg/middleware"
)

type fakeUsers map[int]*storage.User

func (f fakeUsers) GetUser(_ context.Context, id int) (*storage.User, error) {
	u, ok := f[id]
	if !ok {
		return nil, storage.ErrUserNotFound
	}
	return u, nil
}

// TestUserFromAuth covers the step between meltkit's auth middleware and the
// handlers. It exists because the bearer path and the WhoIs fallback do not
// ask the account resolver: a disabled user, or a token whose user row is
// gone, would otherwise keep access.
func TestUserFromAuth(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	off := time.Now()
	users := fakeUsers{
		2: {ID: 2, Login: "owner@example.com", DisplayName: "Owner", IsAdmin: true},
		3: {ID: 3, Login: "off@example.com", DisabledAt: &off},
	}
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := userIDFromContext(r)
		_ = json.NewEncoder(w).Encode(struct {
			ID   int
			Info UserInfo
		}{uid, userInfoFromContext(r)})
	})
	h := userFromAuth(users, quiet)(echo)

	viaAuth := func(id int) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		return r.WithContext(mkmw.WithUser(r.Context(), id, mkmw.UserInfo{}))
	}
	viaWhois := func(id int) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(r.Context(), userIDKey, id)
		ctx = context.WithValue(ctx, userInfoKey, UserInfo{Login: "w", Tailnet: "t.ts.net", AuthMethod: MethodTailscale})
		return r.WithContext(ctx)
	}

	cases := []struct {
		name   string
		req    *http.Request
		status int
		method string
		admin  bool
	}{
		{"session user", viaAuth(2), http.StatusOK, MethodSession, true},
		{"disabled user with credentials", viaAuth(3), http.StatusForbidden, "", false},
		{"credentials of a deleted user", viaAuth(9), http.StatusUnauthorized, "", false},
		{"no identity at all", httptest.NewRequest(http.MethodGet, "/", nil), http.StatusUnauthorized, "", false},
		{"whois fallback keeps its identity", viaWhois(2), http.StatusOK, MethodTailscale, true},
		{"disabled user over whois", viaWhois(3), http.StatusForbidden, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, c.req)
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d", rec.Code, c.status)
			}
			if c.status != http.StatusOK {
				return
			}
			var got struct {
				ID   int
				Info UserInfo
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.ID != 2 || got.Info.AuthMethod != c.method || got.Info.IsAdmin != c.admin {
				t.Fatalf("got %+v", got)
			}
			if c.method == MethodSession && got.Info.Login != "owner@example.com" {
				t.Fatalf("login not taken from the user row: %+v", got.Info)
			}
			if c.method == MethodTailscale && got.Info.Tailnet != "t.ts.net" {
				t.Fatalf("whois identity lost: %+v", got.Info)
			}
		})
	}
}

// TestAccountRoutesWithoutLogin: without an auth: block the account routes
// and /auth/* must answer 404, which is how the frontend learns that there is
// nothing to sign in to.
func TestAccountRoutesWithoutLogin(t *testing.T) {
	s := New(nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, path := range []string{"/api/v1/account/tokens", "/api/v1/account/sessions", "/api/v1/admin/users"} {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, rec.Code)
		}
	}
}
