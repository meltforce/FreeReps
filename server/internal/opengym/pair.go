package opengym

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// pairCodeAlphabet is the character set openGym mints pairing codes from.
const pairCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// ErrDeviceLinkCode is a code in the XXXX-XXXX-XXXX shape of openGym's *Add a
// device* link, which registers a passkey and is not a pairing code. The mix-up
// happened on 2026-11-05, and redeeming it would answer only pair-invalid.
var ErrDeviceLinkCode = errors.New("this is a device link code (XXXX-XXXX-XXXX) from \"Add a device\"; " +
	"use the 8-character code from Settings → Account → \"Pair the mobile app\"")

// NormalizePairCode upper-cases a pairing code and drops blanks, and refuses a
// value that cannot be one before it is sent: a code is valid once, so a
// request with a mistyped one would cost the operator a new code.
func NormalizePairCode(code string) (string, error) {
	c := strings.ToUpper(strings.Join(strings.Fields(code), ""))
	if strings.Contains(c, "-") {
		return "", ErrDeviceLinkCode
	}
	if len(c) != 8 {
		return "", fmt.Errorf("a pairing code has 8 characters, this one has %d", len(c))
	}
	for _, r := range c {
		if !strings.ContainsRune(pairCodeAlphabet, r) {
			return "", fmt.Errorf("a pairing code does not contain %q", r)
		}
	}
	return c, nil
}

// Pair redeems a pairing code and validates the token it yields. It returns the
// token to store, which is the renewed one if /api/me already issued it.
func Pair(ctx context.Context, c *Client, code string) (string, *Me, error) {
	token, err := c.Redeem(ctx, code)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == CodePairInvalid {
			return "", nil, errors.New("openGym does not know this code, or it has expired (codes are valid for 5 minutes and once); mint a new one")
		}
		return "", nil, fmt.Errorf("redeeming the pairing code: %w", err)
	}
	me, err := c.Me(ctx, token)
	if err != nil {
		return "", nil, fmt.Errorf("validating the new token: %w", err)
	}
	if me.Token != "" {
		token = me.Token
	}
	return token, me, nil
}

// ResolveSyncFrom decides the ingest cutoff for a new connection.
//
// openGym's history contains the workouts imported from Hevy, which FreeReps
// holds under source Hevy, and the training aggregates do not filter on source.
// The cutoff therefore has to lie after the day of the last Hevy session. An
// empty request takes the earliest such day, or today when no Hevy session is
// stored. Days are taken in loc, the session timezone.
func ResolveSyncFrom(requested string, lastHevy time.Time, hasHevy bool, loc *time.Location, now time.Time) (time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	earliest := time.Time{}
	if hasHevy {
		y, m, d := lastHevy.In(loc).Date()
		earliest = time.Date(y, m, d+1, 0, 0, 0, 0, time.UTC)
	}

	if requested == "" {
		if hasHevy {
			return earliest, nil
		}
		y, m, d := now.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
	}

	day, err := time.Parse("2006-01-02", requested)
	if err != nil {
		return time.Time{}, errors.New("sync_from must be YYYY-MM-DD")
	}
	if hasHevy && day.Before(earliest) {
		return time.Time{}, fmt.Errorf("sync_from must be %s or later: the last Hevy session is on %s, and openGym's history "+
			"contains the Hevy workouts FreeReps already holds, which would count twice",
			earliest.Format("2006-01-02"), lastHevy.In(loc).Format("2006-01-02"))
	}
	return day, nil
}
