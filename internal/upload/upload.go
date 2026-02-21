package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/claude/freereps/internal/models"
)

// Stats tracks upload progress.
type Stats struct {
	FilesTotal    int
	FilesUploaded int
	FilesSkipped  int
	FilesErrored  int

	MetricPointsSent   int
	SleepStagesSent    int
	WorkoutsSent       int
	RoutePointsSent    int
	HRPointsCorrelated int

	RejectedMetrics []string

	// TCP mode stats
	TCPMetricChunks  int
	TCPWorkoutChunks int
	TCPBytesSent     int64
}

// Uploader walks an AutoSync directory, converts .hae files to REST API format,
// and POSTs them to the FreeReps server.
type Uploader struct {
	client    *Client
	state     *StateDB
	autoSync  string
	dryRun    bool
	batchSize int
	log       *slog.Logger
	stats     Stats
	hrPoints  []hrDataPoint // collected during metric processing for workout HR correlation
}

// New creates a new Uploader.
func New(client *Client, state *StateDB, autoSyncDir string, dryRun bool, batchSize int, log *slog.Logger) *Uploader {
	return &Uploader{
		client:    client,
		state:     state,
		autoSync:  autoSyncDir,
		dryRun:    dryRun,
		batchSize: batchSize,
		log:       log,
	}
}

// Run executes the upload pipeline.
func (u *Uploader) Run() (*Stats, error) {
	// Fetch allowlist from server (skip in dry-run — accept all metrics)
	var allowlist map[string]bool
	if !u.dryRun {
		var err error
		allowlist, err = u.client.FetchAllowlist()
		if err != nil {
			return &u.stats, fmt.Errorf("fetching allowlist: %w", err)
		}
		u.log.Info("fetched allowlist", "metrics", len(allowlist))
	}

	// Phase 1: Health metrics (also collects heart_rate data for workout HR correlation)
	healthDir := filepath.Join(u.autoSync, "HealthMetrics")
	if _, err := os.Stat(healthDir); err == nil {
		if err := u.processMetrics(healthDir, allowlist); err != nil {
			return &u.stats, fmt.Errorf("processing metrics: %w", err)
		}
	}

	// Sort collected HR points by time for binary search during workout correlation
	sort.Slice(u.hrPoints, func(i, j int) bool {
		return u.hrPoints[i].Time.Before(u.hrPoints[j].Time)
	})

	// Phase 2: Workouts + Routes (with embedded HR correlation)
	workoutDir := filepath.Join(u.autoSync, "Workouts")
	routeDir := filepath.Join(u.autoSync, "Routes")
	if _, err := os.Stat(workoutDir); err == nil {
		if err := u.processWorkouts(workoutDir, routeDir); err != nil {
			return &u.stats, fmt.Errorf("processing workouts: %w", err)
		}
	}

	return &u.stats, nil
}

// processMetrics walks HealthMetrics/ subdirectories and uploads each metric.
func (u *Uploader) processMetrics(healthDir string, allowlist map[string]bool) error {
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

		// Check allowlist (skip in dry-run)
		if allowlist != nil && !allowlist[metricName] {
			if !rejectedSet[metricName] {
				u.stats.RejectedMetrics = append(u.stats.RejectedMetrics, metricName)
				rejectedSet[metricName] = true
			}
			continue
		}

		metricDir := filepath.Join(healthDir, metricName)
		if err := u.processMetricDir(metricDir, metricName); err != nil {
			return fmt.Errorf("processing %s: %w", metricName, err)
		}
	}

	return nil
}

// fileInfo tracks a file's metadata for state DB operations.
type fileInfo struct {
	relPath string
	size    int64
	hash    string
}

