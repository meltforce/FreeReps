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

// Shared parameter descriptions. The storage queries bound time as
// start <= t < end, and parseFlexTime reads a bare date as 00:00 UTC.
const (
	descStart  = "Start, inclusive: RFC 3339 timestamp with offset, or YYYY-MM-DD read as 00:00 UTC."
	descEnd    = "End, exclusive: RFC 3339 timestamp with offset, or YYYY-MM-DD read as 00:00 UTC. To include a whole day, pass the following date."
	descMetric = "Metric name as listed by list_available_metrics"
)

var toolGetHealthMetrics = mcp.NewTool("get_health_metrics",
	mcp.WithDescription("Retrieve one metric as a time series, one data point (avg/min/max/count) per bucket. For a cumulative metric (is_cumulative in list_available_metrics, e.g. step count or active energy) the avg field holds the bucket total, not a mean. Values are in the metric's stored unit. For a single figure over a range use get_metric_stats; to compare two ranges use compare_periods."),
	mcp.WithString("metric", mcp.Required(), mcp.Description(descMetric + " (e.g. heart_rate, resting_heart_rate, heart_rate_variability, weight_body_mass).")),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("bucket", mcp.Description("Time bucket size (e.g. '1 hour', '1 day', '1 week', '1 month'). Defaults to '1 day'."), mcp.Enum("1 hour", "1 day", "1 week", "1 month")),
)

var toolGetMetricStats = mcp.NewTool("get_metric_stats",
	mcp.WithDescription("Get aggregate statistics (avg, min, max, stddev, count) for one metric over a time range. For a cumulative metric (is_cumulative in list_available_metrics) the avg field holds the range total, not a mean, and min/max/stddev are over individual samples. Use get_health_metrics for the course over time."),
	mcp.WithString("metric", mcp.Required(), mcp.Description(descMetric + ".")),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
)

var toolGetCorrelation = mcp.NewTool("get_correlation",
	mcp.WithDescription("Compute the Pearson correlation between two metrics. Both are aggregated per bucket — summed for a cumulative metric, averaged otherwise — and only buckets holding both metrics are paired. Returns the paired points, pearson_r (null when it cannot be computed) and the pair count."),
	mcp.WithString("x", mcp.Required(), mcp.Description(descMetric + ", plotted on the x axis.")),
	mcp.WithString("y", mcp.Required(), mcp.Description(descMetric + ", plotted on the y axis.")),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("bucket", mcp.Description("Time bucket for alignment. Defaults to '1 day'."), mcp.Enum("1 hour", "1 day", "1 week", "1 month")),
)

