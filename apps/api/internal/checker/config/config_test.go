package config

import "testing"

// clearEnv unsets every environment variable Load consults so a test starts
// from a known baseline regardless of the host environment.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, k := range []string{
		"GITLAB_TOKEN", "GITLAB_BASE_URL", "RATE_LIMIT_REQUESTS", "RATE_LIMIT_WINDOW_SECONDS",
	} {
		t.Setenv(k, "")
	}
}

// TestLoad_Defaults verifies Load returns the documented defaults when the
// environment is empty.
//
//nolint:paralleltest // clearEnv mutates process env via t.Setenv
func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)

	cfg := Load()

	if cfg.GitLabBaseURL != defaultGitLabBaseURL {
		t.Errorf("GitLabBaseURL = %q, want %q", cfg.GitLabBaseURL, defaultGitLabBaseURL)
	}

	if cfg.GitLabToken != "" {
		t.Errorf("GitLabToken = %q, want empty", cfg.GitLabToken)
	}

	if cfg.RateLimitRequests != defaultRateLimitRequests {
		t.Errorf("RateLimitRequests = %d, want %d", cfg.RateLimitRequests, defaultRateLimitRequests)
	}

	if cfg.RateLimitWindowSeconds != defaultRateLimitWindowSeconds {
		t.Errorf("RateLimitWindowSeconds = %d, want %d", cfg.RateLimitWindowSeconds, defaultRateLimitWindowSeconds)
	}
}

// TestLoad_EnvOverrides verifies every env var is honoured.
func TestLoad_EnvOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("GITLAB_TOKEN", "glpat-xyz")
	t.Setenv("GITLAB_BASE_URL", "https://gitlab.example.com")
	t.Setenv("RATE_LIMIT_REQUESTS", "5")
	t.Setenv("RATE_LIMIT_WINDOW_SECONDS", "30")

	cfg := Load()

	if cfg.GitLabToken != "glpat-xyz" {
		t.Errorf("GitLabToken = %q", cfg.GitLabToken)
	}

	if cfg.GitLabBaseURL != "https://gitlab.example.com" {
		t.Errorf("GitLabBaseURL = %q", cfg.GitLabBaseURL)
	}

	if cfg.RateLimitRequests != 5 {
		t.Errorf("RateLimitRequests = %d, want 5", cfg.RateLimitRequests)
	}

	if cfg.RateLimitWindowSeconds != 30 {
		t.Errorf("RateLimitWindowSeconds = %d, want 30", cfg.RateLimitWindowSeconds)
	}
}

// TestRateLimitFromEnv covers the parsing edge cases directly.
func TestRateLimitFromEnv(t *testing.T) {
	const key = "RATE_LIMIT_REQUESTS"

	cases := []struct {
		name  string
		value string
		set   bool
		want  int
	}{
		{name: "unset keeps current", set: false, want: 42},
		{name: "empty keeps current", value: "", set: true, want: 42},
		{name: "zero disables", value: "0", set: true, want: 0},
		{name: "valid positive", value: "7", set: true, want: 7},
		{name: "negative keeps current", value: "-3", set: true, want: 42},
		{name: "garbage keeps current", value: "abc", set: true, want: 42},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv(key, tc.value)
			} else {
				t.Setenv(key, "")
			}

			got := rateLimitFromEnv(key, 42)
			if got != tc.want {
				t.Errorf("rateLimitFromEnv(%q) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}
