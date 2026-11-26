package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrUserNotFound is returned when no user row matches.
var ErrUserNotFound = errors.New("user not found")

// ErrUserExists is returned when a user with the same login exists.
var ErrUserExists = errors.New("user exists")

// User is one person whose data the server holds. Login is the Tailscale
// login name for users created through WhoIs, the email for users created by
// an OIDC login, the username for password users.
type User struct {
	ID          int
	Login       string
	DisplayName string
	IsAdmin     bool
	DisabledAt  *time.Time
	CreatedAt   time.Time
	LastSeen    time.Time
}

// Disabled reports whether the user is refused on every authentication path.
func (u *User) Disabled() bool { return u.DisabledAt != nil }

const userCols = `id, login, COALESCE(display_name, ''), is_admin, disabled_at, created_at, last_seen`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Login, &u.DisplayName, &u.IsAdmin, &u.DisabledAt, &u.CreatedAt, &u.LastSeen)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetOrCreateUser finds or creates a user by Tailscale login name.
// Returns the user ID. Updates last_seen and display_name on each call.
func (db *DB) GetOrCreateUser(ctx context.Context, login, displayName string) (int, error) {
	var id int
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO users (login, display_name)
		VALUES ($1, $2)
		ON CONFLICT (login) DO UPDATE
			SET last_seen = NOW(), display_name = COALESCE(NULLIF($2, ''), users.display_name)
		RETURNING id
	`, login, displayName).Scan(&id)
	return id, err
}

// GetPrimaryUser returns the first user with a real Tailscale login (contains @).
// Returns pgx.ErrNoRows if no real user has logged in yet.
func (db *DB) GetPrimaryUser(ctx context.Context) (id int, login string, err error) {
	err = db.Pool.QueryRow(ctx, `
		SELECT id, login FROM users
		WHERE login LIKE '%@%'
		ORDER BY created_at ASC
		LIMIT 1
	`).Scan(&id, &login)
	if err == pgx.ErrNoRows {
		return 0, "", pgx.ErrNoRows
	}
	return
}

// CreateUser inserts a user. It returns ErrUserExists when the login is taken.
func (db *DB) CreateUser(ctx context.Context, login, displayName string, admin bool) (*User, error) {
	u, err := scanUser(db.Pool.QueryRow(ctx, `
		INSERT INTO users (login, display_name, is_admin)
		VALUES ($1, $2, $3)
		RETURNING `+userCols, login, displayName, admin))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil, ErrUserExists
	}
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// DeleteUser removes a user row. It serves the rollback of a failed account
// creation; a user with data is disabled, not deleted.
func (db *DB) DeleteUser(ctx context.Context, id int) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// GetUser loads a user by ID. It returns ErrUserNotFound when none exists.
func (db *DB) GetUser(ctx context.Context, id int) (*User, error) {
	u, err := scanUser(db.Pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", id, err)
	}
	return u, nil
}

// GetUserByLogin loads a user by login. It returns ErrUserNotFound when none
// exists.
func (db *DB) GetUserByLogin(ctx context.Context, login string) (*User, error) {
	u, err := scanUser(db.Pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE login = $1`, login))
	if err != nil {
		return nil, fmt.Errorf("get user %q: %w", login, err)
	}
	return u, nil
}

// ListUsers returns every user, ordered by ID.
func (db *DB) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := db.Pool.Query(ctx, `SELECT `+userCols+` FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// TouchUser records a login: last_seen, and the display name when one is given.
func (db *DB) TouchUser(ctx context.Context, id int, displayName string) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE users SET last_seen = NOW(),
		    display_name = CASE WHEN $2 = '' THEN display_name ELSE $2 END
		WHERE id = $1`, id, displayName)
	if err != nil {
		return fmt.Errorf("touch user %d: %w", id, err)
	}
	return nil
}

// AdminExists reports whether an enabled admin exists.
func (db *DB) AdminExists(ctx context.Context) (bool, error) {
	var ok bool
	err := db.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE is_admin AND disabled_at IS NULL)`).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("admin exists: %w", err)
	}
	return ok, nil
}

// SetUserAdmin sets the admin flag.
func (db *DB) SetUserAdmin(ctx context.Context, id int, admin bool) error {
	return db.updateUser(ctx, `UPDATE users SET is_admin = $2 WHERE id = $1`, id, admin)
}

// SetUserDisabled disables or re-enables a user.
func (db *DB) SetUserDisabled(ctx context.Context, id int, disabled bool) error {
	return db.updateUser(ctx, `
		UPDATE users SET disabled_at = CASE WHEN $2 THEN COALESCE(disabled_at, NOW()) END
		WHERE id = $1`, id, disabled)
}

func (db *DB) updateUser(ctx context.Context, sql string, args ...any) error {
	tag, err := db.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
