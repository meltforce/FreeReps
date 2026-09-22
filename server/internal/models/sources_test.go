package models

import "testing"

func TestSourceLabel(t *testing.T) {
	if got := SourceLabel(""); got != "Apple Health" {
		t.Errorf("empty source = %q, want %q", got, "Apple Health")
	}
	if got := SourceLabel("Oura"); got != "Oura" {
		t.Errorf("named source = %q, want %q", got, "Oura")
	}
}

func TestRecordedBy(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		sources []string
		want    string
	}{
		{
			name:    "a named source speaks for itself",
			source:  "Oura",
			sources: []string{"Oura"},
			want:    "Oura",
		},
		{
			name:    "a HealthKit row alone is Apple Health",
			source:  "",
			sources: []string{""},
			want:    "Apple Health",
		},
		{
			name:    "a HealthKit row beside a named one names the recorder",
			source:  "",
			sources: []string{"", "Oura"},
			want:    "Oura",
		},
		{
			name:    "two named sources in one window are both reported",
			source:  "",
			sources: []string{"", "Hevy", "Oura"},
			want:    "Hevy / Oura",
		},
		{
			name:   "without a window the row's own source decides",
			source: "",
			want:   "Apple Health",
		},
		{
			name:    "a named row is not overridden by its window",
			source:  "Hevy",
			sources: []string{"", "Hevy", "Oura"},
			want:    "Hevy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RecordedBy(tt.source, tt.sources); got != tt.want {
				t.Errorf("RecordedBy(%q, %v) = %q, want %q", tt.source, tt.sources, got, tt.want)
			}
		})
	}
}
