package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/claude/freereps/internal/config"
	"github.com/claude/freereps/internal/importer"
	"github.com/claude/freereps/internal/storage"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	autoSyncPath := flag.String("path", "", "path to AutoSync directory (required)")
	dryRun := flag.Bool("dry-run", false, "report counts without inserting into database")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *autoSyncPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: freereps-import -config config.yaml -path /path/to/AutoSync [-dry-run]\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Verify AutoSync directory exists
	info, err := os.Stat(*autoSyncPath)
	if err != nil || !info.IsDir() {
		log.Error("AutoSync path does not exist or is not a directory", "path", *autoSyncPath)
		os.Exit(1)
	}

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	dsn := cfg.Database.DSN()

	// Run migrations
	if err := storage.RunMigrations(dsn, "migrations"); err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}
	log.Info("migrations applied")

	ctx := context.Background()

	if *dryRun {
		log.Info("DRY RUN mode — no data will be written to the database")
	}

	// Connect database
	db, err := storage.New(ctx, dsn)
	if err != nil {
		log.Error("failed to connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("database connected")

	// Run import
	imp := importer.New(db, log, *dryRun)
	stats, err := imp.Import(ctx, *autoSyncPath)
	if err != nil {
		log.Error("import failed", "error", err)
		printStats(log, stats)
		os.Exit(1)
	}

	printStats(log, stats)
	log.Info("import complete")
}

func printStats(log *slog.Logger, stats *importer.Stats) {
	log.Info("import stats",
		"files_processed", stats.FilesProcessed,
		"files_skipped", stats.FilesSkipped,
		"files_errored", stats.FilesErrored,
		"metrics_inserted", stats.MetricsInserted,
		"metrics_duplicated", stats.MetricsDuplicated,
		"sleep_stages_inserted", stats.SleepStagesInserted,
		"workouts_inserted", stats.WorkoutsInserted,
		"workouts_duplicated", stats.WorkoutsDuplicated,
		"route_points_inserted", stats.RoutePointsInserted,
		"hr_correlated", stats.HRCorrelated,
	)
	if len(stats.RejectedMetrics) > 0 {
		log.Info("rejected metrics (not in allowlist)", "metrics", stats.RejectedMetrics)
	}
}
