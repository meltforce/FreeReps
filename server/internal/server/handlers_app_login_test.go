package server

import (
	"strings"
	"testing"
	"time"
)

// TestAppLoginCodes: a code is the only thing between a browser session and
// a token, so it must work once, expire, and not pile up without bound.
func TestAppLoginCodes(t *testing.T) {
	now := time.Now()
	var c appLoginCodes
	code, ok := c.add(appLogin{userID: 2, challenge: "x", expires: now.Add(appLoginCodeTTL)}, now)
	if !ok || code == "" {
		t.Fatal("add failed")
	}
	if l, ok := c.take(code, now); !ok || l.userID != 2 {
		t.Fatalf("first take: %+v %v", l, ok)
	}
	if _, ok := c.take(code, now); ok {
		t.Fatal("a code was redeemed twice")
	}
	if _, ok := c.take("unknown", now); ok {
		t.Fatal("unknown code accepted")
	}

	old, _ := c.add(appLogin{userID: 2, expires: now.Add(appLoginCodeTTL)}, now)
	if _, ok := c.take(old, now.Add(appLoginCodeTTL+time.Second)); ok {
		t.Fatal("expired code accepted")
	}

	var full appLoginCodes
	for range maxPendingAppLogins {
		if _, ok := full.add(appLogin{expires: now.Add(appLoginCodeTTL)}, now); !ok {
			t.Fatal("add below the bound failed")
		}
	}
	if _, ok := full.add(appLogin{expires: now.Add(appLoginCodeTTL)}, now); ok {
		t.Fatal("pending codes are unbounded")
	}
	// Expired entries make room again.
	if _, ok := full.add(appLogin{expires: now.Add(2 * appLoginCodeTTL)}, now.Add(appLoginCodeTTL+time.Second)); !ok {
		t.Fatal("expired codes were not dropped")
	}
}

// TestAppLoginTokenName: the device name comes from the request and is shown
// in Settings → Account.
func TestAppLoginTokenName(t *testing.T) {
	for in, want := range map[string]string{
		"":               "FreeReps iOS",
		"  ":             "FreeReps iOS",
		"Linus’s iPhone": "FreeReps iOS – Linus’s iPhone",
		"a\nb\x00c":      "FreeReps iOS – abc",
	} {
		if got := appLoginTokenName(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
	if got := []rune(appLoginTokenName(strings.Repeat("ä", 200))); len(got) > 80 {
		t.Errorf("name not truncated: %d runes", len(got))
	}
}
