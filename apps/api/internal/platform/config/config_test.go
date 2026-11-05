package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearEnv unsets every variable these tests rely on, for the duration of
// the test.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"CONFIG_PATH", "PORT", "DATABASE_URL", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "JWT_SECRET",
		"GIT_CREDENTIALS_PATH", "USER_SERVICE_URL", "COURSE_SERVICE_URL", "CHECKER_SERVICE_URL",
		"INTERNAL_SERVICE_SECRET", "RATE_LIMIT_REQUESTS", "RATE_LIMIT_WINDOW_SECONDS",
		"LAB_CHECK_RATE_LIMIT_REQUESTS", "LAB_CHECK_RATE_LIMIT_WINDOW_SECONDS", "TRUSTED_PROXIES",
	} {
		t.Setenv(key, "")
	}

	t.Setenv("JWT_SECRET", "a-test-signing-secret-of-enough-length-0123")
}

// TestLoad_Defaults checks the monolith's own defaults.
func TestLoad_Defaults(t *testing.T) { //nolint:paralleltest // sets environment variables
	clearEnv(t)

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 8080 || cfg.Users.Port != 8080 || cfg.Courses.Port != 8080 {
		t.Errorf("port = %d/%d/%d, want 8080", cfg.Port, cfg.Users.Port, cfg.Courses.Port)
	}

	if cfg.Users.DBMaxOpenConns != 30 || cfg.Courses.DBMaxIdleConns != 30 {
		t.Errorf("pool = %d/%d, want 30", cfg.Users.DBMaxOpenConns, cfg.Courses.DBMaxIdleConns)
	}

	if cfg.Courses.DatabaseURL != cfg.Users.DatabaseURL || cfg.Courses.DatabaseURL == "" {
		t.Errorf("database URL not shared: %q / %q", cfg.Users.DatabaseURL, cfg.Courses.DatabaseURL)
	}

	if cfg.Courses.GitCredentialsPath != "/etc/pupitre/git-credentials.yaml" {
		t.Errorf("git credentials path = %q", cfg.Courses.GitCredentialsPath)
	}

	if cfg.Checker.RateLimitRequests != 20 || cfg.Checker.RateLimitWindowSeconds != 60 {
		t.Errorf("lab check limit = %d per %d s", cfg.Checker.RateLimitRequests, cfg.Checker.RateLimitWindowSeconds)
	}
}

// TestLoad_EnvWinsOverFile checks the loading order: defaults, file, env.
func TestLoad_EnvWinsOverFile(t *testing.T) {
	clearEnv(t)

	path := filepath.Join(t.TempDir(), "config.yaml")

	err := os.WriteFile(path, []byte("port: 9100\ndbMaxOpenConns: 12\ndatabaseUrl: postgres://file/db\nlabCheckRateLimitRequests: 5\n"), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CONFIG_PATH", path)
	t.Setenv("PORT", "9000")
	t.Setenv("DATABASE_URL", "postgres://env/db")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 9000 {
		t.Errorf("port = %d, want the env value 9000", cfg.Port)
	}

	if cfg.Users.DBMaxOpenConns != 12 || cfg.Courses.DBMaxOpenConns != 12 {
		t.Errorf("pool = %d, want the file value 12", cfg.Users.DBMaxOpenConns)
	}

	if cfg.Courses.DatabaseURL != "postgres://env/db" {
		t.Errorf("database URL = %q, want the env value", cfg.Courses.DatabaseURL)
	}

	if cfg.Checker.RateLimitRequests != 5 {
		t.Errorf("lab check limit = %d, want the file value 5", cfg.Checker.RateLimitRequests)
	}
}

// TestLoad_RemovedKeysWarn checks that the removed service URLs and secret
// are ignored with a warning naming them.
func TestLoad_RemovedKeysWarn(t *testing.T) {
	clearEnv(t)
	t.Setenv("USER_SERVICE_URL", "http://user-service:8081")
	t.Setenv("INTERNAL_SERVICE_SECRET", "s3cret")

	_, warnings, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	joined := strings.Join(warnings, "\n")
	for _, key := range []string{"USER_SERVICE_URL", "INTERNAL_SERVICE_SECRET"} {
		if !strings.Contains(joined, key+" is ignored") {
			t.Errorf("no warning for %s in %q", key, joined)
		}
	}
}

// TestLoad_TrustedProxies checks that only loopback is trusted by default,
// with a warning, and that a configured list replaces it silently.
func TestLoad_TrustedProxies(t *testing.T) {
	clearEnv(t)

	cfg, warnings, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0].String() != "127.0.0.0/8" {
		t.Errorf("default trusted proxies = %v", cfg.TrustedProxies)
	}

	if !strings.Contains(strings.Join(warnings, "\n"), "TRUSTED_PROXIES is not set") {
		t.Errorf("no warning for the default in %q", warnings)
	}

	t.Setenv("TRUSTED_PROXIES", "10.42.0.0/16")

	cfg, warnings, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.TrustedProxies) != 1 || cfg.TrustedProxies[0].String() != "10.42.0.0/16" {
		t.Errorf("configured trusted proxies = %v", cfg.TrustedProxies)
	}

	if strings.Contains(strings.Join(warnings, "\n"), "TRUSTED_PROXIES") {
		t.Errorf("warning despite the setting: %q", warnings)
	}
}

