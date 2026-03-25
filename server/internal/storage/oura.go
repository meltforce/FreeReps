package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// OuraToken holds per-user Oura OAuth2 credentials and tokens.
// ClientID/ClientSecret are the user's Oura developer app credentials.
// AccessToken/RefreshToken are populated after OAuth2 authorization.
type OuraToken struct {
	UserID       int
	ClientID     string
	ClientSecret string
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    time.Time
	UpdatedAt    time.Time
}

// OuraSyncState tracks the last sync date for a specific data type.
type OuraSyncState struct {
	UserID    int
	DataType  string
	LastSync  time.Time
	UpdatedAt time.Time
}

// UpsertOuraCredentials stores or updates a user's Oura developer app credentials
// (before OAuth2 authorization). Does not overwrite existing tokens.
func (db *DB) UpsertOuraCredentials(ctx context.Context, userID int, clientID, clientSecret string) error {
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO oura_tokens (user_id, client_id, client_secret, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET
		   client_id = EXCLUDED.client_id,
		   client_secret = EXCLUDED.client_secret,
		   updated_at = NOW()`,
		userID, clientID, clientSecret)
	if err != nil {
		return fmt.Errorf("upserting oura credentials: %w", err)
	}
	return nil
}

// UpsertOuraToken stores or updates OAuth2 tokens for a user.
// Preserves existing client_id/client_secret.
func (db *DB) UpsertOuraToken(ctx context.Context, tok OuraToken) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE oura_tokens SET
		   access_token = $2,
		   refresh_token = $3,
		   token_type = $4,
		   expires_at = $5,
		   updated_at = NOW()
		 WHERE user_id = $1`,
		tok.UserID, tok.AccessToken, tok.RefreshToken, tok.TokenType, tok.ExpiresAt)
	if err != nil {
		return fmt.Errorf("upserting oura token: %w", err)
	}
	return nil
}

// GetOuraToken retrieves the Oura credentials and tokens for a user. Returns nil if not found.
func (db *DB) GetOuraToken(ctx context.Context, userID int) (*OuraToken, error) {
	var tok OuraToken
	err := db.Pool.QueryRow(ctx,
		`SELECT user_id, client_id, client_secret, access_token, refresh_token, token_type, expires_at, updated_at
		 FROM oura_tokens WHERE user_id = $1`, userID).
		Scan(&tok.UserID, &tok.ClientID, &tok.ClientSecret, &tok.AccessToken, &tok.RefreshToken, &tok.TokenType, &tok.ExpiresAt, &tok.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting oura token: %w", err)
	}
	return &tok, nil
}

// DeleteOuraToken removes the OAuth2 token for a user.
func (db *DB) DeleteOuraToken(ctx context.Context, userID int) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM oura_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("deleting oura token: %w", err)
	}
	return nil
}

// ListOuraTokenUsers returns all user IDs that have Oura tokens stored.
func (db *DB) ListOuraTokenUsers(ctx context.Context) ([]int, error) {
	rows, err := db.Pool.Query(ctx, `SELECT user_id FROM oura_tokens ORDER BY user_id`)
	if err != nil {
		return nil, fmt.Errorf("listing oura token users: %w", err)
	}
	defer rows.Close()

	var users []int
	for rows.Next() {
		var uid int
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("scanning oura token user: %w", err)
		}
		users = append(users, uid)
	}
	return users, rows.Err()
}

// GetOuraSyncState retrieves the last sync date for a data type.
// Returns nil if no state exists (first sync).
func (db *DB) GetOuraSyncState(ctx context.Context, userID int, dataType string) (*OuraSyncState, error) {
	var s OuraSyncState
	err := db.Pool.QueryRow(ctx,
		`SELECT user_id, data_type, last_sync, updated_at
		 FROM oura_sync_state WHERE user_id = $1 AND data_type = $2`,
		userID, dataType).
		Scan(&s.UserID, &s.DataType, &s.LastSync, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting oura sync state: %w", err)
	}
	return &s, nil
}

// UpsertOuraSyncState updates the last sync date for a data type.
func (db *DB) UpsertOuraSyncState(ctx context.Context, userID int, dataType string, lastSync time.Time) error {
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO oura_sync_state (user_id, data_type, last_sync, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id, data_type) DO UPDATE SET
		   last_sync = EXCLUDED.last_sync,
		   updated_at = NOW()`,
		userID, dataType, lastSync)
	if err != nil {
		return fmt.Errorf("upserting oura sync state: %w", err)
	}
	return nil
}

// DeleteOuraSyncStates removes all sync state for a user (used on disconnect).
func (db *DB) DeleteOuraSyncStates(ctx context.Context, userID int) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM oura_sync_state WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("deleting oura sync states: %w", err)
	}
	return nil
}

// ListOuraSyncStates retrieves all sync states for a user.
func (db *DB) ListOuraSyncStates(ctx context.Context, userID int) ([]OuraSyncState, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT user_id, data_type, last_sync, updated_at
		 FROM oura_sync_state WHERE user_id = $1 ORDER BY data_type`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("listing oura sync states: %w", err)
	}
	defer rows.Close()

	var states []OuraSyncState
	for rows.Next() {
		var s OuraSyncState
		if err := rows.Scan(&s.UserID, &s.DataType, &s.LastSync, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning oura sync state: %w", err)
		}
		states = append(states, s)
	}
	return states, rows.Err()
}
