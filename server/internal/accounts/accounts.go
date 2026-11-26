// Package accounts maps meltkit pkg/auth identities onto FreeReps users and
// holds the account operations behind the account and admin endpoints.
package accounts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/claude/freereps/internal/config"
	"github.com/claude/freereps/internal/storage"
	"github.com/meltforce/meltkit/pkg/auth"
)

// ErrSelf refuses an admin action on the acting user's own account that
// would lock them out.
var ErrSelf = errors.New("not allowed on your own account")

// ErrLastAdmin refuses removing the last enabled admin.
var ErrLastAdmin = errors.New("the last admin cannot be removed")

// Store is the storage the resolver needs.
type Store interface {
	GetUser(ctx context.Context, id int) (*storage.User, error)
	GetUserByLogin(ctx context.Context, login string) (*storage.User, error)
	CreateUser(ctx context.Context, login, displayName string, admin bool) (*storage.User, error)
	ListUsers(ctx context.Context) ([]storage.User, error)
	TouchUser(ctx context.Context, id int, displayName string) error
	AdminExists(ctx context.Context) (bool, error)
	SetUserAdmin(ctx context.Context, id int, admin bool) error
	SetUserDisabled(ctx context.Context, id int, disabled bool) error
}

// Authenticator is the part of *auth.Auth the account operations call.
type Authenticator interface {
	IsInitialAdmin(id auth.Identity) bool
	RevokeUserSessions(ctx context.Context, userID int64) error
	ListTokens(ctx context.Context, userID int64) ([]auth.Token, error)
	RevokeToken(ctx context.Context, userID, tokenID int64) error
}

// Accounts implements auth.UserResolver and auth.LoginLookup.
type Accounts struct {
	store Store
	cfg   config.AccountsConfig
	auth  Authenticator
	log   *slog.Logger
}

// New returns the resolver. Attach the Authenticator once auth.New has
// returned, because auth.New takes the resolver.
func New(store Store, cfg config.AccountsConfig, log *slog.Logger) *Accounts {
	return &Accounts{store: store, cfg: cfg, log: log}
}

// Attach sets the Authenticator.
func (a *Accounts) Attach(au Authenticator) { a.auth = au }

// Resolve maps a verified login onto a user. A linked identity logs in its
// user unless that user is disabled. A first OIDC login creates a user when
// the policy admits it, or when it names the initial admin while no admin
// exists; it never takes over an existing login, which only link_by_email
// links.
func (a *Accounts) Resolve(ctx context.Context, id auth.Identity) (int64, error) {
	if id.UserID != 0 {
		u, err := a.store.GetUser(ctx, int(id.UserID))
		if errors.Is(err, storage.ErrUserNotFound) {
			return 0, auth.ErrNoAccount
		}
		if err != nil {
			return 0, err
		}
		if u.Disabled() {
			a.log.Info("login refused: account disabled", "user_id", u.ID)
			return 0, auth.ErrNoAccount
		}
		a.bootstrapAdmin(ctx, id, u)
		if err := a.store.TouchUser(ctx, u.ID, id.Name); err != nil {
			a.log.Warn("recording login failed", "user_id", u.ID, "error", err)
		}
		return int64(u.ID), nil
	}

	if id.Method != auth.MethodOIDC {
		return 0, auth.ErrNoAccount
	}
	login := strings.TrimSpace(id.Email)
	if login == "" || !id.EmailVerified {
		a.log.Info("login refused: OIDC identity without verified email", "issuer", id.Issuer, "subject", id.Subject)
		return 0, auth.ErrNoAccount
	}

	initial := false
	if a.auth != nil && a.auth.IsInitialAdmin(id) {
		exists, err := a.store.AdminExists(ctx)
		if err != nil {
			return 0, err
		}
		initial = !exists
	}
	if !initial {
		if a.cfg.Policy != config.PolicyOIDCAuto {
			a.log.Info("login refused: accounts are closed", "email", login)
			return 0, auth.ErrNoAccount
		}
		if !a.cfg.AllowsEmail(login) {
			a.log.Info("login refused: email domain not allowed", "email", login)
			return 0, auth.ErrNoAccount
		}
	}

	u, err := a.store.CreateUser(ctx, login, id.Name, initial)
	if errors.Is(err, storage.ErrUserExists) {
		a.log.Info("login refused: a user with this login exists and is not linked", "email", login)
		return 0, auth.ErrNoAccount
	}
	if err != nil {
		return 0, err
	}
	a.log.Info("account created by OIDC login", "user_id", u.ID, "email", login, "admin", initial)
	return int64(u.ID), nil
}

