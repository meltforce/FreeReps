package models

import "testing"

// TestNormalizeSleepStage_English verifies that canonical English names pass
// through unchanged, confirming the map covers all standard stages.
func TestNormalizeSleepStage_English(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Core", "Core"},
		{"Deep", "Deep"},
		{"REM", "REM"},
		{"Awake", "Awake"},
		{"In Bed", "In Bed"},
		{"Asleep", "Asleep"},
	}
	for _, tc := range cases {
		got, known := NormalizeSleepStage(tc.input)
		if !known {
			t.Errorf("NormalizeSleepStage(%q): expected known=true", tc.input)
		}
		if got != tc.want {
			t.Errorf("NormalizeSleepStage(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestNormalizeSleepStage_German verifies that German-localized stage names
// (as sent by iPhones with German locale) are normalized correctly.
func TestNormalizeSleepStage_German(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Kern", "Core"},
		{"Tief", "Deep"},
		{"Wach", "Awake"},
		{"Im Bett", "In Bed"},
	}
	for _, tc := range cases {
		got, known := NormalizeSleepStage(tc.input)
		if !known {
			t.Errorf("NormalizeSleepStage(%q): expected known=true", tc.input)
		}
		if got != tc.want {
			t.Errorf("NormalizeSleepStage(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestNormalizeSleepStage_CaseInsensitive verifies that lookup is
// case-insensitive, since source data may arrive in any casing.
func TestNormalizeSleepStage_CaseInsensitive(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"deep", "Deep"},
		{"DEEP", "Deep"},
		{"kern", "Core"},
		{"KERN", "Core"},
		{"rem", "REM"},
		{"in bed", "In Bed"},
		{"  Deep  ", "Deep"},
	}
	for _, tc := range cases {
		got, known := NormalizeSleepStage(tc.input)
		if !known {
			t.Errorf("NormalizeSleepStage(%q): expected known=true", tc.input)
		}
		if got != tc.want {
			t.Errorf("NormalizeSleepStage(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestNormalizeSleepStage_Unknown verifies that unrecognized stage names are
// returned as-is with known=false, so callers can log a warning.
func TestNormalizeSleepStage_Unknown(t *testing.T) {
	got, known := NormalizeSleepStage("SomeWeirdStage")
	if known {
		t.Error("expected known=false for unknown stage")
	}
	if got != "SomeWeirdStage" {
		t.Errorf("expected original string returned, got %q", got)
	}
}
