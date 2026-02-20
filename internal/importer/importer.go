package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
	"github.com/google/uuid"
)

// Stats tracks import progress.
type Stats struct {
	FilesProcessed int
	FilesSkipped   int
	FilesErrored   int

	MetricsInserted    int64
	MetricsDuplicated  int64
	SleepStagesInserted int64
	WorkoutsInserted   int
	WorkoutsDuplicated int
	RoutePointsInserted int64
	HRCorrelated       int64

	RejectedMetrics []string
}

// Importer reads .hae files from an AutoSync directory and inserts data into the DB.
type Importer struct {
	db     *storage.DB
	log    *slog.Logger
	dryRun bool
	stats  Stats
}

// New creates a new Importer.
func New(db *storage.DB, log *slog.Logger, dryRun bool) *Importer {
	return &Importer{db: db, log: log, dryRun: dryRun}
}

// Import processes all .hae files under the given AutoSync directory.
func (imp *Importer) Import(ctx context.Context, autoSyncDir string) (*Stats, error) {
	healthDir := filepath.Join(autoSyncDir, "HealthMetrics")
	workoutDir := filepath.Join(autoSyncDir, "Workouts")
	routeDir := filepath.Join(autoSyncDir, "Routes")

	// Phase 1: Import health metrics (including heart_rate needed for HR correlation)
	if _, err := os.Stat(healthDir); err == nil {
		if err := imp.importHealthMetrics(ctx, healthDir); err != nil {
			return &imp.stats, fmt.Errorf("importing health metrics: %w", err)
		}
	}

	// Phase 2: Import workouts
	if _, err := os.Stat(workoutDir); err == nil {
		if err := imp.importWorkouts(ctx, workoutDir, routeDir); err != nil {
			return &imp.stats, fmt.Errorf("importing workouts: %w", err)
		}
	}

	// Phase 3: Correlate heart rate data with workouts
	if !imp.dryRun {
		correlated, err := CorrelateWorkoutHR(ctx, imp.db, imp.log)
		if err != nil {
			return &imp.stats, fmt.Errorf("correlating workout HR: %w", err)
		}
		imp.stats.HRCorrelated = correlated
	}

	return &imp.stats, nil
}

// importHealthMetrics walks HealthMetrics/ subdirectories and imports each metric.
func (imp *Importer) importHealthMetrics(ctx context.Context, healthDir string) error {
	entries, err := os.ReadDir(healthDir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", healthDir, err)
	}

	rejectedSet := map[string]bool{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metricName := entry.Name()

		// Check allowlist
		allowed, err := imp.db.IsMetricAllowed(ctx, metricName)
		if err != nil {
			return fmt.Errorf("checking allowlist for %s: %w", metricName, err)
		}
		if !allowed {
			if !rejectedSet[metricName] {
				imp.stats.RejectedMetrics = append(imp.stats.RejectedMetrics, metricName)
				rejectedSet[metricName] = true
			}
			imp.log.Info("skipping metric (not in allowlist)", "metric", metricName)
			continue
		}

		metricDir := filepath.Join(healthDir, metricName)
		if metricName == "sleep_analysis" {
			if err := imp.importSleepDir(ctx, metricDir); err != nil {
				return fmt.Errorf("importing sleep: %w", err)
			}
		} else {
			if err := imp.importMetricDir(ctx, metricDir, metricName); err != nil {
				return fmt.Errorf("importing %s: %w", metricName, err)
			}
		}
	}

	return nil
}

