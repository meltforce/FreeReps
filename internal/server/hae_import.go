package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/claude/freereps/internal/ingest"
	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
	"github.com/claude/freereps/internal/upload"
)

// haeImportState tracks a running HAE TCP import.
type haeImportState struct {
	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	doneCh   chan struct{} // closed when goroutine exits
	step     int
	total    int
	metric   string  // current metric/phase being processed
	chunk    string  // current chunk date range
	done     bool
	err      error
	logID    int64   // import_logs row id
	startedAt time.Time

	// Result counters (accumulated from ingest.Result per chunk)
	metricsReceived  int
	metricsInserted  int64
	workoutsReceived int
	workoutsInserted int
	sleepSessions    int
	bytesFetched     int64
	haeHost          string
	haePort          int

	// SSE subscribers
	subs   map[chan sseEvent]struct{}
	subsMu sync.Mutex
}

// sseEvent is an SSE message to send to subscribers.
type sseEvent struct {
	Event string
	Data  string
}

func (st *haeImportState) broadcast(event sseEvent) {
	st.subsMu.Lock()
	defer st.subsMu.Unlock()
	for ch := range st.subs {
		select {
		case ch <- event:
		default:
			// slow subscriber, skip
		}
	}
}

func (st *haeImportState) subscribe() chan sseEvent {
	ch := make(chan sseEvent, 32)
	st.subsMu.Lock()
	st.subs[ch] = struct{}{}
	st.subsMu.Unlock()
	return ch
}

func (st *haeImportState) unsubscribe(ch chan sseEvent) {
	st.subsMu.Lock()
	delete(st.subs, ch)
	st.subsMu.Unlock()
}

// haeImportRequest is the JSON body for starting an HAE TCP import.
type haeImportRequest struct {
	HAEHost   string `json:"hae_host"`
	HAEPort   int    `json:"hae_port"`
	Start     string `json:"start"`     // YYYY-MM-DD
	End       string `json:"end"`       // YYYY-MM-DD
	ChunkDays int    `json:"chunk_days"`
	DryRun    bool   `json:"dry_run"`
}

