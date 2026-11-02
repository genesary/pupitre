//go:build integration

// schema_integration_test.go checks the data invariant of spec 102 against
// a live PostgreSQL instance (TEST_DATABASE_URL): the monolith's migrations
// produce the same schema as the two services did, and running them on a
// database the services already migrated changes nothing.

package server

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	coursesdb "github.com/jojo91550/pupitre2/apps/api/internal/courses/db"
	usersdb "github.com/jojo91550/pupitre2/apps/api/internal/users/db"
)

// scratchDatabase creates an empty database next to TEST_DATABASE_URL and
// returns a connection to it; the database is dropped after the test.
func scratchDatabase(t *testing.T, admin *gorm.DB, baseURL, name string) *gorm.DB {
	t.Helper()

	err := admin.Exec("CREATE DATABASE " + name).Error
	if err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}

	parsed.Path = "/" + name

	gdb, err := coursesdb.Connect(t.Context(), parsed.String(), 2, 2)
	if err != nil {
		t.Fatalf("connect to %s: %v", name, err)
	}

	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}

		_ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)").Error
	})

	return gdb
}

// schemaOf describes every column, index and constraint of the public
// schema, one sorted line each.
func schemaOf(t *testing.T, gdb *gorm.DB) []string {
	t.Helper()

	queries := []string{
		`SELECT 'column ' || table_name || '.' || column_name || ' ' || data_type || ' ' || is_nullable || ' ' || coalesce(column_default, '')
		   FROM information_schema.columns WHERE table_schema = 'public'`,
		`SELECT 'index ' || tablename || ' ' || indexdef FROM pg_indexes WHERE schemaname = 'public'`,
		`SELECT 'constraint ' || conrelid::regclass::text || ' ' || conname || ' ' || pg_get_constraintdef(oid)
		   FROM pg_constraint WHERE connamespace = 'public'::regnamespace`,
	}

	var lines []string

	for _, query := range queries {
		var rows []string

		err := gdb.Raw(query).Scan(&rows).Error
		if err != nil {
			t.Fatalf("describe schema: %v", err)
		}

		lines = append(lines, rows...)
	}

	slices.Sort(lines)

	return lines
}

// TestMigrate_SameSchemaAsTheServices compares the schema of the monolith
// with the one the two services produced, then re-runs the monolith's
// migrations on the services' database.
func TestMigrate_SameSchemaAsTheServices(t *testing.T) {
	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	admin, err := coursesdb.Connect(ctx, baseURL, 2, 2)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// The services: course-service, then user-service, each migrating the
	// shared database on its own start-up.
	services := scratchDatabase(t, admin, baseURL, "schema_services_"+suffix)

	err = coursesdb.RunMigrations(ctx, services)
	if err != nil {
		t.Fatalf("course-service migrations: %v", err)
	}

	err = usersdb.RunMigrations(ctx, services)
	if err != nil {
		t.Fatalf("user-service migrations: %v", err)
	}

	monolith := scratchDatabase(t, admin, baseURL, "schema_monolith_"+suffix)

	err = Migrate(ctx, monolith)
	if err != nil {
		t.Fatalf("monolith migrations: %v", err)
	}

	want := schemaOf(t, services)
	got := schemaOf(t, monolith)

	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Fatalf("schemas differ:\nservices: %v\nmonolith: %v", want, got)
	}

	// Running the monolith on the services' database applies nothing new.
	err = Migrate(ctx, services)
	if err != nil {
		t.Fatalf("monolith migrations on the services' database: %v", err)
	}

	if after := schemaOf(t, services); strings.Join(after, "\n") != strings.Join(want, "\n") {
		t.Fatal("migrating the services' database again changed its schema")
	}

	var applied []string

	err = services.Raw("SELECT name FROM _schema_migrations ORDER BY name").Scan(&applied).Error
	if err != nil {
		t.Fatalf("read _schema_migrations: %v", err)
	}

	// The user-service migration, then the one of the fixes
	// (docs/specs/190-backend-defects.md B20), each once.
	wantApplied := []string{"20260829_dedupe_user_xp_events", "20261026_group_role_mappings_allow_manager"}
	if !slices.Equal(applied, wantApplied) {
		t.Fatalf("_schema_migrations = %v, want %v", applied, wantApplied)
	}
}
