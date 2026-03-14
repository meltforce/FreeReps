package health

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/claude/freereps/internal/ingest"
	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
	"github.com/google/uuid"
)

// Provider processes health data REST API payloads.
type Provider struct {
	db  *storage.DB
	log *slog.Logger
}

// NewProvider creates a new health ingest provider.
func NewProvider(db *storage.DB, log *slog.Logger) *Provider {
	return &Provider{db: db, log: log}
}

// Ingest processes a health data JSON payload and stores accepted data.
func (p *Provider) Ingest(ctx context.Context, payload *models.HealthPayload, userID int) (*ingest.Result, error) {
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

	// Process ECG recordings
	if len(payload.Data.ECGRecordings) > 0 {
		if err := p.processECGRecordings(ctx, payload.Data.ECGRecordings, userID, result); err != nil {
			return result, fmt.Errorf("processing ECG recordings: %w", err)
		}
	}

	// Process audiograms
	if len(payload.Data.Audiograms) > 0 {
		if err := p.processAudiograms(ctx, payload.Data.Audiograms, userID, result); err != nil {
			return result, fmt.Errorf("processing audiograms: %w", err)
		}
	}

	// Process activity summaries
	if len(payload.Data.ActivitySummaries) > 0 {
		if err := p.processActivitySummaries(ctx, payload.Data.ActivitySummaries, userID, result); err != nil {
			return result, fmt.Errorf("processing activity summaries: %w", err)
		}
	}

	// Process medications
	if len(payload.Data.Medications) > 0 {
		if err := p.processMedications(ctx, payload.Data.Medications, userID, result); err != nil {
			return result, fmt.Errorf("processing medications: %w", err)
		}
	}

	// Process vision prescriptions
	if len(payload.Data.VisionPrescriptions) > 0 {
		if err := p.processVisionPrescriptions(ctx, payload.Data.VisionPrescriptions, userID, result); err != nil {
			return result, fmt.Errorf("processing vision prescriptions: %w", err)
		}
	}

	// Process state of mind
	if len(payload.Data.StateOfMind) > 0 {
		if err := p.processStateOfMind(ctx, payload.Data.StateOfMind, userID, result); err != nil {
			return result, fmt.Errorf("processing state of mind: %w", err)
		}
	}

	// Process category samples
	if len(payload.Data.CategorySamples) > 0 {
		if err := p.processCategorySamples(ctx, payload.Data.CategorySamples, userID, result); err != nil {
			return result, fmt.Errorf("processing category samples: %w", err)
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

func (p *Provider) processMetrics(ctx context.Context, metrics []models.HealthMetric, userID int, result *ingest.Result) error {
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
		var dp models.HeartRateDataPoint
		if err := json.Unmarshal(raw, &dp); err != nil {
			return nil, fmt.Errorf("parsing min/avg/max: %w", err)
		}
		row.Time = dp.Date.Time
		// When the iOS app falls back to individual samples, only qty is set.
		// Promote qty → Min/Avg/Max so the data isn't stored as zeros.
		if dp.Min == 0 && dp.Avg == 0 && dp.Max == 0 && dp.Qty != 0 {
			dp.Min = dp.Qty
			dp.Avg = dp.Qty
			dp.Max = dp.Qty
		}
		row.MinVal = &dp.Min
		row.AvgVal = &dp.Avg
		row.MaxVal = &dp.Max
		if dp.SourceUUID != nil {
			parsed, err := uuid.Parse(*dp.SourceUUID)
			if err == nil {
				row.SourceUUID = &parsed
			}
		}

	case ShapeBloodPressure:
		var dp models.BloodPressureDataPoint
		if err := json.Unmarshal(raw, &dp); err != nil {
			return nil, fmt.Errorf("parsing blood pressure: %w", err)
		}
		row.Time = dp.Date.Time
		row.Systolic = &dp.Systolic
		row.Diastolic = &dp.Diastolic
		if dp.SourceUUID != nil {
			parsed, err := uuid.Parse(*dp.SourceUUID)
			if err == nil {
				row.SourceUUID = &parsed
			}
		}

	default: // ShapeQty
		var dp models.HealthMetricDataPoint
		if err := json.Unmarshal(raw, &dp); err != nil {
			return nil, fmt.Errorf("parsing qty: %w", err)
		}
		row.Time = dp.Date.Time
		row.Qty = &dp.Qty
		if dp.SourceUUID != nil {
			parsed, err := uuid.Parse(*dp.SourceUUID)
			if err == nil {
				row.SourceUUID = &parsed
			}
		}
	}

	return row, nil
}

func (p *Provider) processSleep(ctx context.Context, m models.HealthMetric, userID int, result *ingest.Result) error {
	for _, raw := range m.Data {
		result.MetricsReceived++

		format := DetectSleepFormat(raw)
		switch format {
		case SleepFormatAggregated:
			var dp models.SleepAggregated
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
			var dp models.SleepStage
			if err := json.Unmarshal(raw, &dp); err != nil {
				p.log.Warn("skipping unaggregated sleep point", "error", err)
				continue
			}
			stage, known := models.NormalizeSleepStage(dp.Value)
			if !known {
				p.log.Warn("unknown sleep stage name, storing as-is", "raw", dp.Value)
			}
			stageRow := models.SleepStageRow{
				StartTime:  dp.StartDate.Time,
				EndTime:    dp.EndDate.Time,
				UserID:     userID,
				Stage:      stage,
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

func (p *Provider) processWorkouts(ctx context.Context, workouts []models.HealthWorkout, userID int, result *ingest.Result) error {
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

func (p *Provider) processECGRecordings(ctx context.Context, recordings []models.ECGRecording, userID int, result *ingest.Result) error {
	for _, rec := range recordings {
		id, err := uuid.Parse(rec.ID)
		if err != nil {
			p.log.Warn("skipping ECG recording: invalid UUID", "id", rec.ID, "error", err)
			continue
		}

		var voltageMeasurements []byte
		if len(rec.VoltageMeasurements) > 0 {
			voltageMeasurements, err = json.Marshal(rec.VoltageMeasurements)
			if err != nil {
				p.log.Warn("skipping ECG recording: failed to marshal voltage measurements", "id", rec.ID, "error", err)
				continue
			}
		}

		row := models.ECGRecordingRow{
			ID:                  id,
			UserID:              userID,
			Classification:      rec.Classification,
			AverageHeartRate:    rec.AverageHeartRate,
			SamplingFrequency:   rec.SamplingFrequency,
			VoltageMeasurements: voltageMeasurements,
			StartDate:           rec.StartDate.Time,
			Source:              rec.Source,
		}

		inserted, err := p.db.InsertECGRecording(ctx, row)
		if err != nil {
			p.log.Warn("failed to insert ECG recording", "id", rec.ID, "error", err)
			continue
		}
		if inserted {
			result.ECGRecordingsInserted++
		}
	}
	return nil
}

func (p *Provider) processAudiograms(ctx context.Context, audiograms []models.Audiogram, userID int, result *ingest.Result) error {
	for _, ag := range audiograms {
		id, err := uuid.Parse(ag.ID)
		if err != nil {
			p.log.Warn("skipping audiogram: invalid UUID", "id", ag.ID, "error", err)
			continue
		}

		var sensitivityPoints []byte
		if len(ag.SensitivityPoints) > 0 {
			sensitivityPoints, err = json.Marshal(ag.SensitivityPoints)
			if err != nil {
				p.log.Warn("skipping audiogram: failed to marshal sensitivity points", "id", ag.ID, "error", err)
				continue
			}
		}

		row := models.AudiogramRow{
			ID:                id,
			UserID:            userID,
			SensitivityPoints: sensitivityPoints,
			StartDate:         ag.StartDate.Time,
			Source:            ag.Source,
		}

		inserted, err := p.db.InsertAudiogram(ctx, row)
		if err != nil {
			p.log.Warn("failed to insert audiogram", "id", ag.ID, "error", err)
			continue
		}
		if inserted {
			result.AudiogramsInserted++
		}
	}
	return nil
}

func (p *Provider) processActivitySummaries(ctx context.Context, summaries []models.ActivitySummary, userID int, result *ingest.Result) error {
	var rows []models.ActivitySummaryRow
	for _, s := range summaries {
		date, err := time.Parse("2006-01-02", s.Date)
		if err != nil {
			p.log.Warn("skipping activity summary: bad date", "date", s.Date, "error", err)
			continue
		}

		rows = append(rows, models.ActivitySummaryRow{
			UserID:           userID,
			Date:             date,
			ActiveEnergy:     s.ActiveEnergy,
			ActiveEnergyGoal: s.ActiveEnergyGoal,
			ExerciseTime:     s.ExerciseTime,
			ExerciseTimeGoal: s.ExerciseTimeGoal,
			StandHours:       s.StandHours,
			StandHoursGoal:   s.StandHoursGoal,
		})
	}

	if len(rows) > 0 {
		inserted, err := p.db.InsertActivitySummaries(ctx, rows)
		if err != nil {
			return fmt.Errorf("inserting activity summaries: %w", err)
		}
		result.ActivitySummariesInserted = inserted
	}
	return nil
}

func (p *Provider) processMedications(ctx context.Context, medications []models.Medication, userID int, result *ingest.Result) error {
	for _, med := range medications {
		id, err := uuid.Parse(med.ID)
		if err != nil {
			p.log.Warn("skipping medication: invalid UUID", "id", med.ID, "error", err)
			continue
		}

		row := models.MedicationRow{
			ID:        id,
			UserID:    userID,
			Name:      med.Name,
			Dosage:    med.Dosage,
			LogStatus: med.LogStatus,
			StartDate: med.StartDate.Time,
			Source:    med.Source,
		}
		if med.EndDate != nil {
			t := med.EndDate.Time
			row.EndDate = &t
		}

		inserted, err := p.db.InsertMedication(ctx, row)
		if err != nil {
			p.log.Warn("failed to insert medication", "id", med.ID, "error", err)
			continue
		}
		if inserted {
			result.MedicationsInserted++
		}
	}
	return nil
}

func (p *Provider) processVisionPrescriptions(ctx context.Context, prescriptions []models.VisionPrescription, userID int, result *ingest.Result) error {
	for _, vp := range prescriptions {
		id, err := uuid.Parse(vp.ID)
		if err != nil {
			p.log.Warn("skipping vision prescription: invalid UUID", "id", vp.ID, "error", err)
			continue
		}

		var rightEye, leftEye []byte
		if vp.RightEye != nil {
			rightEye, err = json.Marshal(vp.RightEye)
			if err != nil {
				p.log.Warn("skipping vision prescription: failed to marshal right_eye", "id", vp.ID, "error", err)
				continue
			}
		}
		if vp.LeftEye != nil {
			leftEye, err = json.Marshal(vp.LeftEye)
			if err != nil {
				p.log.Warn("skipping vision prescription: failed to marshal left_eye", "id", vp.ID, "error", err)
				continue
			}
		}

		row := models.VisionPrescriptionRow{
			ID:               id,
			UserID:           userID,
			DateIssued:       vp.DateIssued.Time,
			PrescriptionType: vp.PrescriptionType,
			RightEye:         rightEye,
			LeftEye:          leftEye,
			Source:           vp.Source,
		}
		if vp.ExpirationDate != nil {
			t := vp.ExpirationDate.Time
			row.ExpirationDate = &t
		}

		inserted, err := p.db.InsertVisionPrescription(ctx, row)
		if err != nil {
			p.log.Warn("failed to insert vision prescription", "id", vp.ID, "error", err)
			continue
		}
		if inserted {
			result.VisionPrescriptionsInserted++
		}
	}
	return nil
}

func (p *Provider) processStateOfMind(ctx context.Context, records []models.StateOfMind, userID int, result *ingest.Result) error {
	var rows []models.StateOfMindRow
	for _, som := range records {
		id, err := uuid.Parse(som.ID)
		if err != nil {
			p.log.Warn("skipping state of mind: invalid UUID", "id", som.ID, "error", err)
			continue
		}

		rows = append(rows, models.StateOfMindRow{
			ID:           id,
			UserID:       userID,
			Kind:         som.Kind,
			Valence:      som.Valence,
			Labels:       som.Labels,
			Associations: som.Associations,
			StartDate:    som.StartDate.Time,
			Source:       som.Source,
		})
	}

	if len(rows) > 0 {
		inserted, err := p.db.InsertStateOfMind(ctx, rows)
		if err != nil {
			return fmt.Errorf("inserting state of mind: %w", err)
		}
		result.StateOfMindInserted = inserted
	}
	return nil
}

func (p *Provider) processCategorySamples(ctx context.Context, samples []models.CategorySample, userID int, result *ingest.Result) error {
	var rows []models.CategorySampleRow
	for _, cs := range samples {
		id, err := uuid.Parse(cs.ID)
		if err != nil {
			p.log.Warn("skipping category sample: invalid UUID", "id", cs.ID, "error", err)
			continue
		}

		rows = append(rows, models.CategorySampleRow{
			ID:         id,
			UserID:     userID,
			Type:       cs.Type,
			Value:      cs.Value,
			ValueLabel: cs.ValueLabel,
			StartDate:  cs.StartDate.Time,
			EndDate:    cs.EndDate.Time,
			Source:     cs.Source,
		})
	}

	if len(rows) > 0 {
		inserted, err := p.db.InsertCategorySamples(ctx, rows)
		if err != nil {
			return fmt.Errorf("inserting category samples: %w", err)
		}
		result.CategorySamplesInserted = inserted
	}

	// Extract sleep stages from sleep category samples.
	var sleepStages []models.SleepStageRow
	for _, cs := range samples {
		if !strings.EqualFold(cs.Type, "HKCategoryTypeIdentifierSleepAnalysis") &&
			!strings.EqualFold(cs.Type, "sleep_analysis") &&
			!strings.Contains(strings.ToLower(cs.Type), "sleep") {
			continue
		}
		if cs.ValueLabel == nil {
			continue
		}
		label := *cs.ValueLabel
		// Strip "Asleep " prefix: "Asleep Core" → "Core", "Asleep Deep" → "Deep", etc.
		if after, ok := strings.CutPrefix(label, "Asleep "); ok {
			if after == "Unspecified" {
				label = "Asleep"
			} else {
				label = after
			}
		}
		stage, known := models.NormalizeSleepStage(label)
		if !known {
			p.log.Warn("unknown sleep stage from category sample, storing as-is", "raw", label)
		}
		sleepStages = append(sleepStages, models.SleepStageRow{
			StartTime:  cs.StartDate.Time,
			EndTime:    cs.EndDate.Time,
			UserID:     userID,
			Stage:      stage,
			DurationHr: cs.EndDate.Time.Sub(cs.StartDate.Time).Hours(),
			Source:     cs.Source,
		})
	}
	if len(sleepStages) > 0 {
		inserted, err := p.db.InsertSleepStages(ctx, sleepStages)
		if err != nil {
			return fmt.Errorf("inserting sleep stages from category samples: %w", err)
		}
		result.SleepStagesInserted += inserted
	}

	return nil
}
