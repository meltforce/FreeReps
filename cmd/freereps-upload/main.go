package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/claude/freereps/internal/upload"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	serverURL := flag.String("server", "", "FreeReps server URL (e.g. https://freereps.tail1234.ts.net)")
	autoSyncPath := flag.String("path", "", "path to AutoSync directory (or parent containing AutoSync/)")
	dryRun := flag.Bool("dry-run", false, "parse and convert but don't send to server")
	batchSize := flag.Int("batch-size", 2000, "data points per metric payload")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("freereps-upload", Version)
		return
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *autoSyncPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: freereps-upload -server <URL> -path <AutoSync dir> [-dry-run] [-batch-size N]\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *serverURL == "" && !*dryRun {
		fmt.Fprintf(os.Stderr, "Error: -server is required (or use -dry-run)\n")
		os.Exit(1)
	}

	// Strip trailing slash from server URL
	*serverURL = strings.TrimRight(*serverURL, "/")

	// Resolve AutoSync directory
	autoSync := upload.ResolveAutoSync(*autoSyncPath)
	info, err := os.Stat(autoSync)
	if err != nil || !info.IsDir() {
		log.Error("AutoSync directory not found", "path", autoSync, "original", *autoSyncPath)
		os.Exit(1)
	}
	log.Info("using AutoSync directory", "path", autoSync)

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
	defer state.Close()

	// Create client (nil-safe in dry-run mode)
	var client *upload.Client
	if !*dryRun {
		client = upload.NewClient(*serverURL)
	}

	if *dryRun {
		log.Info("DRY RUN mode — files will be parsed and converted but not sent")
	}

	// Run upload
	uploader := upload.New(client, state, autoSync, *dryRun, *batchSize, log)
	stats, err := uploader.Run()
	if err != nil {
		log.Error("upload failed", "error", err)
		printStats(log, stats)
		os.Exit(1)
	}

	printStats(log, stats)
	log.Info("upload complete")
}

func printStats(log *slog.Logger, stats *upload.Stats) {
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