// TestLoad_LabCheckWindowIsPositive checks that a window of 0, which would
// have the limiter divide by zero and never refuse a check, is not taken,
// under the new name or the old one.
func TestLoad_LabCheckWindowIsPositive(t *testing.T) {
	for _, key := range []string{"LAB_CHECK_RATE_LIMIT_WINDOW_SECONDS", "RATE_LIMIT_WINDOW_SECONDS"} {
		t.Run(key, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(key, "0")

			cfg, _, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			if cfg.Checker.RateLimitWindowSeconds != defaultLabCheckRateLimitWindowSeconds {
				t.Errorf("window = %d, want the default %d", cfg.Checker.RateLimitWindowSeconds, defaultLabCheckRateLimitWindowSeconds)
			}
		})
	}
}

// TestLoad_OldRateLimitVariables checks that checker-service's variables
// still set the limit, unless the new ones are set.
func TestLoad_OldRateLimitVariables(t *testing.T) {
	clearEnv(t)
	t.Setenv("RATE_LIMIT_REQUESTS", "7")

	cfg, warnings, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Checker.RateLimitRequests != 7 {
		t.Errorf("lab check limit = %d, want 7 from RATE_LIMIT_REQUESTS", cfg.Checker.RateLimitRequests)
	}

	if !strings.Contains(strings.Join(warnings, "\n"), "RATE_LIMIT_REQUESTS is deprecated") {
		t.Errorf("no deprecation warning in %q", warnings)
	}

	t.Setenv("LAB_CHECK_RATE_LIMIT_REQUESTS", "3")

	cfg, _, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Checker.RateLimitRequests != 3 {
		t.Errorf("lab check limit = %d, want 3 from LAB_CHECK_RATE_LIMIT_REQUESTS", cfg.Checker.RateLimitRequests)
	}
}

// TestLoad_InvalidPort stops the start-up.
func TestLoad_InvalidPort(t *testing.T) {
	clearEnv(t)
	t.Setenv("PORT", "70000")

	_, _, err := Load()
	if err == nil {
		t.Fatal("Load accepted port 70000")
	}
}

// TestLoad_WeakJWTSecret stops the start-up on the public default secret or
// a short one.
func TestLoad_WeakJWTSecret(t *testing.T) { //nolint:paralleltest // sets environment variables
	for _, secret := range []string{"", "change-me-in-production-use-a-long-random-string", "short-secret"} {
		clearEnv(t)
		t.Setenv("JWT_SECRET", secret)

		_, _, err := Load()
		if err == nil {
			t.Errorf("Load accepted JWT_SECRET %q", secret)
		}
	}
}
