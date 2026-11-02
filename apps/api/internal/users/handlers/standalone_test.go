package handlers

// The standalone user-service is gone: the monolith mounts RegisterRoutes
// on its own router, and the courses domain calls the progress methods of
// progress_service.go directly (docs/specs/104-backend-in-process-calls.md).
//
// This file keeps, for the tests only, what the standalone service had: its
// router with the /internal/* routes, as thin HTTP shims over those methods,
// and its HTTP client of course-service. The handler tests drive the domain
// through them and keep their assertions.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/httpx"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/metrics"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
)

// courseSourceAt is the courses domain as the course-service HTTP server
// of a test answers it.
func courseSourceAt(baseURL string) CourseSource { //nolint:ireturn // test helper
	return &httpCourseSource{baseURL: strings.TrimRight(baseURL, "/")}
}

// BuildRouter assembles the HTTP router, wiring public, authenticated, admin,
// pattern, and internal routes with their respective middleware.
func BuildRouter(state *State, cfg *config.Config, withLogger bool) *chi.Mux {
	router := chi.NewRouter()
	router.Use(chiMiddleware.RequestID)
	//nolint:staticcheck // SA1019: RealIP deprecation acknowledged — service runs behind a trusted reverse proxy that sets X-Forwarded-For; the value is used for logging/rate-limiting only.
	router.Use(chiMiddleware.RealIP)

	if withLogger {
		router.Use(chiMiddleware.Logger)
	}

	router.Use(chiMiddleware.Recoverer)
	router.Use(chiMiddleware.RequestSize(maxRequestBodyBytes))
	router.Use(corsHandler(cfg).Handler)

	router.Get("/health", state.Health)
	router.Get("/metrics", metrics.Handler())
	RegisterRoutes(router, state, cfg)
	registerInternalRoutes(router, state)

	return router
}

// corsHandler builds the CORS middleware from the configured allowed origins.
func corsHandler(cfg *config.Config) *cors.Cors {
	return cors.New(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           corsMaxAgeSeconds,
	})
}

// registerInternalRoutes wires routes reserved for Course Service behind the
// shared-secret middleware (X-Internal-Secret header).
func registerInternalRoutes(router chi.Router, state *State) {
	router.Group(func(group chi.Router) {
		group.Post("/internal/enrollments/auto", state.InternalAutoEnroll)
		group.Get("/internal/enrollments/check", state.InternalCheckEnrollment)
		group.Get("/internal/paths/check", state.InternalCheckPathEnrollment)
		group.Get("/internal/progress/viewed", state.InternalViewedLessons)
		group.Post("/internal/progress/complete", state.InternalMarkComplete)
		group.Post("/internal/progress/course-complete", state.InternalMarkCourseComplete)
		group.Post("/internal/progress/module", state.InternalRecordModuleProgress)
		group.Get("/internal/progress/modules", state.InternalGetModuleProgress)
		group.Get("/internal/progress/course-summary", state.InternalCourseSummary)
		group.Get("/internal/progress/course-summaries", state.InternalCourseSummaries)
		group.Get("/internal/progress/overview", state.InternalProgressOverview)
	})
}

// corsMaxAgeSeconds bounds CORS preflight cache duration, in seconds.
const corsMaxAgeSeconds = 300

// courseBody is the request body for endpoints keyed by user+course.
type courseBody struct {
	UserID string `json:"userId"`

	CourseSlug string `json:"courseSlug"`
}

// lessonCompleteBody is the request body for marking a single lesson complete.
type lessonCompleteBody struct {
	UserID string `json:"userId"`

	CourseSlug string `json:"courseSlug"`

	LessonSlug string `json:"lessonSlug"`
}

// internalDecodeOrBadRequest decodes the JSON request body into dst. On
// failure it writes a 400 response and returns false; callers must return
// immediately in that case.
func internalDecodeOrBadRequest(s *State, writer http.ResponseWriter, req *http.Request, dst any) bool {
	err := decode(req, dst)
	if err != nil {
		zap.L().Error("JSON decode failed", zap.Error(err))
		s.Error(writer, http.StatusBadRequest, "Invalid request body")

		return false
	}

	return true
}

