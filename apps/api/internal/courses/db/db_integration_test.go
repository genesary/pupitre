//go:build integration

// db_integration_test.go exercises Connect and RunMigrations against a live
// PostgreSQL instance. Behind the `integration` build tag; run with
// TEST_DATABASE_URL set.

package db_test

import (
	"os"
	"testing"

	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/db"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// dbURL returns TEST_DATABASE_URL or skips the test.
func dbURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	return url
}

// TestConnectAndMigrate connects, migrates, then migrates again to prove the
// operation is idempotent, and checks the core tables exist.
func TestConnectAndMigrate(t *testing.T) { //nolint:paralleltest // owns the shared schema
	url := dbURL(t)

	gdb, err := db.Connect(t.Context(), url, 5, 2)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	err = db.RunMigrations(t.Context(), gdb)
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	err = db.RunMigrations(t.Context(), gdb)
	if err != nil {
		t.Fatalf("RunMigrations (second run): %v", err)
	}

	for _, table := range []string{"courses", "course_modules", "paths", "lab_checks"} {
		if !gdb.Migrator().HasTable(table) {
			t.Errorf("table %s missing after migration", table)
		}
	}
}

// seedDevDB connects, migrates and wipes the course tables.
func seedDevDB(t *testing.T) *gorm.DB {
	t.Helper()

	url := dbURL(t)

	gdb, err := gorm.Open(gormpg.Open(url), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(gormlogger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	err = db.RunMigrations(t.Context(), gdb)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	err = gdb.WithContext(t.Context()).Exec(`TRUNCATE
		courses, course_modules, course_prerequisites, course_sessions,
		paths, path_courses, path_skills, quiz_question_attempts, lab_checks
		RESTART IDENTITY CASCADE`).Error
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return gdb
}

// TestSeedDevCourses loads the embedded catalogue in "missing" mode, verifies
// it is a no-op on a second run, then re-seeds every course in "overwrite"
// mode.
func TestSeedDevCourses(t *testing.T) { //nolint:paralleltest // owns the shared schema
	gdb := seedDevDB(t)
	repo := repository.NewGormCourseRepository(gdb)
	ctx := t.Context()

	err := db.SeedDevCourses(ctx, repo, "off") // any non-mode value disables seeding
	if err != nil {
		t.Fatalf("SeedDevCourses(off): %v", err)
	}

	var count int64

	err = gdb.WithContext(ctx).Table("courses").Count(&count).Error
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if count != 0 {
		t.Fatalf("disabled seeding still inserted %d courses", count)
	}

	err = db.SeedDevCourses(ctx, repo, db.SeedDevCoursesMissing)
	if err != nil {
		t.Fatalf("SeedDevCourses(missing): %v", err)
	}

	_ = gdb.WithContext(ctx).Table("courses").Count(&count)
	if count == 0 {
		t.Fatal("SeedDevCourses(missing) inserted no courses")
	}

	seeded := count

	// A course carries modules — the child-write path must have run.
	var moduleCount int64

	_ = gdb.WithContext(ctx).Table("course_modules").Count(&moduleCount)
	if moduleCount == 0 {
		t.Error("SeedDevCourses inserted no course modules")
	}

	err = db.SeedDevCourses(ctx, repo, db.SeedDevCoursesMissing) // idempotent
	if err != nil {
		t.Fatalf("SeedDevCourses(missing) second run: %v", err)
	}

	_ = gdb.WithContext(ctx).Table("courses").Count(&count)
	if count != seeded {
		t.Errorf("course count changed on re-seed: %d -> %d", seeded, count)
	}

	err = db.SeedDevCourses(ctx, repo, db.SeedDevCoursesOverwrite)
	if err != nil {
		t.Fatalf("SeedDevCourses(overwrite): %v", err)
	}

	_ = gdb.WithContext(ctx).Table("courses").Count(&count)
	if count != seeded {
		t.Errorf("overwrite changed the course count: %d -> %d", seeded, count)
	}
}

// TestConnect_Unreachable returns an error for a database that is not
// listening.
func TestConnect_Unreachable(t *testing.T) { //nolint:paralleltest // consistent with the file
	_ = dbURL(t) // keep the suite hermetic-friendly: skip when no DB configured

	_, err := db.Connect(t.Context(),
		"postgres://127.0.0.1:1/nope?sslmode=disable&connect_timeout=1", 1, 1)
	if err == nil {
		t.Fatal("expected Connect to fail against an unreachable database")
	}
}