// bootstrapAdmin promotes the configured initial admin while no admin exists.
func (a *Accounts) bootstrapAdmin(ctx context.Context, id auth.Identity, u *storage.User) {
	if u.IsAdmin || a.auth == nil || !a.auth.IsInitialAdmin(id) {
		return
	}
	exists, err := a.store.AdminExists(ctx)
	if err != nil || exists {
		return
	}
	if err := a.store.SetUserAdmin(ctx, u.ID, true); err != nil {
		a.log.Warn("promoting initial admin failed", "user_id", u.ID, "error", err)
		return
	}
	a.log.Info("initial admin promoted", "user_id", u.ID)
}

// UserIDByLogin implements auth.LoginLookup. A Tailscale identity provider
// (tsidp) sends the Tailscale login name as the email claim, so the lookup
// finds the users WhoIs created.
func (a *Accounts) UserIDByLogin(ctx context.Context, login string) (int64, error) {
	u, err := a.store.GetUserByLogin(ctx, login)
	if errors.Is(err, storage.ErrUserNotFound) {
		return 0, auth.ErrNoAccount
	}
	if err != nil {
		return 0, err
	}
	return int64(u.ID), nil
}

// SetDisabled disables or re-enables a user. Disabling ends every session and
// revokes every token, because the bearer path does not ask the resolver.
func (a *Accounts) SetDisabled(ctx context.Context, actorID, userID int, disabled bool) error {
	if disabled && actorID == userID {
		return ErrSelf
	}
	if disabled {
		if err := a.keepAnAdmin(ctx, userID); err != nil {
			return err
		}
	}
	if err := a.store.SetUserDisabled(ctx, userID, disabled); err != nil {
		return err
	}
	if !disabled || a.auth == nil {
		return nil
	}
	if err := a.auth.RevokeUserSessions(ctx, int64(userID)); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	tokens, err := a.auth.ListTokens(ctx, int64(userID))
	if err != nil {
		return fmt.Errorf("list tokens: %w", err)
	}
	for _, t := range tokens {
		if err := a.auth.RevokeToken(ctx, int64(userID), t.ID); err != nil && !errors.Is(err, auth.ErrNotFound) {
			return fmt.Errorf("revoke token %d: %w", t.ID, err)
		}
	}
	return nil
}

// SetAdmin grants or removes the admin flag.
func (a *Accounts) SetAdmin(ctx context.Context, actorID, userID int, admin bool) error {
	if !admin && actorID == userID {
		return ErrSelf
	}
	if !admin {
		if err := a.keepAnAdmin(ctx, userID); err != nil {
			return err
		}
	}
	return a.store.SetUserAdmin(ctx, userID, admin)
}

// keepAnAdmin refuses when userID is the only enabled admin.
func (a *Accounts) keepAnAdmin(ctx context.Context, userID int) error {
	users, err := a.store.ListUsers(ctx)
	if err != nil {
		return err
	}
	others := 0
	target := false
	for _, u := range users {
		if !u.IsAdmin || u.Disabled() {
			continue
		}
		if u.ID == userID {
			target = true
		} else {
			others++
		}
	}
	if target && others == 0 {
		return ErrLastAdmin
	}
	return nil
}

// ParseExpiry turns "90d", "720h" or "" (no expiry) into an absolute time.
func ParseExpiry(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return time.Time{}, fmt.Errorf("invalid expiry %q", s)
		}
		return now.AddDate(0, 0, n), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return time.Time{}, fmt.Errorf("invalid expiry %q", s)
	}
	return now.Add(d), nil
}
