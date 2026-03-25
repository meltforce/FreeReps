package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

const (
	ouraStateCookieName = "oura_state"
	ouraRedirectURI     = "https://freereps.leo-royal.ts.net/oura/callback"
)

// handleOuraStatus returns the Oura connection status for the current user.
func (s *Server) handleOuraStatus(w http.ResponseWriter, r *http.Request) {
	if s.ouraTokenMgr == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	token, err := s.db.GetOuraToken(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if token == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled":   true,
			"connected": false,
		})
		return
	}

	// Get sync states.
	states, err := s.db.ListOuraSyncStates(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	syncMap := make(map[string]string, len(states))
	for _, st := range states {
		syncMap[st.DataType] = st.LastSync.Format("2006-01-02")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":     true,
		"connected":   true,
		"expires_at":  token.ExpiresAt.Format(time.RFC3339),
		"sync_states": syncMap,
	})
}

// handleOuraAuthorize returns the OAuth2 authorization URL for the user to visit.
func (s *Server) handleOuraAuthorize(w http.ResponseWriter, r *http.Request) {
	if s.ouraTokenMgr == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "oura integration not configured"})
		return
	}

	// Generate CSRF state token and store in cookie.
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "generating state"})
		return
	}
	state := hex.EncodeToString(stateBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     ouraStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	url := s.ouraTokenMgr.AuthorizeURL(ouraRedirectURI, state)
	writeJSON(w, http.StatusOK, map[string]string{"authorize_url": url})
}

// handleOuraCallback handles the OAuth2 redirect from Oura after user authorization.
func (s *Server) handleOuraCallback(w http.ResponseWriter, r *http.Request) {
	if s.ouraTokenMgr == nil {
		http.Error(w, "oura integration not configured", http.StatusNotFound)
		return
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	// Verify CSRF state.
	cookie, err := r.Cookie(ouraStateCookieName)
	if err != nil || cookie.Value == "" {
		http.Error(w, "missing state cookie", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != cookie.Value {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}

	// Clear the state cookie.
	http.SetCookie(w, &http.Cookie{
		Name:   ouraStateCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Check for error from Oura.
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, "/settings?tab=oura&error="+errParam, http.StatusFound)
		return
	}

	// Exchange authorization code for tokens.
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	if err := s.ouraTokenMgr.ExchangeCode(r.Context(), code, ouraRedirectURI, uid); err != nil {
		s.log.Error("oura code exchange failed", "error", err)
		http.Redirect(w, r, "/settings?tab=oura&error=exchange_failed", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/settings?tab=oura", http.StatusFound)
}

// handleOuraSync triggers an immediate sync for the current user.
func (s *Server) handleOuraSync(w http.ResponseWriter, r *http.Request) {
	if s.ouraSyncer == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "oura integration not configured"})
		return
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	go func() {
		if err := s.ouraSyncer.TriggerSync(r.Context(), uid); err != nil {
			s.log.Error("manual oura sync failed", "user_id", uid, "error", err)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sync_started"})
}

// handleOuraDisconnect removes Oura tokens and sync state for the current user.
func (s *Server) handleOuraDisconnect(w http.ResponseWriter, r *http.Request) {
	if s.ouraTokenMgr == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "oura integration not configured"})
		return
	}

	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	if err := s.ouraTokenMgr.Disconnect(r.Context(), uid); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}
