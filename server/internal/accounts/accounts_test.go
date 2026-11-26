package accounts

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/claude/freereps/internal/config"
	"github.com/claude/freereps/internal/storage"
	"github.com/meltforce/meltkit/pkg/auth"
)

type fakeStore struct {
	users  map[int]*storage.User
	nextID int
}

func newFakeStore(users ...storage.User) *fakeStore {
	s := &fakeStore{users: map[int]*storage.User{}, nextID: 100}
	for _, u := range users {
		u := u
		s.users[u.ID] = &u
	}
	return s
}

func (s *fakeStore) GetUser(_ context.Context, id int) (*storage.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, storage.ErrUserNotFound
	}
	c := *u
	return &c, nil
}

func (s *fakeStore) GetUserByLogin(_ context.Context, login string) (*storage.User, error) {
	for _, u := range s.users {
		if u.Login == login {
			c := *u
			return &c, nil
		}
	}
	return nil, storage.ErrUserNotFound
}

func (s *fakeStore) CreateUser(_ context.Context, login, name string, admin bool) (*storage.User, error) {
	if _, err := s.GetUserByLogin(context.Background(), login); err == nil {
		return nil, storage.ErrUserExists
	}
	s.nextID++
	u := &storage.User{ID: s.nextID, Login: login, DisplayName: name, IsAdmin: admin}
	s.users[u.ID] = u
	return u, nil
}

func (s *fakeStore) ListUsers(context.Context) ([]storage.User, error) {
	var out []storage.User
	for _, u := range s.users {
		out = append(out, *u)
	}
	return out, nil
}

func (s *fakeStore) TouchUser(context.Context, int, string) error { return nil }

func (s *fakeStore) AdminExists(context.Context) (bool, error) {
	for _, u := range s.users {
		if u.IsAdmin && !u.Disabled() {
			return true, nil
		}
	}
	return false, nil
}

func (s *fakeStore) SetUserAdmin(_ context.Context, id int, admin bool) error {
	u, ok := s.users[id]
	if !ok {
		return storage.ErrUserNotFound
	}
	u.IsAdmin = admin
	return nil
}

func (s *fakeStore) SetUserDisabled(_ context.Context, id int, disabled bool) error {
	u, ok := s.users[id]
	if !ok {
		return storage.ErrUserNotFound
	}
	if disabled {
		now := time.Now()
		u.DisabledAt = &now
	} else {
		u.DisabledAt = nil
	}
	return nil
}

type fakeAuth struct {
	initial        string
	revokedSession []int64
	tokens         map[int64][]auth.Token
	revokedTokens  []int64
}

