package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/claude/freereps/internal/ingest"
	"github.com/claude/freereps/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": Version})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	info := userInfoFromContext(r)
	writeJSON(w, http.StatusOK, info)
}


// ClientHeader names the client that posts to /api/v1/ingest. The FreeReps iOS
// app sends ClientIOSApp; a request without the header is Health Auto Export,
// which shares the endpoint and payload format.
const (
	ClientHeader = "X-FreeReps-Client"
	ClientIOSApp = "freereps-ios"
)

// ingestLogSource returns the import_logs source for an ingest request.
func ingestLogSource(r *http.Request) string {
	if r.Header.Get(ClientHeader) == ClientIOSApp {
		return "freereps_ios"
	}
	return "hae_rest"
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	var payload models.HealthPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	source := ingestLogSource(r)
	start := time.Now()
	result, err := s.health.Ingest(r.Context(), &payload, uid)
	durationMs := int(time.Since(start).Milliseconds())
	if err != nil {
		s.log.Error("ingest error", "error", err)
		if result != nil {
			go s.logImport(uid, source, result, err, durationMs)
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if result.SleepStagesInserted > 0 {
		if err := s.db.BackfillSleepSessions(r.Context(), s.log); err != nil {
			s.log.Warn("sleep session backfill after REST ingest failed", "error", err)
		}
	}

	s.db.InvalidateAllAvailableMetrics()
	go s.logImport(uid, source, result, nil, durationMs)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAlphaIngest(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start := time.Now()
	result, err := s.alpha.Ingest(r.Context(), r.Body, uid)
	durationMs := int(time.Since(start).Milliseconds())
	if err != nil {
		s.log.Error("alpha ingest error", "error", err)
		if result != nil {
			go s.logImport(uid, "alpha", result, err, durationMs)
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// The derived tonnage series is computed from the rows just written.
	if from, to, ok, err := s.db.TrainingMetricsRange(r.Context(), uid); err != nil {
		s.log.Warn("reading training range after alpha ingest", "error", err)
	} else if ok {
		if _, err := s.db.RebuildTrainingMetrics(r.Context(), uid, from, to); err != nil {
			s.log.Warn("rebuilding training metrics after alpha ingest", "error", err)
		}
	}

	s.db.InvalidateAllAvailableMetrics()
	go s.logImport(uid, "alpha", result, nil, durationMs)
	writeJSON(w, http.StatusOK, result)
}

// handleRebuildTrainingMetrics recomputes the derived tonnage series over the
// user's full history. Needed once after the series is introduced, and after any
// bulk change to workout_sets that bypassed the ingest paths.
func (s *Server) handleRebuildTrainingMetrics(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	from, to, hasData, err := s.db.TrainingMetricsRange(r.Context(), uid)
	if err != nil {
		s.log.Error("reading training range", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !hasData {
		writeJSON(w, http.StatusOK, map[string]any{"days_written": 0, "message": "no strength data"})
		return
	}

	n, err := s.db.RebuildTrainingMetrics(r.Context(), uid, from, to)
	if err != nil {
		s.log.Error("rebuilding training metrics", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"days_written": n,
		"from":         from.Format("2006-01-02"),
		"to":           to.Format("2006-01-02"),
	})
}

func (s *Server) handleUnifiedImport(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	format := ingest.DetectFormat(data)
	start := time.Now()

	switch format {
	case ingest.FormatAlpha:
		result, err := s.alpha.Ingest(r.Context(), bytes.NewReader(data), uid)
		durationMs := int(time.Since(start).Milliseconds())
		if err != nil {
			s.log.Error("unified import (alpha) error", "error", err)
			if result != nil {
				go s.logImport(uid, "import_auto", result, err, durationMs)
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.db.InvalidateAllAvailableMetrics()
		go s.logImport(uid, "import_auto", result, nil, durationMs)
		writeJSON(w, http.StatusOK, result)

	default:
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":     "unrecognized file format",
			"supported": []string{"alpha_progression_csv"},
		})
	}
}

func (s *Server) handleQueryMetrics(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name parameter required"})
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	rows, err := s.db.QueryHealthMetrics(r.Context(), name, start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleQuerySleep(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sessions, err := s.db.QuerySleepSessions(r.Context(), start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	stages, err := s.db.QuerySleepStages(r.Context(), start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"stages":   stages,
	})
}

func (s *Server) handleQueryWorkouts(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	nameFilter := r.URL.Query().Get("type")
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	workouts, err := s.db.QueryWorkoutsMerged(r.Context(), start, end, uid, nameFilter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, workouts)
}

func (s *Server) handleGetWorkout(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	workoutID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid workout ID"})
		return
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	detail, err := s.db.GetWorkout(r.Context(), workoutID, uid)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workout not found"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleMetricStats(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "metric parameter required"})
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	stats, err := s.db.GetMetricStats(r.Context(), metric, start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleTimeSeries(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "metric parameter required"})
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	agg := r.URL.Query().Get("agg")
	bucket := "1 day" // default
	switch agg {
	case "hourly":
		bucket = "1 hour"
	case "weekly":
		bucket = "1 week"
	case "monthly":
		bucket = "1 month"
	case "daily", "":
		bucket = "1 day"
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	points, err := s.db.GetTimeSeries(r.Context(), metric, start, end, bucket, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (s *Server) handleCorrelation(w http.ResponseWriter, r *http.Request) {
	xMetric := r.URL.Query().Get("x")
	yMetric := r.URL.Query().Get("y")
	if xMetric == "" || yMetric == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "x and y metric parameters required"})
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	bucket := r.URL.Query().Get("bucket")
	if bucket == "" {
		bucket = "1 day"
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	result, err := s.db.GetCorrelation(r.Context(), xMetric, yMetric, start, end, bucket, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleWorkoutSets(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	workoutID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid workout ID"})
		return
	}

	// Fetch workout to get its date range. For synthetic Alpha-only workouts
	// that don't exist in the DB, fall back to start/end query params.
	var windowStart, windowEnd time.Time
	workout, err := s.db.GetWorkout(r.Context(), workoutID, uid)
	if err != nil {
		// Synthetic workout — use query params as fallback.
		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")
		st, errS := time.Parse(time.RFC3339, startStr)
		et, errE := time.Parse(time.RFC3339, endStr)
		if errS != nil || errE != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "workout not found"})
			return
		}
		windowStart = st.Add(-2 * time.Hour)
		windowEnd = et.Add(2 * time.Hour)
	} else {
		windowStart = workout.StartTime.Add(-2 * time.Hour)
		windowEnd = workout.EndTime.Add(2 * time.Hour)
	}

	sets, err := s.db.QueryWorkoutSets(r.Context(), windowStart, windowEnd, uid, "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sets)
}

// handleAllowlist returns the allowlist with enabled resolved for the calling
// user, so a client can skip metrics the ingest would reject.
func (s *Server) handleAllowlist(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	metrics, err := s.db.GetUserAllowlist(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) handleAvailableMetrics(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	metrics, err := s.db.GetAvailableMetrics(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=60")
	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) handleSaveMetricVisibility(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	var body map[string]bool
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := s.db.SaveMetricVisibility(r.Context(), uid, body); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.db.InvalidateAvailableMetrics(uid)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) handleSaveMetricEnabled(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	var body map[string]bool
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := s.db.SaveMetricEnabled(r.Context(), uid, body); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) handleGetECGRecordings(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	recordings, err := s.db.QueryECGRecordings(r.Context(), start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, recordings)
}

func (s *Server) handleGetAudiograms(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	audiograms, err := s.db.QueryAudiograms(r.Context(), start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, audiograms)
}

func (s *Server) handleGetActivitySummaries(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	summaries, err := s.db.QueryActivitySummaries(r.Context(), start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (s *Server) handleGetMedications(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	medications, err := s.db.QueryMedications(r.Context(), start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, medications)
}

func (s *Server) handleGetVisionPrescriptions(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	prescriptions, err := s.db.QueryVisionPrescriptions(r.Context(), start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, prescriptions)
}

func (s *Server) handleGetStateOfMind(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	records, err := s.db.QueryStateOfMind(r.Context(), start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *Server) handleGetCategorySamples(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	typeFilter := r.URL.Query().Get("type")

	samples, err := s.db.QueryCategorySamples(r.Context(), start, end, uid, typeFilter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseTimeRange(r *http.Request) (start, end time.Time, err error) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if startStr == "" {
		// Default: last 7 days
		end = time.Now()
		start = end.AddDate(0, 0, -7)
		return
	}

	start, err = time.Parse(time.RFC3339, startStr)
	if err != nil {
		start, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if endStr == "" {
		end = time.Now()
	} else {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			end, err = time.Parse("2006-01-02", endStr)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
			// End of day for date-only
			end = end.Add(24 * time.Hour)
		}
	}
	return
}
