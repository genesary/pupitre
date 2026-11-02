// Package db provides the database connection and schema management used
// by course-service to persist optional lab result tracking data.
package db

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// Connect opens a GORM connection to connURL and verifies connectivity with
// a ping before returning it. maxOpenConns and maxIdleConns cap the
// connection pool size.
//
// Three settings past the pool size matter under sustained load:
//
//   - PrepareStmt caches a prepared statement per distinct SQL string, so
//     the repeated queries that make up a request are parsed and planned
//     once per connection rather than on every execution.
//   - ConnMaxLifetime recycles connections, which keeps a long-lived pool
//     from pinning itself to backends that a failover or a rolling restart
//     has moved on from.
//   - ConnMaxIdleTime returns connections the service is no longer using,
//     so an idle replica does not hold a share of the database's
//     connection limit away from a busy one.
func Connect(ctx context.Context, connURL string, maxOpenConns, maxIdleConns int) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(connURL), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(gormlogger.Warn),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("unwrap sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	err = sqlDB.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return gdb, nil
}

// connMaxLifetime bounds how long a pooled connection is reused before
// being closed and reopened.
const connMaxLifetime = 30 * time.Minute

// connMaxIdleTime bounds how long an unused pooled connection is held.
const connMaxIdleTime = 5 * time.Minute

// allModels lists every GORM model whose table AutoMigrate manages.
//
//nolint:gochecknoglobals // static migration configuration, populated once at init
var allModels = []any{
	&models.Course{},
	&models.CourseModule{},
	&models.CoursePrerequisite{},
	&models.CourseSession{},
	&models.Path{},
	&models.PathCourse{},
	&models.PathSkill{},
	&models.QuizQuestionAttempt{},
	&models.LabCheck{},
}

// RunMigrations brings the database schema up to date. Schema management is
// split in two:
//
//   - Additive changes (new tables, columns, indexes) are handled entirely
//     by GORM's AutoMigrate, driven by the `gorm` tags on internal/models
//     structs. This runs on every startup and is safe to run repeatedly.
//   - Breaking changes that AutoMigrate cannot express (renaming/dropping a
//     column, transforming data, splitting a table) go in breakingMigrations
//     below, tracked in _schema_migrations so each runs exactly once. The
//     list is empty today — there is no stable release yet, so the schema is
//     still free to be reshaped directly through model changes. Once a
//     breaking change is needed after a stable release, add an entry there
//     instead of hand-editing already-deployed columns.
func RunMigrations(ctx context.Context, gdb *gorm.DB) error {
	err := applyBreakingMigrations(ctx, gdb)
	if err != nil {
		return err
	}

	err = gdb.WithContext(ctx).AutoMigrate(allModels...)
	if err != nil {
		return fmt.Errorf("auto-migrate schema: %w", err)
	}

	zap.L().Info("schema migrated")

	return nil
}

// breakingMigration is a one-off Go function for a schema change AutoMigrate
// cannot express safely (column rename/drop, type transform, data backfill).
// Each runs at most once, tracked by Name in _schema_migrations.
type breakingMigration struct {
	Name  string
	Apply func(ctx context.Context, gdb *gorm.DB) error
}

// breakingMigrations is empty for now — see the RunMigrations doc comment.
// To add one:
//
//	{
//		Name: "2026xxxx_rename_foo_to_bar",
//		Apply: func(ctx context.Context, gdb *gorm.DB) error {
//			return gdb.WithContext(ctx).Migrator().RenameColumn(&models.X{}, "foo", "bar")
//		},
//	}
//
//nolint:gochecknoglobals // static migration configuration, populated once at init
var breakingMigrations = []breakingMigration{}

// applyBreakingMigrations runs any breakingMigrations entries not yet
// recorded in _schema_migrations, in slice order, each in its own
// transaction so a failure partway through doesn't record it as applied.
func applyBreakingMigrations(ctx context.Context, gdb *gorm.DB) error {
	err := ensureSchemaMigrationsTable(ctx, gdb)
	if err != nil {
		return err
	}

	applied, err := loadAppliedSchemaMigrations(ctx, gdb)
	if err != nil {
		return err
	}

	for _, migration := range breakingMigrations {
		if applied[migration.Name] {
			continue
		}

		err := gdb.WithContext(ctx).Transaction(func(migrationTx *gorm.DB) error {
			applyErr := migration.Apply(ctx, migrationTx)
			if applyErr != nil {
				return fmt.Errorf("apply %s: %w", migration.Name, applyErr)
			}

			return migrationTx.Exec("INSERT INTO _schema_migrations (name) VALUES (?)", migration.Name).Error
		})
		if err != nil {
			return fmt.Errorf("breaking migration %s: %w", migration.Name, err)
		}

		zap.L().Info("breaking migration applied", zap.String("name", migration.Name))
	}

	return nil
}

// ensureSchemaMigrationsTable creates the _schema_migrations bookkeeping
// table if it does not already exist.
func ensureSchemaMigrationsTable(ctx context.Context, gdb *gorm.DB) error {
	err := gdb.WithContext(ctx).Exec(`
		CREATE TABLE IF NOT EXISTS _schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`).Error
	if err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	return nil
}

// loadAppliedSchemaMigrations returns the set of breaking-migration names
// already recorded in _schema_migrations.
func loadAppliedSchemaMigrations(ctx context.Context, gdb *gorm.DB) (map[string]bool, error) {
	var names []string

	err := gdb.WithContext(ctx).Raw("SELECT name FROM _schema_migrations").Scan(&names).Error
	if err != nil {
		return nil, fmt.Errorf("query applied schema migrations: %w", err)
	}

	applied := make(map[string]bool, len(names))
	for _, name := range names {
		applied[name] = true
	}

	return applied, nil
}
