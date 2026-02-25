package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

// HTTPClient implements DataSource by calling the FreeReps REST API.
// Used for remote MCP mode where the binary runs locally (stdio) but
// data lives on the remote server (accessed over Tailscale).
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// Compile-time check: HTTPClient satisfies DataSource.
var _ DataSource = (*HTTPClient)(nil)

// NewHTTPClient creates an HTTPClient targeting the given base URL.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// bucketToAgg maps MCP bucket values to REST API agg parameter values.
func bucketToAgg(bucket string) string {
	switch bucket {
	case "1 hour":
		return "hourly"
	case "1 day":
		return "daily"
	case "1 week":
		return "weekly"
	case "1 month":
		return "monthly"
	default:
		return "daily"
	}
}

func (c *HTTPClient) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("httpclient: create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpclient: %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("httpclient: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("httpclient: %s returned %d: %s", path, resp.StatusCode, body)
	}

	return body, nil
}

func timeParams(start, end time.Time) url.Values {
	v := url.Values{}
	v.Set("start", start.Format(time.RFC3339))
	v.Set("end", end.Format(time.RFC3339))
	return v
}

func (c *HTTPClient) GetTimeSeries(ctx context.Context, metricName string, start, end time.Time, bucketSize string, _ int) ([]storage.TimeSeriesPoint, error) {
	params := timeParams(start, end)
	params.Set("metric", metricName)
	params.Set("agg", bucketToAgg(bucketSize))

	body, err := c.get(ctx, "/api/v1/timeseries", params)
	if err != nil {
		return nil, err
	}

	var points []storage.TimeSeriesPoint
	if err := json.Unmarshal(body, &points); err != nil {
		return nil, fmt.Errorf("httpclient: decode timeseries: %w", err)
	}
	return points, nil
}

func (c *HTTPClient) GetMetricStats(ctx context.Context, metricName string, start, end time.Time, _ int) (*storage.MetricStats, error) {
	params := timeParams(start, end)
	params.Set("metric", metricName)

	body, err := c.get(ctx, "/api/v1/metrics/stats", params)
	if err != nil {
		return nil, err
	}

	var stats storage.MetricStats
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("httpclient: decode metric stats: %w", err)
	}
	return &stats, nil
}

func (c *HTTPClient) GetCorrelation(ctx context.Context, xMetric, yMetric string, start, end time.Time, bucket string, _ int) (*storage.CorrelationResult, error) {
	params := timeParams(start, end)
	params.Set("x", xMetric)
	params.Set("y", yMetric)
	params.Set("bucket", bucket)

	body, err := c.get(ctx, "/api/v1/correlation", params)
	if err != nil {
		return nil, err
	}

	var result storage.CorrelationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("httpclient: decode correlation: %w", err)
	}
	return &result, nil
}

func (c *HTTPClient) QuerySleepSessions(ctx context.Context, start, end time.Time, _ int) ([]storage.SleepSessionResult, error) {
	params := timeParams(start, end)

	body, err := c.get(ctx, "/api/v1/sleep", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Sessions []storage.SleepSessionResult `json:"sessions"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("httpclient: decode sleep sessions: %w", err)
	}
	return resp.Sessions, nil
}

func (c *HTTPClient) QuerySleepStages(ctx context.Context, start, end time.Time, _ int) ([]models.SleepStageRow, error) {
	params := timeParams(start, end)

	body, err := c.get(ctx, "/api/v1/sleep", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Stages []models.SleepStageRow `json:"stages"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("httpclient: decode sleep stages: %w", err)
	}
	return resp.Stages, nil
}

func (c *HTTPClient) GetSleepSummary(ctx context.Context, start, end time.Time, bucket string, _ int) ([]storage.SleepSummaryPeriod, error) {
	params := timeParams(start, end)
	params.Set("bucket", bucket)

	body, err := c.get(ctx, "/api/v1/sleep/summary", params)
	if err != nil {
		return nil, err
	}

	var periods []storage.SleepSummaryPeriod
	if err := json.Unmarshal(body, &periods); err != nil {
		return nil, fmt.Errorf("httpclient: decode sleep summary: %w", err)
	}
	return periods, nil
}

func (c *HTTPClient) QueryWorkouts(ctx context.Context, start, end time.Time, _ int, nameFilter string) ([]models.WorkoutRow, error) {
	params := timeParams(start, end)
	if nameFilter != "" {
		params.Set("type", nameFilter)
	}

	body, err := c.get(ctx, "/api/v1/workouts", params)
	if err != nil {
		return nil, err
	}

	var workouts []models.WorkoutRow
	if err := json.Unmarshal(body, &workouts); err != nil {
		return nil, fmt.Errorf("httpclient: decode workouts: %w", err)
	}
	return workouts, nil
}

func (c *HTTPClient) QueryWorkoutSets(ctx context.Context, start, end time.Time, _ int, exerciseFilter string) ([]models.WorkoutSetRow, error) {
	params := timeParams(start, end)
	if exerciseFilter != "" {
		params.Set("exercise", exerciseFilter)
	}

	body, err := c.get(ctx, "/api/v1/workouts/sets", params)
	if err != nil {
		return nil, err
	}

	var sets []models.WorkoutSetRow
	if err := json.Unmarshal(body, &sets); err != nil {
		return nil, fmt.Errorf("httpclient: decode workout sets: %w", err)
	}
	return sets, nil
}

func (c *HTTPClient) GetTrainingSummary(ctx context.Context, start, end time.Time, bucket string, _ int) ([]storage.TrainingSummaryPeriod, error) {
	params := timeParams(start, end)
	params.Set("bucket", bucket)

	body, err := c.get(ctx, "/api/v1/training/summary", params)
	if err != nil {
		return nil, err
	}

	var periods []storage.TrainingSummaryPeriod
	if err := json.Unmarshal(body, &periods); err != nil {
		return nil, fmt.Errorf("httpclient: decode training summary: %w", err)
	}
	return periods, nil
}

func (c *HTTPClient) GetTrainingIntensity(ctx context.Context, start, end time.Time, _ int, exerciseFilter string) (*storage.TrainingIntensityResult, error) {
	params := timeParams(start, end)
	if exerciseFilter != "" {
		params.Set("exercise", exerciseFilter)
	}

	body, err := c.get(ctx, "/api/v1/training/intensity", params)
	if err != nil {
		return nil, err
	}

	var result storage.TrainingIntensityResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("httpclient: decode training intensity: %w", err)
	}
	return &result, nil
}

func (c *HTTPClient) GetLatestMetrics(ctx context.Context, _ int) ([]models.HealthMetricRow, error) {
	body, err := c.get(ctx, "/api/v1/metrics/latest", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Latest []models.HealthMetricRow `json:"latest"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("httpclient: decode latest metrics: %w", err)
	}
	return resp.Latest, nil
}

func (c *HTTPClient) GetDailySums(ctx context.Context, _ int, _ []string) ([]storage.DailySum, error) {
	body, err := c.get(ctx, "/api/v1/metrics/latest", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		DailySums []storage.DailySum `json:"daily_sums"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("httpclient: decode daily sums: %w", err)
	}
	return resp.DailySums, nil
}

func (c *HTTPClient) GetAllowedMetrics(ctx context.Context) ([]storage.AllowedMetric, error) {
	body, err := c.get(ctx, "/api/v1/allowlist", nil)
	if err != nil {
		return nil, err
	}

	var metrics []storage.AllowedMetric
	if err := json.Unmarshal(body, &metrics); err != nil {
		return nil, fmt.Errorf("httpclient: decode allowlist: %w", err)
	}
	return metrics, nil
}
