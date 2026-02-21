package upload

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"
)

// HAEClient connects to the Health Auto Export TCP server (JSON-RPC 2.0).
// Each method call opens a new TCP connection — the HAE server closes the
// socket after sending the response.
type HAEClient struct {
	host    string
	port    int
	timeout time.Duration
}

// jsonRPCRequest is a JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// callToolParams wraps the tool name and arguments for the callTool method.
type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// jsonRPCError is the error object in a JSON-RPC 2.0 response.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HAE date format: yyyy-MM-dd HH:mm:ss Z
const haeDateFormat = "2006-01-02 15:04:05 -0700"

// NewHAEClient creates a new client for the HAE TCP server.
func NewHAEClient(host string, port int) *HAEClient {
	return &HAEClient{
		host:    host,
		port:    port,
		timeout: 120 * time.Second,
	}
}

// QueryMetrics queries health_metrics for a time range.
// metrics is a comma-separated filter (empty string = all metrics).
func (c *HAEClient) QueryMetrics(start, end time.Time, metrics string, aggregate bool) (json.RawMessage, error) {
	args := map[string]any{
		"start":     start.Format(haeDateFormat),
		"end":       end.Format(haeDateFormat),
		"aggregate": aggregate,
	}
	if metrics != "" {
		args["metrics"] = metrics
	}
	return c.callTool("health_metrics", args)
}

// QueryWorkouts queries workouts for a time range with metadata and routes.
func (c *HAEClient) QueryWorkouts(start, end time.Time) (json.RawMessage, error) {
	args := map[string]any{
		"start":               start.Format(haeDateFormat),
		"end":                 end.Format(haeDateFormat),
		"includeMetadata":     true,
		"includeRoutes":       true,
		"metadataAggregation": "minutes",
	}
	return c.callTool("workouts", args)
}

// callTool sends a JSON-RPC callTool request and returns the result.
func (c *HAEClient) callTool(toolName string, args map[string]any) (json.RawMessage, error) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "callTool",
		Params: callToolParams{
			Name:      toolName,
			Arguments: args,
		},
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	addr := net.JoinHostPort(c.host, fmt.Sprintf("%d", c.port))
	conn, err := net.DialTimeout("tcp", addr, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}
	defer conn.Close() //nolint:errcheck

	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("setting deadline: %w", err)
	}

	// HAE server uses newline-delimited JSON-RPC framing.
	reqData = append(reqData, '\n')

	if _, err := conn.Write(reqData); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	// HAE server closes the connection after sending the response, so read until EOF.
	respData, err := io.ReadAll(conn)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if len(respData) == 0 {
		return nil, fmt.Errorf("empty response from %s", addr)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("HAE error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

const maxRetries = 3

// waitForServer polls the HAE server until it accepts connections or retries are exhausted.
func (c *HAEClient) waitForServer(log *slog.Logger) bool {
	addr := net.JoinHostPort(c.host, fmt.Sprintf("%d", c.port))
	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close() //nolint:errcheck
			return true
		}
		log.Info("waiting for HAE server to come back...", "attempt", i+1)
		time.Sleep(3 * time.Second)
	}
	return false
}

// QueryMetricsWithRetry wraps QueryMetrics with retry logic for server crashes.
func (c *HAEClient) QueryMetricsWithRetry(start, end time.Time, metrics string, aggregate bool, log *slog.Logger) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Info("retrying metric query", "metric", metrics, "attempt", attempt+1)
			if !c.waitForServer(log) {
				return nil, fmt.Errorf("server did not recover after crash")
			}
		}
		result, err := c.QueryMetrics(start, end, metrics, aggregate)
		if err == nil {
			return result, nil
		}
		lastErr = err
		log.Warn("query failed, will retry", "error", err)
	}
	return nil, lastErr
}

// QueryWorkoutsWithRetry wraps QueryWorkouts with retry logic for server crashes.
func (c *HAEClient) QueryWorkoutsWithRetry(start, end time.Time, log *slog.Logger) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Info("retrying workout query", "attempt", attempt+1)
			if !c.waitForServer(log) {
				return nil, fmt.Errorf("server did not recover after crash")
			}
		}
		result, err := c.QueryWorkouts(start, end)
		if err == nil {
			return result, nil
		}
		lastErr = err
		log.Warn("query failed, will retry", "error", err)
	}
	return nil, lastErr
}
