package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	freereps "github.com/claude/freereps"
	"github.com/claude/freereps/internal/config"
	"github.com/claude/freereps/internal/ingest/alpha"
	"github.com/claude/freereps/internal/ingest/hae"
	"github.com/claude/freereps/internal/server"
	"github.com/claude/freereps/internal/storage"
	"tailscale.com/tsnet"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	migrateOnly := flag.Bool("migrate-only", false, "run migrations and exit")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	log.Info("FreeReps starting", "version", Version)

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Run migrations
	dsn := cfg.Database.DSN()
	if err := storage.RunMigrations(dsn, "migrations"); err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}
	log.Info("migrations applied")

	if *migrateOnly {
		log.Info("migrate-only: exiting")
		return
	}

	// Connect database
	ctx := context.Background()
	db, err := storage.New(ctx, dsn)
	if err != nil {
		log.Error("failed to connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("database connected")

	// Backfill sleep sessions from stages (idempotent — ON CONFLICT DO NOTHING)
	if err := db.BackfillSleepSessions(ctx, log); err != nil {
		log.Warn("sleep session backfill failed", "error", err)
	}

	// Create providers
	haeProvider := hae.NewProvider(db, log)
	alphaProvider := alpha.NewProvider(db, log)

	// Create server
	srv := server.New(db, haeProvider, alphaProvider, log)

	// Serve embedded frontend
	webDist, err := fs.Sub(freereps.WebFS, "web/dist")
	if err != nil {
		log.Error("failed to load embedded frontend", "error", err)
		os.Exit(1)
	}
	srv.SetFrontend(webDist)

	// Start server — tsnet or plain HTTP
	var listener net.Listener
	var tsServer *tsnet.Server

	if cfg.Tailscale.Enabled {
		tsServer = &tsnet.Server{
			Hostname: cfg.Tailscale.Hostname,
			Dir:      cfg.Tailscale.StateDir,
		}
		if err := tsServer.Start(); err != nil {
			log.Error("tsnet start failed", "error", err)
			os.Exit(1)
		}
		defer tsServer.Close()

		lc, err := tsServer.LocalClient()
		if err != nil {
			log.Error("tsnet local client failed", "error", err)
			os.Exit(1)
		}
		srv.SetTailscale(lc)

		listener, err = tsServer.Listen("tcp", ":80")
		if err != nil {
			log.Error("tsnet listen failed", "error", err)
			os.Exit(1)
		}
		log.Info("tsnet server starting", "hostname", cfg.Tailscale.Hostname)
	} else {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			log.Error("listen failed", "addr", addr, "error", err)
			os.Exit(1)
		}
		log.Info("server starting", "addr", addr, "mode", "dev (no tailscale)")
	}

	httpSrv := &http.Server{Handler: srv}

	go func() {
		if err := httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("shutting down", "signal", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}
	log.Info("server stopped")
}
