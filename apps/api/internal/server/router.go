// Package server assembles the monolith: one database, the users, courses
// and checker domains wired to each other in-process, and one HTTP router
// (docs/specs/100-backend-scope-and-architecture.md).
package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"

	courseshandlers "github.com/jojo91550/pupitre2/apps/api/internal/courses/handlers"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/clientip"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/metrics"
	usershandlers "github.com/jojo91550/pupitre2/apps/api/internal/users/handlers"
)

// corsMaxAgeSeconds bounds the CORS preflight cache, as both services did.
const corsMaxAgeSeconds = 300

// serviceName identifies the API in /health responses.
const serviceName = "pupitre-api"

// Domains are the request handlers of every domain, wired to each other.
type Domains struct {
	Users   *usershandlers.State
	Courses *courseshandlers.State
}

// BuildRouter builds the single router of the monolith. Every route keeps
// the guard and body limit it had in its service; the domains' internal
// routes are not mounted (docs/specs/103-backend-http-api.md).
func BuildRouter(domains Domains, cfg *config.Config, withLogger bool) *chi.Mux {
	router := chi.NewRouter()
	router.Use(chiMiddleware.RequestID)
	// The client address, from the forwarding headers of trusted proxies
	// only (docs/specs/190-backend-defects.md B12).
	router.Use(clientip.Middleware(cfg.TrustedProxies))

	if withLogger {
		router.Use(chiMiddleware.Logger)
	}

	router.Use(chiMiddleware.Recoverer)
	router.Use(securityHeaders)
	router.Use(cors.New(cors.Options{
		AllowedOrigins:   cfg.Users.CORSOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           corsMaxAgeSeconds,
	}).Handler)

	router.Get("/health", health)
	router.Get("/metrics", metrics.Handler())

	usershandlers.RegisterRoutes(router, domains.Users, cfg.Users)
	courseshandlers.RegisterRoutes(router, domains.Courses, cfg.Courses)

	return router
}

// health answers the liveness and readiness probes.
func health(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)

	err := json.NewEncoder(writer).Encode(map[string]string{
		"status":  "ok",
		"service": serviceName,
		"version": "1.0.0",
	})
	if err != nil {
		zap.L().Error("encode health response", zap.Error(err))
	}
}
