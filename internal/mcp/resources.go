package mcp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

var cumulativeMetrics = []string{"active_energy", "basal_energy_burned", "apple_exercise_time"}

func (h *handlers) dailySummary(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uid := UserIDFromContext(ctx)

	latest, err := h.db.GetLatestMetrics(ctx, uid)
	if err != nil {
		return nil, err
	}

	sums, err := h.db.GetDailySums(ctx, uid, cumulativeMetrics)
	if err != nil {
		h.log.Warn("daily_summary: daily sums failed", "error", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	tomorrow := today.AddDate(0, 0, 1)

	sessions, err := h.db.QuerySleepSessions(ctx, today.AddDate(0, 0, -1), tomorrow, uid)
	if err != nil {
		h.log.Warn("daily_summary: sleep query failed", "error", err)
	}

	workouts, err := h.db.QueryWorkouts(ctx, today, tomorrow, uid, "")
	if err != nil {
		h.log.Warn("daily_summary: workout query failed", "error", err)
	}

	summary := map[string]any{
		"date":           today.Format("2006-01-02"),
		"latest_metrics": latest,
		"daily_sums":     sums,
		"recent_sleep":   sessions,
		"todays_workouts": workouts,
	}

	data, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (h *handlers) recentWorkouts(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uid := UserIDFromContext(ctx)
	end := time.Now()
	start := end.AddDate(0, 0, -14)

	workouts, err := h.db.QueryWorkouts(ctx, start, end, uid, "")
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(workouts)
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (h *handlers) metricCatalog(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	metrics, err := h.db.GetAllowedMetrics(ctx)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}
