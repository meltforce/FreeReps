package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleHAEIngest(w http.ResponseWriter, r *http.Request) {
	var payload models.HAEPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	result, err := s.hae.Ingest(r.Context(), &payload)
	if err != nil {
		s.log.Error("ingest error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAlphaIngest(w http.ResponseWriter, r *http.Request) {
	result, err := s.alpha.Ingest(r.Context(), r.Body)
	if err != nil {
		s.log.Error("alpha ingest error", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleLatestMetrics(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.GetLatestMetrics(r.Context(), 1)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rows)
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

	rows, err := s.db.QueryHealthMetrics(r.Context(), name, start, end, 1)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleQuerySleep(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sessions, err := s.db.QuerySleepSessions(r.Context(), start, end, 1)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	stages, err := s.db.QuerySleepStages(r.Context(), start, end, 1)
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
	workouts, err := s.db.QueryWorkouts(r.Context(), start, end, 1, nameFilter)
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

	detail, err := s.db.GetWorkout(r.Context(), workoutID, 1)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workout not found"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
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
	case "daily", "":
		bucket = "1 day"
	}

	points, err := s.db.GetTimeSeries(r.Context(), metric, start, end, bucket, 1)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (s *Server) handleAllowlist(w http.ResponseWriter, r *http.Request) {
	metrics, err := s.db.GetAllowedMetrics(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
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
