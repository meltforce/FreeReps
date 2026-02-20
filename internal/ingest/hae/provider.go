package hae

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/claude/freereps/internal/ingest"
	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
	"github.com/google/uuid"
)

// Provider processes Health Auto Export REST API payloads.
type Provider struct {
	db  *storage.DB
	log *slog.Logger
}

// NewProvider creates a new HAE ingest provider.
func NewProvider(db *storage.DB, log *slog.Logger) *Provider {
	return &Provider{db: db, log: log}
}

// Ingest processes an HAE JSON payload and stores accepted data.
func (p *Provider) Ingest(ctx context.Context, payload *models.HAEPayload, userID int) (*ingest.Result, error) {
	result := &ingest.Result{}

	// Process metrics
	if len(payload.Data.Metrics) > 0 {
		if err := p.processMetrics(ctx, payload.Data.Metrics, userID, result); err != nil {
			return result, fmt.Errorf("processing metrics: %w", err)
		}
	}

	// Process workouts
	if len(payload.Data.Workouts) > 0 {
		if err := p.processWorkouts(ctx, payload.Data.Workouts, userID, result); err != nil {
			return result, fmt.Errorf("processing workouts: %w", err)
		}
	}

	// Build message for rejected metrics
	if len(result.RejectedNames) > 0 {
		result.Message = fmt.Sprintf(
			"Some metrics were rejected because they are not in the allowlist: %v. "+
				"Accepted metrics are stored. Check GET /api/v1/allowlist for the full list.",
			result.RejectedNames)
	}

	return result, nil
}

func (p *Provider) processMetrics(ctx context.Context, metrics []models.HAEMetric, userID int, result *ingest.Result) error {
	var healthRows []models.HealthMetricRow
	rejectedSet := map[string]bool{}

	for _, m := range metrics {
		// Check allowlist
		allowed, err := p.db.IsMetricAllowed(ctx, m.Name)
		if err != nil {
			return fmt.Errorf("checking allowlist for %s: %w", m.Name, err)
		}
		if !allowed {
			if !rejectedSet[m.Name] {
				result.RejectedNames = append(result.RejectedNames, m.Name)
				rejectedSet[m.Name] = true
			}
			result.MetricsRejected += len(m.Data)
			continue
		}

		// Handle sleep_analysis separately
		if m.Name == "sleep_analysis" {
			if err := p.processSleep(ctx, m, userID, result); err != nil {
				return fmt.Errorf("processing sleep: %w", err)
			}
			continue
		}

		// Detect metric shape and convert to rows
		for _, raw := range m.Data {
			result.MetricsReceived++

			row, err := convertMetricDataPoint(m.Name, m.Units, raw, userID)
			if err != nil {
				p.log.Warn("skipping data point", "metric", m.Name, "error", err)
				continue
			}
			healthRows = append(healthRows, *row)
		}
	}

	// Batch insert health metrics
	if len(healthRows) > 0 {
		inserted, err := p.db.InsertHealthMetrics(ctx, healthRows)
		if err != nil {
			return fmt.Errorf("inserting health metrics: %w", err)
		}
		result.MetricsInserted = inserted
		result.MetricsSkipped = int64(len(healthRows)) - inserted
	}

	return nil
}

// convertMetricDataPoint detects the shape of a metric data point and converts it to a HealthMetricRow.
func convertMetricDataPoint(name, units string, raw json.RawMessage, userID int) (*models.HealthMetricRow, error) {
	row := &models.HealthMetricRow{
		UserID:     userID,
		MetricName: name,
		Units:      units,
	}

	shape := DetectMetricShape(name)
	switch shape {
	case ShapeMinAvgMax:
		var dp models.HAEHeartRateDataPoint
		if err := json.Unmarshal(raw, &dp); err != nil {
			return nil, fmt.Errorf("parsing min/avg/max: %w", err)
		}
		row.Time = dp.Date.Time
		row.MinVal = &dp.Min
		row.AvgVal = &dp.Avg
		row.MaxVal = &dp.Max

	case ShapeBloodPressure:
		var dp models.HAEBloodPressureDataPoint
		if err := json.Unmarshal(raw, &dp); err != nil {
			return nil, fmt.Errorf("parsing blood pressure: %w", err)
		}
		row.Time = dp.Date.Time
		row.Systolic = &dp.Systolic
		row.Diastolic = &dp.Diastolic

	default: // ShapeQty
		var dp models.HAEMetricDataPoint
		if err := json.Unmarshal(raw, &dp); err != nil {
			return nil, fmt.Errorf("parsing qty: %w", err)
		}
		row.Time = dp.Date.Time
		row.Qty = &dp.Qty
	}

	return row, nil
}

