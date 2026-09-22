package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SourceRun is one import_logs row reduced to what the alert rules read.
type SourceRun struct {
	UserID    int
	Status    string
	ErrorMsg  string
	CreatedAt time.Time
}

// RecentRunsBySource returns the newest `limit` runs per user for one import
// source, newest first within each user.
//
// The grouping is per user because a second user whose sync works would
// otherwise mask a first user whose sync does not: the rows of both arrive in
// one `import_logs` stream ordered by time alone.
func (db *DB) RecentRunsBySource(ctx context.Context, source string, limit int) (map[int][]SourceRun, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT user_id, status, COALESCE(error_message, ''), created_at
		   FROM (
		     SELECT user_id, status, error_message, created_at,
		            ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) AS rn
		       FROM import_logs
		      WHERE source = $1
		   ) ranked
		  WHERE rn <= $2
		  ORDER BY user_id, created_at DESC`,
		source, limit)
	if err != nil {
		return nil, fmt.Errorf("querying recent runs for %s: %w", source, err)
	}
	defer rows.Close()

	out := map[int][]SourceRun{}
	for rows.Next() {
		var r SourceRun
		if err := rows.Scan(&r.UserID, &r.Status, &r.ErrorMsg, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning recent run: %w", err)
		}
		out[r.UserID] = append(out[r.UserID], r)
	}
	return out, rows.Err()
}

// LastStoredMetricRunAt returns when a source last wrote an import log that
// stored at least one health metric, and false when it never has.
//
// This is the question the silence rule asks, rather than when a request last
// arrived: Health Auto Export repeats a fixed export window, so a phone whose
// last new sample is two days old keeps posting payloads whose rows are all
// duplicates. Measured on 2026-09-21: three deliveries of the same 24 workouts,
// 0 inserted each, while the rule that counted requests reported the ingress as
// healthy — see the 2026-09-21 entry in INCIDENTS.md.
//
// The condition names the metric channel alone. Health Auto Export runs one
// automation per data type, and they stop independently: from 2026-09-20 10:25
// the metric automation delivered nothing for 46 hours while the workout one
// kept posting every few hours. A rule that accepted any stored row read the
// workout deliveries as proof that the ingress was alive and reported "export
// stored 12h30m ago" throughout.
func (db *DB) LastStoredMetricRunAt(ctx context.Context, source string) (time.Time, bool, error) {
	return db.lastRunAt(ctx, source, "metrics_inserted > 0")
}

// LastWorkoutDeliveryAt returns when a source last delivered a workout, counting
// what arrived rather than what was stored.
//
// The metric channel and the workout channel need opposite tests. New metric
// samples accrue continuously, so an absence of newly stored rows is a signal.
// Workouts are sporadic — a rest day produces none — and the export resends a
// fixed window, so `workouts_inserted` is 0 on most deliveries and counting
// stored rows would fire on every quiet week. What is observable for workouts is
// whether the automation still posts at all.
func (db *DB) LastWorkoutDeliveryAt(ctx context.Context, source string) (time.Time, bool, error) {
	return db.lastRunAt(ctx, source, "workouts_received > 0")
}

// lastRunAt answers both of the above. The predicate is a constant from this
// file, never a caller's string.
func (db *DB) lastRunAt(ctx context.Context, source, predicate string) (time.Time, bool, error) {
	var t *time.Time
	err := db.Pool.QueryRow(ctx,
		`SELECT MAX(created_at) FROM import_logs WHERE source = $1 AND `+predicate,
		source).Scan(&t)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("querying last run for %s: %w", source, err)
	}
	if t == nil {
		return time.Time{}, false, nil
	}
	return *t, true, nil
}

// AlertSettings is the deployment-wide configuration of the alert channel. It
// lives in the database rather than only in config.yaml because the Settings UI
// owns it, like every other integration's configuration in this project.
type AlertSettings struct {
	Enabled          bool
	NtfyURL          string
	Hostname         string
	CheckInterval    time.Duration
	FailureThreshold int
	AppleSilence     time.Duration
	UpdatedAt        time.Time
}

// GetAlertSettings reads the single settings row. Known is false while no row
// exists, which is the state SeedAlertSettings resolves on startup.
func (db *DB) GetAlertSettings(ctx context.Context) (AlertSettings, bool, error) {
	var st AlertSettings
	var checkSec, silenceSec int
	err := db.Pool.QueryRow(ctx,
		`SELECT enabled, ntfy_url, hostname, check_interval_sec, failure_threshold,
		        apple_silence_sec, updated_at
		   FROM alert_settings WHERE id = 1`).
		Scan(&st.Enabled, &st.NtfyURL, &st.Hostname, &checkSec, &st.FailureThreshold,
			&silenceSec, &st.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AlertSettings{}, false, nil
		}
		return AlertSettings{}, false, fmt.Errorf("reading alert settings: %w", err)
	}
	st.CheckInterval = time.Duration(checkSec) * time.Second
	st.AppleSilence = time.Duration(silenceSec) * time.Second
	return st, true, nil
}

// SetAlertSettings replaces the settings row.
func (db *DB) SetAlertSettings(ctx context.Context, st AlertSettings) error {
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO alert_settings (id, enabled, ntfy_url, hostname, check_interval_sec,
		                            failure_threshold, apple_silence_sec, updated_at)
		 VALUES (1, $1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   enabled = EXCLUDED.enabled,
		   ntfy_url = EXCLUDED.ntfy_url,
		   hostname = EXCLUDED.hostname,
		   check_interval_sec = EXCLUDED.check_interval_sec,
		   failure_threshold = EXCLUDED.failure_threshold,
		   apple_silence_sec = EXCLUDED.apple_silence_sec,
		   updated_at = NOW()`,
		st.Enabled, st.NtfyURL, st.Hostname,
		int(st.CheckInterval.Seconds()), st.FailureThreshold,
		int(st.AppleSilence.Seconds())); err != nil {
		return fmt.Errorf("writing alert settings: %w", err)
	}
	return nil
}

