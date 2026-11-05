package config

import (
	"os"
	"testing"
)

// TestLoad_Defaults verifies that Load returns the built-in default
// configuration when no environment variables or config file are set.
func TestLoad_Defaults(t *testing.T) { //nolint:paralleltest // t.Setenv is called in a loop below; Go forbids t.Setenv with t.Parallel()
	// Clear env vars that might affect defaults. t.Setenv (rather than
	// os.Unsetenv) keeps this test's env mutation properly scoped and
	// automatically restored, matching the other tests in this file.
	for _, k := range []string{"JWT_SECRET", "PORT", "CORS_ORIGINS", "GIT_CACHE_TTL",
		"UPLOADS_DIR", "GIT_TOKEN",
		"GIT_CREDENTIALS_PATH", "JWT_EXPIRY_HOURS", "CONFIG_PATH"} {
		t.Setenv(k, "")
	}

	c, _ := Load()

	if c.JWTSecret != defaultJWTSecret {
		t.Errorf("unexpected default JWTSecret: %q", c.JWTSecret)
	}

	if c.Port != defaultPort {
		t.Errorf("unexpected default Port: %d", c.Port)
	}

	if c.JWTExpiryH != defaultJWTExpiryHours {
		t.Errorf("unexpected default JWTExpiryH: %d", c.JWTExpiryH)
	}

	if c.UploadsDir != defaultUploadsDir {
		t.Errorf("unexpected default UploadsDir: %q", c.UploadsDir)
	}

	if c.GitCacheTTL != defaultGitCacheTTLMinutes {
		t.Errorf("unexpected default GitCacheTTL: %d", c.GitCacheTTL)
	}

	if len(c.CORSOrigins) == 0 {
		t.Error("expected non-empty default CORSOrigins")
	}
}

// TestLoad_EnvOverrides verifies that Load prefers environment variable
// values over the built-in defaults.
func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "my-secret")
	t.Setenv("PORT", "9090")
	t.Setenv("JWT_EXPIRY_HOURS", "48")
	t.Setenv("CORS_ORIGINS", "http://a.com,http://b.com")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")
	t.Setenv("GIT_TOKEN", "tok123")
	t.Setenv("GIT_CREDENTIALS_PATH", "/etc/creds.yaml")
	t.Setenv("GIT_CACHE_TTL", "30")

	c, _ := Load()

	if c.JWTSecret != "my-secret" {
		t.Errorf("expected JWTSecret=my-secret, got %q", c.JWTSecret)
	}

	if c.Port != 9090 {
		t.Errorf("expected Port=9090, got %d", c.Port)
	}

	if c.JWTExpiryH != 48 {
		t.Errorf("expected JWTExpiryH=48, got %d", c.JWTExpiryH)
	}

	if len(c.CORSOrigins) != 2 {
		t.Errorf("expected 2 CORS origins, got %d", len(c.CORSOrigins))
	}

	if c.UploadsDir != "/tmp/uploads" {
		t.Errorf("expected UploadsDir=/tmp/uploads, got %q", c.UploadsDir)
	}

	if c.GitToken != "tok123" {
		t.Errorf("expected GitToken=tok123, got %q", c.GitToken)
	}

	if c.GitCredentialsPath != "/etc/creds.yaml" {
		t.Errorf("expected GitCredentialsPath, got %q", c.GitCredentialsPath)
	}

	if c.GitCacheTTL != 30 {
		t.Errorf("expected GitCacheTTL=30, got %d", c.GitCacheTTL)
	}
}

// TestLoad_InvalidPort_Ignored verifies that Load falls back to the default
// port when PORT holds a non-numeric value.
func TestLoad_InvalidPort_Ignored(t *testing.T) {
	t.Setenv("PORT", "notanumber")

	c, _ := Load()
	if c.Port != defaultPort {
		t.Errorf("expected default port %d on invalid PORT env, got %d", defaultPort, c.Port)
	}
}

// TestLoad_InvalidGitCacheTTL_Ignored verifies that Load falls back to the
// default git cache TTL when GIT_CACHE_TTL holds a non-positive value.
func TestLoad_InvalidGitCacheTTL_Ignored(t *testing.T) {
	t.Setenv("GIT_CACHE_TTL", "0")

	c, _ := Load()
	if c.GitCacheTTL != defaultGitCacheTTLMinutes {
		t.Errorf("expected default GitCacheTTL=%d when 0 provided, got %d", defaultGitCacheTTLMinutes, c.GitCacheTTL)
	}
}

// TestLoad_ConfigFile verifies that Load applies values from a YAML file
// referenced by the CONFIG_PATH environment variable.
func TestLoad_ConfigFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.WriteString(`
jwtSecret: from-file
port: 7777
corsOrigins:
  - http://file.com
`)
	if err != nil {
		t.Fatal(err)
	}

	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CONFIG_PATH", f.Name())

	c, _ := Load()
	if c.JWTSecret != "from-file" {
		t.Errorf("expected JWTSecret=from-file, got %q", c.JWTSecret)
	}

	if c.Port != 7777 {
		t.Errorf("expected Port=7777, got %d", c.Port)
	}
}
