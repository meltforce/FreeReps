package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/claude/freereps/internal/storage"
)

// newTestServer creates an httptest server that routes requests to handler functions
// keyed by path. Verifies the HTTP client sends correct paths and query params.
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, ok := handlers[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatal(err)
	}
}

// TestGetTimeSeries verifies the HTTP client sends the right query params
// and correctly parses the JSON array response.
func TestGetTimeSeries(t *testing.T) {
	ts := newTestServer(t, map[string]http.HandlerFunc{
		"/api/v1/timeseries": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("metric"); got != "heart_rate" {
				t.Errorf("metric=%q, want heart_rate", got)
			}
			if got := r.URL.Query().Get("agg"); got != "daily" {
				t.Errorf("agg=%q, want daily", got)
			}

			avg := 72.0
			writeTestJSON(t, w, []storage.TimeSeriesPoint{
				{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Avg: &avg, Count: 10},
			})
		},
	})
	defer ts.Close()

	client := NewHTTPClient(ts.URL)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)

	points, err := client.GetTimeSeries(context.Background(), "heart_rate", start, end, "1 day", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("got %d points, want 1", len(points))
	}
	if points[0].Count != 10 {
		t.Errorf("count=%d, want 10", points[0].Count)
	}
}

// TestGetMetricStats verifies the HTTP client correctly parses a single struct response.
func TestGetMetricStats(t *testing.T) {
	ts := newTestServer(t, map[string]http.HandlerFunc{
		"/api/v1/metrics/stats": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("metric"); got != "resting_heart_rate" {
				t.Errorf("metric=%q, want resting_heart_rate", got)
			}

			avg := 55.0
			writeTestJSON(t, w, storage.MetricStats{
				Metric: "resting_heart_rate",
				Avg:    &avg,
				Count:  30,
			})
		},
	})
	defer ts.Close()

	client := NewHTTPClient(ts.URL)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	stats, err := client.GetMetricStats(context.Background(), "resting_heart_rate", start, end, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Count != 30 {
		t.Errorf("count=%d, want 30", stats.Count)
	}
}

// TestGetCorrelation verifies correlation endpoint parsing.
func TestGetCorrelation(t *testing.T) {
	ts := newTestServer(t, map[string]http.HandlerFunc{
		"/api/v1/correlation": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("x"); got != "sleep_duration" {
				t.Errorf("x=%q, want sleep_duration", got)
			}
			if got := r.URL.Query().Get("y"); got != "heart_rate_variability" {
				t.Errorf("y=%q, want heart_rate_variability", got)
			}
			if got := r.URL.Query().Get("bucket"); got != "1 day" {
				t.Errorf("bucket=%q, want '1 day'", got)
			}

			r2 := 0.85
			writeTestJSON(t, w, storage.CorrelationResult{
				PearsonR: &r2,
				Count:    20,
			})
		},
	})
	defer ts.Close()

	client := NewHTTPClient(ts.URL)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	result, err := client.GetCorrelation(context.Background(), "sleep_duration", "heart_rate_variability", start, end, "1 day", 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 20 {
		t.Errorf("count=%d, want 20", result.Count)
	}
	if *result.PearsonR != 0.85 {
		t.Errorf("pearson_r=%f, want 0.85", *result.PearsonR)
	}
}

// TestGetAllowedMetrics verifies the allowlist endpoint returns a flat array.
func TestGetAllowedMetrics(t *testing.T) {
	ts := newTestServer(t, map[string]http.HandlerFunc{
		"/api/v1/allowlist": func(w http.ResponseWriter, r *http.Request) {
			writeTestJSON(t, w, []storage.AllowedMetric{
				{MetricName: "heart_rate", Category: "vitals", Enabled: true},
			})
		},
	})
	defer ts.Close()

	client := NewHTTPClient(ts.URL)
	metrics, err := client.GetAllowedMetrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 {
		t.Fatalf("got %d metrics, want 1", len(metrics))
	}
	if metrics[0].MetricName != "heart_rate" {
		t.Errorf("metric_name=%q, want heart_rate", metrics[0].MetricName)
	}
}

// TestBucketToAgg verifies the bucket-to-agg mapping used for timeseries requests.
func TestBucketToAgg(t *testing.T) {
	cases := []struct {
		bucket string
		want   string
	}{
		{"1 hour", "hourly"},
		{"1 day", "daily"},
		{"1 week", "weekly"},
		{"1 month", "monthly"},
	}
	for _, tc := range cases {
		if got := bucketToAgg(tc.bucket); got != tc.want {
			t.Errorf("bucketToAgg(%q) = %q, want %q", tc.bucket, got, tc.want)
		}
	}
}

// TestHTTPClientServerError verifies the client returns an error on non-200 responses.
func TestHTTPClientServerError(t *testing.T) {
	ts := newTestServer(t, map[string]http.HandlerFunc{
		"/api/v1/allowlist": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"database down"}`))
		},
	})
	defer ts.Close()

	client := NewHTTPClient(ts.URL)
	_, err := client.GetAllowedMetrics(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// TestGetSleepSummary verifies the new sleep/summary endpoint.
func TestGetSleepSummary(t *testing.T) {
	ts := newTestServer(t, map[string]http.HandlerFunc{
		"/api/v1/sleep/summary": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("bucket"); got != "1 week" {
				t.Errorf("bucket=%q, want '1 week'", got)
			}
			writeTestJSON(t, w, []storage.SleepSummaryPeriod{
				{Period: "2026-W01", Nights: 7, AvgTotalSleepHr: 7.5},
			})
		},
	})
	defer ts.Close()

	client := NewHTTPClient(ts.URL)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)

	periods, err := client.GetSleepSummary(context.Background(), start, end, "1 week", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != 1 {
		t.Fatalf("got %d periods, want 1", len(periods))
	}
	if periods[0].Nights != 7 {
		t.Errorf("nights=%d, want 7", periods[0].Nights)
	}
}

// TestGetTrainingSummary verifies the new training/summary endpoint.
func TestGetTrainingSummary(t *testing.T) {
	ts := newTestServer(t, map[string]http.HandlerFunc{
		"/api/v1/training/summary": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("bucket"); got != "1 month" {
				t.Errorf("bucket=%q, want '1 month'", got)
			}
			writeTestJSON(t, w, []storage.TrainingSummaryPeriod{
				{Period: "2026-01"},
			})
		},
	})
	defer ts.Close()

	client := NewHTTPClient(ts.URL)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	periods, err := client.GetTrainingSummary(context.Background(), start, end, "1 month", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != 1 {
		t.Fatalf("got %d periods, want 1", len(periods))
	}
}

// TestGetTrainingIntensity verifies the new training/intensity endpoint.
func TestGetTrainingIntensity(t *testing.T) {
	ts := newTestServer(t, map[string]http.HandlerFunc{
		"/api/v1/training/intensity": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("exercise"); got != "bench press" {
				t.Errorf("exercise=%q, want 'bench press'", got)
			}
			writeTestJSON(t, w, storage.TrainingIntensityResult{
				TotalSets:   100,
				TrackedSets: 80,
			})
		},
	})
	defer ts.Close()

	client := NewHTTPClient(ts.URL)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	result, err := client.GetTrainingIntensity(context.Background(), start, end, 1, "bench press")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalSets != 100 {
		t.Errorf("total_sets=%d, want 100", result.TotalSets)
	}
}
