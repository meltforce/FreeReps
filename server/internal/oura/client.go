package oura

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://api.ouraring.com"

// Client wraps the Oura API v2.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates an Oura API client with sensible defaults.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
	}
}

// newTestClient creates a client pointing at a test server.
func newTestClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    baseURL,
	}
}

// get performs an authenticated GET and returns the raw response body.
func (c *Client) get(ctx context.Context, path, token string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oura API %s returned %d: %s", path, resp.StatusCode, string(body))
	}
	return body, nil
}

// fetchAll fetches all pages for a date-range endpoint using start_date/end_date.
func fetchAll[T any](c *Client, ctx context.Context, path, token, startDate, endDate string) ([]T, error) {
	var all []T
	params := url.Values{
		"start_date": {startDate},
		"end_date":   {endDate},
	}

	for {
		body, err := c.get(ctx, path, token, params)
		if err != nil {
			return nil, err
		}
		var resp Response[T]
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("decoding %s response: %w", path, err)
		}
		all = append(all, resp.Data...)
		if resp.NextToken == nil {
			break
		}
		params.Set("next_token", *resp.NextToken)
	}
	return all, nil
}

// fetchAllDatetime fetches all pages using start_datetime/end_datetime (for heartrate).
func fetchAllDatetime[T any](c *Client, ctx context.Context, path, token, startDatetime, endDatetime string) ([]T, error) {
	var all []T
	params := url.Values{
		"start_datetime": {startDatetime},
		"end_datetime":   {endDatetime},
	}

	for {
		body, err := c.get(ctx, path, token, params)
		if err != nil {
			return nil, err
		}
		var resp Response[T]
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("decoding %s response: %w", path, err)
		}
		all = append(all, resp.Data...)
		if resp.NextToken == nil {
			break
		}
		params.Set("next_token", *resp.NextToken)
	}
	return all, nil
}

func (c *Client) GetDailyReadiness(ctx context.Context, token, startDate, endDate string) ([]DailyReadinessItem, error) {
	return fetchAll[DailyReadinessItem](c, ctx, "/v2/usercollection/daily_readiness", token, startDate, endDate)
}

func (c *Client) GetDailySleep(ctx context.Context, token, startDate, endDate string) ([]DailySleepItem, error) {
	return fetchAll[DailySleepItem](c, ctx, "/v2/usercollection/daily_sleep", token, startDate, endDate)
}

func (c *Client) GetDailyActivity(ctx context.Context, token, startDate, endDate string) ([]DailyActivityItem, error) {
	return fetchAll[DailyActivityItem](c, ctx, "/v2/usercollection/daily_activity", token, startDate, endDate)
}

func (c *Client) GetSleep(ctx context.Context, token, startDate, endDate string) ([]SleepItem, error) {
	return fetchAll[SleepItem](c, ctx, "/v2/usercollection/sleep", token, startDate, endDate)
}

func (c *Client) GetHeartRate(ctx context.Context, token, startDatetime, endDatetime string) ([]HeartRateItem, error) {
	return fetchAllDatetime[HeartRateItem](c, ctx, "/v2/usercollection/heartrate", token, startDatetime, endDatetime)
}

func (c *Client) GetDailySpO2(ctx context.Context, token, startDate, endDate string) ([]DailySpO2Item, error) {
	return fetchAll[DailySpO2Item](c, ctx, "/v2/usercollection/daily_spo2", token, startDate, endDate)
}

func (c *Client) GetDailyStress(ctx context.Context, token, startDate, endDate string) ([]DailyStressItem, error) {
	return fetchAll[DailyStressItem](c, ctx, "/v2/usercollection/daily_stress", token, startDate, endDate)
}

func (c *Client) GetDailyResilience(ctx context.Context, token, startDate, endDate string) ([]DailyResilienceItem, error) {
	return fetchAll[DailyResilienceItem](c, ctx, "/v2/usercollection/daily_resilience", token, startDate, endDate)
}

func (c *Client) GetDailyCardiovascularAge(ctx context.Context, token, startDate, endDate string) ([]DailyCardiovascularAgeItem, error) {
	return fetchAll[DailyCardiovascularAgeItem](c, ctx, "/v2/usercollection/daily_cardiovascular_age", token, startDate, endDate)
}

func (c *Client) GetVO2Max(ctx context.Context, token, startDate, endDate string) ([]VO2MaxItem, error) {
	return fetchAll[VO2MaxItem](c, ctx, "/v2/usercollection/vo2_max", token, startDate, endDate)
}

func (c *Client) GetWorkouts(ctx context.Context, token, startDate, endDate string) ([]WorkoutItem, error) {
	return fetchAll[WorkoutItem](c, ctx, "/v2/usercollection/workout", token, startDate, endDate)
}
