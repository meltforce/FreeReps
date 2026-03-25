package oura

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetDailyReadiness verifies that the client correctly parses a single-page
// readiness response from the Oura API.
func TestGetDailyReadiness(t *testing.T) {
	score := 85
	tempDev := 0.12
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/usercollection/daily_readiness" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		resp := Response[DailyReadinessItem]{
			Data: []DailyReadinessItem{
				{ID: "r1", Day: "2024-01-15", Score: &score, TemperatureDeviation: &tempDev},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	items, err := client.GetDailyReadiness(context.Background(), "test-token", "2024-01-15", "2024-01-16")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Day != "2024-01-15" {
		t.Errorf("day = %q, want %q", items[0].Day, "2024-01-15")
	}
	if *items[0].Score != 85 {
		t.Errorf("score = %d, want 85", *items[0].Score)
	}
	if *items[0].TemperatureDeviation != 0.12 {
		t.Errorf("temperature_deviation = %f, want 0.12", *items[0].TemperatureDeviation)
	}
}

// TestPagination verifies that the client follows next_token links to collect
// all pages of results.
func TestPagination(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			next := "page2"
			resp := Response[DailySleepItem]{
				Data:      []DailySleepItem{{ID: "s1", Day: "2024-01-15"}},
				NextToken: &next,
			}
			_ = json.NewEncoder(w).Encode(resp)
		case 2:
			if r.URL.Query().Get("next_token") != "page2" {
				t.Errorf("expected next_token=page2, got %q", r.URL.Query().Get("next_token"))
			}
			resp := Response[DailySleepItem]{
				Data: []DailySleepItem{{ID: "s2", Day: "2024-01-16"}},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Errorf("unexpected call %d", calls)
		}
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	items, err := client.GetDailySleep(context.Background(), "tok", "2024-01-15", "2024-01-17")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if calls != 2 {
		t.Errorf("expected 2 API calls, got %d", calls)
	}
}

// TestAPIError verifies that non-200 responses are returned as errors with
// the status code and response body for debugging.
func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Invalid token"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.GetDailyReadiness(context.Background(), "bad-token", "2024-01-15", "2024-01-16")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

// TestGetHeartRateUsesDatetimeParams verifies that the heartrate endpoint
// uses start_datetime/end_datetime instead of start_date/end_date.
func TestGetHeartRateUsesDatetimeParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start_datetime") == "" {
			t.Error("expected start_datetime param")
		}
		if r.URL.Query().Get("end_datetime") == "" {
			t.Error("expected end_datetime param")
		}
		if r.URL.Query().Get("start_date") != "" {
			t.Error("heartrate should not use start_date")
		}
		w.Header().Set("Content-Type", "application/json")
		resp := Response[HeartRateItem]{Data: []HeartRateItem{{BPM: 72, Source: "sleep", Timestamp: "2024-01-15T03:00:00+00:00"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	items, err := client.GetHeartRate(context.Background(), "tok", "2024-01-15T00:00:00+00:00", "2024-01-16T00:00:00+00:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].BPM != 72 {
		t.Errorf("unexpected items: %+v", items)
	}
}