// importMetricDir imports all .hae files in a single metric's directory.
func (imp *Importer) importMetricDir(ctx context.Context, dir, metricName string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.hae"))
	if err != nil {
		return err
	}

	isHeartRate := metricName == "heart_rate"
	isActiveEnergy := metricName == "active_energy"

	for _, f := range files {
		data, err := DecompressLZFSE(f)
		if err != nil {
			imp.log.Warn("decompress failed", "file", f, "error", err)
			imp.stats.FilesErrored++
			continue
		}

		var file models.HAEFileMetric
		if err := json.Unmarshal(data, &file); err != nil {
			imp.log.Warn("parse failed", "file", f, "error", err)
			imp.stats.FilesErrored++
			continue
		}

		if len(file.Data) == 0 {
			imp.stats.FilesSkipped++
			continue
		}

		var rows []models.HealthMetricRow
		for _, dp := range file.Data {
			// Active energy: filter to kcal only (skip kJ duplicates)
			if isActiveEnergy && dp.Unit != "kcal" {
				continue
			}

			row := models.HealthMetricRow{
				Time:       models.AppleTimestampToTime(dp.Start),
				UserID:     1,
				MetricName: metricName,
				Source:     dp.SourceName(),
				Units:      dp.Unit,
			}

			if isHeartRate {
				row.MinVal = dp.Min
				row.AvgVal = dp.Avg
				row.MaxVal = dp.Max
			} else {
				row.Qty = dp.Qty
			}

			rows = append(rows, row)
		}

		if len(rows) == 0 {
			imp.stats.FilesSkipped++
			continue
		}

		imp.stats.FilesProcessed++
		if imp.dryRun {
			imp.stats.MetricsInserted += int64(len(rows))
			continue
		}

		// Batch insert in chunks to avoid exceeding parameter limits
		inserted, err := imp.batchInsertMetrics(ctx, rows)
		if err != nil {
			return fmt.Errorf("inserting %s from %s: %w", metricName, filepath.Base(f), err)
		}
		imp.stats.MetricsInserted += inserted
		imp.stats.MetricsDuplicated += int64(len(rows)) - inserted
	}

	return nil
}

// batchInsertMetrics inserts health metrics in batches to stay within PostgreSQL parameter limits.
// 11 params per row, max 65535 params → ~5957 rows per batch. Use 5000.
func (imp *Importer) batchInsertMetrics(ctx context.Context, rows []models.HealthMetricRow) (int64, error) {
	const batchSize = 5000
	var totalInserted int64

	for i := 0; i < len(rows); i += batchSize {
		end := i + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		inserted, err := imp.db.InsertHealthMetrics(ctx, rows[i:end])
		if err != nil {
			return totalInserted, err
		}
		totalInserted += inserted
	}
	return totalInserted, nil
}

// importSleepDir imports all sleep_analysis .hae files as sleep stages.
func (imp *Importer) importSleepDir(ctx context.Context, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.hae"))
	if err != nil {
		return err
	}

	for _, f := range files {
		data, err := DecompressLZFSE(f)
		if err != nil {
			imp.log.Warn("decompress failed", "file", f, "error", err)
			imp.stats.FilesErrored++
			continue
		}

		var file models.HAEFileMetric
		if err := json.Unmarshal(data, &file); err != nil {
			imp.log.Warn("parse failed", "file", f, "error", err)
			imp.stats.FilesErrored++
			continue
		}

		if len(file.Data) == 0 {
			imp.stats.FilesSkipped++
			continue
		}

		var stages []models.SleepStageRow
		for _, dp := range file.Data {
			stageType := dp.SleepStageType()
			if stageType == "" {
				continue
			}
			stages = append(stages, models.SleepStageRow{
				StartTime:  models.AppleTimestampToTime(dp.Start),
				EndTime:    models.AppleTimestampToTime(dp.End),
				UserID:     1,
				Stage:      stageType,
				DurationHr: dp.SleepStageDuration(),
				Source:     dp.SourceName(),
			})
		}

		if len(stages) == 0 {
			imp.stats.FilesSkipped++
			continue
		}

		imp.stats.FilesProcessed++
		if imp.dryRun {
			imp.stats.SleepStagesInserted += int64(len(stages))
			continue
		}

		// Batch insert in chunks (6 params per row)
		inserted, err := imp.batchInsertSleepStages(ctx, stages)
		if err != nil {
			return fmt.Errorf("inserting sleep from %s: %w", filepath.Base(f), err)
		}
		imp.stats.SleepStagesInserted += inserted
	}

	// Synthesize sleep sessions from the imported stages
	if !imp.dryRun {
		if err := imp.synthesizeSleepSessions(ctx); err != nil {
			return fmt.Errorf("synthesizing sleep sessions: %w", err)
		}
	}

	return nil
}

// batchInsertSleepStages inserts sleep stages in batches.
// 6 params per row, max 65535 params → ~10922 rows per batch. Use 10000.
func (imp *Importer) batchInsertSleepStages(ctx context.Context, rows []models.SleepStageRow) (int64, error) {
	const batchSize = 10000
	var totalInserted int64

	for i := 0; i < len(rows); i += batchSize {
		end := i + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		inserted, err := imp.db.InsertSleepStages(ctx, rows[i:end])
		if err != nil {
			return totalInserted, err
		}
		totalInserted += inserted
	}
	return totalInserted, nil
}

