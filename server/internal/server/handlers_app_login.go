package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/meltforce/meltkit/pkg/auth"
)

// App login hands a personal access token to the iOS app after a browser
// login, as an authorization code flow with PKCE (RFC 7636):
//
//  1. The app opens /app-login/authorize with a code challenge in a system
//     browser sheet. Without a session the browser goes through /login.
//  2. The server redirects to appLoginRedirect with a single-use code.
//  3. The app posts code and verifier to /api/v1/app-login/token and
//     receives a token with scope api.
//
// The redirect target is fixed, so the code reaches only an app that handles
// the freereps scheme on the user's own device, and the challenge binds it to
// the app instance that started the flow. The token is minted at the
// exchange: an authorize request that nobody redeems creates nothing.
const (
	appLoginRedirect = "freereps://app-login"
	appLoginCodeTTL  = 5 * time.Minute
	// maxPendingAppLogins bounds the memory an authenticated user can occupy
	// by starting flows without redeeming them.
	maxPendingAppLogins = 1000
)

type appLogin struct {
	userID     int
	challenge  string
	deviceName string
	expires    time.Time
}

// appLoginCodes holds the pending codes in memory. A restart drops them; the
// app then starts the flow again.
type appLoginCodes struct {
	mu      sync.Mutex
	pending map[[sha256.Size]byte]appLogin
}

func (c *appLoginCodes) add(l appLogin, now time.Time) (string, bool) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", false
	}
	code := base64.RawURLEncoding.EncodeToString(raw)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending == nil {
		c.pending = make(map[[sha256.Size]byte]appLogin)
	}
	for k, v := range c.pending {
		if !v.expires.After(now) {
			delete(c.pending, k)
		}
	}
	if len(c.pending) >= maxPendingAppLogins {
		return "", false
	}
	c.pending[sha256.Sum256([]byte(code))] = l
	return code, true
}

// take removes the code, so a second exchange of the same code fails even
// when the first one failed on its verifier.
func (c *appLoginCodes) take(code string, now time.Time) (appLogin, bool) {
	k := sha256.Sum256([]byte(code))
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.pending[k]
	delete(c.pending, k)
	if !ok || !l.expires.After(now) {
		return appLogin{}, false
	}
	return l, true
}

// mountAppLogin registers the three routes. The authorize route is a browser
// navigation: without a session it answers a redirect to /login, not 401.
func (s *Server) mountAppLogin(a *auth.Auth) {
	toLogin := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		})
	}
	session := a.Middleware(auth.WithScope(auth.ScopeAPI), auth.WithFallback(toLogin))
	fromUser := userFromAuth(s.users, s.log)
	s.router.With(session, fromUser).Get("/app-login/authorize", s.handleAppLoginAuthorize)
	s.router.Post("/api/v1/app-login/token", s.handleAppLoginToken)
	token := a.Middleware(auth.WithScope(auth.ScopeAPI))
	s.router.With(token, fromUser).Delete("/api/v1/app-login/token", s.handleAppLoginRevoke)
}

func appLoginRedirectTo(w http.ResponseWriter, r *http.Request, params url.Values) {
	http.Redirect(w, r, appLoginRedirect+"?"+params.Encode(), http.StatusFound)
}

func (s *Server) handleAppLoginAuthorize(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	state := q.Get("state")
	challenge := q.Get("code_challenge")
	// A request that does not come from the app gets no redirect into it.
	if state == "" || len(state) > 256 || len(challenge) != 43 || q.Get("code_challenge_method") != "S256" {
		http.Error(w, "invalid app login request", http.StatusBadRequest)
		return
	}
	if _, err := base64.RawURLEncoding.DecodeString(challenge); err != nil {
		http.Error(w, "invalid app login request", http.StatusBadRequest)
		return
	}
	// A token must not mint tokens (handleCreateToken); the flow needs the
	// browser session.
	if userInfoFromContext(r).AuthMethod != MethodSession {
		appLoginRedirectTo(w, r, url.Values{"error": {"session_required"}, "state": {state}})
		return
	}
	now := time.Now()
	code, ok := s.appLogins.add(appLogin{
		userID:     uid,
		challenge:  challenge,
		deviceName: q.Get("device_name"),
		expires:    now.Add(appLoginCodeTTL),
	}, now)
	if !ok {
		appLoginRedirectTo(w, r, url.Values{"error": {"temporarily_unavailable"}, "state": {state}})
		return
	}
	appLoginRedirectTo(w, r, url.Values{"code": {code}, "state": {state}})
}

type appLoginTokenRequest struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
}

type appLoginTokenResponse struct {
	Token       string `json:"token"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
}

func (s *Server) handleAppLoginToken(w http.ResponseWriter, r *http.Request) {
	var req appLoginTokenRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	l, ok := s.appLogins.take(req.Code, time.Now())
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown or expired code")
		return
	}
	sum := sha256.Sum256([]byte(req.CodeVerifier))
	if subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(l.challenge)) != 1 {
		writeError(w, http.StatusBadRequest, "code verifier does not match")
		return
	}
	u, err := s.users.GetUser(r.Context(), l.userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unknown user")
		return
	}
	if u.Disabled() {
		writeError(w, http.StatusForbidden, "account disabled")
		return
	}
	// No expiry: an expired token stops the background sync without anyone
	// looking at the phone. The token is revoked in Settings → Account or by
	// signing out in the app.
	secret, _, err := s.auth.CreateToken(r.Context(), int64(u.ID), appLoginTokenName(l.deviceName), []string{auth.ScopeAPI}, time.Time{})
	if err != nil {
		s.log.Error("app login: creating token failed", "user_id", u.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "creating token failed")
		return
	}
	s.log.Info("app login: token issued", "user_id", u.ID, "device", l.deviceName)
	writeJSON(w, http.StatusOK, appLoginTokenResponse{Token: secret, Login: u.Login, DisplayName: u.DisplayName})
}

// appLoginTokenName is the name the token carries in Settings → Account.
func appLoginTokenName(device string) string {
	device = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, device))
	if device == "" {
		return "FreeReps iOS"
	}
	if r := []rune(device); len(r) > 60 {
		device = string(r[:60])
	}
	return "FreeReps iOS – " + device
}

// handleAppLoginRevoke revokes the token the request carries: the app's
// sign-out.
func (s *Server) handleAppLoginRevoke(w http.ResponseWriter, r *http.Request) {
	t, ok := auth.TokenFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusBadRequest, "the request carries no token")
		return
	}
	if err := s.auth.RevokeToken(r.Context(), t.UserID, t.ID); err != nil && !errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
