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

// DetectFormat inspects the first ~2KB of data and returns the detected format.
func DetectFormat(data []byte) Format {
	head := data
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
	alphaSessionRe = regexp.MustCompile(`"[^"]+";"\d{4}-\d{2}-\d{2}\s+\d+:\d+\s+h";"[^"]+"`)
	// Alpha Progression column header
	alphaColumnRe = regexp.MustCompile(`#;KG;REPS;RIR`)
)

func detectAlpha(head []byte) Format {
	if alphaSessionRe.Match(head) || alphaColumnRe.Match(bytes.ToUpper(head)) {
		return FormatAlpha
	}
	return FormatUnknown
}
