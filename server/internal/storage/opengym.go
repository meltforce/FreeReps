package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// OpenGymCredentials holds a user's openGym instance, session token and ingest
// cutoff. The token is a full sign-in to the profile, including write access;
// openGym has no scoped token (server/specs/opengym-api.md).
type OpenGymCredentials struct {
	UserID    int
	BaseURL   string
	Token     string
	SyncFrom  time.Time // a calendar day; the time of day and zone carry no meaning
	UpdatedAt time.Time
}

// OpenGymSyncState is the (rev, wid) pair of the state the last error-free
// cycle read.
type OpenGymSyncState struct {
	Rev       int64
	WID       string
	UpdatedAt time.Time
}

// MappedExercise is the catalog entry an exercise of a source is mapped onto.
type MappedExercise struct {
	TemplateID   string
	Title        string
	ExerciseType string
}

// UpsertOpenGymCredentials stores or replaces a user's openGym connection.
// Replacing it clears the sync state, so the next cycle reads the state of the
// (possibly different) instance in full.
func (db *DB) UpsertOpenGymCredentials(ctx context.Context, userID int, baseURL, token string, syncFrom time.Time) error {
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO opengym_credentials (user_id, base_url, token, sync_from, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET
		   base_url = EXCLUDED.base_url,
		   token = EXCLUDED.token,
		   sync_from = EXCLUDED.sync_from,
		   updated_at = NOW()`,
		userID, baseURL, token, syncFrom); err != nil {
		return fmt.Errorf("upserting openGym credentials: %w", err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM opengym_sync_state WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clearing openGym sync state: %w", err)
	}
	return nil
}

// UpdateOpenGymToken replaces the stored token with the one openGym renewed.
func (db *DB) UpdateOpenGymToken(ctx context.Context, userID int, token string) error {
	if _, err := db.Pool.Exec(ctx,
		`UPDATE opengym_credentials SET token = $2, updated_at = NOW() WHERE user_id = $1`,
		userID, token); err != nil {
		return fmt.Errorf("updating openGym token: %w", err)
	}
	return nil
}

// GetOpenGymCredentials returns a user's credentials, or (nil, nil) if none are stored.
func (db *DB) GetOpenGymCredentials(ctx context.Context, userID int) (*OpenGymCredentials, error) {
	var c OpenGymCredentials
	err := db.Pool.QueryRow(ctx,
		`SELECT user_id, base_url, token, sync_from, updated_at
		 FROM opengym_credentials WHERE user_id = $1`, userID).
		Scan(&c.UserID, &c.BaseURL, &c.Token, &c.SyncFrom, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting openGym credentials: %w", err)
	}
	return &c, nil
}

// DeleteOpenGymCredentials removes a user's connection, sync state and workout
// hashes. Sets already ingested stay in workout_sets.
func (db *DB) DeleteOpenGymCredentials(ctx context.Context, userID int) error {
	for _, stmt := range []string{
		`DELETE FROM opengym_workouts WHERE user_id = $1`,
		`DELETE FROM opengym_sync_state WHERE user_id = $1`,
		`DELETE FROM opengym_credentials WHERE user_id = $1`,
	} {
		if _, err := db.Pool.Exec(ctx, stmt, userID); err != nil {
			return fmt.Errorf("deleting openGym connection: %w", err)
		}
	}
	return nil
}

// ListOpenGymCredentialUsers returns the IDs of all users with a stored connection.
func (db *DB) ListOpenGymCredentialUsers(ctx context.Context) ([]int, error) {
	rows, err := db.Pool.Query(ctx, `SELECT user_id FROM opengym_credentials ORDER BY user_id`)
	if err != nil {
		return nil, fmt.Errorf("listing openGym users: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning openGym user: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetOpenGymSyncState returns the stored (rev, wid), or (nil, nil) before the
// first error-free cycle.
func (db *DB) GetOpenGymSyncState(ctx context.Context, userID int) (*OpenGymSyncState, error) {
	var s OpenGymSyncState
	err := db.Pool.QueryRow(ctx,
		`SELECT rev, wid, updated_at FROM opengym_sync_state WHERE user_id = $1`, userID).
		Scan(&s.Rev, &s.WID, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting openGym sync state: %w", err)
	}
	return &s, nil
}

// UpsertOpenGymSyncState records the (rev, wid) of a state read without errors.
func (db *DB) UpsertOpenGymSyncState(ctx context.Context, userID int, rev int64, wid string) error {
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO opengym_sync_state (user_id, rev, wid, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET rev = EXCLUDED.rev, wid = EXCLUDED.wid, updated_at = NOW()`,
		userID, rev, wid); err != nil {
		return fmt.Errorf("upserting openGym sync state: %w", err)
	}
	return nil
}

