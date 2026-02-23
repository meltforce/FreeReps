package mcp

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// defaultTimeRange returns start/end defaulting to the last 7 days.
func defaultTimeRange(startStr, endStr string) (time.Time, time.Time, error) {
	var start, end time.Time
	var err error

	if endStr != "" {
		end, err = parseFlexTime(endStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	} else {
		end = time.Now()
	}

	if startStr != "" {
		start, err = parseFlexTime(startStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	} else {
		start = end.AddDate(0, 0, -7)
	}

	return start, end, nil
}

func parseFlexTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}
	return time.Time{}, err
}

// --- Tool definitions ---

var toolGetHealthMetrics = mcp.NewTool("get_health_metrics",
	mcp.WithDescription("Retrieve time-bucketed health metrics. Returns aggregated data points (avg/min/max/count) per time bucket."),
	mcp.WithString("metric", mcp.Required(), mcp.Description("Metric name (e.g. heart_rate, resting_heart_rate, heart_rate_variability, weight_body_mass)")),
	mcp.WithString("start", mcp.Description("Start date (ISO 8601 or YYYY-MM-DD). Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description("End date (ISO 8601 or YYYY-MM-DD). Defaults to now.")),
	mcp.WithString("bucket", mcp.Description("Time bucket size (e.g. '1 hour', '1 day', '1 week', '1 month'). Defaults to '1 day'."), mcp.Enum("1 hour", "1 day", "1 week", "1 month")),
)

var toolGetMetricStats = mcp.NewTool("get_metric_stats",
	mcp.WithDescription("Get aggregate statistics (avg, min, max, stddev, count) for a metric over a time range."),
	mcp.WithString("metric", mcp.Required(), mcp.Description("Metric name")),
	mcp.WithString("start", mcp.Description("Start date. Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description("End date. Defaults to now.")),
)

var toolGetCorrelation = mcp.NewTool("get_correlation",
	mcp.WithDescription("Compute Pearson correlation between two health metrics. Returns time-aligned data points and the correlation coefficient."),
	mcp.WithString("x", mcp.Required(), mcp.Description("X-axis metric name")),
	mcp.WithString("y", mcp.Required(), mcp.Description("Y-axis metric name")),
	mcp.WithString("start", mcp.Description("Start date. Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description("End date. Defaults to now.")),
	mcp.WithString("bucket", mcp.Description("Time bucket for alignment. Defaults to '1 day'."), mcp.Enum("1 hour", "1 day", "1 week", "1 month")),
)

var toolGetSleepData = mcp.NewTool("get_sleep_data",
	mcp.WithDescription("Retrieve sleep sessions and individual sleep stages. Sessions include total sleep, stage durations (core/deep/REM), and timing. Stages are individual segments with start/end times."),
	mcp.WithString("start", mcp.Description("Start date. Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description("End date. Defaults to now.")),
)

var toolGetWorkouts = mcp.NewTool("get_workouts",
	mcp.WithDescription("Query workouts with optional type filter. Returns workout summaries including duration, energy, distance, and heart rate data."),
	mcp.WithString("start", mcp.Description("Start date. Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description("End date. Defaults to now.")),
	mcp.WithString("type", mcp.Description("Filter by workout type (e.g. 'Traditional Strength Training', 'Running')")),
)

var toolGetWorkoutSets = mcp.NewTool("get_workout_sets",
	mcp.WithDescription("Query strength training set data (Alpha Progression). Returns exercise details including weight, reps, and RIR for each set."),
	mcp.WithString("start", mcp.Description("Start date. Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description("End date. Defaults to now.")),
	mcp.WithString("exercise", mcp.Description("Filter by exercise name (partial match, e.g. 'bench press')")),
)

var toolListAvailableMetrics = mcp.NewTool("list_available_metrics",
	mcp.WithDescription("List all available health metrics with their categories and enabled status."),
)

var toolGetTrainingSummary = mcp.NewTool("get_training_summary",
	mcp.WithDescription("Monthly/weekly aggregated workout and strength training volume. Returns workout counts, duration, calories by type, plus strength set/rep/tonnage totals per period."),
	mcp.WithString("start", mcp.Description("Start date. Defaults to 6 months ago.")),
	mcp.WithString("end", mcp.Description("End date. Defaults to now.")),
	mcp.WithString("bucket", mcp.Description("Aggregation period. Defaults to '1 month'."), mcp.Enum("1 week", "1 month")),
)

var toolGetTrainingIntensity = mcp.NewTool("get_training_intensity",
	mcp.WithDescription("RIR distribution, failure rate, per-exercise stats, and optional exercise progression. Returns intensity analysis for strength training."),
	mcp.WithString("start", mcp.Description("Start date. Defaults to 90 days ago.")),
	mcp.WithString("end", mcp.Description("End date. Defaults to now.")),
	mcp.WithString("exercise", mcp.Description("Filter by exercise name (partial match). When set, includes session-by-session progression.")),
)

var toolGetSleepSummary = mcp.NewTool("get_sleep_summary",
	mcp.WithDescription("Aggregated sleep stats per period: duration, stage percentages, efficiency, bedtime/waketime consistency."),
	mcp.WithString("start", mcp.Description("Start date. Defaults to 90 days ago.")),
	mcp.WithString("end", mcp.Description("End date. Defaults to now.")),
	mcp.WithString("bucket", mcp.Description("Aggregation period. Defaults to '1 month'."), mcp.Enum("1 week", "1 month")),
)

var toolComparePeriods = mcp.NewTool("compare_periods",
	mcp.WithDescription("Compare a metric's statistics between two time periods (e.g. this week vs last week)."),
	mcp.WithString("metric", mcp.Required(), mcp.Description("Metric name")),
	mcp.WithString("period_a_start", mcp.Required(), mcp.Description("Period A start date")),
	mcp.WithString("period_a_end", mcp.Required(), mcp.Description("Period A end date")),
	mcp.WithString("period_b_start", mcp.Required(), mcp.Description("Period B start date")),
	mcp.WithString("period_b_end", mcp.Required(), mcp.Description("Period B end date")),
)

// --- Tool handlers ---

func (h *handlers) getHealthMetrics(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	metric, err := req.RequireString("metric")
	if err != nil {
		return mcp.NewToolResultError("metric parameter is required"), nil
	}

	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	bucket := req.GetString("bucket", "1 day")
	uid := UserIDFromContext(ctx)

	points, err := h.ds.GetTimeSeries(ctx, metric, start, end, bucket, uid)
	if err != nil {
		h.log.Error("mcp get_health_metrics", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(points)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getMetricStats(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	metric, err := req.RequireString("metric")
	if err != nil {
		return mcp.NewToolResultError("metric parameter is required"), nil
	}

	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)
	stats, err := h.ds.GetMetricStats(ctx, metric, start, end, uid)
	if err != nil {
		h.log.Error("mcp get_metric_stats", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(stats)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getCorrelation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	xMetric, err := req.RequireString("x")
	if err != nil {
		return mcp.NewToolResultError("x parameter is required"), nil
	}
	yMetric, err := req.RequireString("y")
	if err != nil {
		return mcp.NewToolResultError("y parameter is required"), nil
	}

	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	bucket := req.GetString("bucket", "1 day")
	uid := UserIDFromContext(ctx)

	corr, err := h.ds.GetCorrelation(ctx, xMetric, yMetric, start, end, bucket, uid)
	if err != nil {
		h.log.Error("mcp get_correlation", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(corr)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getSleepData(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)

	sessions, err := h.ds.QuerySleepSessions(ctx, start, end, uid)
	if err != nil {
		h.log.Error("mcp get_sleep_data sessions", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	stages, err := h.ds.QuerySleepStages(ctx, start, end, uid)
	if err != nil {
		h.log.Error("mcp get_sleep_data stages", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{
		"sessions": sessions,
		"stages":   stages,
	})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getWorkouts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	nameFilter := req.GetString("type", "")
	uid := UserIDFromContext(ctx)

	workouts, err := h.ds.QueryWorkouts(ctx, start, end, uid, nameFilter)
	if err != nil {
		h.log.Error("mcp get_workouts", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(workouts)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getWorkoutSets(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)
	exerciseFilter := req.GetString("exercise", "")

	sets, err := h.ds.QueryWorkoutSets(ctx, start, end, uid, exerciseFilter)
	if err != nil {
		h.log.Error("mcp get_workout_sets", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(sets)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) listAvailableMetrics(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	metrics, err := h.ds.GetAllowedMetrics(ctx)
	if err != nil {
		h.log.Error("mcp list_available_metrics", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(metrics)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) comparePeriods(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	metric, err := req.RequireString("metric")
	if err != nil {
		return mcp.NewToolResultError("metric parameter is required"), nil
	}

	aStartStr, err := req.RequireString("period_a_start")
	if err != nil {
		return mcp.NewToolResultError("period_a_start is required"), nil
	}
	aEndStr, err := req.RequireString("period_a_end")
	if err != nil {
		return mcp.NewToolResultError("period_a_end is required"), nil
	}
	bStartStr, err := req.RequireString("period_b_start")
	if err != nil {
		return mcp.NewToolResultError("period_b_start is required"), nil
	}
	bEndStr, err := req.RequireString("period_b_end")
	if err != nil {
		return mcp.NewToolResultError("period_b_end is required"), nil
	}

	aStart, err := parseFlexTime(aStartStr)
	if err != nil {
		return mcp.NewToolResultError("invalid period_a_start: " + err.Error()), nil
	}
	aEnd, err := parseFlexTime(aEndStr)
	if err != nil {
		return mcp.NewToolResultError("invalid period_a_end: " + err.Error()), nil
	}
	bStart, err := parseFlexTime(bStartStr)
	if err != nil {
		return mcp.NewToolResultError("invalid period_b_start: " + err.Error()), nil
	}
	bEnd, err := parseFlexTime(bEndStr)
	if err != nil {
		return mcp.NewToolResultError("invalid period_b_end: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)

	statsA, err := h.ds.GetMetricStats(ctx, metric, aStart, aEnd, uid)
	if err != nil {
		h.log.Error("mcp compare_periods A", "error", err)
		return mcp.NewToolResultError("query failed for period A: " + err.Error()), nil
	}

	statsB, err := h.ds.GetMetricStats(ctx, metric, bStart, bEnd, uid)
	if err != nil {
		h.log.Error("mcp compare_periods B", "error", err)
		return mcp.NewToolResultError("query failed for period B: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{
		"metric":   metric,
		"period_a": statsA,
		"period_b": statsB,
	})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getTrainingSummary(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	endStr := req.GetString("end", "")
	startStr := req.GetString("start", "")

	var start, end time.Time
	var err error

	if endStr != "" {
		end, err = parseFlexTime(endStr)
		if err != nil {
			return mcp.NewToolResultError("invalid end date: " + err.Error()), nil
		}
	} else {
		end = time.Now()
	}

	if startStr != "" {
		start, err = parseFlexTime(startStr)
		if err != nil {
			return mcp.NewToolResultError("invalid start date: " + err.Error()), nil
		}
	} else {
		start = end.AddDate(0, -6, 0)
	}

	bucket := req.GetString("bucket", "1 month")
	uid := UserIDFromContext(ctx)

	summary, err := h.ds.GetTrainingSummary(ctx, start, end, bucket, uid)
	if err != nil {
		h.log.Error("mcp get_training_summary", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(summary)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getTrainingIntensity(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	endStr := req.GetString("end", "")
	startStr := req.GetString("start", "")

	var start, end time.Time
	var err error

	if endStr != "" {
		end, err = parseFlexTime(endStr)
		if err != nil {
			return mcp.NewToolResultError("invalid end date: " + err.Error()), nil
		}
	} else {
		end = time.Now()
	}

	if startStr != "" {
		start, err = parseFlexTime(startStr)
		if err != nil {
			return mcp.NewToolResultError("invalid start date: " + err.Error()), nil
		}
	} else {
		start = end.AddDate(0, 0, -90)
	}

	uid := UserIDFromContext(ctx)
	exerciseFilter := req.GetString("exercise", "")

	intensity, err := h.ds.GetTrainingIntensity(ctx, start, end, uid, exerciseFilter)
	if err != nil {
		h.log.Error("mcp get_training_intensity", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(intensity)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getSleepSummary(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	endStr := req.GetString("end", "")
	startStr := req.GetString("start", "")

	var start, end time.Time
	var err error

	if endStr != "" {
		end, err = parseFlexTime(endStr)
		if err != nil {
			return mcp.NewToolResultError("invalid end date: " + err.Error()), nil
		}
	} else {
		end = time.Now()
	}

	if startStr != "" {
		start, err = parseFlexTime(startStr)
		if err != nil {
			return mcp.NewToolResultError("invalid start date: " + err.Error()), nil
		}
	} else {
		start = end.AddDate(0, 0, -90)
	}

	bucket := req.GetString("bucket", "1 month")
	uid := UserIDFromContext(ctx)

	summary, err := h.ds.GetSleepSummary(ctx, start, end, bucket, uid)
	if err != nil {
		h.log.Error("mcp get_sleep_summary", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(summary)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}
