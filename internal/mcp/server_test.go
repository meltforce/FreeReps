package mcp

import (
	"context"
	"testing"
)

// TestUserIDFromContextDefault verifies the default user ID (1) when no value
// is set in the context.
func TestUserIDFromContextDefault(t *testing.T) {
	ctx := context.Background()
	if id := UserIDFromContext(ctx); id != 1 {
		t.Errorf("UserIDFromContext(empty) = %d, want 1", id)
	}
}

// TestUserIDFromContextSet verifies the user ID is extracted from context
// after being set by WithUserID.
func TestUserIDFromContextSet(t *testing.T) {
	ctx := WithUserID(context.Background(), 42)
	if id := UserIDFromContext(ctx); id != 42 {
		t.Errorf("UserIDFromContext = %d, want 42", id)
	}
}

// TestDefaultTimeRange verifies time range defaults (last 7 days) and parsing.
func TestDefaultTimeRange(t *testing.T) {
	// Both empty → defaults to last 7 days
	start, end, err := defaultTimeRange("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	diff := end.Sub(start)
	if diff.Hours() < 167 || diff.Hours() > 169 { // ~168 hours = 7 days
		t.Errorf("default range = %.0f hours, want ~168", diff.Hours())
	}

	// Explicit dates
	start, end, err = defaultTimeRange("2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start.Year() != 2024 || start.Month() != 1 || start.Day() != 1 {
		t.Errorf("start = %v, want 2024-01-01", start)
	}
	if end.Year() != 2024 || end.Month() != 1 || end.Day() != 31 {
		t.Errorf("end = %v, want 2024-01-31", end)
	}

	// RFC3339
	start, _, err = defaultTimeRange("2024-06-15T10:30:00Z", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start.Hour() != 10 || start.Minute() != 30 {
		t.Errorf("start = %v, want 10:30", start)
	}

	// Invalid
	_, _, err = defaultTimeRange("not-a-date", "")
	if err == nil {
		t.Error("expected error for invalid date")
	}
}
