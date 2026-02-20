package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/claude/freereps/internal/storage"
	"tailscale.com/client/local"
)

type contextKey int

const (
	userIDKey   contextKey = iota
	userInfoKey            // stores UserInfo alongside userID
)

// UserInfo holds the authenticated user's identity details.
type UserInfo struct {
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
}

// userIDFromContext returns the authenticated user's ID from the request context.
// Returns 1 (local dev user) if no identity middleware is active.
func userIDFromContext(r *http.Request) int {
	if id, ok := r.Context().Value(userIDKey).(int); ok {
		return id
	}
	return 1
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
// Tagged devices (no personal owner) are rejected with 403.
func TailscaleIdentity(lc *local.Client, db *storage.DB, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			whois, err := lc.WhoIs(r.Context(), r.RemoteAddr)
			if err != nil {
				log.Error("tailscale whois failed", "remote", r.RemoteAddr, "error", err)
				http.Error(w, `{"error":"identity lookup failed"}`, http.StatusInternalServerError)
				return
			}

			if whois.Node != nil && whois.Node.IsTagged() {
				http.Error(w, `{"error":"access denied: personal Tailscale login required"}`, http.StatusForbidden)
				return
			}

			login := whois.UserProfile.LoginName
			if login == "" {
				http.Error(w, `{"error":"access denied: personal Tailscale login required"}`, http.StatusForbidden)
				return
			}

			displayName := whois.UserProfile.DisplayName
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

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, userInfoKey, UserInfo{Login: login, DisplayName: displayName})
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