// processMetricDir processes all .hae files in a single metric's directory.
func (u *Uploader) processMetricDir(dir, metricName string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.hae"))
	if err != nil {
		return err
	}

	var allPoints []json.RawMessage
	var allHRPoints []hrDataPoint
	var newFiles []fileInfo
	var units string

	for _, f := range files {
		u.stats.FilesTotal++

		// Check state DB
		relPath, _ := filepath.Rel(u.autoSync, f)
		info, err := os.Stat(f)
		if err != nil {
			u.log.Warn("stat failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}

		hash, err := HashFile(f)
		if err != nil {
			u.log.Warn("hash failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}

		uploaded, err := u.state.IsUploaded(relPath, info.Size(), hash)
		if err != nil {
			u.log.Warn("state check failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}
		if uploaded {
			u.stats.FilesSkipped++
			continue
		}

		// Decompress and parse
		data, err := decompressLZFSE(f)
		if err != nil {
			u.log.Warn("decompress failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}

		var file models.HAEFileMetric
		if err := json.Unmarshal(data, &file); err != nil {
			u.log.Warn("parse failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}

		if len(file.Data) == 0 {
			u.stats.FilesSkipped++
			// Mark empty files as uploaded so we don't re-check them
			_ = u.state.MarkUploaded(relPath, info.Size(), hash)
			continue
		}

		// Convert
		metric, hrPoints, err := convertMetric(file, metricName)
		if err != nil {
			u.log.Warn("convert failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}

		allPoints = append(allPoints, metric.Data...)
		allHRPoints = append(allHRPoints, hrPoints...)
		if units == "" {
			units = metric.Units
		}
		newFiles = append(newFiles, fileInfo{relPath: relPath, size: info.Size(), hash: hash})
	}

	if len(allPoints) == 0 {
		return nil
	}

	// Batch and send
	isSleep := metricName == "sleep_analysis"
	for i := 0; i < len(allPoints); i += u.batchSize {
		end := i + u.batchSize
		if end > len(allPoints) {
			end = len(allPoints)
		}
		batch := allPoints[i:end]

		payload := models.HAEPayload{
			Data: models.HAEData{
				Metrics: []models.HAEMetric{{
					Name:  metricName,
					Units: units,
					Data:  batch,
				}},
			},
		}

		if u.dryRun {
			u.log.Info("dry-run: would send",
				"metric", metricName,
				"points", len(batch),
			)
		} else {
			if err := u.client.SendPayload(payload); err != nil {
				return fmt.Errorf("sending %s batch: %w", metricName, err)
			}
		}

		if isSleep {
			u.stats.SleepStagesSent += len(batch)
		} else {
			u.stats.MetricPointsSent += len(batch)
		}
	}

	// Collect HR points for workout correlation
	u.hrPoints = append(u.hrPoints, allHRPoints...)

	// Mark files as uploaded
	for _, fi := range newFiles {
		if err := u.state.MarkUploaded(fi.relPath, fi.size, fi.hash); err != nil {
			u.log.Warn("failed to mark uploaded", "file", fi.relPath, "error", err)
		}
		u.stats.FilesUploaded++
	}

	u.log.Info("uploaded metric",
		"metric", metricName,
		"files", len(newFiles),
		"points", len(allPoints),
	)

	return nil
}

// processWorkouts walks Workouts/ and Routes/, converts and uploads them.
func (u *Uploader) processWorkouts(workoutDir, routeDir string) error {
	files, err := filepath.Glob(filepath.Join(workoutDir, "*.hae"))
	if err != nil {
		return err
	}

	var batch []models.HAEWorkout
	var batchFiles []fileInfo

	for _, f := range files {
		u.stats.FilesTotal++

		relPath, _ := filepath.Rel(u.autoSync, f)
		info, err := os.Stat(f)
		if err != nil {
			u.log.Warn("stat failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}

		hash, err := HashFile(f)
		if err != nil {
			u.log.Warn("hash failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}

		uploaded, err := u.state.IsUploaded(relPath, info.Size(), hash)
		if err != nil {
			u.log.Warn("state check failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}
		if uploaded {
			u.stats.FilesSkipped++
			continue
		}

		// Decompress and parse workout
		data, err := decompressLZFSE(f)
		if err != nil {
			u.log.Warn("decompress failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}

		var fileWorkout models.HAEFileWorkout
		if err := json.Unmarshal(data, &fileWorkout); err != nil {
			u.log.Warn("parse failed", "file", f, "error", err)
			u.stats.FilesErrored++
			continue
		}

		// Try to load matching route
		var route *models.HAEFileRoute
		routeFile := filepath.Join(routeDir, fileWorkout.ID+".hae")
		if _, err := os.Stat(routeFile); err == nil {
			routeData, err := decompressLZFSE(routeFile)
			if err != nil {
				u.log.Warn("route decompress failed", "file", routeFile, "error", err)
			} else {
				var r models.HAEFileRoute
				if err := json.Unmarshal(routeData, &r); err != nil {
					u.log.Warn("route parse failed", "file", routeFile, "error", err)
				} else {
					route = &r
				}
			}
		}

		// Convert with route + HR correlation
		workout := convertWorkout(fileWorkout, route, u.hrPoints)

		if route != nil {
			u.stats.RoutePointsSent += len(workout.Route)
		}
		u.stats.HRPointsCorrelated += len(workout.HeartRateData)

		batch = append(batch, workout)
		batchFiles = append(batchFiles, fileInfo{relPath: relPath, size: info.Size(), hash: hash})

		// Send batch of 5 workouts
		if len(batch) >= 5 {
			if err := u.sendWorkoutBatch(batch, batchFiles); err != nil {
				return err
			}
			batch = nil
			batchFiles = nil
		}
	}

	// Send remaining workouts
	if len(batch) > 0 {
		if err := u.sendWorkoutBatch(batch, batchFiles); err != nil {
			return err
		}
	}

	return nil
}

// sendWorkoutBatch sends a batch of workouts and marks their files as uploaded.
func (u *Uploader) sendWorkoutBatch(workouts []models.HAEWorkout, files []fileInfo) error {
	payload := models.HAEPayload{
		Data: models.HAEData{
			Workouts: workouts,
		},
	}

	if u.dryRun {
		u.log.Info("dry-run: would send workouts", "count", len(workouts))
	} else {
		if err := u.client.SendPayload(payload); err != nil {
			return fmt.Errorf("sending workout batch: %w", err)
		}
	}

	u.stats.WorkoutsSent += len(workouts)

	for _, fi := range files {
		if err := u.state.MarkUploaded(fi.relPath, fi.size, fi.hash); err != nil {
			u.log.Warn("failed to mark uploaded", "file", fi.relPath, "error", err)
		}
		u.stats.FilesUploaded++
	}

	return nil
}

// TCPMetric defines a metric to query from the HAE server.
type TCPMetric struct {
	Name      string
	Aggregate bool // true = daily summary, false = raw data points
}

// TCPMetrics is the list of metrics to query individually from the HAE server.
// Querying all metrics at once overwhelms the HAE TCP server, so we query
// one metric per request and let the FreeReps DB merge them.
var TCPMetrics = []TCPMetric{
	{Name: "heart_rate"},
	{Name: "resting_heart_rate"},
	{Name: "heart_rate_variability"},
	{Name: "blood_oxygen_saturation"},
	{Name: "respiratory_rate"},
	{Name: "vo2_max"},
	{Name: "sleep_analysis"},
	{Name: "apple_sleeping_wrist_temperature"},
	{Name: "weight_body_mass"},
	{Name: "body_fat_percentage"},
	{Name: "active_energy", Aggregate: true},
	// "basal_energy_burned" — skipped: ~8 MB/day of estimated BMR data, not useful
	{Name: "apple_exercise_time", Aggregate: true},
}

// RunTCP queries the HAE TCP server for health data and forwards it to FreeReps.
// It processes metrics individually (one per request) and workouts in time-range chunks.
func (u *Uploader) RunTCP(haeHost string, haePort int, start, end time.Time, chunkDays int) (*Stats, error) {
	hae := NewHAEClient(haeHost, haePort)
	chunkDur := time.Duration(chunkDays) * 24 * time.Hour

	// Count total chunks for progress display
	numChunks := 0
	for cs := start; cs.Before(end); cs = cs.Add(chunkDur) {
		numChunks++
	}
	totalSteps := len(TCPMetrics)*numChunks + numChunks // metrics + workouts
	currentStep := 0

	// Phase 1: Health metrics — query each metric individually
	u.log.Info("querying health metrics", "start", start.Format("2006-01-02"), "end", end.Format("2006-01-02"), "chunk_days", chunkDays, "metrics", len(TCPMetrics), "total_requests", totalSteps)

	for _, m := range TCPMetrics {
		for chunkStart := start; chunkStart.Before(end); chunkStart = chunkStart.Add(chunkDur) {
			chunkEnd := chunkStart.Add(chunkDur)
			if chunkEnd.After(end) {
				chunkEnd = end
			}
			currentStep++

			fmt.Fprintf(os.Stderr, "\r[%d/%d] %s %s → %s    ",
				currentStep, totalSteps, m.Name,
				chunkStart.Format("2006-01-02"), chunkEnd.Format("2006-01-02"))


			result, err := hae.QueryMetricsWithRetry(chunkStart, chunkEnd, m.Name, m.Aggregate, u.log)
			if err != nil {
				u.log.Warn("failed to query metric, skipping",
					"metric", m.Name,
					"from", chunkStart.Format("2006-01-02"),
					"to", chunkEnd.Format("2006-01-02"),
					"error", err,
				)
				continue
			}

			if len(result) == 0 || string(result) == "null" {
				u.log.Info("no data", "metric", m.Name)
				continue
			}

			if u.dryRun {
				u.log.Info("dry-run: would forward metric", "metric", m.Name, "bytes", len(result))
			} else {
				if err := u.client.SendRawJSON(result); err != nil {
					return &u.stats, fmt.Errorf("forwarding %s: %w", m.Name, err)
				}
			}

			u.stats.TCPMetricChunks++
			u.stats.TCPBytesSent += int64(len(result))
		}
	}

	// Phase 2: Workouts
	for chunkStart := start; chunkStart.Before(end); chunkStart = chunkStart.Add(chunkDur) {
		chunkEnd := chunkStart.Add(chunkDur)
		if chunkEnd.After(end) {
			chunkEnd = end
		}

		currentStep++

		fmt.Fprintf(os.Stderr, "\r[%d/%d] workouts %s → %s    ",
			currentStep, totalSteps,
			chunkStart.Format("2006-01-02"), chunkEnd.Format("2006-01-02"))


		result, err := hae.QueryWorkoutsWithRetry(chunkStart, chunkEnd, u.log)
		if err != nil {
			u.log.Warn("failed to query workouts, skipping",
				"from", chunkStart.Format("2006-01-02"),
				"to", chunkEnd.Format("2006-01-02"),
				"error", err,
			)
			continue
		}

		if len(result) == 0 || string(result) == "null" {
			u.log.Info("no workout data in chunk")
			continue
		}

		if u.dryRun {
			u.log.Info("dry-run: would forward workouts", "bytes", len(result))
		} else {
			if err := u.client.SendRawJSON(result); err != nil {
				return &u.stats, fmt.Errorf("forwarding workouts: %w", err)
			}
		}

		u.stats.TCPWorkoutChunks++
		u.stats.TCPBytesSent += int64(len(result))
	}

	fmt.Fprintln(os.Stderr)

	// Update sync state
	if !u.dryRun {
		endStr := end.Format("2006-01-02")
		if err := u.state.SetSyncState("tcp_last_metrics_sync", endStr); err != nil {
			u.log.Warn("failed to save metrics sync state", "error", err)
		}
		if err := u.state.SetSyncState("tcp_last_workouts_sync", endStr); err != nil {
			u.log.Warn("failed to save workouts sync state", "error", err)
		}
	}

	return &u.stats, nil
}

// decompressLZFSE decompresses an LZFSE-compressed file using the lzfse CLI tool.
func decompressLZFSE(path string) ([]byte, error) {
	cmd := exec.Command("lzfse", "-decode", "-i", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("lzfse decode %s: %w (stderr: %s)", path, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// ResolveAutoSync resolves the AutoSync directory from a user-provided path.
// If the path contains an AutoSync subdirectory, returns its path.
// Otherwise returns the original path.
func ResolveAutoSync(path string) string {
	if filepath.Base(path) == "AutoSync" {
		return path
	}
	candidate := filepath.Join(path, "AutoSync")
	if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
		return candidate
	}
	// Check common iCloud path patterns
	parts := strings.Split(path, string(filepath.Separator))
	for i, part := range parts {
		if part == "AutoSync" {
			return filepath.Join(string(filepath.Separator), filepath.Join(parts[:i+1]...))
		}
	}
	return path
}
