package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/claude/freereps/internal/accounts"
	"github.com/claude/freereps/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/meltforce/meltkit/pkg/auth"
)

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// accountUser returns the user of an account request. Without application
// login the account routes answer 404.
func (s *Server) accountUser(w http.ResponseWriter, r *http.Request) (int, bool) {
	if s.auth == nil {
		writeError(w, http.StatusNotFound, "application login is not configured")
		return 0, false
	}
	return mustUserID(w, r)
}

// adminUser is accountUser for admin routes. A non-admin gets 404, so the
// routes stay invisible.
func (s *Server) adminUser(w http.ResponseWriter, r *http.Request) (int, bool) {
	uid, ok := s.accountUser(w, r)
	if !ok {
		return 0, false
	}
	if !userInfoFromContext(r).IsAdmin {
		writeError(w, http.StatusNotFound, "not found")
		return 0, false
	}
	return uid, true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

// --- sessions ---

type sessionJSON struct {
	ID         int64     `json:"id"`
	Method     string    `json:"method"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	ClientIP   string    `json:"client_ip"`
	UserAgent  string    `json:"user_agent"`
	Current    bool      `json:"current"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.accountUser(w, r)
	if !ok {
		return
	}
	sessions, err := s.auth.ListSessions(r.Context(), int64(uid))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cur, _ := auth.SessionFromContext(r.Context())
	out := make([]sessionJSON, 0, len(sessions))
	for _, se := range sessions {
		out = append(out, sessionJSON{
			ID: se.ID, Method: se.Method, CreatedAt: se.CreatedAt, ExpiresAt: se.ExpiresAt,
			LastUsedAt: se.LastUsedAt, ClientIP: se.ClientIP, UserAgent: se.UserAgent,
			Current: se.ID == cur.ID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.accountUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.auth.RevokeSession(r.Context(), int64(uid), id); err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- personal access tokens ---

type tokenJSON struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	Expired    bool       `json:"expired"`
}

func tokenView(t auth.Token, now time.Time) tokenJSON {
	v := tokenJSON{ID: t.ID, Name: t.Name, Scopes: t.Scopes, CreatedAt: t.CreatedAt}
	if !t.ExpiresAt.IsZero() {
		e := t.ExpiresAt
		v.ExpiresAt = &e
		v.Expired = !e.After(now)
	}
	if !t.LastUsedAt.IsZero() {
		l := t.LastUsedAt
		v.LastUsedAt = &l
	}
	return v
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.accountUser(w, r)
	if !ok {
		return
	}
	tokens, err := s.auth.ListTokens(r.Context(), int64(uid))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := time.Now()
	out := make([]tokenJSON, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, tokenView(t, now))
	}
	writeJSON(w, http.StatusOK, out)
}

type createTokenRequest struct {
	Name    string   `json:"name"`
	Scopes  []string `json:"scopes"`
	Expires string   `json:"expires"` // "90d", "720h" or "" for no expiry
}

type createTokenResponse struct {
	Token  string    `json:"token"`
	Detail tokenJSON `json:"detail"`
}

// handleCreateToken refuses a request that itself carries a token: an api
// token could otherwise mint an mcp token, or a replacement for itself after
// its revocation.
func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.accountUser(w, r)
	if !ok {
		return
	}
	if userInfoFromContext(r).AuthMethod == MethodToken {
		writeError(w, http.StatusForbidden, "a token cannot create tokens")
		return
	}
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	now := time.Now()
	expires, err := accounts.ParseExpiry(req.Expires, now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	secret, t, err := s.auth.CreateToken(r.Context(), int64(uid), req.Name, req.Scopes, expires)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidTokenRequest) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, createTokenResponse{Token: secret, Detail: tokenView(t, now)})
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.accountUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.auth.RevokeToken(r.Context(), int64(uid), id); err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			writeError(w, http.StatusNotFound, "token not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- admin: users ---

type userJSON struct {
	ID          int        `json:"id"`
	Login       string     `json:"login"`
	DisplayName string     `json:"display_name"`
	IsAdmin     bool       `json:"is_admin"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastSeen    time.Time  `json:"last_seen"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.adminUser(w, r); !ok {
		return
	}
	users, err := s.db.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]userJSON, 0, len(users))
	for _, u := range users {
		out = append(out, userJSON{
			ID: u.ID, Login: u.Login, DisplayName: u.DisplayName, IsAdmin: u.IsAdmin,
			DisabledAt: u.DisabledAt, CreatedAt: u.CreatedAt, LastSeen: u.LastSeen,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type updateUserRequest struct {
	IsAdmin  *bool `json:"is_admin"`
	Disabled *bool `json:"disabled"`
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.adminUser(w, r)
	if !ok {
		return
	}
	id64, ok := pathID(w, r)
	if !ok {
		return
	}
	id := int(id64)
	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.IsAdmin != nil {
		if err := s.accounts.SetAdmin(r.Context(), actor, id, *req.IsAdmin); err != nil {
			writeAccountError(w, err)
			return
		}
	}
	if req.Disabled != nil {
		if err := s.accounts.SetDisabled(r.Context(), actor, id, *req.Disabled); err != nil {
			writeAccountError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeAccountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, accounts.ErrSelf), errors.Is(err, accounts.ErrLastAdmin):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