func (p *Provider) processSleep(ctx context.Context, m models.HAEMetric, userID int, result *ingest.Result) error {
	for _, raw := range m.Data {
		result.MetricsReceived++

		format := DetectSleepFormat(raw)
		switch format {
		case SleepFormatAggregated:
			var dp models.HAESleepAggregated
			if err := json.Unmarshal(raw, &dp); err != nil {
				p.log.Warn("skipping aggregated sleep point", "error", err)
				continue
			}
			date, err := time.Parse("2006-01-02", dp.Date)
			if err != nil {
				p.log.Warn("skipping sleep: bad date", "date", dp.Date, "error", err)
				continue
			}
			row := models.SleepSessionRow{
				UserID:     userID,
				Date:       date,
				TotalSleep: dp.TotalSleep,
				Asleep:     dp.Asleep,
				Core:       dp.Core,
				Deep:       dp.Deep,
				REM:        dp.REM,
				InBed:      dp.InBed,
				SleepStart: dp.SleepStart.Time,
				SleepEnd:   dp.SleepEnd.Time,
				InBedStart: dp.InBedStart.Time,
				InBedEnd:   dp.InBedEnd.Time,
			}
			if err := p.db.InsertSleepSession(ctx, row); err != nil {
				return err
			}
			result.SleepSessionsInserted++

			// Also write sleep_analysis to health_metrics for correlation queries
			qty := dp.TotalSleep
			sleepMetric := models.HealthMetricRow{
				Time:       dp.SleepEnd.Time,
				UserID:     userID,
				MetricName: "sleep_analysis",
				Source:     "Health Auto Export",
				Units:      "hr",
				Qty:        &qty,
			}
			if _, err := p.db.InsertHealthMetrics(ctx, []models.HealthMetricRow{sleepMetric}); err != nil {
				p.log.Warn("failed to insert sleep_analysis metric", "error", err)
			}

		case SleepFormatUnaggregated:
			var dp models.HAESleepStage
			if err := json.Unmarshal(raw, &dp); err != nil {
				p.log.Warn("skipping unaggregated sleep point", "error", err)
				continue
			}
			stageRow := models.SleepStageRow{
				StartTime:  dp.StartDate.Time,
				EndTime:    dp.EndDate.Time,
				UserID:     userID,
				Stage:      dp.Value,
				DurationHr: dp.Qty,
			}
			inserted, err := p.db.InsertSleepStages(ctx, []models.SleepStageRow{stageRow})
			if err != nil {
				return err
			}
			result.SleepStagesInserted += inserted
		}
	}
	return nil
}

func (p *Provider) processWorkouts(ctx context.Context, workouts []models.HAEWorkout, userID int, result *ingest.Result) error {
	for _, w := range workouts {
		result.WorkoutsReceived++

		workoutID, err := uuid.Parse(w.ID)
		if err != nil {
			p.log.Warn("skipping workout: invalid UUID", "id", w.ID, "error", err)
			continue
		}

		// Marshal the full workout as raw_json for fields we don't explicitly model
		rawJSON, _ := json.Marshal(w)

		row := models.WorkoutRow{
			ID:          workoutID,
			UserID:      userID,
			Name:        w.Name,
			StartTime:   w.Start.Time,
			EndTime:     w.End.Time,
			DurationSec: w.Duration,
			Location:    w.Location,
			IsIndoor:    w.IsIndoor,
			RawJSON:     rawJSON,
		}

		// Extract quantity fields
		if w.ActiveEnergyBurned != nil {
			row.ActiveEnergyBurned = &w.ActiveEnergyBurned.Qty
			row.ActiveEnergyUnits = w.ActiveEnergyBurned.Units
		}
		if w.TotalEnergy != nil {
			row.TotalEnergy = &w.TotalEnergy.Qty
			row.TotalEnergyUnits = w.TotalEnergy.Units
		}
		if w.Distance != nil {
			row.Distance = &w.Distance.Qty
			row.DistanceUnits = w.Distance.Units
		}
		if w.ElevationUp != nil {
			row.ElevationUp = &w.ElevationUp.Qty
		}
		if w.ElevationDown != nil {
			row.ElevationDown = &w.ElevationDown.Qty
		}

		// Extract HR summary
		if w.HeartRate != nil {
			row.AvgHeartRate = &w.HeartRate.Avg.Qty
			row.MaxHeartRate = &w.HeartRate.Max.Qty
			row.MinHeartRate = &w.HeartRate.Min.Qty
		} else {
			if w.AvgHR != nil {
				row.AvgHeartRate = &w.AvgHR.Qty
			}
			if w.MaxHR != nil {
				row.MaxHeartRate = &w.MaxHR.Qty
			}
		}

		inserted, err := p.db.InsertWorkout(ctx, row)
		if err != nil {
			return fmt.Errorf("inserting workout %s: %w", w.ID, err)
		}
		if inserted {
			result.WorkoutsInserted++
		}

		// Only insert HR and route data if the workout was newly inserted
		// (avoid re-inserting on duplicate)
		if !inserted {
			continue
		}

		// Insert HR time-series
		if len(w.HeartRateData) > 0 {
			hrRows := make([]models.WorkoutHRRow, len(w.HeartRateData))
			for i, hr := range w.HeartRateData {
				hrRows[i] = models.WorkoutHRRow{
					Time:      hr.Date.Time,
					WorkoutID: workoutID,
					UserID:    userID,
					MinBPM:    &hr.Min,
					AvgBPM:    &hr.Avg,
					MaxBPM:    &hr.Max,
					Source:    hr.Source,
				}
			}
			n, err := p.db.InsertWorkoutHeartRate(ctx, hrRows)
			if err != nil {
				return fmt.Errorf("inserting workout HR: %w", err)
			}
			result.WorkoutHRPoints += n
		}

		// Insert route data
		if len(w.Route) > 0 {
			routeRows := make([]models.WorkoutRouteRow, len(w.Route))
			for i, rp := range w.Route {
				routeRows[i] = models.WorkoutRouteRow{
					Time:               rp.Timestamp.Time,
					WorkoutID:          workoutID,
					UserID:             userID,
					Latitude:           rp.Latitude,
					Longitude:          rp.Longitude,
					Altitude:           &rp.Altitude,
					Speed:              &rp.Speed,
					Course:             &rp.Course,
					HorizontalAccuracy: &rp.HorizontalAccuracy,
					VerticalAccuracy:   &rp.VerticalAccuracy,
				}
			}
			n, err := p.db.InsertWorkoutRoutes(ctx, routeRows)
			if err != nil {
				return fmt.Errorf("inserting workout routes: %w", err)
			}
			result.WorkoutRoutePoints += n
		}
	}
	return nil
}
