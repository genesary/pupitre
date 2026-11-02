package handlers

// The standalone course-service is gone: the monolith mounts RegisterRoutes
// on its own router and wires State.Users and State.Checker to the users
// domain and the checker (docs/specs/104-backend-in-process-calls.md).
//
// This file keeps, for the tests only, what the standalone service had: its
// router, and its HTTP clients of user-service and checker-service. The
// handler tests stand in for those services with httptest servers and keep
// their assertions.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/httpx"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/metrics"
)

// usersAt is the users domain as the user-service HTTP server of a test
// answers it.
func usersAt(baseURL string) UserProgress { //nolint:ireturn // test helper
	return &httpUserProgress{baseURL: baseURL}
}

// checkerAt is the checker as the checker-service HTTP server of a test
// answers it.
func checkerAt(baseURL string) LabChecker { //nolint:ireturn // test helper
	return &httpLabChecker{baseURL: baseURL}
}

// withPeers attaches the test servers standing in for the other domains to
// a State; an empty URL leaves that domain unwired. With a checker, the
// student of authHeader also gets the identity-provider login lab checks
// need (docs/specs/190-backend-defects.md B11).
func withPeers(s *State, usersURL, checkerURL string) *State {
	if usersURL != "" {
		s.Users = usersAt(usersURL)
	}

	if checkerURL != "" {
		s.Checker = checkerAt(checkerURL)

		base := s.Users
		if base == nil {
			base = noUsers{}
		}

		s.Users = identityUsers{UserProgress: base, name: testLabIdentity}
	}

	return s
}

// testLabIdentity is the identity-provider login of the student of
// authHeader, the same as their username.
const testLabIdentity = "testuser"

// identityUsers adds a learner's identity-provider login to a users domain.
type identityUsers struct {
	UserProgress

	name string
}

// LabIdentity implements UserProgress.
func (u identityUsers) LabIdentity(context.Context, string) (string, error) {
	return u.name, nil
}

// BuildRouter constructs the course-service chi router, wiring public
// routes, user-authenticated routes, and admin-only routes behind their
// respective middleware, plus CORS/logging middleware.
func BuildRouter(state *State, cfg *config.Config, withLogger bool) *chi.Mux {
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

	router.Get("/health", state.Health)
	router.Get("/metrics", metrics.Handler())
	RegisterRoutes(router, state, cfg)

	return router
}

// routerCORSMaxAgeSeconds is the max-age advertised for CORS preflight
// response caching.
const routerCORSMaxAgeSeconds = 300

// httpUserProgress implements UserProgress over the internal HTTP routes of
// user-service.
type httpUserProgress struct {
	baseURL string
}

// CourseOverview implements UserProgress.
func (c *httpUserProgress) CourseOverview(ctx context.Context, userID, courseSlug string) (ProgressOverview, error) {
	var body ProgressOverview

	err := c.getInternalJSON(ctx, "/internal/progress/overview",
		map[string]string{userIDJSONKey: userID, courseSlugJSONKey: courseSlug}, &body)
	if err != nil {
		return ProgressOverview{}, err
	}

	return body, nil
}

// CourseSummaries implements UserProgress.
func (c *httpUserProgress) CourseSummaries(ctx context.Context, userID string, courseSlugs []string) (map[string]CourseSummary, error) {
	var body courseSummariesBody

	err := c.getInternalJSON(ctx, "/internal/progress/course-summaries",
		map[string]string{userIDJSONKey: userID, "courseSlugs": strings.Join(courseSlugs, ",")}, &body)
	if err != nil {
		return nil, err
	}

	return body.Summaries, nil
}

// EnrolledInAnyPath implements UserProgress.
func (c *httpUserProgress) EnrolledInAnyPath(ctx context.Context, userID string, pathSlugs []string) (bool, error) {
	var result pathCheckResponse

	err := c.getInternalJSON(ctx, "/internal/paths/check",
		map[string]string{userIDJSONKey: userID, "pathSlugs": strings.Join(pathSlugs, ",")}, &result)
	if err != nil {
		return false, err
	}

	return result.Enrolled, nil
}

// IsEnrolled implements UserProgress.
func (c *httpUserProgress) IsEnrolled(ctx context.Context, userID, courseSlug string) (bool, error) {
	var result struct {
		Enrolled bool `json:"enrolled"`
	}

	err := c.getInternalJSON(ctx, "/internal/enrollments/check",
		map[string]string{userIDJSONKey: userID, courseSlugJSONKey: courseSlug}, &result)
	if err != nil {
		return false, err
	}

	return result.Enrolled, nil
}

