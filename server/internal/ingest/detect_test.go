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

// TestDetectAlphaWithBOM verifies detection works when file starts with a UTF-8 BOM.
func TestDetectAlphaWithBOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	input := append(bom, []byte(`"Push A";"2026-02-19 4:54 h";"1:02 hr"`)...)
	if got := DetectFormat(input); got != FormatAlpha {
		t.Errorf("DetectFormat(BOM + session header) = %q, want %q", got, FormatAlpha)
	}
}

// TestDetectAlphaExerciseHeader verifies detection via exercise header pattern.
func TestDetectAlphaExerciseHeader(t *testing.T) {
	input := []byte(`"1. Bench Press · Barbell · 8 reps"` + "\n" + "#;KG;REPS;RIR\n1;100;8;2\n")
	if got := DetectFormat(input); got != FormatAlpha {
		t.Errorf("DetectFormat(exercise header) = %q, want %q", got, FormatAlpha)
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
