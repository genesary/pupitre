// Package db provides database connection setup and schema management for
// the user service.
package db

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
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

// allModels lists every GORM model whose table AutoMigrate manages. Order
// doesn't matter for AutoMigrate itself (it creates all tables before
// ensureForeignKeys wires up cross-table constraints), but grouping
// user-anchored tables first keeps the list readable.
//
//nolint:gochecknoglobals // static migration configuration, populated once at init
var allModels = []any{
	&models.User{},
	&models.PlatformSetting{},
	&models.Enrollment{},
	&models.LessonProgress{},
	&models.ModuleProgress{},
	&models.Group{},
	&models.UserGroup{},
	&models.GroupRoleMapping{},
	&models.GroupEnrollment{},
	&models.MarkdownPattern{},
	&models.PathEnrollment{},
	&models.UserBadge{},
	&models.UserSkillLevel{},
	&models.SessionBooking{},
	&models.UserXPEvent{},
	&models.ExportLog{},
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
//
// A small set of constraints (foreign keys with ON DELETE behavior) aren't
// expressible via AutoMigrate without modeling full GORM associations, which
// this codebase's flat repository style deliberately avoids. ensureForeignKeys
// adds them idempotently after AutoMigrate runs.
func RunMigrations(ctx context.Context, gdb *gorm.DB) error {
	err := applyBreakingMigrations(ctx, gdb)
	if err != nil {
		return err
	}

	err = gdb.WithContext(ctx).AutoMigrate(allModels...)
	if err != nil {
		return fmt.Errorf("auto-migrate schema: %w", err)
	}

	err = ensureForeignKeys(ctx, gdb)
	if err != nil {
		return err
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

// breakingMigrations lists one-off schema changes that AutoMigrate cannot
// express safely (constraint drops, renames, data transforms). Each entry
// runs at most once, tracked by Name in _schema_migrations.
//
//nolint:gochecknoglobals // static migration configuration, populated once at init
var breakingMigrations = []breakingMigration{
	{
		// group_role_mappings.platformrole accepted admin and student only,
		// while the API and the admin page offered manager too: saving such a
		// mapping failed with "Database error" (docs/specs/190-backend-defects.md
		// B20). AutoMigrate does not change an existing check constraint, so
		// drop it here; AutoMigrate then creates it again from the model,
		// which lists the three roles.
		Name: "20261026_group_role_mappings_allow_manager",
		Apply: func(ctx context.Context, gdb *gorm.DB) error {
			exists, err := tableExists(ctx, gdb, "group_role_mappings")
			if err != nil || !exists {
				return err
			}

			err = gdb.WithContext(ctx).Exec(
				"ALTER TABLE group_role_mappings DROP CONSTRAINT IF EXISTS chk_group_role_mappings_platformrole").Error
			if err != nil {
				return fmt.Errorf("drop platformrole check: %w", err)
			}

			return nil
		},
	},
	{
		// user_xp_events gained a unique index on (userid, source,
		// source_slug). Award() always intended one — it upserts with ON
		// CONFLICT DO NOTHING — but without the index there was nothing to
		// conflict against, so repeat awards piled up extra rows and
		// inflated the learner's total. AutoMigrate cannot create the index
		// while those duplicates exist, so collapse them first, keeping the
		// earliest event of each group.
		Name: "20260829_dedupe_user_xp_events",
		Apply: func(ctx context.Context, gdb *gorm.DB) error {
			// Breaking migrations run before AutoMigrate, so on a fresh
			// database this table does not exist yet — and there is nothing
			// to collapse.
			exists, err := tableExists(ctx, gdb, "user_xp_events")
			if err != nil || !exists {
				return err
			}

			err = gdb.WithContext(ctx).Exec(`
				DELETE FROM user_xp_events a
				USING user_xp_events b
				WHERE a.userid = b.userid
				  AND a.source = b.source
				  AND a.source_slug = b.source_slug
				  AND a.id > b.id`).Error
			if err != nil {
				return fmt.Errorf("collapse duplicate xp events: %w", err)
			}

			return nil
		},
	},
}

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

// tableExists reports whether a table of that name is present in the
// database. Breaking migrations run before AutoMigrate has created any
// table, so one that rewrites data has to ask.
func tableExists(ctx context.Context, gdb *gorm.DB, name string) (bool, error) {
	var found *string

	err := gdb.WithContext(ctx).Raw("SELECT to_regclass(?)::text", name).Scan(&found).Error
	if err != nil {
		return false, fmt.Errorf("check table %s: %w", name, err)
	}

	return found != nil, nil
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

// foreignKey describes a FK constraint AutoMigrate doesn't create on its
// own (this codebase's models use raw ID columns, not GORM associations).
type foreignKey struct {
	Name string
	DDL  string
}

// foreignKeys lists every FK constraint the live schema needs. Referential
// integrity for cascading deletes (e.g. DeleteUser) relies on these existing
// in the database — the application does not manually clean up dependent
// rows.
//
//nolint:gochecknoglobals // static migration configuration, populated once at init
var foreignKeys = []foreignKey{
	{
		Name: "enrollments_userid_fkey",
		DDL:  "ALTER TABLE enrollments ADD CONSTRAINT enrollments_userid_fkey FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE",
	},
	{
		Name: "lesson_progress_userid_fkey",
		DDL:  "ALTER TABLE lesson_progress ADD CONSTRAINT lesson_progress_userid_fkey FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE",
	},
	{
		Name: "module_progress_userid_fkey",
		DDL:  "ALTER TABLE module_progress ADD CONSTRAINT module_progress_userid_fkey FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE",
	},
	{
		Name: "user_groups_userid_fkey",
		DDL:  "ALTER TABLE user_groups ADD CONSTRAINT user_groups_userid_fkey FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE",
	},
	{
		Name: "user_groups_groupid_fkey",
		DDL:  "ALTER TABLE user_groups ADD CONSTRAINT user_groups_groupid_fkey FOREIGN KEY (groupid) REFERENCES groups(id) ON DELETE CASCADE",
	},
	{
		Name: "group_enrollments_groupid_fkey",
		DDL:  "ALTER TABLE group_enrollments ADD CONSTRAINT group_enrollments_groupid_fkey FOREIGN KEY (groupid) REFERENCES groups(id) ON DELETE CASCADE",
	},
	{
		Name: "markdown_patterns_createdby_fkey",
		DDL:  "ALTER TABLE markdown_patterns ADD CONSTRAINT markdown_patterns_createdby_fkey FOREIGN KEY (createdby) REFERENCES users(id) ON DELETE SET NULL",
	},
	{
		Name: "path_enrollments_userid_fkey",
		DDL:  "ALTER TABLE path_enrollments ADD CONSTRAINT path_enrollments_userid_fkey FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE",
	},
	{
		Name: "user_badges_userid_fkey",
		DDL:  "ALTER TABLE user_badges ADD CONSTRAINT user_badges_userid_fkey FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE",
	},
	{
		Name: "user_skill_levels_userid_fkey",
		DDL:  "ALTER TABLE user_skill_levels ADD CONSTRAINT user_skill_levels_userid_fkey FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE",
	},
	{
		Name: "session_bookings_userid_fkey",
		DDL:  "ALTER TABLE session_bookings ADD CONSTRAINT session_bookings_userid_fkey FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE",
	},
	{
		Name: "user_xp_events_userid_fkey",
		DDL:  "ALTER TABLE user_xp_events ADD CONSTRAINT user_xp_events_userid_fkey FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE",
	},
	{
		Name: "groups_parent_id_fkey",
		DDL:  "ALTER TABLE groups ADD CONSTRAINT groups_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES groups(id) ON DELETE RESTRICT",
	},
}

// ensureForeignKeys adds every entry in foreignKeys that isn't already
// present, keyed by constraint name via pg_constraint. Safe to run on every
// startup.
func ensureForeignKeys(ctx context.Context, gdb *gorm.DB) error {
	for _, constraint := range foreignKeys {
		var count int64

		err := gdb.WithContext(ctx).Raw(
			"SELECT count(*) FROM pg_constraint WHERE conname = ?", constraint.Name,
		).Scan(&count).Error
		if err != nil {
			return fmt.Errorf("check constraint %s: %w", constraint.Name, err)
		}

		if count > 0 {
			continue
		}

		err = gdb.WithContext(ctx).Exec(constraint.DDL).Error
		if err != nil {
			return fmt.Errorf("add constraint %s: %w", constraint.Name, err)
		}

		zap.L().Info("foreign key constraint added", zap.String("name", constraint.Name))
	}

	return nil
}
