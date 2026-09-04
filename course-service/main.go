// Package main is the entry point for the course-service HTTP API, which
// serves course and module content backed by PostgreSQL and Git.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"gorm.io/gorm"

	"github.com/genesary/pupitre/course-service/internal/config"
	coursedb "github.com/genesary/pupitre/course-service/internal/db"
	"github.com/genesary/pupitre/course-service/internal/handlers"
	"github.com/genesary/pupitre/course-service/internal/repository"
)

const (
	// readTimeout bounds how long the server waits to read an entire request.
	readTimeout = 30 * time.Second
	// writeTimeout bounds how long the server waits to write a response.
	writeTimeout = 60 * time.Second
	// idleTimeout bounds how long the server keeps idle keep-alive connections.
	idleTimeout = 120 * time.Second
	// shutdownTimeout bounds how long graceful shutdown waits for requests.
	shutdownTimeout = 15 * time.Second
	// dbConnectAttempts is how many times startup retries the initial
	// database connection before giving up.
	dbConnectAttempts = 10
	// dbConnectRetryDelay is the wait between initial connection attempts.
	dbConnectRetryDelay = 3 * time.Second
)

// main starts the course-service HTTP API server.
//
// @title          Course Service API
// @version        1.0.0
// @description    Stateless micro-service for course/module content delivery.
// @host           localhost:8082
// @BasePath       /
// @securityDefinitions.apikey BearerAuth
// @in             header
// @name           Authorization.
func main() {
	logger := initLogger()
	zap.ReplaceGlobals(logger)

	err := run()
	if err != nil {
		zap.L().Error("fatal startup error", zap.Error(err))

		_ = logger.Sync()

		os.Exit(1)
	}

	_ = logger.Sync()
}

// run wires up the database and HTTP server, then blocks until a shutdown
// signal is received. Keeping this separate from main lets deferred
// cleanup run to completion instead of being skipped by a direct
// [os.Exit] call.
func run() error {
	zap.L().Info("starting course-service")

	// ── Configuration ────────────────────────────────────────────────────────
	zap.L().Info("loading configuration")

	cfg, warnings := config.Load()

	for _, w := range warnings {
		zap.L().Warn("configuration warning", zap.String("detail", w))
	}

	logConfig(cfg)

	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required: course content is served from the database")
	}

	ctx := context.Background()

	// ── Database ──────────────────────────────────────────────────────────────
	zap.L().Info("connecting to database")

	gdb, err := connectWithRetry(ctx, cfg)
	if err != nil {
		return err
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return fmt.Errorf("unwrap sql.DB: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	err = coursedb.RunMigrations(ctx, gdb)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	zap.L().Info("database connected and schema migrated")

	// ── Dev seed ──────────────────────────────────────────────────────────────
	// No-op unless SEED_DEV_COURSES is set; see internal/db/seed_dev.go.
	repos := repository.NewGormRepositories(gdb)

	err = seedDev(ctx, repos)
	if err != nil {
		return err
	}

	// ── HTTP router ───────────────────────────────────────────────────────────
	zap.L().Info("building HTTP router")

	state := handlers.NewState(cfg, repos)
	router := handlers.BuildRouter(state, cfg, true)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	return serve(srv)
}

// connectWithRetry opens the database connection, retrying a bounded
// number of times so that starting alongside a Postgres pod that is not
// ready yet is a slow start rather than a crash loop.
func connectWithRetry(ctx context.Context, cfg *config.Config) (*gorm.DB, error) {
	var lastErr error

	for attempt := 1; attempt <= dbConnectAttempts; attempt++ {
		gdb, err := coursedb.Connect(ctx, cfg.DatabaseURL, cfg.DBMaxOpenConns, cfg.DBMaxIdleConns)
		if err == nil {
			return gdb, nil
		}

		lastErr = err

		zap.L().Warn("database not ready, retrying",
			zap.Int("attempt", attempt), zap.Int("of", dbConnectAttempts), zap.Error(err))

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("connect to database: %w", ctx.Err())
		case <-time.After(dbConnectRetryDelay):
		}
	}

	return nil, fmt.Errorf("connect to database after %d attempts: %w", dbConnectAttempts, lastErr)
}

// serve starts srv in the background, blocks until an interrupt/terminate
// signal is received or the server exits unexpectedly, then gracefully
// shuts srv down.
func serve(srv *http.Server) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serveErr := make(chan error, 1)

	go func() {
		zap.L().Info("API listening", zap.String("addr", srv.Addr))

		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("server error: %w", err)

			return
		}

		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-quit:
	}

	zap.L().Info("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	err := srv.Shutdown(shutCtx)
	if err != nil {
		return fmt.Errorf("forced shutdown: %w", err)
	}

	zap.L().Info("server stopped")

	return nil
}

// seedDev runs the dev-mode seed operations for courses and learning paths.
// It is a no-op unless SEED_DEV_COURSES is set; extracted to keep run() within
// the project's function-length limit.
func seedDev(ctx context.Context, repos *repository.Repositories) error {
	mode := os.Getenv("SEED_DEV_COURSES")

	err := coursedb.SeedDevCourses(ctx, repos.Courses, mode)
	if err != nil {
		return fmt.Errorf("seed dev courses: %w", err)
	}

	err = coursedb.SeedDevPaths(ctx, repos.Paths, mode)
	if err != nil {
		return fmt.Errorf("seed dev paths: %w", err)
	}

	return nil
}

// logConfig logs the effective (post-override) configuration values,
// omitting secrets and connection strings.
func logConfig(cfg *config.Config) {
	zap.L().Info("configuration loaded",
		zap.Int("port", cfg.Port),
		zap.Bool("databaseConfigured", cfg.DatabaseURL != ""),
		zap.Int("dbMaxOpenConns", cfg.DBMaxOpenConns),
		zap.Int("dbMaxIdleConns", cfg.DBMaxIdleConns),
		zap.String("userServiceURL", cfg.UserServiceURL),
		zap.String("checkerServiceURL", cfg.CheckerServiceURL),
		zap.Int("gitCacheTTLMin", cfg.GitCacheTTL),
		zap.Int("corsOriginsCount", len(cfg.CORSOrigins)),
	)
}

// initLogger builds a zap logger driven by two environment variables:
//   - LOG_LEVEL  — debug/info/warn/error (default: info)
//   - LOG_FORMAT — json (default) or text (human-readable, for local dev)
func initLogger() *zap.Logger {
	level := zap.NewAtomicLevel()
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		_ = level.UnmarshalText([]byte(lvl))
	}

	var cfg zap.Config
	if os.Getenv("LOG_FORMAT") == "text" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	cfg.Level = level

	return zap.Must(cfg.Build())
}
