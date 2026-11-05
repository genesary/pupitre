// Package config provides application configuration loading for
// course-service, merging built-in defaults, an optional YAML file, and
// environment variable overrides.
package config

import (
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// defaultJWTSecret is the fallback JWT signing secret; it must be
	// overridden in production via the JWT_SECRET env var or config file.
	defaultJWTSecret = "change-me-in-production-use-a-long-random-string"
	// defaultJWTExpiryHours is the default JWT token lifetime, in hours.
	defaultJWTExpiryHours = 24
	// defaultPort is the default HTTP listen port.
	defaultPort = 8082
	// defaultUploadsDir is the default directory for uploaded files.
	defaultUploadsDir = "./uploads"
	// defaultGitCacheTTLMinutes is the default git cache TTL, in minutes.
	defaultGitCacheTTLMinutes = 10
	// defaultDBMaxOpenConns is the default maximum number of open database
	// connections held in the pool.
	defaultDBMaxOpenConns = 10
	// defaultDBMaxIdleConns is the default maximum number of idle database
	// connections held in the pool.
	defaultDBMaxIdleConns = 10
)

// Config holds the application configuration for course-service.
type Config struct {
	JWTSecret string `yaml:"jwtSecret"`

	JWTExpiryH int `yaml:"jwtExpiryHours"`
	Port       int `yaml:"port"`

	CORSOrigins []string `yaml:"corsOrigins"`

	UploadsDir string `yaml:"uploadsDir"`

	GitToken string `yaml:"gitToken"`

	GitCredentialsPath string `yaml:"gitCredentialsPath"`

	GitCacheTTL int `yaml:"gitCacheTtlMinutes"`

	DatabaseURL string `yaml:"databaseUrl"`

	// DBMaxOpenConns and DBMaxIdleConns cap the database connection pool size.
	DBMaxOpenConns int `yaml:"dbMaxOpenConns"`
	DBMaxIdleConns int `yaml:"dbMaxIdleConns"`
}

// Load builds a Config from built-in defaults, then overlays an optional
// YAML file referenced by CONFIG_PATH, then overlays environment variable
// overrides. The returned warnings slice contains non-fatal diagnostic
// messages (e.g. config file parse errors) to be logged by the caller.
func Load() (cfg *Config, warnings []string) { //nolint:nonamedreturns // gocritic requires named results here
	cfg = defaultConfig()

	warnings = loadFromFile(cfg)

	cfg.JWTSecret = stringFromEnv("JWT_SECRET", cfg.JWTSecret)
	cfg.JWTExpiryH = intFromEnv("JWT_EXPIRY_HOURS", cfg.JWTExpiryH)
	cfg.Port = intFromEnv("PORT", cfg.Port)
	cfg.CORSOrigins = sliceFromEnv("CORS_ORIGINS", cfg.CORSOrigins)
	cfg.UploadsDir = stringFromEnv("UPLOADS_DIR", cfg.UploadsDir)
	cfg.GitToken = stringFromEnv("GIT_TOKEN", cfg.GitToken)
	cfg.GitCredentialsPath = stringFromEnv("GIT_CREDENTIALS_PATH", cfg.GitCredentialsPath)
	cfg.GitCacheTTL = positiveIntFromEnv("GIT_CACHE_TTL", cfg.GitCacheTTL)
	cfg.DatabaseURL = stringFromEnv("DATABASE_URL", cfg.DatabaseURL)
	cfg.DBMaxOpenConns = positiveIntFromEnv("DB_MAX_OPEN_CONNS", cfg.DBMaxOpenConns)
	cfg.DBMaxIdleConns = positiveIntFromEnv("DB_MAX_IDLE_CONNS", cfg.DBMaxIdleConns)

	return cfg, warnings
}

// defaultConfig returns a Config populated with the built-in default values.
func defaultConfig() *Config {
	return &Config{
		JWTSecret:          defaultJWTSecret,
		JWTExpiryH:         defaultJWTExpiryHours,
		Port:               defaultPort,
		CORSOrigins:        []string{"http://localhost:3000", "http://localhost:5173"},
		UploadsDir:         defaultUploadsDir,
		GitCredentialsPath: "/etc/pupitre/git-credentials.yaml",
		GitCacheTTL:        defaultGitCacheTTLMinutes,
		DBMaxOpenConns:     defaultDBMaxOpenConns,
		DBMaxIdleConns:     defaultDBMaxIdleConns,
	}
}

// loadFromFile overlays cfg with values from the YAML file referenced by the
// CONFIG_PATH environment variable, if set. It returns a non-empty warnings
// slice when the file is set but cannot be read or parsed, so the caller can
// log the issue without taking a hard dependency on a logger.
func loadFromFile(cfg *Config) []string {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is operator-controlled via the CONFIG_PATH env var, not user input
	if err != nil {
		return []string{"CONFIG_PATH=" + path + ": cannot read file: " + err.Error()}
	}

	unmarshErr := yaml.Unmarshal(data, cfg)
	if unmarshErr != nil {
		return []string{"CONFIG_PATH=" + path + ": YAML parse error: " + unmarshErr.Error()}
	}

	return nil
}

// stringFromEnv returns the value of the environment variable key, or
// current if the variable is unset or empty.
func stringFromEnv(key, current string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return current
}

// sliceFromEnv returns the comma-separated value of the environment
// variable key split into a slice, or current if the variable is unset or
// empty.
func sliceFromEnv(key string, current []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return current
	}

	return strings.Split(v, ",")
}

// intFromEnv returns the parsed integer value of the environment variable
// key, or current if the variable is unset, empty, or not a valid integer.
func intFromEnv(key string, current int) int {
	v := os.Getenv(key)
	if v == "" {
		return current
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return current
	}

	return n
}

// positiveIntFromEnv behaves like intFromEnv but also falls back to current
// when the parsed value is not strictly positive.
func positiveIntFromEnv(key string, current int) int {
	n := intFromEnv(key, current)
	if n <= 0 {
		return current
	}

	return n
}
