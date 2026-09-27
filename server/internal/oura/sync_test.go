package oura

import "testing"

// TestRunVerdict catches a run whose token was rejected by every endpoint being
// logged as success: before 2026-09-27 every 401/403 was a debug line, so such
// a run wrote status 'success' with zero rows and the failure alert never read
// it. It also catches the opposite error, a single missing scope failing a run
// whose other endpoints delivered.
func TestRunVerdict(t *testing.T) {
	cases := []struct {
		name                    string
		succeeded, unauthorized int
		wantErr                 bool
	}{
		{"every data type answered 401", 0, 11, true},
		{"401 everywhere except a 404 endpoint", 0, 10, true},
		{"a missing scope on one endpoint", 10, 1, false},
		{"only 404 answers", 0, 0, false},
		{"all succeeded", 11, 0, false},
	}
	for _, c := range cases {
		err := runVerdict(c.succeeded, c.unauthorized, 11)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: runVerdict(%d, %d) = %v, want error %v", c.name, c.succeeded, c.unauthorized, err, c.wantErr)
		}
	}
}
