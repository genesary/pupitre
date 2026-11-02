// Package main is the entry point of pupitre-api, the single backend of
// pupitre: authentication, users, courses, progress and lab checks
// (docs/specs/100-backend-scope-and-architecture.md).
package main

import (
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/server"
)

// main runs the API and exits non-zero on a fatal start-up or serving error.
func main() {
	// A .env file in the working directory, if any, fills in variables the
	// environment does not set (local development: apps/api/.env).
	_ = godotenv.Load()

	logger := initLogger()
	zap.ReplaceGlobals(logger)

	err := server.Run()
	if err != nil {
		zap.L().Error("fatal startup error", zap.Error(err))

		_ = logger.Sync()

		os.Exit(1)
	}

	_ = logger.Sync()
}

// initLogger builds a zap logger driven by two environment variables:
//   - LOG_LEVEL  — debug/info/warn/error (default: info)
//   - LOG_FORMAT — json (default) or text (human-readable, for local dev)
func initLogger() *zap.Logger {
	level := zap.NewAtomicLevel()
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		_ = level.UnmarshalText([]byte(lvl))
	}

	var cfg zap.Config
	if os.Getenv("LOG_FORMAT") == "text" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	cfg.Level = level

	return zap.Must(cfg.Build())
}