// SeedAlertSettings writes the row from config.yaml when none exists yet, and
// leaves an existing row untouched.
//
// The asymmetry is deliberate: a redeploy must not overwrite what was entered in
// the UI, and a fresh database must not come up silent just because nobody has
// opened the Settings screen yet.
func (db *DB) SeedAlertSettings(ctx context.Context, st AlertSettings) (bool, error) {
	tag, err := db.Pool.Exec(ctx,
		`INSERT INTO alert_settings (id, enabled, ntfy_url, hostname, check_interval_sec,
		                            failure_threshold, apple_silence_sec)
		 VALUES (1, $1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO NOTHING`,
		st.Enabled, st.NtfyURL, st.Hostname,
		int(st.CheckInterval.Seconds()), st.FailureThreshold,
		int(st.AppleSilence.Seconds()))
	if err != nil {
		return false, fmt.Errorf("seeding alert settings: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// AlertConditionState is one row of alert_state as the Settings screen shows it.
type AlertConditionState struct {
	MonitorID int       `json:"monitor_id"`
	Firing    bool      `json:"firing"`
	Since     time.Time `json:"since"`
	LastMsg   string    `json:"last_msg"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertStates returns every stored condition, lowest monitor_id first.
func (db *DB) AlertStates(ctx context.Context) ([]AlertConditionState, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT monitor_id, firing, since, last_msg, updated_at
		   FROM alert_state ORDER BY monitor_id`)
	if err != nil {
		return nil, fmt.Errorf("querying alert states: %w", err)
	}
	defer rows.Close()

	var out []AlertConditionState
	for rows.Next() {
		var c AlertConditionState
		if err := rows.Scan(&c.MonitorID, &c.Firing, &c.Since, &c.LastMsg, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning alert state: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AlertState is the stored state of one alert condition. Known is false when no
// row exists yet, which counts as "not firing" and, for a condition that is not
// firing now, as "nothing to announce".
type AlertState struct {
	Firing bool
	Since  time.Time
	Known  bool
}

// GetAlertState reads the stored state of one condition.
func (db *DB) GetAlertState(ctx context.Context, monitorID int) (AlertState, error) {
	var st AlertState
	err := db.Pool.QueryRow(ctx,
		`SELECT firing, since FROM alert_state WHERE monitor_id = $1`, monitorID).
		Scan(&st.Firing, &st.Since)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AlertState{}, nil
		}
		return AlertState{}, fmt.Errorf("reading alert state %d: %w", monitorID, err)
	}
	st.Known = true
	return st, nil
}

// SetAlertState stores the state of one condition. The caller writes it only
// after a transition has been reported, so a send that fails leaves the previous
// state in place and the next check tries again.
func (db *DB) SetAlertState(ctx context.Context, monitorID int, firing bool, since time.Time, msg string) error {
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO alert_state (monitor_id, firing, since, last_msg, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (monitor_id) DO UPDATE SET
		   firing = EXCLUDED.firing,
		   since = EXCLUDED.since,
		   last_msg = EXCLUDED.last_msg,
		   updated_at = NOW()`,
		monitorID, firing, since, msg); err != nil {
		return fmt.Errorf("writing alert state %d: %w", monitorID, err)
	}
	return nil
}
