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
	"github.com/claude/freereps/internal/demo"
	"github.com/claude/freereps/internal/ingest/alpha"
	"github.com/claude/freereps/internal/ingest/health"
	freerepsmcp "github.com/claude/freereps/internal/mcp"
	"github.com/claude/freereps/internal/oura"
	"github.com/claude/freereps/internal/server"
	"github.com/claude/freereps/internal/storage"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"tailscale.com/tsnet"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	migrateOnly := flag.Bool("migrate-only", false, "run migrations and exit")
	mcpMode := flag.Bool("mcp", false, "run as MCP server over stdio (for Claude Code integration)")
	demoMode := flag.Bool("demo", false, "seed database with demo data for testing")
	flag.Parse()

	// In MCP stdio mode, logs go to stderr to keep stdout clean for JSON-RPC.
	logOutput := os.Stdout
	if *mcpMode {
		logOutput = os.Stderr
	}
	log := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{Level: slog.LevelInfo}))
	log.Info("FreeReps starting", "version", Version)

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Run migrations (skip in MCP stdio mode — DB is managed by the server)
	dsn := cfg.Database.DSN()
	if !*mcpMode {
		if err := storage.RunMigrations(dsn, "migrations"); err != nil {
			log.Error("migration failed", "error", err)
			os.Exit(1)
		}
		log.Info("migrations applied")
	}

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
	db.SetSourcePriority(cfg.SourcePriority)
	log.Info("database connected")

	// Backfill sleep sessions from stages (idempotent — ON CONFLICT DO NOTHING)
	if err := db.BackfillSleepSessions(ctx, log); err != nil {
		log.Warn("sleep session backfill failed", "error", err)
	}

	// Seed demo data if requested (via -demo flag or FREEREPS_DEMO=true env var)
	if *demoMode || os.Getenv("FREEREPS_DEMO") == "true" {
		if err := demo.Seed(ctx, db, log); err != nil {
			log.Error("demo seed failed", "error", err)
			os.Exit(1)
		}
	}

	// MCP stdio mode: serve MCP protocol over stdin/stdout, then exit
	if *mcpMode {
		log.Info("starting MCP stdio server")
		mcpSrv := freerepsmcp.New(db, Version, log)
		if err := mcpserver.ServeStdio(mcpSrv,
			mcpserver.WithStdioContextFunc(func(ctx context.Context) context.Context {
				return freerepsmcp.WithUserID(ctx, 1)
			}),
		); err != nil {
			log.Error("MCP stdio server error", "error", err)
			os.Exit(1)
		}
		return
	}

	// Create providers
	healthProvider := health.NewProvider(db, log)
	alphaProvider := alpha.NewProvider(db, log)

	// Create server
	server.Version = Version
	srv := server.New(db, healthProvider, alphaProvider, log)

	// Start Oura sync if enabled
	if cfg.Oura.Enabled {
		ouraClient := oura.NewClient()
		tokenMgr := oura.NewTokenManager(cfg.Oura.ClientID, cfg.Oura.ClientSecret, db)
		syncer := oura.NewSyncer(ouraClient, tokenMgr, db, cfg.Oura, log)

		syncCtx, syncCancel := context.WithCancel(ctx)
		defer syncCancel()
		go syncer.Run(syncCtx)

		srv.SetOura(tokenMgr, syncer)
		log.Info("oura sync started", "interval", cfg.Oura.SyncInterval)
	}

	// Mount MCP SSE server
	mcpSrv := freerepsmcp.New(db, Version, log)
	srv.SetMCP(mcpSrv)

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
		defer func() { _ = tsServer.Close() }()

		lc, err := tsServer.LocalClient()
		if err != nil {
			log.Error("tsnet local client failed", "error", err)
			os.Exit(1)
		}
		srv.SetTailscale(lc)

		listener, err = tsServer.ListenTLS("tcp", ":443")
		if err != nil {
			log.Error("tsnet listen failed", "error", err)
			os.Exit(1)
		}
		log.Info("tsnet server starting", "hostname", cfg.Tailscale.Hostname, "tls", true)
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
