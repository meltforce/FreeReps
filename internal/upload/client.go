package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/claude/freereps/internal/models"
)

// allowedMetric mirrors storage.AllowedMetric without importing the storage package
// (which would pull in pgx and other server-side dependencies).
type allowedMetric struct {
	MetricName string `json:"metric_name"`
	Enabled    bool   `json:"enabled"`
}

// Client sends data to the FreeReps server over HTTP.
type Client struct {
	serverURL  string
	httpClient *http.Client
}

// NewClient creates a new HTTP client for the FreeReps server.
func NewClient(serverURL string) *Client {
	return &Client{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// FetchAllowlist retrieves the enabled metric names from the server.
func (c *Client) FetchAllowlist() (map[string]bool, error) {
	resp, err := c.httpClient.Get(c.serverURL + "/api/v1/allowlist")
	if err != nil {
		return nil, fmt.Errorf("fetching allowlist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("allowlist request failed (status %d): %s", resp.StatusCode, body)
	}

	var metrics []allowedMetric
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return nil, fmt.Errorf("decoding allowlist: %w", err)
	}

	allowlist := make(map[string]bool, len(metrics))
	for _, m := range metrics {
		if m.Enabled {
			allowlist[m.MetricName] = true
		}
	}
	return allowlist, nil
}

// SendPayload POSTs an HAEPayload to the server's ingest endpoint.
// Retries up to 3 times with exponential backoff on failure.
func (c *Client) SendPayload(payload models.HAEPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}

		resp, err := c.httpClient.Post(
			c.serverURL+"/api/v1/ingest/",
			"application/json",
			bytes.NewReader(data),
		)
		if err != nil {
			lastErr = err
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}
		lastErr = fmt.Errorf("ingest failed (status %d): %s", resp.StatusCode, body)
	}

	return fmt.Errorf("after 3 attempts: %w", lastErr)
}
