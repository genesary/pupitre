// Package config loads the checker's configuration from env vars: the GitLab
// instance it reads learners' projects from, and its rate limit
// (docs/specs/101-backend-configuration.md).
package config

import (
	"os"
	"strconv"
)

// defaultRateLimitRequests is the default max checks per window.
const defaultRateLimitRequests = 20

// defaultRateLimitWindowSeconds is the default rate-limit window in seconds.
const defaultRateLimitWindowSeconds = 60

// defaultGitLabBaseURL is the default GitLab instance base URL.
const defaultGitLabBaseURL = "http://gitlab.local:8880"

// Config holds the checker's runtime configuration.
type Config struct {
	GitLabToken            string
	GitLabBaseURL          string
	RateLimitRequests      int
	RateLimitWindowSeconds int
}

// Load builds a Config from environment variables, falling back to defaults.
// RATE_LIMIT_REQUESTS and RATE_LIMIT_WINDOW_SECONDS are checker-service's
// names; the monolith's loader also reads the LAB_CHECK_* names.
func Load() *Config {
	cfg := &Config{
		GitLabBaseURL:          defaultGitLabBaseURL,
		RateLimitRequests:      defaultRateLimitRequests,
		RateLimitWindowSeconds: defaultRateLimitWindowSeconds,
	}

	if v := os.Getenv("GITLAB_TOKEN"); v != "" {
		cfg.GitLabToken = v
	}

	if v := os.Getenv("GITLAB_BASE_URL"); v != "" {
		cfg.GitLabBaseURL = v
	}

	cfg.RateLimitRequests = rateLimitFromEnv("RATE_LIMIT_REQUESTS", cfg.RateLimitRequests)
	cfg.RateLimitWindowSeconds = rateLimitFromEnv("RATE_LIMIT_WINDOW_SECONDS", cfg.RateLimitWindowSeconds)

	return cfg
}

// rateLimitFromEnv parses the env var as a non-negative integer. 0 means
// disabled; negatives and non-integers fall back to current.
func rateLimitFromEnv(key string, current int) int {
	v := os.Getenv(key)
	if v == "" {
		return current
	}

	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return current
	}

	return n
}
