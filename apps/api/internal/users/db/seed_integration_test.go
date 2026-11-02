//go:build integration

// seed_integration_test.go covers SeedAdmin, SeedOIDC and SeedMockData
// against a live PostgreSQL instance. Behind the `integration` build tag;
// run with TEST_DATABASE_URL set.

package db_test

import (
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/db"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// seedDB connects, migrates and wipes the tables the seeders write to.
func seedDB(t *testing.T) *gorm.DB {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	gdb, err := gorm.Open(postgres.Open(url), &gorm.Config{
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

	err = gdb.WithContext(t.Context()).
		Exec("TRUNCATE users, enrollments, platform_settings RESTART IDENTITY CASCADE").Error
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return gdb
}

// TestSeedAdmin_DefaultThenOverride inserts the fallback admin, then updates
// it when an explicit password is supplied.
func TestSeedAdmin_DefaultThenOverride(t *testing.T) { //nolint:paralleltest // shared db
	gdb := seedDB(t)
	users := repository.NewGormUserRepository(gdb)
	ctx := t.Context()

	err := db.SeedAdmin(ctx, users, "")
	if err != nil {
		t.Fatalf("SeedAdmin(default): %v", err)
	}

	n, err := users.CountByRole(ctx, "admin")
	if err != nil || n != 1 {
		t.Fatalf("CountByRole(admin) after default = %d, %v", n, err)
	}

	err = db.SeedAdmin(ctx, users, "S3cure-Admin-Pass")
	if err != nil {
		t.Fatalf("SeedAdmin(explicit): %v", err)
	}

	err = db.SeedAdmin(ctx, users, "$2a$10$abcdefghijklmnopqrstuvwx0123456789ABCDEFGHIJKLMNOPqr")
	if err != nil {
		t.Fatalf("SeedAdmin(prehashed): %v", err)
	}

	n, _ = users.CountByRole(ctx, "admin")
	if n != 1 {
		t.Errorf("admin count should stay 1 across re-seeds, got %d", n)
	}
}

// TestSeedOIDC_DisabledIsNoOp and enabled path writes the settings.
func TestSeedOIDC_DisabledIsNoOp(t *testing.T) { //nolint:paralleltest // shared db
	gdb := seedDB(t)
	settings := repository.NewGormSettingRepository(gdb)
	ctx := t.Context()

	err := db.SeedOIDC(ctx, settings, &config.Config{OIDC: config.OIDCBootstrap{Enabled: false}})
	if err != nil {
		t.Fatalf("SeedOIDC(disabled): %v", err)
	}

	rows, err := settings.List(ctx)
	if err != nil || len(rows) != 0 {
		t.Fatalf("disabled SeedOIDC wrote %d settings, %v", len(rows), err)
	}

	err = db.SeedOIDC(ctx, settings, &config.Config{OIDC: config.OIDCBootstrap{
		Enabled:      true,
		ProviderURL:  "https://idp.test",
		IssuerURL:    "https://idp.test",
		ClientID:     "pupitre",
		ClientSecret: "shh",
		Scopes:       "openid profile email",
		GroupClaim:   "groups",
	}})
	if err != nil {
		t.Fatalf("SeedOIDC(enabled): %v", err)
	}

	val, ok, err := settings.Get(ctx, "oidc_enabled")
	if err != nil || !ok || val != "true" {
		t.Fatalf("oidc_enabled = %q ok:%v err:%v", val, ok, err)
	}

	secret, ok, _ := settings.Get(ctx, "oidc_client_secret")
	if !ok || secret != "shh" {
		t.Errorf("client secret not seeded: %q", secret)
	}
}

// TestSeedMockData inserts the demo students and their enrollments.
func TestSeedMockData(t *testing.T) { //nolint:paralleltest // shared db
	gdb := seedDB(t)
	users := repository.NewGormUserRepository(gdb)
	enrollments := repository.NewGormEnrollmentRepository(gdb)
	ctx := t.Context()

	err := db.SeedMockData(ctx, users, enrollments)
	if err != nil {
		t.Fatalf("SeedMockData: %v", err)
	}

	var userCount int64

	err = gdb.WithContext(ctx).Table("users").Where("role = ?", "student").Count(&userCount).Error
	if err != nil {
		t.Fatalf("count students: %v", err)
	}

	if userCount == 0 {
		t.Fatal("SeedMockData inserted no students")
	}

	total, err := enrollments.CountAll(ctx)
	if err != nil {
		t.Fatalf("CountAll enrollments: %v", err)
	}

	if total == 0 {
		t.Error("SeedMockData created no enrollments")
	}

	// Idempotent: a second run must not error or duplicate users.
	err = db.SeedMockData(ctx, users, enrollments)
	if err != nil {
		t.Fatalf("SeedMockData (second run): %v", err)
	}

	var afterCount int64

	_ = gdb.WithContext(ctx).Table("users").Where("role = ?", "student").Count(&afterCount).Error
	if afterCount != userCount {
		t.Errorf("student count changed on re-seed: %d -> %d", userCount, afterCount)
	}
}