var toolGetSleepData = mcp.NewTool("get_sleep_data",
	mcp.WithDescription("Retrieve individual sleep sessions and sleep stages. Sessions include total sleep, stage durations (core/deep/REM), and timing. Stages are individual segments with start/end times. For averages over weeks or months use get_sleep_summary."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
)

var toolGetWorkouts = mcp.NewTool("get_workouts",
	mcp.WithDescription("Query workouts with optional type filter. Returns workout summaries including duration, energy, distance, and heart rate data."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("type", mcp.Description("Filter by workout type (e.g. 'Traditional Strength Training', 'Running')")),
)

var toolGetWorkoutSets = mcp.NewTool("get_workout_sets",
	mcp.WithDescription("FreeReps database: individual strength training sets. Returns exercise, muscle group, weight, reps, and the effort rating on the scale its source recorded — RIR for Alpha Progression, RPE for Hevy."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("exercise", mcp.Description("Filter by exercise name (partial match, e.g. 'bench press')")),
)

var toolListAvailableMetrics = mcp.NewTool("list_available_metrics",
	mcp.WithDescription("List every health metric with its category, enabled status, display_unit, display_multiplier and is_cumulative. The other tools return stored values: multiply by display_multiplier to get display_unit (e.g. a stored fraction becomes a percentage), and read is_cumulative to know whether an avg field is a total."),
)

// The strength_* tools are named for their data basis rather than for "training"
// because a Hevy MCP server offers near-identical names against training data
// alone. These read the full FreeReps history, which spans every logging app
// used so far and sits alongside sleep, HRV and readiness.

var toolGetStrengthSummary = mcp.NewTool("get_strength_summary",
	mcp.WithDescription("FreeReps database: monthly/weekly aggregated workout and strength volume across all logging sources. Returns workout counts, duration, calories by type, plus strength set/rep/tonnage totals per period. In the strength block, 'sessions' counts distinct session start times and is the denominator of 'avg_sets_per_session'; 'training_days' counts calendar days on which anything was logged. The two differ only on a day holding more than one session."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 6 months ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("bucket", mcp.Description("Aggregation period. Defaults to '1 month'."), mcp.Enum("1 week", "1 month")),
)

var toolGetStrengthIntensity = mcp.NewTool("get_strength_intensity",
	mcp.WithDescription("FreeReps database: effort distribution in reps-in-reserve bands, failure rate, per-exercise stats, and optional per-session progression. Covers sets logged as RIR and as RPE alike."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 90 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("exercise", mcp.Description("Filter by exercise name (partial match). When set, includes session-by-session progression.")),
)

var toolGetStrengthVolume = mcp.NewTool("get_strength_volume",
	mcp.WithDescription("FreeReps database: sets per muscle group per period, as primary-target sets and as sets weighted with assisting muscles at 0.5. Also reports training frequency per muscle, and how much of each figure rests on approximate exercise mapping or on exercises with no muscle data at all."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 12 weeks ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("bucket", mcp.Description("Aggregation period. Defaults to '1 week'."), mcp.Enum("1 week", "1 month")),
)

var toolGetStrengthE1RM = mcp.NewTool("get_strength_1rm",
	mcp.WithDescription("FreeReps database: estimated one-rep max per exercise per session, using Epley over repetitions plus reps in reserve. Sets without an effort rating are excluded. The estimate loses accuracy above roughly ten effective repetitions."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 6 months ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("exercise", mcp.Description("Filter by exercise name (partial match). Without it, every exercise is returned.")),
)

var toolGetSleepSummary = mcp.NewTool("get_sleep_summary",
	mcp.WithDescription("Aggregated sleep stats per week or month: duration, stage percentages, efficiency, bedtime/waketime consistency. For individual nights use get_sleep_data."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 90 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("bucket", mcp.Description("Aggregation period. Defaults to '1 month'."), mcp.Enum("1 week", "1 month")),
)

var toolComparePeriods = mcp.NewTool("compare_periods",
	mcp.WithDescription("Compare one metric's statistics between two time periods (e.g. this week vs last week). Returns the get_metric_stats figures for each period, so for a cumulative metric avg is the period total."),
	mcp.WithString("metric", mcp.Required(), mcp.Description(descMetric + ".")),
	mcp.WithString("period_a_start", mcp.Required(), mcp.Description(descStart)),
	mcp.WithString("period_a_end", mcp.Required(), mcp.Description(descEnd)),
	mcp.WithString("period_b_start", mcp.Required(), mcp.Description(descStart)),
	mcp.WithString("period_b_end", mcp.Required(), mcp.Description(descEnd)),
)

var toolGetECGRecordings = mcp.NewTool("get_ecg_recordings",
	mcp.WithDescription("Query ECG recordings by date range. Returns id, classification, average heart rate, start date, and source."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
)

var toolGetAudiograms = mcp.NewTool("get_audiograms",
	mcp.WithDescription("Query audiograms by date range. Returns id, sensitivity points, start date, and source."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
)

var toolGetActivitySummaries = mcp.NewTool("get_activity_summaries",
	mcp.WithDescription("Query daily activity summaries by date range. Returns date, active energy, exercise time, stand hours, and their goals."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
)

var toolGetMedications = mcp.NewTool("get_medications",
	mcp.WithDescription("Query medication records by date range. Returns id, name, dosage, log status, start date, and source."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
)

var toolGetVisionPrescriptions = mcp.NewTool("get_vision_prescriptions",
	mcp.WithDescription("Query vision prescriptions by date range. Returns all prescription fields including eye details."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
)

var toolGetStateOfMind = mcp.NewTool("get_state_of_mind",
	mcp.WithDescription("Query state of mind records by date range. Returns id, kind, valence, labels, associations, start date."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
)

var toolGetCategorySamples = mcp.NewTool("get_category_samples",
	mcp.WithDescription("Query category samples by date range and optional type filter. Returns id, type, value, value label, start/end dates."),
	mcp.WithString("start", mcp.Description(descStart + " Defaults to 7 days ago.")),
	mcp.WithString("end", mcp.Description(descEnd + " Defaults to now.")),
	mcp.WithString("type", mcp.Description("Filter by category sample type (e.g. 'sleepAnalysis', 'menstrualFlow')")),
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

	result, err := mcp.NewToolResultJSON(map[string]any{"data": points})
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

	result, err := mcp.NewToolResultJSON(map[string]any{"data": workouts})
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

	result, err := mcp.NewToolResultJSON(map[string]any{"data": sets})
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

	result, err := mcp.NewToolResultJSON(map[string]any{"data": metrics})
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

func (h *handlers) getStrengthSummary(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		h.log.Error("mcp get_strength_summary", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": summary})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getStrengthIntensity(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		h.log.Error("mcp get_strength_intensity", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(intensity)
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getStrengthVolume(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	end := time.Now()
	if s := req.GetString("end", ""); s != "" {
		parsed, err := parseFlexTime(s)
		if err != nil {
			return mcp.NewToolResultError("invalid end date: " + err.Error()), nil
		}
		end = parsed
	}

	start := end.AddDate(0, 0, -84) // 12 weeks
	if s := req.GetString("start", ""); s != "" {
		parsed, err := parseFlexTime(s)
		if err != nil {
			return mcp.NewToolResultError("invalid start date: " + err.Error()), nil
		}
		start = parsed
	}

	bucket := req.GetString("bucket", "1 week")
	uid := UserIDFromContext(ctx)

	volume, err := h.ds.GetTrainingVolume(ctx, start, end, bucket, uid)
	if err != nil {
		h.log.Error("mcp get_strength_volume", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": volume})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getStrengthE1RM(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	end := time.Now()
	if s := req.GetString("end", ""); s != "" {
		parsed, err := parseFlexTime(s)
		if err != nil {
			return mcp.NewToolResultError("invalid end date: " + err.Error()), nil
		}
		end = parsed
	}

	start := end.AddDate(0, -6, 0)
	if s := req.GetString("start", ""); s != "" {
		parsed, err := parseFlexTime(s)
		if err != nil {
			return mcp.NewToolResultError("invalid start date: " + err.Error()), nil
		}
		start = parsed
	}

	uid := UserIDFromContext(ctx)

	estimates, err := h.ds.GetExerciseE1RM(ctx, start, end, uid, req.GetString("exercise", ""))
	if err != nil {
		h.log.Error("mcp get_strength_1rm", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": estimates})
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

	result, err := mcp.NewToolResultJSON(map[string]any{"data": summary})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getECGRecordings(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)

	recordings, err := h.ds.QueryECGRecordings(ctx, start, end, uid)
	if err != nil {
		h.log.Error("mcp get_ecg_recordings", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": recordings})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getAudiograms(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)

	audiograms, err := h.ds.QueryAudiograms(ctx, start, end, uid)
	if err != nil {
		h.log.Error("mcp get_audiograms", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": audiograms})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getActivitySummaries(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)

	summaries, err := h.ds.QueryActivitySummaries(ctx, start, end, uid)
	if err != nil {
		h.log.Error("mcp get_activity_summaries", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": summaries})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getMedications(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)

	medications, err := h.ds.QueryMedications(ctx, start, end, uid)
	if err != nil {
		h.log.Error("mcp get_medications", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": medications})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getVisionPrescriptions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)

	prescriptions, err := h.ds.QueryVisionPrescriptions(ctx, start, end, uid)
	if err != nil {
		h.log.Error("mcp get_vision_prescriptions", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": prescriptions})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getStateOfMind(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)

	records, err := h.ds.QueryStateOfMind(ctx, start, end, uid)
	if err != nil {
		h.log.Error("mcp get_state_of_mind", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": records})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}

func (h *handlers) getCategorySamples(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start, end, err := defaultTimeRange(req.GetString("start", ""), req.GetString("end", ""))
	if err != nil {
		return mcp.NewToolResultError("invalid date format: " + err.Error()), nil
	}

	uid := UserIDFromContext(ctx)
	typeFilter := req.GetString("type", "")

	samples, err := h.ds.QueryCategorySamples(ctx, start, end, uid, typeFilter)
	if err != nil {
		h.log.Error("mcp get_category_samples", "error", err)
		return mcp.NewToolResultError("query failed: " + err.Error()), nil
	}

	result, err := mcp.NewToolResultJSON(map[string]any{"data": samples})
	if err != nil {
		return mcp.NewToolResultError("serialization failed"), nil
	}
	return result, nil
}
