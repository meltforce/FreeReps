package storage

import "context"

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