// AutoEnroll implements UserProgress.
func (c *httpUserProgress) AutoEnroll(ctx context.Context, userID, courseSlug string) error {
	return c.postInternal(ctx, "/internal/enrollments/auto", map[string]string{
		userIDJSONKey:     userID,
		courseSlugJSONKey: courseSlug,
	})
}

// MarkLessonComplete implements UserProgress.
func (c *httpUserProgress) MarkLessonComplete(ctx context.Context, userID, courseSlug, lessonSlug string) error {
	return c.postInternal(ctx, "/internal/progress/complete", map[string]string{
		userIDJSONKey:     userID,
		courseSlugJSONKey: courseSlug,
		"lessonSlug":      lessonSlug,
	})
}

// MarkCourseComplete implements UserProgress.
func (c *httpUserProgress) MarkCourseComplete(ctx context.Context, completion CourseCompletion) error {
	return c.postInternal(ctx, "/internal/progress/course-complete", completion)
}

// RecordModule implements UserProgress.
func (c *httpUserProgress) RecordModule(ctx context.Context, result ModuleResult) error {
	return c.postInternal(ctx, "/internal/progress/module", result)
}

// LabIdentity implements UserProgress: user-service had no such route; the
// tests that need an identity use withPeers with a checker.
func (c *httpUserProgress) LabIdentity(context.Context, string) (string, error) {
	return "", nil
}

// internalURL builds a user-service internal URL with the given query
// parameters.
func (c *httpUserProgress) internalURL(path string, params map[string]string) (string, error) {
	target, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", fmt.Errorf("build user-service URL %s: %w", path, err)
	}

	query := target.Query()
	for key, value := range params {
		query.Set(key, value)
	}

	target.RawQuery = query.Encode()

	return target.String(), nil
}

// getInternalJSON performs an authenticated GET against a user-service
// internal endpoint and decodes the response into dest.
func (c *httpUserProgress) getInternalJSON(ctx context.Context, path string, params map[string]string, dest any) error {
	target, err := c.internalURL(path, params)
	if err != nil {
		return err
	}

	err = httpx.GetJSON(ctx, target, nil, dest)
	if err != nil {
		return fmt.Errorf("user-service %s: %w", path, err)
	}

	return nil
}

// postInternal sends payload as JSON to a user-service internal endpoint
// and reports whether it was accepted. The response body is drained so the
// connection returns to the keep-alive pool.
func (c *httpUserProgress) postInternal(ctx context.Context, path string, payload any) error {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(payload)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", path, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &buf)
	if err != nil {
		return fmt.Errorf("build POST %s: %w", path, err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpx.Do(req) //nolint:bodyclose // httpx.Drain closes it
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer httpx.Drain(resp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("user-service %s returned %d", path, resp.StatusCode)
	}

	return nil
}

// courseSummariesBody is the wire shape of
// GET /internal/progress/course-summaries.
type courseSummariesBody struct {
	Summaries map[string]CourseSummary `json:"summaries"`
}

// httpLabChecker implements LabChecker over checker-service's HTTP routes.
type httpLabChecker struct {
	baseURL string
}

// Evaluate implements LabChecker.
func (c *httpLabChecker) Evaluate(ctx context.Context, req EvaluateRequest) (CheckResponse, error) {
	return c.post(ctx, "/evaluate", req)
}

// CheckStep implements LabChecker.
func (c *httpLabChecker) CheckStep(ctx context.Context, req StepCheckRequest) (CheckResponse, error) {
	return c.post(ctx, "/check-step", req)
}

// post marshals body, POSTs it to checker-service at the given path, and
// decodes the CheckResponse.
func (c *httpLabChecker) post(ctx context.Context, route string, body any) (CheckResponse, error) {
	buildErr := &CheckerError{Status: http.StatusInternalServerError, Message: buildCheckerRequestFailed}

	payload, err := json.Marshal(body)
	if err != nil {
		return CheckResponse{}, buildErr
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+route, bytes.NewReader(payload))
	if err != nil {
		return CheckResponse{}, buildErr
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpx.Do(httpReq) //nolint:bodyclose // httpx.Drain closes it
	if err != nil {
		return CheckResponse{}, fmt.Errorf("POST %s: %w", route, err)
	}

	defer httpx.Drain(resp)

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]string

		_ = json.NewDecoder(resp.Body).Decode(&errBody)

		return CheckResponse{}, checkerRejected(errBody[errorJSONKey])
	}

	var result CheckResponse

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return CheckResponse{}, &CheckerError{Status: http.StatusInternalServerError, Message: "Failed to decode checker response"}
	}

	return result, nil
}
