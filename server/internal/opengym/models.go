// Package opengym ingests strength sessions from an openGym instance.
//
// openGym serves a profile as one JSON document, the state, through
// GET /api/data. The types here cover the part of that document FreeReps reads;
// every other key is ignored. Wire format: server/specs/opengym-api.md.
package opengym

import "encoding/json"

// DataResponse is the body of GET /api/data.
type DataResponse struct {
	State State `json:"state"`
	Rev   int64 `json:"rev"`
}

// State is the subset of an openGym profile FreeReps reads.
//
// The per-exercise unit override `exUnit` is deliberately absent: it changes
// only how the app shows and takes input for an exercise, and every weight in
// the state stays in Unit (upstream frontend/src/lib/ex-unit.js, header
// comment, checked at commit 8fb98b16).
type State struct {
	Rev int64  `json:"_rev"`
	WID string `json:"_wid"`
	// Unit is the profile's weight unit. The app reads "lb" as pounds and any
	// other value, including an absent one, as kilograms (ex-unit.js
	// profileUnit); the mapper does the same.
	Unit     string           `json:"unit"`
	Workouts []Workout        `json:"workouts"`
	CustomEx []CustomExercise `json:"customEx"`
}

// CustomExercise is an exercise the user created in openGym. Workouts reference
// it by ID like a catalogue exercise; only here does it carry a name.
type CustomExercise struct {
	ID        string `json:"id"`
	Name      string `json:"n"`
	Equipment string `json:"eq"`
}

// Workout is one logged session.
type Workout struct {
	ID string `json:"id"`
	// Day is the local date as the phone saw it, YYYY-MM-DD.
	Day string `json:"d"`
	// Start and End are epoch milliseconds. Either can be absent on a workout
	// logged on paper; End is absent on an unfinished one.
	Start     *int64  `json:"start"`
	End       *int64  `json:"end"`
	RoutineID *string `json:"routineId"`
	Name      string  `json:"name"`
	Entries   []Entry `json:"entries"`
}

// Entry is one exercise inside a workout. ID is a catalogue id ("0025") or the
// id of a CustomExercise; the workout carries no exercise name.
type Entry struct {
	ID   string `json:"id"`
	Sets []Set  `json:"sets"`
}

// Set is one planned or performed set.
//
// The fields after Failure belong to set shapes that have not been observed in
// a captured state (timed, cardio, per-side and rest-pause sets). They are
// decoded as raw values only so the mapper can tell they are present and refuse
// the workout instead of storing it with fields it does not understand.
type Set struct {
	Weight  *float64 `json:"w"`
	Reps    *int     `json:"r"`
	Done    bool     `json:"done"`
	Phase   string   `json:"phase"`
	RPE     *float64 `json:"rpe"`
	RIR     *float64 `json:"rir"`
	Failure bool     `json:"failure"`

	Type     string          `json:"type"`
	Sec      json.RawMessage `json:"sec"`
	Min      json.RawMessage `json:"min"`
	Speed    json.RawMessage `json:"speed"`
	Incline  json.RawMessage `json:"incline"`
	Sides    json.RawMessage `json:"sides"`
	Clusters json.RawMessage `json:"clusters"`
}