// ListOpenGymWorkoutHashes returns the content hash of every stored openGym
// workout of a user, keyed by openGym workout id.
func (db *DB) ListOpenGymWorkoutHashes(ctx context.Context, userID int) (map[string]string, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT workout_id, content_hash FROM opengym_workouts WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing openGym workout hashes: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return nil, fmt.Errorf("scanning openGym workout hash: %w", err)
		}
		out[id] = hash
	}
	return out, rows.Err()
}

// UpsertOpenGymWorkoutHash records the hash of the rows stored for a workout.
func (db *DB) UpsertOpenGymWorkoutHash(ctx context.Context, userID int, workoutID, hash string) error {
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO opengym_workouts (user_id, workout_id, content_hash, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (user_id, workout_id) DO UPDATE SET content_hash = EXCLUDED.content_hash, updated_at = NOW()`,
		userID, workoutID, hash); err != nil {
		return fmt.Errorf("upserting openGym workout hash: %w", err)
	}
	return nil
}

// DeleteOpenGymWorkoutHash forgets a workout that was removed in openGym.
func (db *DB) DeleteOpenGymWorkoutHash(ctx context.Context, userID int, workoutID string) error {
	if _, err := db.Pool.Exec(ctx,
		`DELETE FROM opengym_workouts WHERE user_id = $1 AND workout_id = $2`, userID, workoutID); err != nil {
		return fmt.Errorf("deleting openGym workout hash: %w", err)
	}
	return nil
}

// MappedExercisesBySource returns the catalog entry behind every mapped key of
// one source in exercise_name_map. Keys mapped to no entry (NULL template) are
// left out.
func (db *DB) MappedExercisesBySource(ctx context.Context, source string) (map[string]MappedExercise, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT m.source_key, t.id, t.title, t.exercise_type
		 FROM exercise_name_map m
		 JOIN exercise_templates t ON t.id = m.exercise_template_id
		 WHERE m.source = $1`, source)
	if err != nil {
		return nil, fmt.Errorf("listing mapped exercises: %w", err)
	}
	defer rows.Close()

	out := map[string]MappedExercise{}
	for rows.Next() {
		var key string
		var e MappedExercise
		if err := rows.Scan(&key, &e.TemplateID, &e.Title, &e.ExerciseType); err != nil {
			return nil, fmt.Errorf("scanning mapped exercise: %w", err)
		}
		out[key] = e
	}
	return out, rows.Err()
}

// LastSessionDate returns the start of the newest session a source stored for a
// user, and false when the source stored none.
func (db *DB) LastSessionDate(ctx context.Context, userID int, source string) (time.Time, bool, error) {
	var t *time.Time
	if err := db.Pool.QueryRow(ctx,
		`SELECT MAX(session_date) FROM workout_sets WHERE user_id = $1 AND source = $2`,
		userID, source).Scan(&t); err != nil {
		return time.Time{}, false, fmt.Errorf("reading last session date: %w", err)
	}
	if t == nil {
		return time.Time{}, false, nil
	}
	return *t, true, nil
}
