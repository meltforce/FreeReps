package mcp

import (
	"context"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

// DataSource abstracts the data layer for MCP tools. Both *storage.DB (local)
// and HTTPClient (remote via REST API) satisfy this interface.
type DataSource interface {
	GetTimeSeries(ctx context.Context, metricName string, start, end time.Time, bucketSize string, userID int) ([]storage.TimeSeriesPoint, error)
	GetMetricStats(ctx context.Context, metricName string, start, end time.Time, userID int) (*storage.MetricStats, error)
	GetCorrelation(ctx context.Context, xMetric, yMetric string, start, end time.Time, bucket string, userID int) (*storage.CorrelationResult, error)
	QuerySleepSessions(ctx context.Context, start, end time.Time, userID int) ([]storage.SleepSessionResult, error)
	QuerySleepStages(ctx context.Context, start, end time.Time, userID int) ([]models.SleepStageRow, error)
	GetSleepSummary(ctx context.Context, start, end time.Time, bucket string, userID int) ([]storage.SleepSummaryPeriod, error)
	QueryWorkouts(ctx context.Context, start, end time.Time, userID int, nameFilter string) ([]models.WorkoutRow, error)
	QueryWorkoutSets(ctx context.Context, start, end time.Time, userID int, exerciseFilter string) ([]models.WorkoutSetRow, error)
	GetTrainingSummary(ctx context.Context, start, end time.Time, bucket string, userID int) ([]storage.TrainingSummaryPeriod, error)
	GetTrainingIntensity(ctx context.Context, start, end time.Time, userID int, exerciseFilter string) (*storage.TrainingIntensityResult, error)
	GetLatestMetrics(ctx context.Context, userID int) ([]models.HealthMetricRow, error)
	GetDailySums(ctx context.Context, userID int, metricNames []string) ([]storage.DailySum, error)
	GetAllowedMetrics(ctx context.Context) ([]storage.AllowedMetric, error)
}

// Compile-time check: *storage.DB satisfies DataSource.
var _ DataSource = (*storage.DB)(nil)
