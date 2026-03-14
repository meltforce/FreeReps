package ingest

import (
	"bytes"
	"regexp"
)

// Format identifies the type of an imported file.
type Format string

const (
	FormatAlpha   Format = "alpha_progression_csv"
	FormatUnknown Format = "unknown"
)

// detector is a function that inspects the first portion of a file and returns
// a Format if it recognises the content, or FormatUnknown otherwise.
type detector func(head []byte) Format

// detectors is an ordered list of format detectors. The first match wins.
var detectors = []detector{
	detectAlpha,
}

// utf8BOM is the byte order mark that some editors prepend to UTF-8 files.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// DetectFormat inspects the first ~2KB of data and returns the detected format.
func DetectFormat(data []byte) Format {
	head := data
	// Strip UTF-8 BOM if present
	head = bytes.TrimPrefix(head, utf8BOM)
	if len(head) > 2048 {
		head = head[:2048]
	}
	for _, d := range detectors {
		if f := d(head); f != FormatUnknown {
			return f
		}
	}
	return FormatUnknown
}

var (
	// Alpha Progression session header: "Name";"2026-02-19 4:54 h";"1:02 hr"
	// Delimiter may be semicolon or tab.
	alphaSessionRe = regexp.MustCompile(`"[^"]+"\s*[;\t]\s*"\d{4}-\d{2}-\d{2}\s+\d+:\d+\s*h"\s*[;\t]\s*"[^"]+"`)
	// Alpha Progression column header (semicolon or tab delimited)
	alphaColumnRe = regexp.MustCompile(`#[;\t]KG[;\t]REPS[;\t]RIR`)
	// Alpha Progression exercise header: "1. Exercise Name · Equipment · 8 reps"
	alphaExerciseRe = regexp.MustCompile(`"\d+\.\s+.+\s+·\s+\d+\s+reps`)
)

func detectAlpha(head []byte) Format {
	if alphaSessionRe.Match(head) {
		return FormatAlpha
	}
	upper := bytes.ToUpper(head)
	if alphaColumnRe.Match(upper) {
		return FormatAlpha
	}
	// Fall back to exercise header pattern (catches files where session header
	// is outside the first 2KB or uses an unexpected date format)
	if alphaExerciseRe.Match(head) {
		return FormatAlpha
	}
	return FormatUnknown
}
