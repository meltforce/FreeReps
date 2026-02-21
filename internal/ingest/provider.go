package ingest

// Result holds the outcome of an ingest operation.
type Result struct {
	MetricsReceived int      `json:"metrics_received"`
	MetricsInserted int64    `json:"metrics_inserted"`
	MetricsSkipped  int64    `json:"metrics_skipped"`
	MetricsRejected int      `json:"metrics_rejected"`
	RejectedNames   []string `json:"rejected_names,omitempty"`

	SleepSessionsInserted int `json:"sleep_sessions_inserted,omitempty"`
	SleepStagesInserted   int64 `json:"sleep_stages_inserted,omitempty"`

	WorkoutsReceived int   `json:"workouts_received,omitempty"`
	WorkoutsInserted int   `json:"workouts_inserted,omitempty"`
	WorkoutHRPoints  int64 `json:"workout_hr_points,omitempty"`
	WorkoutRoutePoints int64 `json:"workout_route_points,omitempty"`

	SetsReceived int   `json:"sets_received"`
	SetsInserted int64 `json:"sets_inserted"`

	Message string `json:"message,omitempty"`
}