// internalRespondExecResult logs execErr (if any) alongside logMsg/fields and
// writes the {"ok": bool} response shared by internal write endpoints.
func internalRespondExecResult(state *State, writer http.ResponseWriter, execErr error, logMsg string, fields ...zap.Field) {
	if execErr != nil {
		zap.L().Error(logMsg, append(fields, zap.Error(execErr))...)
		state.Error(writer, http.StatusInternalServerError, "DB error")

		return
	}

	state.JSON(writer, http.StatusOK, map[string]bool{"ok": true})
}

// InternalAutoEnroll godoc
// @Summary  Auto-enroll a user in a public course (internal)
// @Tags     Internal
// @Accept   json
// @Produce  json
// @Param    body  body  object  true  "userId, courseSlug"
// @Success  200   {object}  map[string]bool
// @Router   /internal/enrollments/auto [post].
func (s *State) InternalAutoEnroll(writer http.ResponseWriter, req *http.Request) {
	var body courseBody
	if !internalDecodeOrBadRequest(s, writer, req, &body) {
		return
	}

	err := s.AutoEnroll(req.Context(), body.UserID, body.CourseSlug)
	internalRespondExecResult(s, writer, err, "failed to auto-enroll user",
		zap.String("userID", body.UserID), zap.String("courseSlug", body.CourseSlug))
}

