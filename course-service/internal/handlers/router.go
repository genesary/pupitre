package handlers

import (
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/genesary/pupitre/course-service/internal/config"
	"github.com/genesary/pupitre/internal/metrics"
	apimiddleware "github.com/genesary/pupitre/internal/middleware"
)

// routerCORSMaxAgeSeconds is the max-age advertised for CORS preflight
// response caching.
const routerCORSMaxAgeSeconds = 300

// BuildRouter constructs the course-service chi router, wiring public
// routes, user-authenticated routes, and admin-only routes behind their
// respective middleware, plus CORS/logging middleware.
func BuildRouter(state *State, cfg *config.Config, withLogger bool) *chi.Mux {
	authMW := apimiddleware.Auth(cfg.JWTSecret)
	adminMW := apimiddleware.Admin(cfg.JWTSecret)
	trainerMW := apimiddleware.ManagerOrAdmin(cfg.JWTSecret)

	corsOptions := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           routerCORSMaxAgeSeconds,
	})

	router := chi.NewRouter()
	router.Use(chiMiddleware.RequestID)
	//nolint:staticcheck // SA1019: RealIP deprecation acknowledged — service runs behind a trusted reverse proxy that sets X-Forwarded-For; the value is used for logging/rate-limiting only.
	router.Use(chiMiddleware.RealIP)

	if withLogger {
		router.Use(chiMiddleware.Logger)
	}

	router.Use(chiMiddleware.Recoverer)
	router.Use(corsOptions.Handler)

	// The body cap is per group rather than global: an admin importing a
	// whole course as one markdown document sends far more than any
	// learner request ever should, and a single limit would have to be
	// the loose one everywhere.
	router.Group(func(group chi.Router) {
		group.Use(chiMiddleware.RequestSize(maxRequestBodyBytes))
		registerPublicRoutes(group, state)
	})

	router.Group(func(group chi.Router) {
		group.Use(chiMiddleware.RequestSize(maxRequestBodyBytes))
		group.Use(authMW)
		registerAuthRoutes(group, state)
	})

	router.Group(func(group chi.Router) {
		group.Use(chiMiddleware.RequestSize(maxAdminRequestBodyBytes))
		group.Use(adminMW)
		registerAdminRoutes(group, state)
	})

	router.Group(func(group chi.Router) {
		group.Use(chiMiddleware.RequestSize(maxRequestBodyBytes))
		group.Use(trainerMW)
		registerTrainerRoutes(group, state)
	})

	return router
}

// registerPublicRoutes wires the unauthenticated course-service endpoints.
func registerPublicRoutes(router chi.Router, state *State) {
	router.Get("/health", state.Health)
	router.Get("/metrics", metrics.Handler())

	router.Get("/api/courses", state.ListCourses)
	router.Get("/api/courses/{slug}", state.GetCourse)

	router.Get("/api/paths", state.ListPaths)
	router.Get("/api/paths/{slug}", state.GetPath)

	router.Get("/api/skills/{slug}/modules", state.ListSkillModules)

	// Batch lookups live under their own prefix rather than as a literal
	// segment of the collection they read (/api/courses/batch), which a
	// course slugged "batch" would have shadowed — chi matches a static
	// segment before a wildcard, so that course would have become
	// unreachable and this endpoint would have answered in its place.
	registerBatchRoutes(router, state)

	router.Get("/uploads/{filename}", state.ServeUpload)
}

// registerBatchRoutes wires the batch lookups. They are server-to-server
// reads: user-service resolves whole sets of slugs through them instead of
// issuing one request per slug.
func registerBatchRoutes(router chi.Router, state *State) {
	router.Get("/api/batch/courses", state.ListCoursesBatch)
	router.Get("/api/batch/paths", state.ListPathsBatch)
	router.Get("/api/batch/skills", state.ListSkillModulesBatch)
}

// registerAuthRoutes wires the course-content endpoints that require a valid
// JWT (any authenticated user).
func registerAuthRoutes(router chi.Router, state *State) {
	router.Get("/api/courses/{slug}/modules", state.ListModules)
	router.Get("/api/courses/{slug}/modules/{index}", state.GetModule)
	router.Post("/api/courses/{slug}/modules/{index}/submit", state.SubmitModule)
	router.Post("/api/courses/{slug}/modules/{index}/check", state.CheckModule)
	router.Post("/api/courses/{slug}/modules/{index}/steps/{stepIndex}/check", state.CheckModuleStep)
	router.Post("/api/courses/{slug}/modules/{index}/record", state.RecordLocalCheck)

	router.Get("/api/courses/{slug}/labs", state.ListLabs)
	router.Get("/api/courses/{slug}/labs/{lab_id}", state.GetLab)
	router.Get("/api/courses/{slug}/progress", state.GetCourseProgress)

	router.Get("/api/courses/{slug}/lessons", state.ListLessons)
	router.Get("/api/courses/{slug}/lessons/{lessonSlug}", state.GetLesson)
	router.Post("/api/courses/{slug}/lessons/{lessonSlug}/complete", state.MarkLessonComplete)
}

// registerTrainerRoutes wires endpoints for admin and manager roles.
func registerTrainerRoutes(router chi.Router, state *State) {
	router.Post("/api/admin/courses/{slug}/sessions", state.CreateSession)
	router.Put("/api/admin/courses/{slug}/sessions/{sessionId}", state.UpdateSession)
	router.Delete("/api/admin/courses/{slug}/sessions/{sessionId}", state.DeleteSession)
}

// registerAdminRoutes wires the admin-only endpoints behind the Admin
// middleware, which enforces role == "admin" in addition to JWT validity.
func registerAdminRoutes(router chi.Router, state *State) {
	router.Get("/api/admin/courses", state.ListAdminCourses)
	router.Post("/api/admin/courses", state.CreateCourse)
	router.Post("/api/admin/courses/import", state.ImportCourseMarkdown)
	router.Post("/api/admin/courses/import/preview", state.PreviewCourseMarkdownImport)
	router.Get("/api/admin/courses/{slug}/export/markdown", state.ExportCourseMarkdown)
	router.Get("/api/admin/courses/{slug}/definition", state.GetCourseDefinition)
	router.Put("/api/admin/courses/{slug}/definition", state.UpdateCourse)
	router.Delete("/api/admin/courses/{slug}/definition", state.DeleteCourse)
	router.Post("/api/admin/courses/paths", state.CreatePath)
	router.Get("/api/admin/courses/paths/{slug}/definition", state.GetPathDefinition)
	router.Put("/api/admin/courses/paths/{slug}/definition", state.UpdatePath)
	router.Delete("/api/admin/courses/paths/{slug}/definition", state.DeletePath)
	router.Get("/api/admin/lab-checks", state.GetLabResults)
	router.Get("/api/admin/exports/lab-checks/categories", state.ExportLabCategories)
	router.Post("/api/admin/exports/lab-checks/preview", state.ExportLabPreview)
	router.Post("/api/admin/exports/lab-checks/download", state.ExportLabDownload)
	router.Post("/api/admin/cache/clear", state.ClearCache)
	router.Post("/api/admin/courses/{slug}/cache/clear", state.ClearCourseCache)
	router.Post("/api/admin/courses/{slug}/modules/{index}/cache/clear", state.ClearModuleCache)
}