// synthesizeSleepSessions groups all sleep stages into nights and creates
// sleep sessions + health_metrics rows for each night.
func (imp *Importer) synthesizeSleepSessions(ctx context.Context) error {
	// Query all stages for user 1 (import is single-user)
	stages, err := imp.db.QuerySleepStages(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC), 1)
	if err != nil {
		return fmt.Errorf("querying stages: %w", err)
	}
	if len(stages) == 0 {
		return nil
	}

	// Sort by start time
	sort.Slice(stages, func(i, j int) bool {
		return stages[i].StartTime.Before(stages[j].StartTime)
	})

	// Group into nights: stages within 12 hours of each other
	var nights [][]models.SleepStageRow
	var currentNight []models.SleepStageRow

	for _, stage := range stages {
		if len(currentNight) == 0 {
			currentNight = append(currentNight, stage)
			continue
		}
		lastEnd := currentNight[len(currentNight)-1].EndTime
		if stage.StartTime.Sub(lastEnd) > 12*time.Hour {
			nights = append(nights, currentNight)
			currentNight = []models.SleepStageRow{stage}
		} else {
			currentNight = append(currentNight, stage)
		}
	}
	if len(currentNight) > 0 {
		nights = append(nights, currentNight)
	}

	// Create a session for each night
	var sessionsCreated int
	for _, night := range nights {
		sleepStart := night[0].StartTime
		sleepEnd := night[len(night)-1].EndTime

		var deep, core, rem, awake float64
		for _, s := range night {
			switch s.Stage {
			case "Deep":
				deep += s.DurationHr
			case "Core":
				core += s.DurationHr
			case "REM":
				rem += s.DurationHr
			case "Awake":
				awake += s.DurationHr
			}
		}

		totalSleep := deep + core + rem
		inBed := sleepEnd.Sub(sleepStart).Hours()
		// Date = the day the person woke up
		date := sleepEnd.Truncate(24 * time.Hour)

		session := models.SleepSessionRow{
			UserID:     1,
			Date:       date,
			TotalSleep: totalSleep,
			Asleep:     totalSleep,
			Core:       core,
			Deep:       deep,
			REM:        rem,
			InBed:      inBed,
			SleepStart: sleepStart,
			SleepEnd:   sleepEnd,
			InBedStart: sleepStart,
			InBedEnd:   sleepEnd,
		}

		if err := imp.db.InsertSleepSession(ctx, session); err != nil {
			return fmt.Errorf("inserting synthesized session: %w", err)
		}
		sessionsCreated++

		// Also insert sleep_analysis health metric for correlation queries
		qty := totalSleep
		sleepMetric := models.HealthMetricRow{
			Time:       sleepEnd,
			UserID:     1,
			MetricName: "sleep_analysis",
			Source:     "FreeReps Import",
			Units:      "hr",
			Qty:        &qty,
		}
		if _, err := imp.db.InsertHealthMetrics(ctx, []models.HealthMetricRow{sleepMetric}); err != nil {
			return fmt.Errorf("inserting sleep_analysis metric: %w", err)
		}
	}

	imp.log.Info("synthesized sleep sessions", "nights", len(nights), "sessions_created", sessionsCreated)
	return nil
}

