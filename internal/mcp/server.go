package mcp

import (
	"context"
	"log/slog"

	"github.com/claude/freereps/internal/storage"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type contextKey int

const userIDKey contextKey = iota

// UserIDFromContext extracts the user ID injected by the transport layer.
func UserIDFromContext(ctx context.Context) int {
	if id, ok := ctx.Value(userIDKey).(int); ok {
		return id
	}
	return 1
}

// WithUserID returns a context with the given user ID.
func WithUserID(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// New creates an MCP server with all tools and resources registered.
func New(ds *storage.DB, version string, log *slog.Logger) *server.MCPServer {
	s := server.NewMCPServer("FreeReps", version,
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithInstructions("FreeReps health data server. Query health metrics, workouts, sleep data, and correlations. All data is scoped to the authenticated user."),
	)

	h := &handlers{ds: ds, log: log}

	// Tools
	s.AddTools(
		server.ServerTool{Tool: toolGetHealthMetrics, Handler: h.getHealthMetrics},
		server.ServerTool{Tool: toolGetMetricStats, Handler: h.getMetricStats},
		server.ServerTool{Tool: toolGetCorrelation, Handler: h.getCorrelation},
		server.ServerTool{Tool: toolGetSleepData, Handler: h.getSleepData},
		server.ServerTool{Tool: toolGetWorkouts, Handler: h.getWorkouts},
		server.ServerTool{Tool: toolGetWorkoutSets, Handler: h.getWorkoutSets},
		server.ServerTool{Tool: toolListAvailableMetrics, Handler: h.listAvailableMetrics},
		server.ServerTool{Tool: toolComparePeriods, Handler: h.comparePeriods},
		server.ServerTool{Tool: toolGetTrainingSummary, Handler: h.getTrainingSummary},
		server.ServerTool{Tool: toolGetTrainingIntensity, Handler: h.getTrainingIntensity},
		server.ServerTool{Tool: toolGetSleepSummary, Handler: h.getSleepSummary},
	)

	// Resources
	s.AddResources(
		server.ServerResource{Resource: resDailySummary, Handler: h.dailySummary},
		server.ServerResource{Resource: resRecentWorkouts, Handler: h.recentWorkouts},
		server.ServerResource{Resource: resMetricCatalog, Handler: h.metricCatalog},
	)

	return s
}

// handlers holds dependencies for MCP tool/resource handlers.
type handlers struct {
	ds  *storage.DB
	log *slog.Logger
}

// --- Resource definitions ---

var resDailySummary = mcp.NewResource(
	"freereps://daily_summary",
	"Daily Summary",
	mcp.WithResourceDescription("Summary of today's key health metrics, latest sleep session, and daily activity totals"),
	mcp.WithMIMEType("application/json"),
)

var resRecentWorkouts = mcp.NewResource(
	"freereps://recent_workouts",
	"Recent Workouts",
	mcp.WithResourceDescription("Workouts from the last 14 days"),
	mcp.WithMIMEType("application/json"),
)

var resMetricCatalog = mcp.NewResource(
	"freereps://metric_catalog",
	"Metric Catalog",
	mcp.WithResourceDescription("All available health metrics with categories and enabled status"),
	mcp.WithMIMEType("application/json"),
)
