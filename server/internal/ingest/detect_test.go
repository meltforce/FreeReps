package ingest

import "testing"

// TestDetectAlphaSessionHeader verifies that DetectFormat recognises Alpha
// Progression CSV exports by their session header line.
func TestDetectAlphaSessionHeader(t *testing.T) {
	input := []byte(`"Push A";"2026-02-19 4:54 h";"1:02 hr"` + "\n" + `"1. Bench Press · Barbell · 8 reps"`)
	if got := DetectFormat(input); got != FormatAlpha {
		t.Errorf("DetectFormat(session header) = %q, want %q", got, FormatAlpha)
	}
}

// TestDetectAlphaColumnHeader verifies detection via the #;KG;REPS;RIR column header.
func TestDetectAlphaColumnHeader(t *testing.T) {
	input := []byte("#;KG;REPS;RIR\n1;100;8;2\n")
	if got := DetectFormat(input); got != FormatAlpha {
		t.Errorf("DetectFormat(column header) = %q, want %q", got, FormatAlpha)
	}
}

// TestDetectUnknownCSV verifies that random CSV content returns FormatUnknown.
func TestDetectUnknownCSV(t *testing.T) {
	input := []byte("name,age,city\nAlice,30,NYC\n")
	if got := DetectFormat(input); got != FormatUnknown {
		t.Errorf("DetectFormat(random CSV) = %q, want %q", got, FormatUnknown)
	}
}

// TestDetectEmpty verifies that empty input returns FormatUnknown.
func TestDetectEmpty(t *testing.T) {
	if got := DetectFormat(nil); got != FormatUnknown {
		t.Errorf("DetectFormat(nil) = %q, want %q", got, FormatUnknown)
	}
	if got := DetectFormat([]byte{}); got != FormatUnknown {
		t.Errorf("DetectFormat(empty) = %q, want %q", got, FormatUnknown)
	}
}
