package models

import "strings"

// SourceLabel names a source for display.
//
// An empty source is not an unknown one: Health Auto Export delivers HealthKit
// rows without setting the field, and the source-priority rules match it as the
// empty string. See DECISIONS.md, 2026-08-05 and 2026-09-22.
func SourceLabel(source string) string {
	if source == "" {
		return "Apple Health"
	}
	return source
}

// RecordedBy names the provider that recorded a workout, rather than the path it
// arrived on.
//
// A session recorded in the Oura app reaches FreeReps twice: over the Oura API,
// and as the copy the app writes into HealthKit, which Health Auto Export
// forwards without a source name. The row the source priority ranks first is
// the HealthKit copy, and calling that one "Apple Health" names the hub instead
// of the recorder. A named source among `sources` — every source of the
// workout's 5-minute window — is the recorder, because Apple Health is the only
// source that arrives without a name. See DECISIONS.md, 2026-09-21.
//
// The rule lives here rather than in the browser so that the MCP tools report
// the same name the workout list shows. It previously existed only in
// `web/src/utils/sourceLabel.ts`, where `get_workouts` could not reach it.
func RecordedBy(source string, sources []string) string {
	if source != "" {
		return SourceLabel(source)
	}
	named := make([]string, 0, len(sources))
	for _, s := range sources {
		if s != "" {
			named = append(named, s)
		}
	}
	if len(named) == 0 {
		return SourceLabel(source)
	}
	return strings.Join(named, " / ")
}
