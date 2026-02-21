package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/claude/freereps/internal/ingest"
	"github.com/claude/freereps/internal/storage"
)

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	stats, err := s.db.GetDataStats(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleImportLogs(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	logs, err := s.db.QueryImportLogs(r.Context(), uid, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// logImport records an import operation's result to the import_logs table.
func (s *Server) logImport(uid int, source string, result *ingest.Result, importErr error, durationMs int) {
	status := "success"
	var errMsg *string
	if importErr != nil {
		status = "error"
		msg := importErr.Error()
		errMsg = &msg
	}

	log := storage.ImportLog{
		UserID:           uid,
		Source:           source,
		Status:           status,
		MetricsReceived:  result.MetricsReceived,
		MetricsInserted:  result.MetricsInserted,
		WorkoutsReceived: result.WorkoutsReceived,
		WorkoutsInserted: result.WorkoutsInserted,
		SleepSessions:    result.SleepSessionsInserted,
		SetsInserted:     result.SetsInserted,
		DurationMs:       &durationMs,
		ErrorMessage:     errMsg,
	}

	ctx, cancel := contextWithTimeout()
	defer cancel()

	if _, err := s.db.InsertImportLog(ctx, log); err != nil {
		s.log.Error("failed to log import", "source", source, "error", err)
	}
}

// contextWithTimeout returns a background context with a 5-second timeout for async logging.
func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second) //nolint:mnd
}
