package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

const (
	ouraStateCookieName = "oura_state"
	ouraRedirectURI     = "https://freereps.leo-royal.ts.net/oura/callback"
)

// handleOuraStatus returns the Oura connection status for the current user.
func (s *Server) handleOuraStatus(w http.ResponseWriter, r *http.Request) {
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
			"configured": false,
			"connected":  false,
		})
		return
	}

	connected := token.AccessToken != ""

	result := map[string]any{
		"configured": true,
		"connected":  connected,
		"client_id":  token.ClientID,
	}

	if connected {
		result["expires_at"] = token.ExpiresAt.Format(time.RFC3339)

		states, err := s.db.ListOuraSyncStates(r.Context(), uid)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		syncMap := make(map[string]string, len(states))
		for _, st := range states {
			syncMap[st.DataType] = st.LastSync.Format("2006-01-02")
		}
		result["sync_states"] = syncMap
	}

	writeJSON(w, http.StatusOK, result)
}

// handleOuraCredentials saves the user's Oura developer app credentials.
func (s *Server) handleOuraCredentials(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	var body struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.ClientID == "" || body.ClientSecret == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client_id and client_secret are required"})
		return
	}

	if err := s.db.UpsertOuraCredentials(r.Context(), uid, body.ClientID, body.ClientSecret); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// handleOuraAuthorize returns the OAuth2 authorization URL for the user to visit.
func (s *Server) handleOuraAuthorize(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
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

	url, err := s.ouraTokenMgr.AuthorizeURL(r.Context(), uid, ouraRedirectURI, state)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"authorize_url": url})
}

// handleOuraCallback handles the OAuth2 redirect from Oura after user authorization.
func (s *Server) handleOuraCallback(w http.ResponseWriter, r *http.Request) {
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
