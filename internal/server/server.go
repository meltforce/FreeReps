package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/claude/freereps/internal/ingest/alpha"
	"github.com/claude/freereps/internal/ingest/hae"
	freerepsmcp "github.com/claude/freereps/internal/mcp"
	"github.com/claude/freereps/internal/storage"
	"github.com/go-chi/chi/v5"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"tailscale.com/client/local"
)

// Server holds dependencies for HTTP handlers.
type Server struct {
	db     *storage.DB
	hae    *hae.Provider
	alpha  *alpha.Provider
	log    *slog.Logger
	lc     *local.Client
	router chi.Router

	// HAE TCP import state (only one import at a time)
	importMu     sync.Mutex
	activeImport *haeImportState
}

// New creates a new Server with all routes configured.
func New(db *storage.DB, haeProvider *hae.Provider, alphaProvider *alpha.Provider, log *slog.Logger) *Server {
	s := &Server{
		db:     db,
		hae:    haeProvider,
		alpha:  alphaProvider,
		log:    log,
		router: chi.NewRouter(),
	}
	s.routes()
	return s
}

// SetTailscale configures the Tailscale LocalClient for identity resolution.
// Must be called before the server starts handling requests.
// When set, all requests are authenticated via Tailscale identity.
// When nil (default), all requests use user_id=1 (dev mode).
func (s *Server) SetTailscale(lc *local.Client) {
	s.lc = lc
}

// SetMCP mounts an MCP Streamable HTTP server at /mcp.
// The HTTP context function injects the authenticated user ID from the HTTP
// request into the MCP handler context, giving tools automatic user scoping.
// MCP routes use the same Tailscale identity middleware as all other endpoints.
func (s *Server) SetMCP(mcpSrv *mcpserver.MCPServer) {
	httpServer := mcpserver.NewStreamableHTTPServer(mcpSrv,
		mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			uid, _ := userIDFromContext(r)
			return freerepsmcp.WithUserID(ctx, uid)
		}),
	)
	identity := s.identityMiddleware()
	s.router.Handle("/mcp", identity(httpServer))
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// identityMiddleware returns middleware that resolves user identity via Tailscale
// (production) or assigns user_id=1 (dev mode). Checks s.lc at request time so
// SetTailscale can be called after New().
func (s *Server) identityMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.lc != nil {
				TailscaleIdentity(s.lc, s.db, s.log)(next).ServeHTTP(w, r)
			} else {
				DevIdentity(next).ServeHTTP(w, r)
			}
		})
	}
}

func (s *Server) routes() {
	s.router.Use(RequestLogging(s.log))
	s.router.Use(CORS)

	// All routes require identity (Tailscale or dev fallback).
	s.router.Group(func(r chi.Router) {
		r.Use(s.identityMiddleware())

		// Ingest endpoints
		r.Route("/api/v1/ingest", func(r chi.Router) {
			r.Post("/", s.handleHAEIngest)
			r.Post("/alpha", s.handleAlphaIngest)
		})

		// User identity
		r.Get("/api/v1/me", s.handleMe)

		// Dashboard API endpoints
		r.Get("/api/v1/metrics/latest", s.handleLatestMetrics)
		r.Get("/api/v1/metrics", s.handleQueryMetrics)
		r.Get("/api/v1/sleep", s.handleQuerySleep)
		r.Get("/api/v1/workouts", s.handleQueryWorkouts)
		r.Get("/api/v1/workouts/{id}", s.handleGetWorkout)
		r.Get("/api/v1/workouts/{id}/sets", s.handleWorkoutSets)
		r.Get("/api/v1/metrics/stats", s.handleMetricStats)
		r.Get("/api/v1/timeseries", s.handleTimeSeries)
		r.Get("/api/v1/correlation", s.handleCorrelation)
		r.Get("/api/v1/allowlist", s.handleAllowlist)

		// Settings / admin endpoints
		r.Get("/api/v1/stats", s.handleStats)
		r.Get("/api/v1/import-logs", s.handleImportLogs)

		// HAE TCP import
		r.Post("/api/v1/import/hae-tcp/check", s.handleCheckHAE)
		r.Post("/api/v1/import/hae-tcp", s.handleStartHAEImport)
		r.Delete("/api/v1/import/hae-tcp", s.handleCancelHAEImport)
		r.Get("/api/v1/import/hae-tcp/status", s.handleHAEImportStatus)
		r.Get("/api/v1/import/hae-tcp/events", s.handleHAEImportEvents)
	})
}

// SetFrontend mounts the embedded SPA filesystem.
// Unmatched routes serve index.html for client-side routing.
// Hashed assets get long cache; index.html is never cached.
func (s *Server) SetFrontend(webFS fs.FS) {
	fileServer := http.FileServerFS(webFS)

	s.router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[1:] // strip leading /

		// API and well-known paths must not fall through to the SPA.
		if strings.HasPrefix(path, "api/") || strings.HasPrefix(path, ".well-known/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the exact file first
		f, err := webFS.Open(path)
		if err == nil {
			_ = f.Close()
			// Vite hashed assets (assets/*) are immutable — cache forever.
			// Everything else (index.html) must not be cached.
			if len(path) > 7 && path[:7] == "assets/" {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		// Fallback to index.html for SPA routing
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
