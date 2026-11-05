package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/claude/freereps/internal/hevy"
	"github.com/claude/freereps/internal/opengym"
)

// SetOpenGym configures the openGym integration.
// Must be called before the server starts handling requests.
func (s *Server) SetOpenGym(syncer *opengym.Syncer) {
	s.openGymSyncer = syncer
}

// handleOpenGymStatus reports the stored connection and the earliest sync_from
// a new connection may use. The token is never returned.
func (s *Server) handleOpenGymStatus(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	creds, err := s.db.GetOpenGymCredentials(ctx, uid)
	if err != nil {
		s.log.Error("getting openGym credentials", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read credentials"})
		return
	}

	resp := map[string]any{"configured": creds != nil}
	if creds != nil {
		resp["base_url"] = creds.BaseURL
		resp["sync_from"] = creds.SyncFrom.Format("2006-01-02")
		if state, err := s.db.GetOpenGymSyncState(ctx, uid); err != nil {
			s.log.Warn("getting openGym sync state", "error", err)
		} else if state != nil {
			resp["last_change_read"] = state.UpdatedAt.Format(time.RFC3339)
		}
	} else if s.openGymSyncer != nil {
		// The cutoff a new connection would get, so the form can show it.
		lastHevy, hasHevy, err := s.db.LastSessionDate(ctx, uid, hevy.SourceName)
		if err != nil {
			s.log.Warn("reading last Hevy session", "error", err)
		} else {
			suggested, err := opengym.ResolveSyncFrom("", lastHevy, hasHevy, s.openGymSyncer.Location(), time.Now())
			if err == nil {
				resp["suggested_sync_from"] = suggested.Format("2006-01-02")
			}
			if hasHevy {
				resp["last_hevy_session"] = lastHevy.In(s.openGymSyncer.Location()).Format("2006-01-02")
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOpenGymPair connects an openGym instance: it redeems a pairing code
// minted in openGym's settings, validates the token and stores it. The token
// never passes through the browser.
//
// Everything that can be checked without the code is checked before it is
// redeemed, because a code is valid once.
func (s *Server) handleOpenGymPair(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	if s.openGymSyncer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "openGym sync not configured"})
		return
	}
	ctx := r.Context()

	var body struct {
		BaseURL  string `json:"base_url"`
		Code     string `json:"code"`
		SyncFrom string `json:"sync_from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	baseURL, err := opengym.NormalizeBaseURL(body.BaseURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	code, err := opengym.NormalizePairCode(body.Code)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	lastHevy, hasHevy, err := s.db.LastSessionDate(ctx, uid, hevy.SourceName)
	if err != nil {
		s.log.Error("reading last Hevy session", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the Hevy history"})
		return
	}
	syncFrom, err := opengym.ResolveSyncFrom(body.SyncFrom, lastHevy, hasHevy, s.openGymSyncer.Location(), time.Now())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	token, me, err := opengym.Pair(ctx, opengym.NewClient(baseURL), code)
	if err != nil {
		s.log.Warn("openGym pairing failed", "base_url", baseURL, "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := s.db.UpsertOpenGymCredentials(ctx, uid, baseURL, token, syncFrom); err != nil {
		s.log.Error("saving openGym credentials", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save credentials"})
		return
	}
	s.log.Info("openGym paired", "user_id", uid, "base_url", baseURL, "sync_from", syncFrom.Format("2006-01-02"))
	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "paired",
		"profile":   me.User.Name,
		"sync_from": syncFrom.Format("2006-01-02"),
	})
}

// handleOpenGymSync starts a sync in the background and returns immediately.
func (s *Server) handleOpenGymSync(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	if s.openGymSyncer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "openGym sync not configured"})
		return
	}
	// Detached context: the sync outlives the request it was started from.
	go s.openGymSyncer.SyncUser(context.Background(), uid)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sync_started"})
}

// handleOpenGymDisconnect removes the stored connection. Sets already ingested
// stay in workout_sets. The token stays valid in openGym until it expires or
// "Sign out everywhere" revokes every token of the profile; openGym has no
// per-token revocation.
func (s *Server) handleOpenGymDisconnect(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	if err := s.db.DeleteOpenGymCredentials(r.Context(), uid); err != nil {
		s.log.Error("deleting openGym credentials", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to disconnect"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}
