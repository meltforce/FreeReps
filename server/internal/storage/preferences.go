package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// PrefFrontPageHeroes names the four metrics the dashboard shows as hero
// numbers. Stored as a JSON array of metric names.
const PrefFrontPageHeroes = "front_page_heroes"

// PrefMaxHeartRate is the user's own maximum heart rate in bpm, which the
// training zones derive from. Stored as a JSON number.
const PrefMaxHeartRate = "max_heart_rate"

// PrefBirthDate is the user's date of birth as "YYYY-MM-DD". Its only use is
// estimating a maximum heart rate when none is configured; nothing else reads
// it, and the app asks for no other personal detail.
const PrefBirthDate = "birth_date"

// GetBirthDate returns the stored date of birth. The second value is false when
// the user has not set one.
func (db *DB) GetBirthDate(ctx context.Context, userID int) (time.Time, bool, error) {
	var raw string
	found, err := db.GetPreference(ctx, userID, PrefBirthDate, &raw)
	if err != nil || !found || raw == "" {
		return time.Time{}, false, err
	}
	d, err := time.Parse("2006-01-02", raw)
	if err != nil {
		// A malformed value costs the estimate, not the request.
		return time.Time{}, false, nil
	}
	return d, true, nil
}

// AgeYears returns completed years between birth and now.
//
// Compares month and day rather than day-of-year: 29 February shifts every
// later day-of-year by one, which would count a March birthday a year early in
// every leap year.
func AgeYears(birth, now time.Time) int {
	years := now.Year() - birth.Year()
	// Subtract one when this year's birthday has not come round yet.
	if now.Month() < birth.Month() ||
		(now.Month() == birth.Month() && now.Day() < birth.Day()) {
		years--
	}
	return years
}

// EstimatedMaxHeartRate is the Tanaka estimate, 208 minus 0.7 times age.
//
// Tanaka replaced Haskell-Fox (220 minus age) on 2026-09-27. The Apple Watch
// zone edges for the run of that day (130, 140, 151, 161 bpm at age 52) are
// reproduced by a maximum of 171.6, which is Tanaka's figure; Haskell-Fox gives
// 168. Tanaka is also the better fit above age 40, where Haskell-Fox
// underestimates.
//
// It is a population average with a standard deviation around 10 to 12 bpm, so
// it is a starting point rather than a measurement — which is why a figure the
// user configured always wins, and why a genuinely observed rate above the
// estimate wins too.
func EstimatedMaxHeartRate(age int) float64 {
	return 208 - 0.7*float64(age)
}

// DefaultFrontPageHeroes is used until the user picks their own. Readiness,
// sleep, HRV and resting heart rate answer "how am I doing today" without
// needing a chart.
var DefaultFrontPageHeroes = []string{
	"oura_readiness_score",
	"sleep_analysis",
	"heart_rate_variability",
	"resting_heart_rate",
}

// GetPreference reads one preference into dest. Returns false when the user has
// not set it, leaving dest untouched.
func (db *DB) GetPreference(ctx context.Context, userID int, key string, dest any) (bool, error) {
	var raw []byte
	err := db.Pool.QueryRow(ctx,
		`SELECT value FROM user_preferences WHERE user_id = $1 AND key = $2`,
		userID, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading preference %s: %w", key, err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, fmt.Errorf("decoding preference %s: %w", key, err)
	}
	return true, nil
}

// SetPreference writes one preference, replacing any previous value.
func (db *DB) SetPreference(ctx context.Context, userID int, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding preference %s: %w", key, err)
	}
	_, err = db.Pool.Exec(ctx,
		`INSERT INTO user_preferences (user_id, key, value)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, key) DO UPDATE SET value = EXCLUDED.value`,
		userID, key, raw)
	if err != nil {
		return fmt.Errorf("saving preference %s: %w", key, err)
	}
	return nil
}

// GetFrontPageHeroes returns the user's hero metric selection, falling back to
// the default set.
func (db *DB) GetFrontPageHeroes(ctx context.Context, userID int) ([]string, error) {
	var heroes []string
	found, err := db.GetPreference(ctx, userID, PrefFrontPageHeroes, &heroes)
	if err != nil {
		return nil, err
	}
	if !found || len(heroes) == 0 {
		return DefaultFrontPageHeroes, nil
	}
	return heroes, nil
}
