package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleGetSourcePriorities returns the user's source priority rules and known sources.
func (s *Server) handleGetSourcePriorities(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	rules, err := s.db.GetSourcePriorities(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sources, err := s.db.GetDistinctSources(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	categories, err := s.db.GetAllowlistCategories(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"rules":      rules,
		"sources":    sources,
		"categories": categories,
		"default":    s.db.SourcePriority,
	})
}

// handleUpsertSourcePriority saves a source priority rule for a category.
func (s *Server) handleUpsertSourcePriority(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	var body struct {
		Category string   `json:"category"`
		Sources  []string `json:"sources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.Category == "" || len(body.Sources) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category and sources are required"})
		return
	}

	if err := s.db.UpsertSourcePriority(r.Context(), uid, body.Category, body.Sources); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// handleDeleteSourcePriority removes a category override.
func (s *Server) handleDeleteSourcePriority(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	category := chi.URLParam(r, "category")
	if category == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category is required"})
		return
	}

	if err := s.db.DeleteSourcePriority(r.Context(), uid, category); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
