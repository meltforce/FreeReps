package upload

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

// startMockTCPServer starts a TCP server that reads a request and sends back a
// fixed response, then closes the connection. Returns the listener port.
func startMockTCPServer(t *testing.T, response []byte) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	port := ln.Addr().(*net.TCPAddr).Port

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read the request (consume all available data)
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buf) //nolint:errcheck

		// Send the response
		conn.Write(response) //nolint:errcheck
	}()

	return port
}

// TestCallTool verifies that a successful JSON-RPC response returns the result.
func TestCallTool(t *testing.T) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  json.RawMessage(`{"data":{"metrics":[]}}`),
	}
	respBytes, _ := json.Marshal(resp)

	port := startMockTCPServer(t, respBytes)

	client := NewHAEClient("127.0.0.1", port)
	client.timeout = 5 * time.Second

	result, err := client.callTool("health_metrics", map[string]any{
		"start": "2025-01-01 00:00:00 +0000",
		"end":   "2025-01-31 00:00:00 +0000",
	})
	if err != nil {
		t.Fatalf("callTool returned error: %v", err)
	}

	if string(result) != `{"data":{"metrics":[]}}` {
		t.Errorf("unexpected result: %s", result)
	}
}

// TestCallToolError verifies that a JSON-RPC error response is surfaced.
func TestCallToolError(t *testing.T) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error:   &jsonRPCError{Code: -32600, Message: "Invalid request"},
	}
	respBytes, _ := json.Marshal(resp)

	port := startMockTCPServer(t, respBytes)

	client := NewHAEClient("127.0.0.1", port)
	client.timeout = 5 * time.Second

	_, err := client.callTool("health_metrics", map[string]any{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "HAE error -32600: Invalid request" {
		t.Errorf("unexpected error: %s", got)
	}
}

// TestQueryMetrics verifies the JSON-RPC request structure for health_metrics.
func TestQueryMetrics(t *testing.T) {
	// Server reads the request and echoes it back in the result for inspection.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	port := ln.Addr().(*net.TCPAddr).Port

	var receivedReq jsonRPCRequest
	done := make(chan struct{})

	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)

		json.Unmarshal(buf[:n], &receivedReq) //nolint:errcheck

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"data":{"metrics":[]}}`),
		}
		respBytes, _ := json.Marshal(resp)
		conn.Write(respBytes) //nolint:errcheck
	}()

	client := NewHAEClient("127.0.0.1", port)
	client.timeout = 5 * time.Second

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	_, err = client.QueryMetrics(start, end, "heart_rate,hrv")
	if err != nil {
		t.Fatalf("QueryMetrics returned error: %v", err)
	}

	<-done

	if receivedReq.Method != "callTool" {
		t.Errorf("expected method callTool, got %s", receivedReq.Method)
	}

	// Parse the params to verify tool name and arguments
	paramsBytes, _ := json.Marshal(receivedReq.Params)
	var params callToolParams
	json.Unmarshal(paramsBytes, &params) //nolint:errcheck

	if params.Name != "health_metrics" {
		t.Errorf("expected tool name health_metrics, got %s", params.Name)
	}

	if params.Arguments["metrics"] != "heart_rate,hrv" {
		t.Errorf("expected metrics filter heart_rate,hrv, got %v", params.Arguments["metrics"])
	}

	if params.Arguments["aggregate"] != false {
		t.Errorf("expected aggregate=false, got %v", params.Arguments["aggregate"])
	}
}

// TestQueryWorkouts verifies the JSON-RPC request structure for workouts.
func TestQueryWorkouts(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	port := ln.Addr().(*net.TCPAddr).Port

	var receivedReq jsonRPCRequest
	done := make(chan struct{})

	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(buf)

		json.Unmarshal(buf[:n], &receivedReq) //nolint:errcheck

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"data":{"workouts":[]}}`),
		}
		respBytes, _ := json.Marshal(resp)
		conn.Write(respBytes) //nolint:errcheck
	}()

	client := NewHAEClient("127.0.0.1", port)
	client.timeout = 5 * time.Second

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	_, err = client.QueryWorkouts(start, end)
	if err != nil {
		t.Fatalf("QueryWorkouts returned error: %v", err)
	}

	<-done

	paramsBytes, _ := json.Marshal(receivedReq.Params)
	var params callToolParams
	json.Unmarshal(paramsBytes, &params) //nolint:errcheck

	if params.Name != "workouts" {
		t.Errorf("expected tool name workouts, got %s", params.Name)
	}

	if params.Arguments["includeMetadata"] != true {
		t.Errorf("expected includeMetadata=true, got %v", params.Arguments["includeMetadata"])
	}

	if params.Arguments["includeRoutes"] != true {
		t.Errorf("expected includeRoutes=true, got %v", params.Arguments["includeRoutes"])
	}

	if params.Arguments["metadataAggregation"] != "minutes" {
		t.Errorf("expected metadataAggregation=minutes, got %v", params.Arguments["metadataAggregation"])
	}
}

// TestConnectionRefused verifies that a connection error is returned gracefully.
func TestConnectionRefused(t *testing.T) {
	// Use a port that's guaranteed to be unused
	client := NewHAEClient("127.0.0.1", 1)
	client.timeout = 1 * time.Second

	_, err := client.callTool("health_metrics", map[string]any{})
	if err == nil {
		t.Fatal("expected error for refused connection")
	}
}

// TestEmptyResponse verifies that an empty response is handled as an error.
func TestEmptyResponse(t *testing.T) {
	port := startMockTCPServer(t, []byte{})

	client := NewHAEClient("127.0.0.1", port)
	client.timeout = 5 * time.Second

	_, err := client.callTool("health_metrics", map[string]any{})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

// TestSyncState verifies the sync_state table operations.
func TestSyncState(t *testing.T) {
	dir := t.TempDir()
	state, err := OpenStateDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()

	// Get non-existent key returns empty string
	val, err := state.GetSyncState("tcp_last_metrics_sync")
	if err != nil {
		t.Fatalf("GetSyncState returned error: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}

	// Set and get
	if err := state.SetSyncState("tcp_last_metrics_sync", "2025-02-01"); err != nil {
		t.Fatalf("SetSyncState returned error: %v", err)
	}

	val, err = state.GetSyncState("tcp_last_metrics_sync")
	if err != nil {
		t.Fatalf("GetSyncState returned error: %v", err)
	}
	if val != "2025-02-01" {
		t.Errorf("expected 2025-02-01, got %q", val)
	}

	// Overwrite
	if err := state.SetSyncState("tcp_last_metrics_sync", "2025-03-01"); err != nil {
		t.Fatalf("SetSyncState returned error: %v", err)
	}

	val, err = state.GetSyncState("tcp_last_metrics_sync")
	if err != nil {
		t.Fatalf("GetSyncState returned error: %v", err)
	}
	if val != "2025-03-01" {
		t.Errorf("expected 2025-03-01, got %q", val)
	}
}

