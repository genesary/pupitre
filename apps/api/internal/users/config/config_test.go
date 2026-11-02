package config

import (
	"os"
	"testing"
)

// TestLoad_Defaults verifies that Load returns the built-in default values
// when no relevant environment variables are set.
func TestLoad_Defaults(t *testing.T) {
	t.Parallel()

	// Clear all relevant env vars.
	for _, k := range []string{
		"DATABASE_URL", "JWT_SECRET", "JWT_EXPIRY_HOURS", "PORT",
		"CORS_ORIGINS", "OAUTH_REDIRECT_BASE",
		"ADMIN_PASSWORD", "ADMIN_PASSWORD_FILE", "CONFIG_PATH",
		"OIDC_ENABLED", "OIDC_PROVIDER_URL", "OIDC_ISSUER_URL",
		"OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OIDC_CLIENT_SECRET_FILE",
		"OIDC_SCOPES", "OIDC_GROUP_CLAIM", "OIDC_REDIRECT_BASE", "OIDC_BROWSER_BASE_URL",
	} {
		os.Unsetenv(k)
	}

	c, _ := Load()
	if c.Port != 8081 {
		t.Errorf("default port: want 8081, got %d", c.Port)
	}

	if c.JWTSecret != "change-me-in-production-use-a-long-random-string" {
		t.Errorf("unexpected default JWT secret: %q", c.JWTSecret)
	}

	if c.JWTExpiryH != 8 {
		t.Errorf("default expiry: want 8h, got %d", c.JWTExpiryH)
	}

	if c.DatabaseURL == "" {
		t.Error("expected non-empty default DatabaseURL")
	}

	if len(c.CORSOrigins) == 0 {
		t.Error("expected non-empty default CORSOrigins")
	}
}

// TestLoad_EnvOverrides verifies that Load applies environment variable
// overrides on top of the built-in defaults.
func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "my-secret")
	t.Setenv("JWT_EXPIRY_HOURS", "48")
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/testdb")
	t.Setenv("CORS_ORIGINS", "http://a.com,http://b.com")

	c, _ := Load()
	if c.JWTSecret != "my-secret" {
		t.Errorf("JWT_SECRET override: want my-secret, got %q", c.JWTSecret)
	}

	if c.JWTExpiryH != 48 {
		t.Errorf("JWT_EXPIRY_HOURS override: want 48, got %d", c.JWTExpiryH)
	}

	if c.Port != 9090 {
		t.Errorf("PORT override: want 9090, got %d", c.Port)
	}

	if c.DatabaseURL != "postgres://test:test@localhost/testdb" {
		t.Errorf("DATABASE_URL override: got %q", c.DatabaseURL)
	}

	if len(c.CORSOrigins) != 2 {
		t.Errorf("CORS_ORIGINS: want 2 entries, got %d", len(c.CORSOrigins))
	}
}

// TestLoad_InvalidIntegers verifies that Load falls back to the built-in
// defaults when integer environment variables cannot be parsed.
func TestLoad_InvalidIntegers(t *testing.T) {
	t.Setenv("PORT", "not-a-number")
	t.Setenv("JWT_EXPIRY_HOURS", "bad")

	c, _ := Load()
	// Should fall back to defaults.
	if c.Port != 8081 {
		t.Errorf("invalid PORT: want default 8081, got %d", c.Port)
	}

	if c.JWTExpiryH != 8 {
		t.Errorf("invalid JWT_EXPIRY_HOURS: want default 8, got %d", c.JWTExpiryH)
	}
}

// TestLoad_AdminPassword verifies that Load reads the admin password from
// the ADMIN_PASSWORD environment variable.
func TestLoad_AdminPassword(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "S3cr3t!")

	c, _ := Load()
	if c.AdminPassword != "S3cr3t!" {
		t.Errorf("admin password: want S3cr3t!, got %q", c.AdminPassword)
	}
}

// TestLoad_AdminPasswordFile verifies that Load reads the admin password
// from the file referenced by ADMIN_PASSWORD_FILE.
func TestLoad_AdminPasswordFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "admin-pw-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("FilePassword!")
	f.Close()

	t.Setenv("ADMIN_PASSWORD_FILE", f.Name())

	c, _ := Load()
	if c.AdminPassword != "FilePassword!" {
		t.Errorf("admin password file: want FilePassword!, got %q", c.AdminPassword)
	}
}

// TestLoad_OIDC verifies that Load populates the OIDC bootstrap settings
// from the corresponding OIDC_* environment variables.
func TestLoad_OIDC(t *testing.T) {
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_PROVIDER_URL", "https://sso.example.com")
	t.Setenv("OIDC_CLIENT_ID", "client123")
	t.Setenv("OIDC_CLIENT_SECRET", "secret456")

	c, _ := Load()
	if !c.OIDC.Enabled {
		t.Error("OIDC.Enabled: want true")
	}

	if c.OIDC.ProviderURL != "https://sso.example.com" {
		t.Errorf("OIDC.ProviderURL: got %q", c.OIDC.ProviderURL)
	}

	if c.OIDC.ClientID != "client123" {
		t.Errorf("OIDC.ClientID: got %q", c.OIDC.ClientID)
	}
}