// InternalCheckEnrollment godoc
// @Summary  Check if a user is enrolled (internal)
// @Tags     Internal
// @Produce  json
// @Param    userId      query  string  true  "User UUID"
// @Param    courseSlug  query  string  true  "Course slug"
// @Success  200  {object}  map[string]bool
// @Router   /internal/enrollments/check [get].
func (s *State) InternalCheckEnrollment(writer http.ResponseWriter, req *http.Request) {
	userID := req.URL.Query().Get("userId")
	courseSlug := req.URL.Query().Get("courseSlug")

	enrolled, err := s.IsEnrolled(req.Context(), userID, courseSlug)
	if err != nil {
		zap.L().Error("failed to check enrollment", zap.String("userID", userID), zap.String("courseSlug", courseSlug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "DB error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]bool{adminJSONKeyEnrolled: enrolled})
}

// InternalCheckPathEnrollment godoc
// @Summary  Check if a user is enrolled in any of the given paths (internal)
// @Tags     Internal
// @Produce  json
// @Param    userId     query  string  true  "User UUID"
// @Param    pathSlugs  query  string  true  "Comma-separated path slugs"
// @Success  200  {object}  map[string]bool
// @Router   /internal/paths/check [get].
func (s *State) InternalCheckPathEnrollment(writer http.ResponseWriter, req *http.Request) {
	userID := req.URL.Query().Get("userId")
	slugs := splitCommaList(req.URL.Query().Get("pathSlugs"))

	enrolled, err := s.EnrolledInAnyPath(req.Context(), userID, slugs)
	if err != nil {
		zap.L().Error("failed to check path enrollment", zap.String("userID", userID), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "DB error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]bool{adminJSONKeyEnrolled: enrolled})
}

// InternalViewedLessons godoc
// @Summary  Get viewed lessons for a user (internal)
// @Tags     Internal
// @Produce  json
// @Param    userId      query  string  true  "User UUID"
// @Param    courseSlug  query  string  true  "Course slug"
// @Success  200  {object}  map[string]interface{}
// @Router   /internal/progress/viewed [get].
func (s *State) InternalViewedLessons(writer http.ResponseWriter, req *http.Request) {
	userID := req.URL.Query().Get("userId")
	courseSlug := req.URL.Query().Get("courseSlug")

	slugs, err := s.Repos.LessonProgress.ViewedSlugs(req.Context(), userID, courseSlug)
	if err != nil {
		zap.L().Error("failed to query viewed lessons", zap.String("userID", userID), zap.String("courseSlug", courseSlug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "DB error")

		return
	}

	if slugs == nil {
		slugs = []string{}
	}

	s.JSON(writer, http.StatusOK, map[string]any{"viewed": slugs})
}

// InternalMarkComplete godoc
// @Summary  Mark a lesson complete (internal)
// @Tags     Internal
// @Accept   json
// @Produce  json
// @Param    body  body  object  true  "userId, courseSlug, lessonSlug"
// @Success  200   {object}  map[string]bool
// @Router   /internal/progress/complete [post].
func (s *State) InternalMarkComplete(writer http.ResponseWriter, req *http.Request) {
	var body lessonCompleteBody
	if !internalDecodeOrBadRequest(s, writer, req, &body) {
		return
	}

	err := s.MarkLessonComplete(req.Context(), body.UserID, body.CourseSlug, body.LessonSlug)
	internalRespondExecResult(s, writer, err, "failed to mark lesson complete",
		zap.String("userID", body.UserID), zap.String("courseSlug", body.CourseSlug), zap.String("lessonSlug", body.LessonSlug))
}

// InternalMarkCourseComplete godoc
// @Summary  Mark a whole course as complete (internal)
// @Tags     Internal
// @Accept   json
// @Produce  json
// @Param    body  body  object  true  "userId, courseSlug, skills, difficulty"
// @Success  200   {object}  map[string]bool
// @Router   /internal/progress/course-complete [post].
func (s *State) InternalMarkCourseComplete(writer http.ResponseWriter, req *http.Request) {
	var body CourseCompletion
	if !internalDecodeOrBadRequest(s, writer, req, &body) {
		return
	}

	err := s.MarkCourseComplete(req.Context(), body)
	internalRespondExecResult(s, writer, err, "failed to mark course complete",
		zap.String("userID", body.UserID), zap.String("courseSlug", body.CourseSlug))
}

// InternalRecordModuleProgress godoc
// @Summary  Record quiz result for a module (internal)
// @Tags     Internal
// @Accept   json
// @Produce  json
// @Param    body  body  object  true  "see ModuleResult"
// @Success  200   {object}  map[string]bool
// @Router   /internal/progress/module [post].
func (s *State) InternalRecordModuleProgress(writer http.ResponseWriter, req *http.Request) {
	var body ModuleResult
	if !internalDecodeOrBadRequest(s, writer, req, &body) {
		return
	}

	err := s.RecordModuleProgress(req.Context(), body)
	internalRespondExecResult(s, writer, err, "failed to record module progress",
		zap.String("userID", body.UserID), zap.String("courseSlug", body.CourseSlug))
}

// InternalCourseSummary godoc
// @Summary  Get total score and passed modules for a user in a course
// @Tags     Internal
// @Produce  json
// @Param    userId      query  string  true  "User UUID"
// @Param    courseSlug  query  string  true  "Course slug"
// @Success  200  {object}  map[string]interface{}
// @Router   /internal/progress/course-summary [get].
// It is the single-course shape of InternalCourseSummaries and shares its
// implementation: two queries whose rows are summed in Go, rather than the
// three aggregate queries (SUM, passed slugs, COUNT) it used to run over
// the same two index scans.
func (s *State) InternalCourseSummary(writer http.ResponseWriter, req *http.Request) {
	userID := req.URL.Query().Get("userId")
	courseSlug := req.URL.Query().Get("courseSlug")
	slugs := []string{courseSlug}

	viewedByCourse, moduleRows, err := s.loadCourseProgress(req.Context(), userID, slugs)
	if err != nil {
		zap.L().Error("failed to query course summary",
			zap.String("userID", userID), zap.String("courseSlug", courseSlug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "DB error")

		return
	}

	summary := summarizeCourseProgress(moduleRows[courseSlug], viewedByCourse[courseSlug])

	s.JSON(writer, http.StatusOK, map[string]any{
		"totalScore":    summary.TotalScore,
		"passedModules": summary.PassedModules,
		"viewedCount":   summary.ViewedCount,
	})
}

// InternalGetModuleProgress godoc
// @Summary  Get module progress for a user in a course (internal)
// @Tags     Internal
// @Produce  json
// @Param    userId      query  string  true  "User UUID"
// @Param    courseSlug  query  string  true  "Course slug"
// @Success  200  {object}  map[string]interface{}
// @Router   /internal/progress/modules [get].
func (s *State) InternalGetModuleProgress(writer http.ResponseWriter, req *http.Request) {
	userID := req.URL.Query().Get("userId")
	courseSlug := req.URL.Query().Get("courseSlug")

	list, err := s.Repos.ModuleProgress.ListByUserCourse(req.Context(), userID, courseSlug)
	if err != nil {
		zap.L().Error("failed to query module progress", zap.String("userID", userID), zap.String("courseSlug", courseSlug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "DB error")

		return
	}

	progress := make([]moduleProgressRow, 0, len(list))
	for _, p := range list {
		progress = append(progress, moduleProgressRow{
			ModuleIndex: p.ModuleIndex, BestScore: p.BestScore, MaxScore: p.MaxScore, Passed: p.Passed, Attempts: p.Attempts,
		})
	}

	s.JSON(writer, http.StatusOK, map[string]any{"progress": progress})
}

// summariesJSONKey is the JSON key carrying the per-course summary map.
const summariesJSONKey = "summaries"

// InternalProgressOverview godoc
// @Summary  Get a learner's full progress for one course (internal)
// @Tags     Internal
// @Produce  json
// @Param    userId      query  string  true  "User UUID"
// @Param    courseSlug  query  string  true  "Course slug"
// @Success  200  {object}  ProgressOverview
// @Router   /internal/progress/overview [get].
func (s *State) InternalProgressOverview(writer http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	userID := req.URL.Query().Get("userId")
	courseSlug := req.URL.Query().Get("courseSlug")

	overview, err := s.ProgressOverview(ctx, userID, courseSlug)
	if errors.Is(err, errMissingProgressKeys) {
		s.Error(writer, http.StatusBadRequest, err.Error())

		return
	}

	if err != nil {
		zap.L().Error("failed to load progress overview",
			zap.String("userID", userID), zap.String("courseSlug", courseSlug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "DB error")

		return
	}

	s.JSON(writer, http.StatusOK, overview)
}

// InternalCourseSummaries godoc
// @Summary  Get aggregate progress for several courses at once (internal)
// @Tags     Internal
// @Produce  json
// @Param    userId       query  string  true  "User UUID"
// @Param    courseSlugs  query  string  true  "Comma-separated course slugs"
// @Success  200  {object}  map[string]interface{}
// @Router   /internal/progress/course-summaries [get].
func (s *State) InternalCourseSummaries(writer http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	userID := req.URL.Query().Get("userId")
	slugs := splitCommaList(req.URL.Query().Get("courseSlugs"))

	summaries, err := s.CourseSummaries(ctx, userID, slugs)
	if errors.Is(err, errMissingUserID) || errors.Is(err, errTooManyCourseSlugs) {
		s.Error(writer, http.StatusBadRequest, err.Error())

		return
	}

	if err != nil {
		zap.L().Error("failed to load course summaries", zap.String("userID", userID), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "DB error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{summariesJSONKey: summaries})
}

// splitCommaList splits a comma-separated query parameter into its
// non-empty, trimmed, deduplicated entries, preserving first-seen order.
func splitCommaList(raw string) []string {
	if raw == "" {
		return nil
	}

	seen := make(map[string]struct{})

	var out []string

	for entry := range strings.SplitSeq(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if _, dup := seen[entry]; dup {
			continue
		}

		seen[entry] = struct{}{}

		out = append(out, entry)
	}

	return out
}

// NewCatalog builds a Catalog reading from the course-service at baseURL.
func NewCatalog(baseURL string) *Catalog {
	return NewCatalogFrom(&httpCourseSource{baseURL: strings.TrimRight(baseURL, "/")})
}

// httpCourseSource implements CourseSource over course-service's public
// routes.
type httpCourseSource struct {
	baseURL string
}

// Courses implements CourseSource.
func (c *httpCourseSource) Courses(ctx context.Context, slugs []string) ([]CourseInfo, error) {
	var body struct {
		Courses []CourseInfo `json:"courses"`
	}

	err := httpx.GetJSON(ctx, c.batchURL("/api/batch/courses", slugs), nil, &body)
	if err != nil {
		return nil, fmt.Errorf("GET /api/batch/courses: %w", err)
	}

	return body.Courses, nil
}

// Paths implements CourseSource.
func (c *httpCourseSource) Paths(ctx context.Context, slugs []string) ([]PathInfo, error) {
	var body struct {
		Paths []PathInfo `json:"paths"`
	}

	err := httpx.GetJSON(ctx, c.batchURL("/api/batch/paths", slugs), nil, &body)
	if err != nil {
		return nil, fmt.Errorf("GET /api/batch/paths: %w", err)
	}

	return body.Paths, nil
}

// SkillModules implements CourseSource.
func (c *httpCourseSource) SkillModules(ctx context.Context, skills []string) (map[string][]SkillModule, error) {
	var body struct {
		Skills map[string][]SkillModule `json:"skills"`
	}

	err := httpx.GetJSON(ctx, c.batchURL("/api/batch/skills", skills), nil, &body)
	if err != nil {
		return nil, fmt.Errorf("GET /api/batch/skills: %w", err)
	}

	return body.Skills, nil
}

// CourseSessions implements CourseSource. Over HTTP it reads the course
// anonymously, as user-service did, so a private course has no sessions.
func (c *httpCourseSource) CourseSessions(ctx context.Context, courseSlug string) ([]CourseSessionInfo, error) {
	var info struct {
		Sessions []CourseSessionInfo `json:"sessions,omitempty"`
	}

	err := httpx.GetJSON(ctx, c.baseURL+"/api/courses/"+url.PathEscape(courseSlug), nil, &info)
	if err != nil {
		return nil, fmt.Errorf("GET /api/courses/%s: %w", courseSlug, err)
	}

	return info.Sessions, nil
}

// PublicCourseTitles implements CourseSource.
func (c *httpCourseSource) PublicCourseTitles(ctx context.Context) (map[string]string, error) {
	return c.fetchTitleMap(ctx, "/api/courses", "courses")
}

// PathTitles implements CourseSource.
func (c *httpCourseSource) PathTitles(ctx context.Context) (map[string]string, error) {
	return c.fetchTitleMap(ctx, "/api/paths", "paths")
}

// fetchTitleMap calls the internal course-service at the given path constant
// and returns a slug→title map. endpointPath must be a hardcoded constant —
// it is never derived from user input.
func (c *httpCourseSource) fetchTitleMap(ctx context.Context, endpointPath, payloadKey string) (map[string]string, error) {
	rawURL, joinErr := url.JoinPath(c.baseURL, endpointPath)
	if joinErr != nil {
		return nil, fmt.Errorf("build URL for %s: %w", endpointPath, joinErr)
	}

	var raw map[string]json.RawMessage

	err := httpx.GetJSON(ctx, rawURL, nil, &raw)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", endpointPath, err)
	}

	var entries []titleEntry

	unmarshalErr := json.Unmarshal(raw[payloadKey], &entries)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", payloadKey, unmarshalErr)
	}

	titles := make(map[string]string, len(entries))

	for _, entry := range entries {
		titles[entry.Slug] = entry.Title
	}

	return titles, nil
}

// batchURL builds a course-service batch URL for the given slugs.
func (c *httpCourseSource) batchURL(path string, slugs []string) string {
	return c.baseURL + path + "?" + url.Values{"slugs": {strings.Join(slugs, ",")}}.Encode()
}
