package server

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

	checkerhandlers "github.com/jojo91550/pupitre2/apps/api/internal/checker/handlers"
	coursesdb "github.com/jojo91550/pupitre2/apps/api/internal/courses/db"
	courseshandlers "github.com/jojo91550/pupitre2/apps/api/internal/courses/handlers"
	coursesrepo "github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/config"
	usersdb "github.com/jojo91550/pupitre2/apps/api/internal/users/db"
	usershandlers "github.com/jojo91550/pupitre2/apps/api/internal/users/handlers"
	usersrepo "github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

const (
	// readHeaderTimeout bounds how long the server waits for request headers.
	readHeaderTimeout = 10 * time.Second
	// readTimeout bounds how long the server waits to read an entire request.
	readTimeout = 30 * time.Second
	// writeTimeout bounds how long the server waits to write a response.
	writeTimeout = 60 * time.Second
	// idleTimeout bounds how long the server keeps idle keep-alive connections.
	idleTimeout = 120 * time.Second
	// shutdownTimeout bounds how long graceful shutdown waits for requests.
	shutdownTimeout = 15 * time.Second
	// dbConnectAttempts is how many times start-up retries the initial
	// database connection before giving up.
	dbConnectAttempts = 10
	// dbConnectRetryDelay is the wait between initial connection attempts.
	dbConnectRetryDelay = 3 * time.Second
	// seedMockDataValue is the SEED_MOCK_DATA value that seeds demo users.
	seedMockDataValue = "true"
)

