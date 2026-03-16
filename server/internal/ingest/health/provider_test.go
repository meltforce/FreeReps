package health

import "testing"

// TestNormalizeWorkoutName verifies that German and location-prefixed workout names
// from Health Auto Export are mapped to their English base names, while unknown names
// pass through unchanged.
func TestNormalizeWorkoutName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Traditionelles Krafttraining", "Traditional Strength Training"},
		{"Outdoor Radfahren", "Cycling"},
		{"Innenräume Radfahren", "Cycling"},
		{"Indoor Cycling", "Cycling"},
		{"Outdoor Walk", "Walking"},
		{"Schwimmbad Schwimmen", "Swimming"},
		{"Freiwasser Schwimmen", "Swimming"},
		{"Wandern", "Hiking"},
		// Already-English names pass through unchanged
		{"Cycling", "Cycling"},
		{"Running", "Running"},
		{"Some Future Workout", "Some Future Workout"},
	}

	for _, tt := range tests {
		got := normalizeWorkoutName(tt.input)
		if got != tt.want {
			t.Errorf("normalizeWorkoutName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
