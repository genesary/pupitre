// Package config loads the configuration of the monolith: one set of
// built-in defaults, one optional YAML file (CONFIG_PATH) and one set of
// environment variables for the whole process
// (docs/specs/101-backend-configuration.md).
//
// Each domain keeps the configuration package it had as a service, so its
// code reads the same fields as before. Load runs the three loaders on the
// same file and environment, then sets the values that are now shared or
// renamed: the port, the connection pool, the git credentials path and the
// lab check rate limit.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	checkerconfig "github.com/jojo91550/pupitre2/apps/api/internal/checker/config"
	coursesconfig "github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/clientip"
	usersconfig "github.com/jojo91550/pupitre2/apps/api/internal/users/config"
)

const (
	// defaultPort is the single listen port of the monolith.
	defaultPort = 8080
	// maxPort is the highest valid TCP port.
	maxPort = 65535
	// defaultDBMaxConns is the pool size: the sum of user-service's (20)
	// and course-service's (10), so one replica opens no more connections
	// than one replica of each service did.
	defaultDBMaxConns = 30
	// defaultGitCredentialsPath is where per-repository git tokens are read.
	defaultGitCredentialsPath = "/etc/pupitre/git-credentials.yaml"
	// defaultLabCheckRateLimitRequests is how many lab checks the whole
	// process runs per window.
	defaultLabCheckRateLimitRequests = 20
	// defaultLabCheckRateLimitWindowSeconds is the lab check window.
	defaultLabCheckRateLimitWindowSeconds = 60
	// minJWTSecretLength is the shortest signing secret accepted: 32 bytes,
	// the size of the HS256 key.
	minJWTSecretLength = 32
	// publicDefaultJWTSecret is the secret the services shipped with as a
	// default. Anyone can mint an admin token with it.
	publicDefaultJWTSecret = "change-me-in-production-use-a-long-random-string"
)

// errInvalidPort is returned when the configured port cannot be listened on.
var errInvalidPort = errors.New("invalid port")

// errWeakJWTSecret is returned when the signing secret is the public default
// or too short (docs/specs/190-backend-defects.md B4).
var errWeakJWTSecret = fmt.Errorf("JWT_SECRET must be set to a random secret of at least %d characters", minJWTSecretLength)

// Config is the configuration of the monolith.
type Config struct {
	// Port is the single HTTP listen port.
	Port int

	// TrustedProxies are the networks whose forwarding headers are believed
	// (docs/specs/190-backend-defects.md B12).
	TrustedProxies []netip.Prefix

	// Users, Courses and Checker are the configurations each domain reads,
	// with the shared values already applied.
	Users   *usersconfig.Config
	Courses *coursesconfig.Config
	Checker *checkerconfig.Config
}

// fileConfig holds the keys of the YAML file that the monolith resolves
// itself. Pointers tell an absent key from a zero value.
type fileConfig struct {
	Port                           *int     `yaml:"port"`
	DBMaxOpenConns                 *int     `yaml:"dbMaxOpenConns"`
	DBMaxIdleConns                 *int     `yaml:"dbMaxIdleConns"`
	GitCredentialsPath             *string  `yaml:"gitCredentialsPath"`
	LabCheckRateLimitRequests      *int     `yaml:"labCheckRateLimitRequests"`
	LabCheckRateLimitWindowSeconds *int     `yaml:"labCheckRateLimitWindowSeconds"`
	TrustedProxies                 []string `yaml:"trustedProxies"`
	UserServiceURL                 *string  `yaml:"userServiceUrl"`
	CourseServiceURL               *string  `yaml:"courseServiceUrl"`
	CheckerServiceURL              *string  `yaml:"checkerServiceUrl"`
}

// removedEnv lists the environment variables of the services that the
// monolith no longer reads, with the reason given in the start-up warning.
//
//nolint:gochecknoglobals // static lookup table, read-only
var removedEnv = []struct{ key, reason string }{
	{"USER_SERVICE_URL", "user-service is now called in-process"},
	{"COURSE_SERVICE_URL", "course-service is now called in-process"},
	{"CHECKER_SERVICE_URL", "checker-service is now called in-process"},
	{"INTERNAL_SERVICE_SECRET", "no internal route is left"},
}

// Load builds the configuration. Warnings are non-fatal and meant to be
// logged; an error means the process cannot start.
func Load() (*Config, []string, error) {
	users, warnings := usersconfig.Load()
	courses, _ := coursesconfig.Load() // same file: its warnings repeat the users ones
	checker := checkerconfig.Load()

	file := loadFile()
	warnings = append(warnings, removedKeyWarnings(file)...)

	port := intSetting("PORT", file.Port, defaultPort, false)
	if port <= 0 || port > maxPort {
		return nil, warnings, fmt.Errorf("%w: %d", errInvalidPort, port)
	}

	if users.JWTSecret == publicDefaultJWTSecret || len(users.JWTSecret) < minJWTSecretLength {
		return nil, warnings, errWeakJWTSecret
	}

	maxOpen := intSetting("DB_MAX_OPEN_CONNS", file.DBMaxOpenConns, defaultDBMaxConns, true)
	maxIdle := intSetting("DB_MAX_IDLE_CONNS", file.DBMaxIdleConns, defaultDBMaxConns, true)

	users.Port, courses.Port = port, port
	users.DBMaxOpenConns, courses.DBMaxOpenConns = maxOpen, maxOpen
	users.DBMaxIdleConns, courses.DBMaxIdleConns = maxIdle, maxIdle

	// One database and one signing secret for the whole process.
	courses.DatabaseURL = users.DatabaseURL
	courses.JWTSecret = users.JWTSecret

	courses.GitCredentialsPath = stringSetting("GIT_CREDENTIALS_PATH", file.GitCredentialsPath, defaultGitCredentialsPath)

	limitWarnings := applyLabCheckRateLimit(checker, file)
	warnings = append(warnings, limitWarnings...)

	trusted, err := trustedProxies(file)
	if err != nil {
		return nil, warnings, err
	}

	return &Config{Port: port, TrustedProxies: trusted, Users: users, Courses: courses, Checker: checker}, warnings, nil
}

