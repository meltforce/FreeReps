package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/claude/freereps/internal/storage"
	"github.com/meltforce/meltkit/pkg/auth"
	mkmw "github.com/meltforce/meltkit/pkg/middleware"
)

// userLoader is the storage userFromAuth reads.
type userLoader interface {
	GetUser(ctx context.Context, id int) (*storage.User, error)
}

// userFromAuth runs behind meltkit's auth middleware and stores the user in
// the request context the handlers read. The user ID comes from the session
// or token, or from the WhoIs fallback when that served the request. The user
// row is loaded on every path, because a disabled user must be refused also
// with a token that survived a failed revocation and over WhoIs.
func userFromAuth(users userLoader, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info := UserInfo{}
			uid, viaWhois := userIDFromContext(r)
			if viaWhois {
				info = userInfoFromContext(r)
			} else {
				var ok bool
				uid, ok = mkmw.UserIDFromContext(r)
				if !ok || uid <= 0 {
					writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
					return
				}
			}
			u, err := users.GetUser(r.Context(), uid)
			if errors.Is(err, storage.ErrUserNotFound) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unknown user"})
				return
			}
			if err != nil {
				log.Error("user lookup failed", "user_id", uid, "error", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
				return
			}
			if u.Disabled() {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "account disabled"})
				return
			}
			if !viaWhois {
				// A token request carries no login or display name; both come
				// from the user row.
				info.Login = u.Login
				info.DisplayName = u.DisplayName
				info.AuthMethod = MethodSession
				if _, ok := auth.TokenFromContext(r.Context()); ok {
					info.AuthMethod = MethodToken
				}
			}
			info.IsAdmin = u.IsAdmin
			ctx := context.WithValue(r.Context(), userIDKey, u.ID)
			ctx = context.WithValue(ctx, userInfoKey, info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
