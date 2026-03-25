package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool.Pool and provides repository methods.
type DB struct {
	Pool           *pgxpool.Pool
	SourcePriority []string

	// Available metrics cache (per user_id, bounded).
	availMetricsMu    sync.RWMutex
	availMetricsCache map[int]*availMetricsCacheEntry
}

const (
	availMetricsCacheTTL     = 5 * time.Minute
	availMetricsCacheMaxSize = 64
)

type availMetricsCacheEntry struct {
	metrics   []AllowedMetric
	fetchedAt time.Time
}

// SetSourcePriority configures the source priority list used for query-time
// deduplication. Lower index = higher priority. Sources not in the list get
// the lowest priority.
func (db *DB) SetSourcePriority(priorities []string) {
	db.SourcePriority = priorities
}

// New creates a new DB with a connection pool.
func New(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing pool config: %w", err)
	}
	cfg.MaxConns = 16
	cfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Close closes the connection pool.
func (db *DB) Close() {
	db.Pool.Close()
}

// RunMigrations applies all pending migrations from the given directory.
func RunMigrations(dsn, migrationsPath string) error {
	m, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}