func (f *fakeAuth) IsInitialAdmin(id auth.Identity) bool {
	return f.initial != "" && id.Email == f.initial
}
func (f *fakeAuth) RevokeUserSessions(_ context.Context, uid int64) error {
	f.revokedSession = append(f.revokedSession, uid)
	return nil
}
func (f *fakeAuth) ListTokens(_ context.Context, uid int64) ([]auth.Token, error) {
	return f.tokens[uid], nil
}
func (f *fakeAuth) RevokeToken(_ context.Context, _ int64, id int64) error {
	f.revokedTokens = append(f.revokedTokens, id)
	return nil
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func oidcID(email string) auth.Identity {
	return auth.Identity{Method: auth.MethodOIDC, Issuer: "https://idp", Subject: "s-" + email, Email: email, EmailVerified: true, Name: "N"}
}

// TestResolve covers the account policy: which logins get a user, and that a
// disabled or unlinked existing user is never signed in or taken over.
func TestResolve(t *testing.T) {
	ctx := context.Background()
	disabledAt := time.Now()
	base := func() *fakeStore {
		return newFakeStore(
			storage.User{ID: 2, Login: "linus@meltforce.net", IsAdmin: true},
			storage.User{ID: 3, Login: "off@meltforce.net", DisabledAt: &disabledAt},
		)
	}
	auto := config.AccountsConfig{Policy: config.PolicyOIDCAuto, AllowedEmailDomains: []string{"meltforce.net"}}

	t.Run("linked user", func(t *testing.T) {
		a := New(base(), auto, quiet)
		id := oidcID("linus@meltforce.net")
		id.UserID = 2
		got, err := a.Resolve(ctx, id)
		if err != nil || got != 2 {
			t.Fatalf("got %d, %v", got, err)
		}
	})
	t.Run("linked but disabled", func(t *testing.T) {
		a := New(base(), auto, quiet)
		id := oidcID("off@meltforce.net")
		id.UserID = 3
		if _, err := a.Resolve(ctx, id); !errors.Is(err, auth.ErrNoAccount) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("oidc_auto creates a user", func(t *testing.T) {
		st := base()
		a := New(st, auto, quiet)
		got, err := a.Resolve(ctx, oidcID("new@meltforce.net"))
		if err != nil {
			t.Fatal(err)
		}
		u := st.users[int(got)]
		if u.IsAdmin || u.Login != "new@meltforce.net" {
			t.Fatalf("created %+v", u)
		}
	})
	t.Run("oidc_auto refuses other domain", func(t *testing.T) {
		a := New(base(), auto, quiet)
		if _, err := a.Resolve(ctx, oidcID("x@evil.net")); !errors.Is(err, auth.ErrNoAccount) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unverified email refused", func(t *testing.T) {
		a := New(base(), auto, quiet)
		id := oidcID("new@meltforce.net")
		id.EmailVerified = false
		if _, err := a.Resolve(ctx, id); !errors.Is(err, auth.ErrNoAccount) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("closed refuses", func(t *testing.T) {
		a := New(base(), config.AccountsConfig{Policy: config.PolicyClosed}, quiet)
		if _, err := a.Resolve(ctx, oidcID("new@meltforce.net")); !errors.Is(err, auth.ErrNoAccount) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("existing unlinked login is not taken over", func(t *testing.T) {
		a := New(base(), auto, quiet)
		if _, err := a.Resolve(ctx, oidcID("linus@meltforce.net")); !errors.Is(err, auth.ErrNoAccount) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("initial admin while no admin exists, even when closed", func(t *testing.T) {
		st := newFakeStore()
		a := New(st, config.AccountsConfig{Policy: config.PolicyClosed}, quiet)
		a.Attach(&fakeAuth{initial: "boss@x.org"})
		got, err := a.Resolve(ctx, oidcID("boss@x.org"))
		if err != nil || !st.users[int(got)].IsAdmin {
			t.Fatalf("got %d, %v", got, err)
		}
	})
	t.Run("password login without user id refused", func(t *testing.T) {
		a := New(base(), auto, quiet)
		if _, err := a.Resolve(ctx, auth.Identity{Method: auth.MethodPassword, Subject: "x"}); !errors.Is(err, auth.ErrNoAccount) {
			t.Fatalf("err = %v", err)
		}
	})
}

// TestUserIDByLogin guards link_by_email, which carries the users WhoIs
// created over to their first OIDC login.
func TestUserIDByLogin(t *testing.T) {
	a := New(newFakeStore(storage.User{ID: 2, Login: "linus@meltforce.net"}), config.AccountsConfig{}, quiet)
	if id, err := a.UserIDByLogin(context.Background(), "linus@meltforce.net"); err != nil || id != 2 {
		t.Fatalf("hit: %d, %v", id, err)
	}
	if _, err := a.UserIDByLogin(context.Background(), "nobody"); !errors.Is(err, auth.ErrNoAccount) {
		t.Fatalf("miss: %v", err)
	}
}

// TestSetDisabledRevokes: the bearer path does not ask the resolver, so a
// disabled user stays signed in unless sessions and tokens are revoked.
func TestSetDisabledRevokes(t *testing.T) {
	ctx := context.Background()
	st := newFakeStore(
		storage.User{ID: 2, IsAdmin: true},
		storage.User{ID: 4},
	)
	fa := &fakeAuth{tokens: map[int64][]auth.Token{4: {{ID: 7}, {ID: 8}}}}
	a := New(st, config.AccountsConfig{}, quiet)
	a.Attach(fa)

	if err := a.SetDisabled(ctx, 2, 4, true); err != nil {
		t.Fatal(err)
	}
	if !st.users[4].Disabled() || len(fa.revokedSession) != 1 || len(fa.revokedTokens) != 2 {
		t.Fatalf("user %+v sessions %v tokens %v", st.users[4], fa.revokedSession, fa.revokedTokens)
	}
	if err := a.SetDisabled(ctx, 2, 2, true); !errors.Is(err, ErrSelf) {
		t.Fatalf("self disable: %v", err)
	}
}

// TestLastAdminKept: without an enabled admin nobody can administer users.
func TestLastAdminKept(t *testing.T) {
	ctx := context.Background()
	st := newFakeStore(storage.User{ID: 2, IsAdmin: true}, storage.User{ID: 4, IsAdmin: true})
	a := New(st, config.AccountsConfig{}, quiet)
	if err := a.SetAdmin(ctx, 4, 2, false); err != nil {
		t.Fatalf("demote with another admin left: %v", err)
	}
	st.users[2].IsAdmin = true
	st.users[4].IsAdmin = false
	if err := a.SetDisabled(ctx, 4, 2, true); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable last admin: %v", err)
	}
}

// TestParseExpiry: an unparsable lifetime must fail instead of creating a
// token without expiry.
func TestParseExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if got, _ := ParseExpiry("", now); !got.IsZero() {
		t.Error("empty must mean no expiry")
	}
	if got, _ := ParseExpiry("90d", now); !got.Equal(now.AddDate(0, 0, 90)) {
		t.Errorf("90d = %v", got)
	}
	if got, _ := ParseExpiry("2h", now); !got.Equal(now.Add(2 * time.Hour)) {
		t.Errorf("2h = %v", got)
	}
	for _, bad := range []string{"0d", "-1h", "9x", "xd"} {
		if _, err := ParseExpiry(bad, now); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