// importWorkouts imports all workout .hae files and matches routes.
func (imp *Importer) importWorkouts(ctx context.Context, workoutDir, routeDir string) error {
	files, err := filepath.Glob(filepath.Join(workoutDir, "*.hae"))
	if err != nil {
		return err
	}

	for _, f := range files {
		data, err := DecompressLZFSE(f)
		if err != nil {
			imp.log.Warn("decompress failed", "file", f, "error", err)
			imp.stats.FilesErrored++
			continue
		}

		var workout models.HAEFileWorkout
		if err := json.Unmarshal(data, &workout); err != nil {
			imp.log.Warn("parse failed", "file", f, "error", err)
			imp.stats.FilesErrored++
			continue
		}

		workoutID, err := uuid.Parse(workout.ID)
		if err != nil {
			imp.log.Warn("invalid workout UUID", "file", f, "id", workout.ID, "error", err)
			imp.stats.FilesErrored++
			continue
		}

		row := models.WorkoutRow{
			ID:          workoutID,
			UserID:      1,
			Name:        workout.Name,
			StartTime:   models.AppleTimestampToTime(workout.Start),
			EndTime:     models.AppleTimestampToTime(workout.End),
			DurationSec: workout.Duration,
			Location:    workout.Location,
			RawJSON:     data,
		}

		if workout.ActiveEnergy != nil {
			row.ActiveEnergyBurned = workout.ActiveEnergy
			row.ActiveEnergyUnits = "kcal"
		}
		if workout.TotalDistance != nil {
			row.Distance = workout.TotalDistance
			row.DistanceUnits = "km"
		}
		if workout.ElevationUp != nil {
			row.ElevationUp = workout.ElevationUp
		}

		imp.stats.FilesProcessed++
		if imp.dryRun {
			imp.stats.WorkoutsInserted++
			continue
		}

		inserted, err := imp.db.InsertWorkout(ctx, row)
		if err != nil {
			return fmt.Errorf("inserting workout %s: %w", workout.ID, err)
		}
		if inserted {
			imp.stats.WorkoutsInserted++
		} else {
			imp.stats.WorkoutsDuplicated++
			continue
		}

		// Try to match route file
		routeFile := filepath.Join(routeDir, workout.ID+".hae")
		if _, err := os.Stat(routeFile); err == nil {
			routeInserted, err := imp.importRoute(ctx, routeFile, workoutID)
			if err != nil {
				imp.log.Warn("route import failed", "workout", workout.ID, "error", err)
			} else {
				imp.stats.RoutePointsInserted += routeInserted
			}
		}
	}

	return nil
}

// importRoute imports a single route .hae file for a workout.
func (imp *Importer) importRoute(ctx context.Context, routeFile string, workoutID uuid.UUID) (int64, error) {
	data, err := DecompressLZFSE(routeFile)
	if err != nil {
		return 0, fmt.Errorf("decompress route: %w", err)
	}

	var route models.HAEFileRoute
	if err := json.Unmarshal(data, &route); err != nil {
		return 0, fmt.Errorf("parse route: %w", err)
	}

	if len(route.Locations) == 0 {
		return 0, nil
	}

	rows := make([]models.WorkoutRouteRow, len(route.Locations))
	for i, loc := range route.Locations {
		elevation := loc.Elevation
		speed := loc.Speed
		course := loc.Course
		hAcc := loc.HAcc
		vAcc := loc.VAcc
		rows[i] = models.WorkoutRouteRow{
			Time:               models.AppleTimestampToTime(loc.Time),
			WorkoutID:          workoutID,
			UserID:             1,
			Latitude:           loc.Latitude,
			Longitude:          loc.Longitude,
			Altitude:           &elevation,
			Speed:              &speed,
			Course:             &course,
			HorizontalAccuracy: &hAcc,
			VerticalAccuracy:   &vAcc,
		}
	}

	// Batch insert routes (10 params per row)
	const batchSize = 6000
	var totalInserted int64
	for i := 0; i < len(rows); i += batchSize {
		end := i + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		inserted, err := imp.db.InsertWorkoutRoutes(ctx, rows[i:end])
		if err != nil {
			return totalInserted, err
		}
		totalInserted += inserted
	}
	return totalInserted, nil
}

// ParseWorkoutUUID extracts the UUID from a workout filename like
// "cycling_20251219_585BDA5C-5A64-4D5A-A432-6BCA6C7BCDBE.hae".
func ParseWorkoutUUID(filename string) (string, error) {
	base := strings.TrimSuffix(filename, ".hae")
	parts := strings.Split(base, "_")
	if len(parts) < 3 {
		return "", fmt.Errorf("unexpected workout filename format: %s", filename)
	}
	// UUID is everything after the second underscore (date part)
	// e.g. "cycling_20251219_585BDA5C-5A64-4D5A-A432-6BCA6C7BCDBE"
	// parts[0] = type (may contain underscores itself), parts[-1] = UUID, parts[-2] = date
	// UUID is always the last 36 chars of the base (standard UUID format)
	if len(base) < 36 {
		return "", fmt.Errorf("filename too short to contain UUID: %s", filename)
	}
	uuidStr := base[len(base)-36:]
	if _, err := uuid.Parse(uuidStr); err != nil {
		return "", fmt.Errorf("invalid UUID in filename %s: %w", filename, err)
	}
	return uuidStr, nil
}