// trustedProxies resolves TRUSTED_PROXIES (comma-separated), then the
// file's trustedProxies, then the private networks of a cluster.
func trustedProxies(file fileConfig) ([]netip.Prefix, error) {
	values := clientip.DefaultTrustedProxies
	if raw := os.Getenv("TRUSTED_PROXIES"); raw != "" {
		values = strings.Split(raw, ",")
	} else if len(file.TrustedProxies) > 0 {
		values = file.TrustedProxies
	}

	prefixes, err := clientip.ParsePrefixes(values)
	if err != nil {
		return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}

	return prefixes, nil
}

// loadFile reads the monolith's own keys from the CONFIG_PATH file. The
// domain loaders already warn about an unreadable or invalid file.
func loadFile() fileConfig {
	var file fileConfig

	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		return file
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is operator-controlled via the CONFIG_PATH env var, not user input
	if err != nil {
		return file
	}

	err = yaml.Unmarshal(data, &file)
	if err != nil {
		return fileConfig{}
	}

	return file
}

// removedKeyWarnings names every removed key that is still set, in the
// environment or the file, so an old ConfigMap starts the monolith with a
// warning rather than an error.
func removedKeyWarnings(file fileConfig) []string {
	var warnings []string

	for _, removed := range removedEnv {
		if os.Getenv(removed.key) != "" {
			warnings = append(warnings, removed.key+" is ignored: "+removed.reason)
		}
	}

	fileKeys := []struct {
		key   string
		value *string
	}{
		{"userServiceUrl", file.UserServiceURL},
		{"courseServiceUrl", file.CourseServiceURL},
		{"checkerServiceUrl", file.CheckerServiceURL},
	}
	for _, removed := range fileKeys {
		if removed.value != nil {
			warnings = append(warnings, "configuration key "+removed.key+" is ignored: the services are now called in-process")
		}
	}

	return warnings
}

// applyLabCheckRateLimit sets the process-wide lab check limit. The new
// LAB_CHECK_* variables win; checker-service's RATE_LIMIT_* variables are
// still honoured, with a warning, so an old deployment keeps its limit.
func applyLabCheckRateLimit(checker *checkerconfig.Config, file fileConfig) []string {
	var warnings []string

	for _, old := range []string{"RATE_LIMIT_REQUESTS", "RATE_LIMIT_WINDOW_SECONDS"} {
		if os.Getenv(old) != "" {
			warnings = append(warnings, old+" is deprecated: use LAB_CHECK_"+old)
		}
	}

	requests, window := defaultLabCheckRateLimitRequests, defaultLabCheckRateLimitWindowSeconds
	if os.Getenv("RATE_LIMIT_REQUESTS") != "" {
		requests = checker.RateLimitRequests
	}

	if os.Getenv("RATE_LIMIT_WINDOW_SECONDS") != "" {
		window = checker.RateLimitWindowSeconds
	}

	checker.RateLimitRequests = nonNegativeSetting("LAB_CHECK_RATE_LIMIT_REQUESTS", file.LabCheckRateLimitRequests, requests)
	checker.RateLimitWindowSeconds = nonNegativeSetting("LAB_CHECK_RATE_LIMIT_WINDOW_SECONDS", file.LabCheckRateLimitWindowSeconds, window)

	return warnings
}

// intSetting resolves an integer from the environment, then the file, then
// the default. An unparsable value, or a non-positive one when positive is
// set, falls through to the next source.
func intSetting(key string, fileValue *int, fallback int, positive bool) int {
	if raw := os.Getenv(key); raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && (!positive || n > 0) {
			return n
		}
	}

	if fileValue != nil && (!positive || *fileValue > 0) {
		return *fileValue
	}

	return fallback
}

// nonNegativeSetting resolves an integer that may be 0 (disabled) but not
// negative, from the environment, then the file, then fallback.
func nonNegativeSetting(key string, fileValue *int, fallback int) int {
	if raw := os.Getenv(key); raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && n >= 0 {
			return n
		}
	}

	if fileValue != nil && *fileValue >= 0 {
		return *fileValue
	}

	return fallback
}

// stringSetting resolves a string from the environment, then the file,
// then the default.
func stringSetting(key string, fileValue *string, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	if fileValue != nil && *fileValue != "" {
		return *fileValue
	}

	return fallback
}