// Run loads the configuration, prepares the database, wires the domains and
// serves until SIGINT or SIGTERM, then shuts down gracefully.
func Run() error {
	zap.L().Info("starting pupitre-api")

	cfg, warnings, err := config.Load()
	for _, w := range warnings {
		zap.L().Warn("configuration warning", zap.String("detail", w))
	}

	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logConfig(cfg)

	ctx := context.Background()

	gdb, err := connectWithRetry(ctx, cfg)
	if err != nil {
		return err
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return fmt.Errorf("unwrap sql.DB: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	usersRepos := usersrepo.NewGormRepositories(gdb)
	coursesRepos := coursesrepo.NewGormRepositories(gdb)

	err = prepareDatabase(ctx, gdb, usersRepos, cfg)
	if err != nil {
		return err
	}

	watchCtx, watchCancel := context.WithCancel(ctx)
	defer watchCancel()

	if cfg.Users.AdminPasswordFile != "" {
		zap.L().Info("watching admin password file", zap.String("path", cfg.Users.AdminPasswordFile))

		go usersdb.WatchAdminPassword(watchCtx, usersRepos.Users, cfg.Users.AdminPasswordFile)
	}

	domains := Wire(cfg, usersRepos, coursesRepos)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           BuildRouter(domains, cfg, true),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	return serve(srv)
}

// Wire builds the request handlers of every domain and connects them to
// each other through the in-process adapters.
func Wire(cfg *config.Config, usersRepos *usersrepo.Repositories, coursesRepos *coursesrepo.Repositories) Domains {
	users := &usershandlers.State{Repos: usersRepos, Config: cfg.Users}
	courses := courseshandlers.NewState(cfg.Courses, coursesRepos)

	users.Courses = &coursesSource{courses: courses}
	users.Sessions = users.CheckSession
	courses.Sessions = users.CheckSession
	courses.Users = &usersProgress{users: users}
	courses.Checker = newLabChecker(checkerhandlers.New(cfg.Checker),
		cfg.Checker.RateLimitRequests, cfg.Checker.RateLimitWindowSeconds)

	return Domains{Users: users, Courses: courses}
}

// prepareDatabase migrates the schema of both domains, then seeds it: the
// demo catalogue first, so demo progress can refer to demo courses, then
// the admin account, the OIDC settings and the demo users
// (docs/specs/102-backend-data-and-migrations.md).
func prepareDatabase(ctx context.Context, gdb *gorm.DB, usersRepos *usersrepo.Repositories, cfg *config.Config) error {
	err := Migrate(ctx, gdb)
	if err != nil {
		return err
	}

	err = seedDevCatalogue(ctx, gdb)
	if err != nil {
		return err
	}

	err = usersdb.SeedAdmin(ctx, usersRepos.Users, cfg.Users.AdminPassword)
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}

	err = usersdb.SeedOIDC(ctx, usersRepos.Settings, cfg.Users)
	if err != nil {
		return fmt.Errorf("seed OIDC settings: %w", err)
	}

	if os.Getenv("SEED_MOCK_DATA") == seedMockDataValue {
		err := usersdb.SeedMockData(ctx, usersRepos.Users, usersRepos.Enrollments)
		if err != nil {
			zap.L().Error("failed to seed mock data", zap.Error(err))
		}
	}

	return nil
}

// Migrate runs the migrations of the users domain, then of the courses
// domain. Each runs its breaking migrations (tracked in the shared
// _schema_migrations table), its AutoMigrate and its constraints, exactly
// as its service did against the same database.
func Migrate(ctx context.Context, gdb *gorm.DB) error {
	err := usersdb.RunMigrations(ctx, gdb)
	if err != nil {
		return fmt.Errorf("run users migrations: %w", err)
	}

	err = coursesdb.RunMigrations(ctx, gdb)
	if err != nil {
		return fmt.Errorf("run courses migrations: %w", err)
	}

	zap.L().Info("schema migrated")

	return nil
}

// seedDevCatalogue seeds the demo courses and paths in one transaction. It
// is a no-op unless SEED_DEV_COURSES is set.
func seedDevCatalogue(ctx context.Context, gdb *gorm.DB) error {
	mode := os.Getenv("SEED_DEV_COURSES")

	err := gdb.Transaction(func(tx *gorm.DB) error {
		txRepos := coursesrepo.NewGormRepositories(tx)

		seedErr := coursesdb.SeedDevCourses(ctx, txRepos.Courses, mode)
		if seedErr != nil {
			return fmt.Errorf("seed dev courses: %w", seedErr)
		}

		seedErr = coursesdb.SeedDevPaths(ctx, txRepos.Paths, mode)
		if seedErr != nil {
			return fmt.Errorf("seed dev paths: %w", seedErr)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("seed dev transaction: %w", err)
	}

	return nil
}

// connectWithRetry opens the database connection, retrying a bounded number
// of times so that starting alongside a database that is not ready yet is a
// slow start rather than a crash loop.
func connectWithRetry(ctx context.Context, cfg *config.Config) (*gorm.DB, error) {
	var lastErr error

	for attempt := 1; attempt <= dbConnectAttempts; attempt++ {
		gdb, err := coursesdb.Connect(ctx, cfg.Users.DatabaseURL, cfg.Users.DBMaxOpenConns, cfg.Users.DBMaxIdleConns)
		if err == nil {
			zap.L().Info("database connected")

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

	defer signal.Stop(quit)

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

// logConfig logs the effective configuration, omitting secrets and
// connection strings.
func logConfig(cfg *config.Config) {
	zap.L().Info("configuration loaded",
		zap.Int("port", cfg.Port),
		zap.Bool("databaseConfigured", cfg.Users.DatabaseURL != ""),
		zap.Int("dbMaxOpenConns", cfg.Users.DBMaxOpenConns),
		zap.Int("dbMaxIdleConns", cfg.Users.DBMaxIdleConns),
		zap.Bool("oidcEnabled", cfg.Users.OIDC.Enabled),
		zap.Int("providersCount", len(cfg.Users.Providers)),
		zap.Int("corsOriginsCount", len(cfg.Users.CORSOrigins)),
		zap.Int("gitCacheTTLMin", cfg.Courses.GitCacheTTL),
		zap.String("gitlabBaseURL", cfg.Checker.GitLabBaseURL),
		zap.Bool("gitlabTokenConfigured", cfg.Checker.GitLabToken != ""),
		zap.Int("labCheckRateLimitRequests", cfg.Checker.RateLimitRequests),
		zap.Int("labCheckRateLimitWindowSeconds", cfg.Checker.RateLimitWindowSeconds),
	)
}
