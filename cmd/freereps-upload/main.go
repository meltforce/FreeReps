package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/claude/freereps/internal/upload"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	// Common flags
	serverURL := flag.String("server", "", "FreeReps server URL (e.g. https://freereps.tail1234.ts.net)")
	dryRun := flag.Bool("dry-run", false, "parse and convert but don't send to server")
	version := flag.Bool("version", false, "print version and exit")

	// File mode flags
	autoSyncPath := flag.String("path", "", "path to AutoSync directory (file mode)")
	batchSize := flag.Int("batch-size", 2000, "data points per metric payload (file mode)")

	// TCP mode flags
	haeHost := flag.String("hae-host", "", "HAE TCP server IP address (TCP mode)")
	haePort := flag.Int("hae-port", 9000, "HAE TCP server port")
	startDate := flag.String("start", "", "start date for backfill (yyyy-MM-dd, default: 1 year ago)")
	endDate := flag.String("end", "", "end date (yyyy-MM-dd, default: today)")
	chunkDays := flag.Int("chunk-days", 1, "days per query chunk (TCP mode)")
	flag.Parse()

	if *version {
		fmt.Println("freereps-upload", Version)
		return
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Mode selection
	if *haeHost == "" && *autoSyncPath == "" {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  TCP mode:  freereps-upload -hae-host <IP> -server <URL> [-start yyyy-MM-dd] [-end yyyy-MM-dd] [-chunk-days N]\n")
		fmt.Fprintf(os.Stderr, "  File mode: freereps-upload -path <AutoSync dir> -server <URL> [-batch-size N]\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *serverURL == "" && !*dryRun {
		fmt.Fprintf(os.Stderr, "Error: -server is required (or use -dry-run)\n")
		os.Exit(1)
	}

	// Strip trailing slash from server URL
	*serverURL = strings.TrimRight(*serverURL, "/")

	// Open state database
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error("failed to get home directory", "error", err)
		os.Exit(1)
	}
	stateDir := filepath.Join(homeDir, ".freereps-upload")

	state, err := upload.OpenStateDB(stateDir)
	if err != nil {
		log.Error("failed to open state database", "error", err)
		os.Exit(1)
	}
	defer state.Close() //nolint:errcheck

	// Create client (nil in dry-run mode)
	var client *upload.Client
	if !*dryRun {
		client = upload.NewClient(*serverURL)
	}

	if *dryRun {
		log.Info("DRY RUN mode — data will be fetched but not sent")
	}

	if *haeHost != "" {
		// TCP server mode
		start, end := parseDateRange(*startDate, *endDate, state, log)

		uploader := upload.New(client, state, "", *dryRun, 0, log)
		stats, err := uploader.RunTCP(*haeHost, *haePort, start, end, *chunkDays)
		if err != nil {
			log.Error("TCP upload failed", "error", err)
			printTCPStats(stats)
			os.Exit(1)
		}

		printTCPStats(stats)
		log.Info("TCP upload complete")
	} else {
		// File mode
		autoSync := upload.ResolveAutoSync(*autoSyncPath)
		info, err := os.Stat(autoSync)
		if err != nil || !info.IsDir() {
			log.Error("AutoSync directory not found", "path", autoSync, "original", *autoSyncPath)
			os.Exit(1)
		}
		log.Info("using AutoSync directory", "path", autoSync)

		uploader := upload.New(client, state, autoSync, *dryRun, *batchSize, log)
		stats, err := uploader.Run()
		if err != nil {
			log.Error("upload failed", "error", err)
			printFileStats(stats)
			os.Exit(1)
		}

		printFileStats(stats)
		log.Info("upload complete")
	}
}

// parseDateRange parses start/end flags, falling back to sync state or defaults.
func parseDateRange(startStr, endStr string, state *upload.StateDB, log *slog.Logger) (time.Time, time.Time) {
	now := time.Now()
	end := now

	if endStr != "" {
		t, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			log.Error("invalid -end date (expected yyyy-MM-dd)", "value", endStr)
			os.Exit(1)
		}
		// Set to end of day
		end = t.Add(24*time.Hour - time.Second)
	}

	var start time.Time
	if startStr != "" {
		t, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			log.Error("invalid -start date (expected yyyy-MM-dd)", "value", startStr)
			os.Exit(1)
		}
		start = t
	} else {
		// Check sync state for last successful sync
		lastSync, err := state.GetSyncState("tcp_last_metrics_sync")
		if err == nil && lastSync != "" {
			t, err := time.Parse("2006-01-02", lastSync)
			if err == nil {
				start = t
				log.Info("resuming from last sync", "date", lastSync)
			}
		}
		// Default: 1 year ago
		if start.IsZero() {
			start = now.AddDate(-1, 0, 0)
		}
	}

	return start, end
}

func printTCPStats(stats *upload.Stats) {
	fmt.Println()
	fmt.Println("=== TCP Upload Summary ===")
	fmt.Printf("  Metric chunks:    %d\n", stats.TCPMetricChunks)
	fmt.Printf("  Workout chunks:   %d\n", stats.TCPWorkoutChunks)
	fmt.Printf("  Bytes forwarded:  %d\n", stats.TCPBytesSent)
	fmt.Println()
}

func printFileStats(stats *upload.Stats) {
	fmt.Println()
	fmt.Println("=== Upload Summary ===")
	fmt.Printf("  Files total:      %d\n", stats.FilesTotal)
	fmt.Printf("  Files uploaded:   %d\n", stats.FilesUploaded)
	fmt.Printf("  Files skipped:    %d (already uploaded)\n", stats.FilesSkipped)
	fmt.Printf("  Files errored:    %d\n", stats.FilesErrored)
	fmt.Println()
	fmt.Printf("  Metric points:    %d\n", stats.MetricPointsSent)
	fmt.Printf("  Sleep stages:     %d\n", stats.SleepStagesSent)
	fmt.Printf("  Workouts:         %d\n", stats.WorkoutsSent)
	fmt.Printf("  Route points:     %d\n", stats.RoutePointsSent)
	fmt.Printf("  HR correlated:    %d\n", stats.HRPointsCorrelated)

	if len(stats.RejectedMetrics) > 0 {
		fmt.Printf("\n  Rejected metrics (not in allowlist):\n")
		for _, m := range stats.RejectedMetrics {
			fmt.Printf("    - %s\n", m)
		}
	}
	fmt.Println()
}
