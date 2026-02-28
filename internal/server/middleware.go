package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tailscale.com/client/tailscale/apitype"
)

// whoisClient abstracts Tailscale WhoIs lookups for testing.
type whoisClient interface {
	WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)
}

// userStore abstracts user database operations for testing.
type userStore interface {
	GetOrCreateUser(ctx context.Context, login, displayName string) (int, error)
	GetPrimaryUser(ctx context.Context) (int, string, error)
}

type contextKey int

const (
	userIDKey   contextKey = iota
	userInfoKey            // stores UserInfo alongside userID
)

// UserInfo holds the authenticated user's identity details.
type UserInfo struct {
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
	TailscaleID string `json:"tailscale_id,omitempty"`
	Tailnet     string `json:"tailnet,omitempty"`
}

// userIDFromContext returns the authenticated user's ID from the request context.
// Returns (0, false) if no identity middleware has set a value.
func userIDFromContext(r *http.Request) (int, bool) {
	id, ok := r.Context().Value(userIDKey).(int)
	return id, ok
}

// mustUserID extracts the user ID from the request context and writes a 500
// error if identity middleware has not run. Returns false when the response
// has been written and the caller should return immediately.
func mustUserID(w http.ResponseWriter, r *http.Request) (int, bool) {
	uid, ok := userIDFromContext(r)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no authenticated user in request context"})
		return 0, false
	}
	return uid, true
}

// userInfoFromContext returns the authenticated user's identity from the request context.
func userInfoFromContext(r *http.Request) UserInfo {
	if info, ok := r.Context().Value(userInfoKey).(UserInfo); ok {
		return info
	}
	return UserInfo{Login: "local", DisplayName: "Local Dev User"}
}

// TailscaleIdentity returns middleware that resolves the Tailscale user identity
// from each request and stores the user ID in the request context.
// Tagged devices (e.g. MCP proxies) are resolved to the tailnet owner by looking
// up the primary user from the database. If no real user has logged in yet, tagged
// devices are rejected with 403.
func TailscaleIdentity(lc whoisClient, db userStore, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			whois, err := lc.WhoIs(r.Context(), r.RemoteAddr)
			if err != nil {
				log.Error("tailscale whois failed", "remote", r.RemoteAddr, "error", err)
				http.Error(w, `{"error":"identity lookup failed"}`, http.StatusInternalServerError)
				return
			}

			var login, displayName string

			if whois.Node != nil && whois.Node.IsTagged() {
				// Tagged device (e.g. tsmcp proxy) — resolve to tailnet owner.
				ownerID, ownerLogin, err := db.GetPrimaryUser(r.Context())
				if err != nil {
					log.Warn("tagged device access denied: no registered user yet",
						"node", whois.Node.ComputedName)
					http.Error(w, `{"error":"access denied: no registered user yet; log in from a personal device first"}`, http.StatusForbidden)
					return
				}
				login = ownerLogin
				displayName = ownerLogin

				log.Info("tagged device resolved to owner",
					"node", whois.Node.ComputedName,
					"owner_login", ownerLogin,
					"owner_id", ownerID,
				)
			} else {
				// Personal device — use WhoIs identity.
				login = whois.UserProfile.LoginName
				if login == "" {
					http.Error(w, `{"error":"access denied: personal Tailscale login required"}`, http.StatusForbidden)
					return
				}
				displayName = whois.UserProfile.DisplayName
			}

			nodeName := ""
			if whois.Node != nil {
				nodeName = whois.Node.ComputedName
			}

			userID, err := db.GetOrCreateUser(r.Context(), login, displayName)
			if err != nil {
				log.Error("user resolution failed", "login", login, "error", err)
				http.Error(w, `{"error":"user resolution failed"}`, http.StatusInternalServerError)
				return
			}

			log.Info("request authenticated",
				"tailscale_user", login,
				"tailscale_node", nodeName,
				"user_id", userID,
			)

			// Extract hostname and tailnet from FQDN (e.g. "linus-macbook.tailnet-name.ts.net.")
			var tsID, tailnet string
			if whois.Node != nil {
				parts := strings.Split(strings.TrimSuffix(whois.Node.Name, "."), ".")
				if len(parts) >= 3 {
					tsID = parts[0]                           // just hostname
					tailnet = strings.Join(parts[1:], ".")    // tailnet-name.ts.net
				} else {
					tsID = whois.Node.Name
				}
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, userInfoKey, UserInfo{
				Login:       login,
				DisplayName: displayName,
				TailscaleID: tsID,
				Tailnet:     tailnet,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DevIdentity returns middleware that sets user_id=1 for all requests.
// Used when Tailscale is disabled (local development).
func DevIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), userIDKey, 1)
		ctx = context.WithValue(ctx, userInfoKey, UserInfo{Login: "local", DisplayName: "Local Dev User"})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestLogging returns middleware that logs each request.
func RequestLogging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration", time.Since(start).String(),
			)
		})
	}
}

// CORS adds permissive CORS headers for local development.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusWriter wraps ResponseWriter to capture the status code.
// It also implements http.Flusher so SSE streaming works through the logging middleware.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
