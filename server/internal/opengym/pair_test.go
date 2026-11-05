package opengym

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizePairCode(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr error
	}{
		{"abcd2345", "ABCD2345", nil},
		{" ab cd 23 45 ", "ABCD2345", nil},
		{"ABCD-EFGH-JKLM", "", ErrDeviceLinkCode},
		{"ABC1234O", "", errors.New("")}, // 1 and O are not in openGym's alphabet
		{"ABCD234", "", errors.New("")},
	}
	for _, tt := range tests {
		got, err := NormalizePairCode(tt.in)
		switch {
		case tt.wantErr == nil && err != nil:
			t.Errorf("%q: unexpected error %v", tt.in, err)
		case tt.wantErr != nil && err == nil:
			t.Errorf("%q: got %q, want an error", tt.in, got)
		case errors.Is(tt.wantErr, ErrDeviceLinkCode) && !errors.Is(err, ErrDeviceLinkCode):
			t.Errorf("%q: err = %v, want the device-link hint", tt.in, err)
		case tt.wantErr == nil && got != tt.want:
			t.Errorf("%q: got %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	good := map[string]string{
		"https://gym.meltrocker.net":   "https://gym.meltrocker.net",
		" https://gym.meltrocker.net/": "https://gym.meltrocker.net",
	}
	for in, want := range good {
		if got, err := NormalizeBaseURL(in); err != nil || got != want {
			t.Errorf("%q: got %q, %v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"http://gym.meltrocker.net", "gym.meltrocker.net", "https://gym.meltrocker.net/api", "https://u:p@gym.meltrocker.net"} {
		if got, err := NormalizeBaseURL(in); err == nil {
			t.Errorf("%q: accepted as %q, want an error", in, got)
		}
	}
}

func TestResolveSyncFrom(t *testing.T) {
	loc := berlin(t)
	// 2026-11-05 23:30 UTC is 2026-11-06 00:30 in Berlin, so the last Hevy
	// session falls on the 6th and the earliest cutoff is the 7th.
	lastHevy := time.Date(2026, 11, 5, 23, 30, 0, 0, time.UTC)
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

	got, err := ResolveSyncFrom("", lastHevy, true, loc, now)
	if err != nil || got.Format("2006-01-02") != "2026-11-07" {
		t.Errorf("default after a Berlin-time Hevy session on the 6th = %v, %v, want 2026-11-07", got, err)
	}
	if _, err := ResolveSyncFrom("2026-11-06", lastHevy, true, loc, now); err == nil {
		t.Error("a cutoff on the day of the last Hevy session was accepted")
	}
	if got, err := ResolveSyncFrom("2026-11-22", lastHevy, true, loc, now); err != nil || got.Format("2006-01-02") != "2026-11-22" {
		t.Errorf("explicit later cutoff = %v, %v", got, err)
	}
	if got, err := ResolveSyncFrom("", time.Time{}, false, loc, now); err != nil || got.Format("2006-01-02") != "2026-11-07" {
		t.Errorf("default without Hevy = %v, %v, want today", got, err)
	}
	if got, err := ResolveSyncFrom("2026-01-01", time.Time{}, false, loc, now); err != nil || got.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("explicit cutoff without Hevy = %v, %v", got, err)
	}
	if _, err := ResolveSyncFrom("05.11.2026", time.Time{}, false, loc, now); err == nil {
		t.Error("a non-ISO date was accepted")
	}
}

// The answers recorded from the instance on 2026-11-05 (spec, Part 1).
func TestPairAgainstRecordedAnswers(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/pair/redeem":
			var body struct{ Code string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Code != "ABCD2345" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid or expired code","code":"pair-invalid"}`))
				return
			}
			_, _ = w.Write([]byte(`{"token":"tok-new"}`))
		case "/api/me":
			gotAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"user":{"id":"u1","name":"Linus","admin":true,"owner":true}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	token, me, err := Pair(context.Background(), c, "ABCD2345")
	if err != nil {
		t.Fatal(err)
	}
	if token != "tok-new" || me.User.Name != "Linus" || gotAuth != "Bearer tok-new" {
		t.Errorf("token/name/auth = %q/%q/%q", token, me.User.Name, gotAuth)
	}

	_, _, err = Pair(context.Background(), c, "ZZZZ2345")
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("pair-invalid: err = %v, want the expired-code hint", err)
	}
}
