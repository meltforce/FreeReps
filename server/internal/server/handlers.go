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

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	info := userInfoFromContext(r)
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleHAEIngest(w http.ResponseWriter, r *http.Request) {
	var payload models.HAEPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start := time.Now()
	result, err := s.hae.Ingest(r.Context(), &payload, uid)
	durationMs := int(time.Since(start).Milliseconds())
	if err != nil {
		s.log.Error("ingest error", "error", err)
		if result != nil {
			go s.logImport(uid, "hae_rest", result, err, durationMs)
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if result.SleepStagesInserted > 0 {
		if err := s.db.BackfillSleepSessions(r.Context(), s.log); err != nil {
			s.log.Warn("sleep session backfill after REST ingest failed", "error", err)
		}
	}

	go s.logImport(uid, "hae_rest", result, nil, durationMs)
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

	go s.logImport(uid, "alpha", result, nil, durationMs)
	writeJSON(w, http.StatusOK, result)
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
		go s.logImport(uid, "import_auto", result, nil, durationMs)
		writeJSON(w, http.StatusOK, result)

	default:
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":     "unrecognized file format",
			"supported": []string{"alpha_progression_csv"},
		})
	}
}

// cumulativeMetrics are metrics that should show daily totals instead of latest value.
var cumulativeMetrics = []string{
	"active_energy", "basal_energy_burned", "apple_exercise_time",
	"step_count", "distance_walking_running", "distance_cycling",
	"distance_swimming", "distance_wheelchair", "flights_climbed",
	"apple_move_time", "apple_stand_time", "push_count",
	"swimming_stroke_count", "distance_downhill_snow_sports",
}

func (s *Server) handleLatestMetrics(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	rows, err := s.db.GetLatestMetrics(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get daily sums for cumulative metrics
	sums, err := s.db.GetDailySums(r.Context(), uid, cumulativeMetrics)
	if err != nil {
		s.log.Error("daily sums error", "error", err)
		// Non-fatal: continue with latest values
		writeJSON(w, http.StatusOK, rows)
		return
	}

	// Build response with daily_sums field
	writeJSON(w, http.StatusOK, map[string]any{
		"latest":     rows,
		"daily_sums": sums,
	})
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

	workouts, err := s.db.QueryWorkouts(r.Context(), start, end, uid, nameFilter)
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

	// Fetch workout to get its date range
	workout, err := s.db.GetWorkout(r.Context(), workoutID, uid)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workout not found"})
		return
	}

	// Query sets using workout time window (±2 hours) instead of full day
	// to avoid leaking exercises from other workouts on the same day
	windowStart := workout.StartTime.Add(-2 * time.Hour)
	windowEnd := workout.EndTime.Add(2 * time.Hour)

	sets, err := s.db.QueryWorkoutSets(r.Context(), windowStart, windowEnd, uid, "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sets)
}

func (s *Server) handleAllowlist(w http.ResponseWriter, r *http.Request) {
	metrics, err := s.db.GetAllowedMetrics(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, metrics)
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
