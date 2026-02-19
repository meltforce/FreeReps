package server

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/claude/freereps/internal/ingest/alpha"
	"github.com/claude/freereps/internal/ingest/hae"
	"github.com/claude/freereps/internal/storage"
	"github.com/go-chi/chi/v5"
)

// Server holds dependencies for HTTP handlers.
type Server struct {
	db       *storage.DB
	hae      *hae.Provider
	alpha    *alpha.Provider
	log      *slog.Logger
	apiKey   string
	router   chi.Router
}

// New creates a new Server with all routes configured.
func New(db *storage.DB, haeProvider *hae.Provider, alphaProvider *alpha.Provider, apiKey string, log *slog.Logger) *Server {
	s := &Server{
		db:     db,
		hae:    haeProvider,
		alpha:  alphaProvider,
		log:    log,
		apiKey: apiKey,
		router: chi.NewRouter(),
	}
	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.router.Use(RequestLogging(s.log))
	s.router.Use(CORS)

	// Ingest endpoints (API key required)
	s.router.Route("/api/v1/ingest", func(r chi.Router) {
		r.Use(APIKeyAuth(s.apiKey))
		r.Post("/", s.handleHAEIngest)
		r.Post("/alpha", s.handleAlphaIngest)
	})

	// Dashboard API endpoints (no auth — tsnet handles access)
	s.router.Get("/api/v1/metrics/latest", s.handleLatestMetrics)
	s.router.Get("/api/v1/metrics", s.handleQueryMetrics)
	s.router.Get("/api/v1/sleep", s.handleQuerySleep)
	s.router.Get("/api/v1/workouts", s.handleQueryWorkouts)
	s.router.Get("/api/v1/workouts/{id}", s.handleGetWorkout)
	s.router.Get("/api/v1/metrics/stats", s.handleMetricStats)
	s.router.Get("/api/v1/timeseries", s.handleTimeSeries)
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
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// Fallback to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