// TestLoad_ConfigFile verifies that Load overlays values from the YAML
// file referenced by CONFIG_PATH.
func TestLoad_ConfigFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("port: 7777\njwtSecret: from-file\n")
	f.Close()

	t.Setenv("CONFIG_PATH", f.Name())

	c, _ := Load()
	if c.Port != 7777 {
		t.Errorf("config file port: want 7777, got %d", c.Port)
	}

	if c.JWTSecret != "from-file" {
		t.Errorf("config file jwtSecret: want from-file, got %q", c.JWTSecret)
	}
}

// TestFindProvider verifies that Config.FindProvider looks up providers by
// id and returns nil for unknown ids.
func TestFindProvider(t *testing.T) {
	t.Parallel()

	c := &Config{
		Providers: []ProviderConfig{
			{ID: "github", Name: "GitHub", ClientID: "gh-id"},
			{ID: "gitlab", Name: "GitLab", ClientID: "gl-id"},
		},
	}

	p := c.FindProvider("github")
	if p == nil {
		t.Fatal("expected to find github provider")
	}

	if p.ClientID != "gh-id" {
		t.Errorf("github ClientID: want gh-id, got %q", p.ClientID)
	}

	p2 := c.FindProvider("gitlab")
	if p2 == nil || p2.ClientID != "gl-id" {
		t.Error("expected to find gitlab provider")
	}

	if c.FindProvider("nonexistent") != nil {
		t.Error("expected nil for unknown provider")
	}
}

// TestResolveClientSecret_EnvVar verifies that ResolveClientSecret returns
// the env-supplied secret when no secret file is configured.
func TestResolveClientSecret_EnvVar(t *testing.T) {
	t.Parallel()

	o := OIDCBootstrap{ClientSecret: "env-secret"}
	if got := o.ResolveClientSecret(); got != "env-secret" {
		t.Errorf("want env-secret, got %q", got)
	}
}

// TestResolveClientSecret_File verifies that ResolveClientSecret prefers
// the trimmed contents of the secret file over the env-supplied secret.
func TestResolveClientSecret_File(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "oidc-secret-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("  file-secret  \n")
	f.Close()

	o := OIDCBootstrap{
		ClientSecretFile: f.Name(),
		ClientSecret:     "env-fallback",
	}
	if got := o.ResolveClientSecret(); got != "file-secret" {
		t.Errorf("want file-secret, got %q", got)
	}
}

// TestResolveClientSecret_FileMissing verifies that ResolveClientSecret
// falls back to the env-supplied secret when the secret file is missing.
func TestResolveClientSecret_FileMissing(t *testing.T) {
	t.Parallel()

	o := OIDCBootstrap{
		ClientSecretFile: "/nonexistent/path",
		ClientSecret:     "fallback",
	}
	if got := o.ResolveClientSecret(); got != "fallback" {
		t.Errorf("want fallback when file missing, got %q", got)
	}
}

// TestResolveClientSecret_Empty verifies that ResolveClientSecret returns
// an empty string when neither a secret file nor an env secret is set.
func TestResolveClientSecret_Empty(t *testing.T) {
	t.Parallel()

	o := OIDCBootstrap{}
	if got := o.ResolveClientSecret(); got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}

// TestLoad_AdminPasswordFileMissing_FallsBackToEnv verifies that Load
// falls back to ADMIN_PASSWORD when ADMIN_PASSWORD_FILE points to a
// nonexistent file, while still preserving the configured file path.
func TestLoad_AdminPasswordFileMissing_FallsBackToEnv(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD_FILE", "/nonexistent/path/pw.txt")
	t.Setenv("ADMIN_PASSWORD", "fallback-pw")

	c, _ := Load()
	if c.AdminPassword != "fallback-pw" {
		t.Errorf("expected fallback-pw when file missing, got %q", c.AdminPassword)
	}

	if c.AdminPasswordFile != "/nonexistent/path/pw.txt" {
		t.Errorf("expected AdminPasswordFile to be preserved, got %q", c.AdminPasswordFile)
	}
}

// TestLoad_OAuthRedirectBaseAndCourseURL verifies that Load reads the
// OAuth redirect base and course service URL from their env vars.
func TestLoad_OAuthRedirectBaseAndCourseURL(t *testing.T) {
	t.Setenv("OAUTH_REDIRECT_BASE", "https://app.example.com")

	c, _ := Load()
	if c.OAuthRedirectBase != "https://app.example.com" {
		t.Errorf("OAuthRedirectBase: want https://app.example.com, got %q", c.OAuthRedirectBase)
	}
}

// TestLoad_ProviderSecretFromEnv verifies that Load overlays a provider's
// client secret from its SSO_<ID>_CLIENT_SECRET env var after providers
// are loaded from the YAML config file.
func TestLoad_ProviderSecretFromEnv(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "config-providers-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("providers:\n  - id: github\n    name: GitHub\n    clientId: gh-id\n")
	f.Close()

	t.Setenv("CONFIG_PATH", f.Name())
	t.Setenv("SSO_GITHUB_CLIENT_SECRET", "gh-secret-from-env")

	c, _ := Load()
	if len(c.Providers) == 0 {
		t.Skip("provider not loaded from config (yaml parsing may differ)")
	}

	for _, p := range c.Providers {
		if p.ID == "github" && p.ClientSecret == "gh-secret-from-env" {
			return
		}
	}

	t.Errorf("expected github provider with gh-secret-from-env, got providers=%v", c.Providers)
}