func (s *Server) handleCheckHAE(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HAEHost string `json:"hae_host"`
		HAEPort int    `json:"hae_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.HAEHost == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hae_host is required"})
		return
	}
	if req.HAEPort == 0 {
		req.HAEPort = 9000
	}

	client := upload.NewHAEClient(req.HAEHost, req.HAEPort)
	if err := client.Ping(); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"reachable": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reachable": true})
}

func (s *Server) handleStartHAEImport(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	var req haeImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.HAEHost == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hae_host is required"})
		return
	}
	if req.HAEPort == 0 {
		req.HAEPort = 9000
	}
	if req.ChunkDays == 0 {
		req.ChunkDays = 7
	}

	startDate, err := time.Parse("2006-01-02", req.Start)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start date (YYYY-MM-DD): " + err.Error()})
		return
	}
	endDate, err := time.Parse("2006-01-02", req.End)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end date (YYYY-MM-DD): " + err.Error()})
		return
	}
	// Make end date inclusive: advance to start of next day so queries
	// cover the entire end date (YYYY-MM-DD 00:00 → YYYY-MM-DD+1 00:00).
	endDate = endDate.AddDate(0, 0, 1)

	s.importMu.Lock()
	if s.activeImport != nil && s.activeImport.running {
		// If context was already canceled, wait briefly for the goroutine to finish
		prev := s.activeImport
		s.importMu.Unlock()
		select {
		case <-prev.doneCh:
			// Goroutine finished, proceed to start a new import
		case <-time.After(5 * time.Second):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "an import is already running"})
			return
		}
		s.importMu.Lock()
	}

	// Calculate total steps
	chunkDur := time.Duration(req.ChunkDays) * 24 * time.Hour
	numChunks := 0
	for cs := startDate; cs.Before(endDate); cs = cs.Add(chunkDur) {
		numChunks++
	}
	totalSteps := len(upload.TCPMetrics)*numChunks + numChunks

	ctx, cancel := context.WithCancel(context.Background())
	state := &haeImportState{
		running:   true,
		cancel:    cancel,
		doneCh:    make(chan struct{}),
		total:     totalSteps,
		startedAt: time.Now(),
		subs:      make(map[chan sseEvent]struct{}),
		haeHost:   req.HAEHost,
		haePort:   req.HAEPort,
	}

	// Create import log with "running" status
	metaJSON, _ := json.Marshal(map[string]any{
		"hae_host":   req.HAEHost,
		"hae_port":   req.HAEPort,
		"start":      req.Start,
		"end":        req.End,
		"chunk_days": req.ChunkDays,
		"dry_run":    req.DryRun,
	})
	rawMeta := json.RawMessage(metaJSON)
	logID, logErr := s.db.InsertImportLog(r.Context(), storage.ImportLog{
		UserID:   uid,
		Source:   "hae_tcp",
		Status:   "running",
		Metadata: &rawMeta,
	})
	if logErr != nil {
		s.log.Error("failed to create import log", "error", logErr)
	}
	state.logID = logID

	s.activeImport = state
	s.importMu.Unlock()

	// Start background goroutine
	go s.runHAEImport(ctx, state, uid, req, startDate, endDate)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":      "started",
		"total_steps": totalSteps,
		"log_id":      logID,
	})
}

func (s *Server) runHAEImport(ctx context.Context, state *haeImportState, userID int, req haeImportRequest, start, end time.Time) {
	defer func() {
		state.mu.Lock()
		state.running = false
		state.done = true
		state.mu.Unlock()
		close(state.doneCh)
	}()

	haeClient := upload.NewHAEClient(req.HAEHost, req.HAEPort)
	chunkDur := time.Duration(req.ChunkDays) * 24 * time.Hour
	currentStep := 0

	// Phase 1: Health metrics
	for _, m := range upload.TCPMetrics {
		for chunkStart := start; chunkStart.Before(end); chunkStart = chunkStart.Add(chunkDur) {
			if ctx.Err() != nil {
				state.mu.Lock()
				state.err = fmt.Errorf("import canceled by user")
				state.mu.Unlock()
				s.finalizeImport(state, userID)
				return
			}

			chunkEnd := chunkStart.Add(chunkDur)
			if chunkEnd.After(end) {
				chunkEnd = end
			}
			currentStep++

			chunkRange := fmt.Sprintf("%s → %s", chunkStart.Format("2006-01-02"), chunkEnd.Format("2006-01-02"))
			state.mu.Lock()
			state.step = currentStep
			state.metric = m.Name
			state.chunk = chunkRange
			state.mu.Unlock()

			state.broadcast(sseEvent{
				Event: "progress",
				Data: mustJSON(map[string]any{
					"step":   currentStep,
					"total":  state.total,
					"metric": m.Name,
					"chunk":  chunkRange,
					"phase":  "metrics",
				}),
			})

			result, err := haeClient.QueryMetricsWithRetry(chunkStart, chunkEnd, m.Name, m.Aggregate, s.log)
			if err != nil {
				s.log.Warn("HAE TCP query failed, skipping",
					"metric", m.Name, "chunk", chunkRange, "error", err)
				continue
			}

			if len(result) == 0 || string(result) == "null" {
				continue
			}

			if !req.DryRun {
				ir, err := s.ingestRawHAEResult(ctx, result, userID)
				if err != nil {
					s.log.Warn("ingest failed", "metric", m.Name, "chunk", chunkRange, "error", err)
					continue
				}
				state.mu.Lock()
				state.metricsReceived += ir.MetricsReceived
				state.metricsInserted += ir.MetricsInserted
				state.sleepSessions += ir.SleepSessionsInserted
				state.mu.Unlock()
			}

			state.mu.Lock()
			state.bytesFetched += int64(len(result))
			state.mu.Unlock()
		}
	}

	// Phase 2: Workouts
	for chunkStart := start; chunkStart.Before(end); chunkStart = chunkStart.Add(chunkDur) {
		if ctx.Err() != nil {
			state.mu.Lock()
			state.err = fmt.Errorf("import canceled by user")
			state.mu.Unlock()
			s.finalizeImport(state, userID)
			return
		}

		chunkEnd := chunkStart.Add(chunkDur)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		currentStep++

		chunkRange := fmt.Sprintf("%s → %s", chunkStart.Format("2006-01-02"), chunkEnd.Format("2006-01-02"))
		state.mu.Lock()
		state.step = currentStep
		state.metric = "workouts"
		state.chunk = chunkRange
		state.mu.Unlock()

		state.broadcast(sseEvent{
			Event: "progress",
			Data: mustJSON(map[string]any{
				"step":   currentStep,
				"total":  state.total,
				"metric": "workouts",
				"chunk":  chunkRange,
				"phase":  "workouts",
			}),
		})

		result, err := haeClient.QueryWorkoutsWithRetry(chunkStart, chunkEnd, s.log)
		if err != nil {
			s.log.Warn("HAE TCP workout query failed", "chunk", chunkRange, "error", err)
			continue
		}

		if len(result) == 0 || string(result) == "null" {
			continue
		}

		if !req.DryRun {
			ir, err := s.ingestRawHAEResult(ctx, result, userID)
			if err != nil {
				s.log.Warn("workout ingest failed", "chunk", chunkRange, "error", err)
				continue
			}
			state.mu.Lock()
			state.workoutsReceived += ir.WorkoutsReceived
			state.workoutsInserted += ir.WorkoutsInserted
			state.mu.Unlock()
		}

		state.mu.Lock()
		state.bytesFetched += int64(len(result))
		state.mu.Unlock()
	}

	// Broadcast completion
	state.broadcast(sseEvent{
		Event: "complete",
		Data: mustJSON(map[string]any{
			"metrics_received":  state.metricsReceived,
			"metrics_inserted":  state.metricsInserted,
			"workouts_received": state.workoutsReceived,
			"workouts_inserted": state.workoutsInserted,
			"sleep_sessions":    state.sleepSessions,
			"bytes_fetched":     state.bytesFetched,
		}),
	})

	s.finalizeImport(state, userID)
}

// ingestRawHAEResult parses a raw HAE JSON-RPC result and ingests it via the HAE provider.
func (s *Server) ingestRawHAEResult(ctx context.Context, raw json.RawMessage, userID int) (*ingest.Result, error) {
	var payload models.HAEPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshaling HAE result: %w", err)
	}
	return s.hae.Ingest(ctx, &payload, userID)
}

// finalizeImport updates the import_logs row with final results.
func (s *Server) finalizeImport(state *haeImportState, userID int) {
	if state.logID == 0 {
		return
	}

	durationMs := int(time.Since(state.startedAt).Milliseconds())
	status := "success"
	var errMsg *string
	if state.err != nil {
		msg := state.err.Error()
		errMsg = &msg
		if msg == "import canceled by user" {
			status = "cancelled"
		} else {
			status = "error"
		}
	}

	ctx, cancel := contextWithTimeout()
	defer cancel()

	metaJSON, _ := json.Marshal(map[string]any{
		"bytes_fetched": state.bytesFetched,
		"hae_host":      state.haeHost,
		"hae_port":      state.haePort,
	})
	rawMeta := json.RawMessage(metaJSON)

	if err := s.db.UpdateImportLog(ctx, state.logID, storage.ImportLog{
		Status:           status,
		MetricsReceived:  state.metricsReceived,
		MetricsInserted:  state.metricsInserted,
		WorkoutsReceived: state.workoutsReceived,
		WorkoutsInserted: state.workoutsInserted,
		SleepSessions:    state.sleepSessions,
		DurationMs:       &durationMs,
		ErrorMessage:     errMsg,
		Metadata:         &rawMeta,
	}); err != nil {
		s.log.Error("failed to finalize import log", "log_id", state.logID, "error", err)
	}
}

func (s *Server) handleCancelHAEImport(w http.ResponseWriter, r *http.Request) {
	s.importMu.Lock()
	if s.activeImport == nil || !s.activeImport.running {
		s.importMu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no import running"})
		return
	}

	state := s.activeImport
	state.cancel()
	s.importMu.Unlock()

	// Wait briefly for goroutine to finish
	select {
	case <-state.doneCh:
	case <-time.After(3 * time.Second):
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *Server) handleHAEImportStatus(w http.ResponseWriter, r *http.Request) {
	s.importMu.Lock()
	state := s.activeImport
	s.importMu.Unlock()

	if state == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"running": false,
		})
		return
	}

	state.mu.Lock()
	resp := map[string]any{
		"running":           state.running,
		"done":              state.done,
		"step":              state.step,
		"total":             state.total,
		"metric":            state.metric,
		"chunk":             state.chunk,
		"metrics_received":  state.metricsReceived,
		"metrics_inserted":  state.metricsInserted,
		"workouts_received": state.workoutsReceived,
		"workouts_inserted": state.workoutsInserted,
		"sleep_sessions":    state.sleepSessions,
		"bytes_fetched":     state.bytesFetched,
		"log_id":            state.logID,
	}
	if state.err != nil {
		resp["error"] = state.err.Error()
	}
	state.mu.Unlock()

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleHAEImportEvents(w http.ResponseWriter, r *http.Request) {
	s.importMu.Lock()
	state := s.activeImport
	s.importMu.Unlock()

	if state == nil || !state.running {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no import running"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := state.subscribe()
	defer state.unsubscribe(ch)

	// Send current status immediately
	state.mu.Lock()
	fmt.Fprintf(w, "event: status\ndata: %s\n\n", mustJSON(map[string]any{
		"step":  state.step,
		"total": state.total,
		"metric": state.metric,
		"chunk":  state.chunk,
	}))
	state.mu.Unlock()
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Event, evt.Data)
			flusher.Flush()

			if evt.Event == "complete" || evt.Event == "error" {
				return
			}
		}
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{}`
	}
	return string(b)
}
