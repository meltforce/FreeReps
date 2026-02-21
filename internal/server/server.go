package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"

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

// SetMCP mounts an MCP SSE server at /mcp/.
// The SSE context function injects the authenticated user ID from the HTTP
// request into the MCP handler context, giving tools automatic user scoping.
func (s *Server) SetMCP(mcpSrv *mcpserver.MCPServer) {
	sseServer := mcpserver.NewSSEServer(mcpSrv,
		mcpserver.WithDynamicBasePath(func(r *http.Request, sessionID string) string {
			return "/mcp"
		}),
		mcpserver.WithSSEContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			uid, _ := userIDFromContext(r)
			return freerepsmcp.WithUserID(ctx, uid)
		}),
	)
	s.router.Handle("/mcp/sse", sseServer.SSEHandler())
	s.router.Handle("/mcp/message", sseServer.MessageHandler())
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.router.Use(RequestLogging(s.log))
	s.router.Use(CORS)

	// Identity middleware: tsnet-based or dev fallback.
	// Applied after routes() via a lazy middleware that checks s.lc at request time.
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.lc != nil {
				TailscaleIdentity(s.lc, s.db, s.log)(next).ServeHTTP(w, r)
			} else {
				DevIdentity(next).ServeHTTP(w, r)
			}
		})
	})

	// Ingest endpoints
	s.router.Route("/api/v1/ingest", func(r chi.Router) {
		r.Post("/", s.handleHAEIngest)
		r.Post("/alpha", s.handleAlphaIngest)
	})

	// User identity
	s.router.Get("/api/v1/me", s.handleMe)

	// Dashboard API endpoints
	s.router.Get("/api/v1/metrics/latest", s.handleLatestMetrics)
	s.router.Get("/api/v1/metrics", s.handleQueryMetrics)
	s.router.Get("/api/v1/sleep", s.handleQuerySleep)
	s.router.Get("/api/v1/workouts", s.handleQueryWorkouts)
	s.router.Get("/api/v1/workouts/{id}", s.handleGetWorkout)
	s.router.Get("/api/v1/workouts/{id}/sets", s.handleWorkoutSets)
	s.router.Get("/api/v1/metrics/stats", s.handleMetricStats)
	s.router.Get("/api/v1/timeseries", s.handleTimeSeries)
	s.router.Get("/api/v1/correlation", s.handleCorrelation)
	s.router.Get("/api/v1/allowlist", s.handleAllowlist)
}

// SetFrontend mounts the embedded SPA filesystem.
// Unmatched routes serve index.html for client-side routing.
func (s *Server) SetFrontend(webFS fs.FS) {
	fileServer := http.FileServerFS(webFS)

	s.router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the exact file first
		f, err := webFS.Open(r.URL.Path[1:]) // strip leading /
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// Fallback to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
