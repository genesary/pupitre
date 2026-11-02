package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
	apimiddleware "github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
)

// newStateWith builds a State backed by in-memory repository fakes seeded
// with the given courses, so handler tests exercise the same code paths as
// production without needing a database.
func newStateWith(cfg *config.Config, courses ...*content.Course) *State {
	return NewState(cfg, &repository.Repositories{
		Courses:      fake.NewCourseRepository(courses...),
		Paths:        fake.NewPathRepository(),
		QuizAttempts: fake.NewQuizAttemptRepository(),
		LabChecks:    fake.NewLabCheckRepository(),
	})
}

// newTestState builds a State seeded with published and unpublished test
// courses, backed by userSrv for auth calls.
func newTestState(t *testing.T, userSrv *httptest.Server) *State {
	t.Helper()

	seed := []*content.Course{
		{
			Slug:        "kubernetes-basics",
			Title:       "Kubernetes Basics",
			Description: "Intro to Kubernetes",
			Category:    "kubernetes",
			Difficulty:  "beginner",
			IsPublic:    true,
			Modules: []content.Module{
				{Name: "What is K8s", Type: "text"},
				{Name: "Architecture", Type: "video", Src: "/uploads/arch.mp4"},
				{Name: "Pods Explained", Type: "image", Src: "/uploads/pods.png"},
			},
		},
		{
			Slug:        "docker-fundamentals",
			Title:       "Docker Fundamentals",
			Description: "Learn Docker from scratch",
			Category:    "docker",
			Difficulty:  "beginner",
			IsPublic:    false,
		},
		{
			Slug:        "advanced-kubernetes",
			Title:       "Advanced Kubernetes",
			Description: "Deep dive into K8s networking and scheduling",
			Category:    "kubernetes",
			Difficulty:  "advanced",
			IsPublic:    true,
		},
	}

	cfg := &config.Config{
		JWTSecret:  "test-secret",
		JWTExpiryH: 24,
		UploadsDir: "./testdata",

		GitCredentialsPath: ""}

	return withPeers(newStateWith(cfg, seed...), userSrv.URL, "")
}

// newUserServiceMock returns a test server with sensible defaults for all
// internal user-service endpoints used by the course-service.
func newUserServiceMock() *httptest.Server {
	return newUserServiceMockWith(nil)
}

// newUserServiceMockWith returns a test server where the caller can override
// individual path handlers via the overrides map.  Any path not overridden
// falls back to the standard default response.
func newUserServiceMockWith(overrides map[string]http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fn, ok := overrides[r.URL.Path]; ok {
			fn(w, r)

			return
		}

		switch r.URL.Path {
		case "/internal/enrollments/auto":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		case "/internal/enrollments/check":
			json.NewEncoder(w).Encode(map[string]bool{"enrolled": true})
		case "/internal/progress/viewed":
			json.NewEncoder(w).Encode(map[string]any{"viewed": []string{}})
		case "/internal/progress/complete":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		case "/internal/progress/modules":
			json.NewEncoder(w).Encode(map[string]any{"progress": []any{}})
		case "/internal/progress/course-summary":
			json.NewEncoder(w).Encode(map[string]any{
				"totalScore":    0,
				"passedModules": []string{},
			})
		case "/internal/progress/course-summaries":
			json.NewEncoder(w).Encode(map[string]any{"summaries": map[string]any{}})
		case "/internal/progress/overview":
			json.NewEncoder(w).Encode(map[string]any{
				"enrolled":      true,
				"viewed":        []string{},
				"modules":       []any{},
				"totalScore":    0,
				"passedModules": []string{},
				"viewedCount":   0,
			})
		case "/internal/paths/check":
			json.NewEncoder(w).Encode(map[string]bool{"enrolled": false})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// authHeader returns a Bearer token header for a student user.
func authHeader(t *testing.T, secret string) string {
	t.Helper()

	token, err := apimiddleware.CreateToken(
		"00000000-0000-0000-0000-000000000001",
		"test@test.com",
		"student",
		"testuser",
		secret,
		24,
	)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	return "Bearer " + token
}

// adminAuthHeader returns a Bearer token header for an admin user.
func adminAuthHeader(t *testing.T, secret string) string {
	t.Helper()

	token, err := apimiddleware.CreateToken(
		"00000000-0000-0000-0000-000000000000",
		"admin@test.com",
		"admin",
		"testadmin",
		secret,
		24,
	)
	if err != nil {
		t.Fatalf("create admin token: %v", err)
	}

	return "Bearer " + token
}

// TestHealth verifies health behavior.
func TestHealth(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", body["status"])
	}

	if body["service"] != "course-service" {
		t.Errorf("expected service=course-service, got %s", body["service"])
	}
}

// TestListCourses verifies list courses behavior.
func TestListCourses(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Courses []CourseResponse `json:"courses"`
		Total   int              `json:"total"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 2 {
		t.Fatalf("expected 2 published courses, got %d", resp.Total)
	}

	slugs := make(map[string]bool)
	for _, c := range resp.Courses {
		slugs[c.Slug] = true
	}

	if !slugs["kubernetes-basics"] {
		t.Error("expected kubernetes-basics in list")
	}

	if !slugs["advanced-kubernetes"] {
		t.Error("expected advanced-kubernetes in list")
	}

	if slugs["docker-fundamentals"] {
		t.Error("docker-fundamentals is unpublished, should not appear")
	}
}

// TestListCoursesFiltered verifies list courses filtered behavior.
func TestListCoursesFiltered(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses?category=kubernetes", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp struct {
		Courses []CourseResponse `json:"courses"`
		Total   int              `json:"total"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 2 {
		t.Fatalf("expected 2 kubernetes courses, got %d", resp.Total)
	}

	slugs := make(map[string]bool)
	for _, c := range resp.Courses {
		slugs[c.Slug] = true
	}

	if !slugs["kubernetes-basics"] {
		t.Error("expected kubernetes-basics in filtered results")
	}

	if !slugs["advanced-kubernetes"] {
		t.Error("expected advanced-kubernetes in filtered results")
	}
}

// TestGetCourse verifies get course behavior.
func TestGetCourse(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var c CourseResponse
	json.NewDecoder(rec.Body).Decode(&c)

	if c.Slug != "kubernetes-basics" {
		t.Errorf("expected slug=kubernetes-basics, got %s", c.Slug)
	}

	if c.ModuleCount != 3 {
		t.Errorf("expected 3 modules, got %d", c.ModuleCount)
	}

	if c.Category != "kubernetes" {
		t.Errorf("expected category=kubernetes, got %s", c.Category)
	}
}

// TestGetCourseNotFound verifies get course not found behavior.
func TestGetCourseNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/does-not-exist", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestGetCourseUnpublished verifies get course unpublished behavior.
func TestGetCourseUnpublished(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unpublished course, got %d", rec.Code)
	}
}

// TestModulesListed verifies modules listed behavior.
func TestModulesListed(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Modules []moduleResponse `json:"modules"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Modules) != 3 {
		t.Fatalf("expected 3 modules, got %d", len(resp.Modules))
	}

	expected := []struct {
		name string
		typ  string
	}{
		{"What is K8s", "text"},
		{"Architecture", "video"},
		{"Pods Explained", "image"},
	}
	for i, e := range expected {
		if resp.Modules[i].Name != e.name {
			t.Errorf("module %d: expected name %s, got %s", i, e.name, resp.Modules[i].Name)
		}

		if resp.Modules[i].Type != e.typ {
			t.Errorf("module %d: expected type %s, got %s", i, e.typ, resp.Modules[i].Type)
		}

		if resp.Modules[i].Slug == "" {
			t.Errorf("module %d: expected non-empty slug", i)
		}
	}
}

// TestModulesAuthRequired verifies modules auth required behavior.
func TestModulesAuthRequired(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/modules", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

// TestAdminBypassUnpublishedModule verifies admin bypass unpublished module
// behavior.
func TestAdminBypassUnpublishedModule(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals/modules", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin on unpublished, got %d", rec.Code)
	}
}

// TestGetModuleContent verifies get module content behavior.
func TestGetModuleContent(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/modules/1", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp moduleResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Type != "video" {
		t.Errorf("expected type video, got %s", resp.Type)
	}

	if resp.Content != "/uploads/arch.mp4" {
		t.Errorf("expected content /uploads/arch.mp4, got %s", resp.Content)
	}
}

// TestGetModuleNotFound verifies get module not found behavior.
func TestGetModuleNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/modules/99", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestLessonsListedFromModules verifies lessons listed from modules
// behavior.
func TestLessonsListedFromModules(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/lessons", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lessons []lessonSummary `json:"lessons"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Lessons) != 3 {
		t.Fatalf("expected 3 lessons from modules, got %d", len(resp.Lessons))
	}

	expected := []string{"what-is-k8s", "architecture", "pods-explained"}
	for i, slug := range expected {
		if resp.Lessons[i].Slug != slug {
			t.Errorf("lesson %d: expected slug %s, got %s", i, slug, resp.Lessons[i].Slug)
		}
	}
}

// TestLessonsAuthRequired verifies lessons auth required behavior.
func TestLessonsAuthRequired(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/lessons", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

// TestLessonComplete verifies lesson complete behavior.
func TestLessonComplete(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/kubernetes-basics/lessons/what-is-k8s/complete", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Message string `json:"message"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Message != "Lesson marked as complete" {
		t.Errorf("unexpected message: %s", resp.Message)
	}
}

// TestMetrics verifies metrics behavior.
func TestMetrics(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	var lastBody string

	for _, path := range []string{"/metrics", "/health", "/api/courses"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		lastBody = rec.Body.String()
		_ = lastBody
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for metrics, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "pupitre_active_courses_total") {
		t.Error("expected pupitre_active_courses_total metric")
	}

	if !strings.Contains(body, "go_goroutines") {
		t.Error("expected go_goroutines metric")
	}
}

// ── Helpers for cross-course lock tests ─────────────────────────────────────

// courseSummariesHandler returns an HTTP handler that serves a fixed
// /internal/progress/course-summaries response, reporting the same summary
// for every course slug the request asks about.
func courseSummariesHandler(totalScore int, passedModules []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summaries := make(map[string]any)

		for slug := range strings.SplitSeq(r.URL.Query().Get("courseSlugs"), ",") {
			if slug == "" {
				continue
			}

			summaries[slug] = map[string]any{
				"totalScore":    totalScore,
				"passedModules": passedModules,
			}
		}

		json.NewEncoder(w).Encode(map[string]any{"summaries": summaries})
	}
}

// overviewProgress is the learner state a mocked
// /internal/progress/overview reports.
type overviewProgress struct {
	Enrolled      bool
	Viewed        []string
	PassedModules []string
	TotalScore    int
	ViewedCount   int
}

// overviewHandler returns an HTTP handler serving a fixed
// /internal/progress/overview response.
func overviewHandler(progress overviewProgress) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		viewed := progress.Viewed
		if viewed == nil {
			viewed = []string{}
		}

		passed := progress.PassedModules
		if passed == nil {
			passed = []string{}
		}

		json.NewEncoder(w).Encode(map[string]any{
			"enrolled":      progress.Enrolled,
			"viewed":        viewed,
			"modules":       []any{},
			"totalScore":    progress.TotalScore,
			"passedModules": passed,
			"viewedCount":   progress.ViewedCount,
		})
	}
}

// enrollmentOverrides returns the mock overrides that make every internal
// endpoint agree on whether the learner is enrolled.
func enrollmentOverrides(enrolled bool) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/internal/enrollments/check": func(w http.ResponseWriter, _ *http.Request) {
			json.NewEncoder(w).Encode(map[string]bool{"enrolled": enrolled})
		},
		"/internal/progress/overview": overviewHandler(overviewProgress{Enrolled: enrolled}),
	}
}

// newCrossCourseLockState returns a State seeded with:
//   - "linux-intro"    — no prerequisites (the prerequisite course)
//   - "kubernetes-adv" — requires linux-intro with minScore=30 AND
//     module "quiz-bases-linux"
//   - "network-basics" — requires linux-intro with minScore=50 only
//     (no specific module)
//   - "free-course"    — requires linux-intro with no conditions
//     (any progress)
func newCrossCourseLockState(t *testing.T, userSrv *httptest.Server) *State {
	t.Helper()

	seed := []*content.Course{
		{
			Slug:     "linux-intro",
			Title:    "Intro Linux",
			IsPublic: true,
			Modules: []content.Module{
				{Name: "What is Linux", Type: "text"},
				{
					Name: "Quiz Bases Linux",
					Type: "quiz",
					Questions: []content.Question{
						{ID: "q1", Type: "single", Points: 1,
							Question: "Q?",
							Answers:  []content.Answer{{ID: "a", Text: "A", Correct: true}}},
					},
					PassingScore: 1,
				},
			},
		},
		{
			Slug:     "kubernetes-adv",
			Title:    "Advanced Kubernetes",
			IsPublic: true,
			Prerequisites: []content.CoursePrerequisite{
				{Course: "linux-intro", MinScore: 30, Modules: []string{"quiz-bases-linux"}},
			},
			Modules: []content.Module{
				{Name: "K8s Networking", Type: "text"},
			},
		},
		{
			Slug:     "network-basics",
			Title:    "Network Basics",
			IsPublic: true,
			Prerequisites: []content.CoursePrerequisite{
				{Course: "linux-intro", MinScore: 50},
			},
			Modules: []content.Module{
				{Name: "OSI Model", Type: "text"},
			},
		},
		{
			Slug:     "free-course",
			Title:    "Free Course (any linux-intro progress)",
			IsPublic: true,
			Prerequisites: []content.CoursePrerequisite{
				{Course: "linux-intro"},
			},
			Modules: []content.Module{
				{Name: "Intro", Type: "text"},
			},
		},
	}

	cfg := &config.Config{
		JWTSecret:  "test-secret",
		JWTExpiryH: 24}

	return withPeers(newStateWith(cfg, seed...), userSrv.URL, "")
}

// ── Cross-course lock: ListModules ────────────────────────────────────────────

// TestCrossCourseLock_ListModules_Blocked verifies that a student who has NOT
// completed the prerequisite course receives 403 on ListModules.
func TestCrossCourseLock_ListModules_Blocked(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(nil) // course-summary returns 0 score by default
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-adv/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when prereq not met, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCrossCourseLock_ListModules_AdminBypass verifies that an admin can always
// list modules regardless of cross-course prerequisites.
func TestCrossCourseLock_ListModules_AdminBypass(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(nil)
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-adv/modules", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("admin should bypass prereq check, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCrossCourseLock_ListModules_MetScoreAndModule verifies that a
// student who has both the required score AND the required module
// passed is allowed through.
func TestCrossCourseLock_ListModules_MetScoreAndModule(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		"/internal/progress/course-summaries": courseSummariesHandler(35, []string{"quiz-bases-linux"}),
	})
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-adv/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when prereq met, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCrossCourseLock_ScoreMetButModuleMissing verifies that meeting the score
// threshold alone is not enough when specific modules are also required.
func TestCrossCourseLock_ScoreMetButModuleMissing(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		// Score is enough but the required module slug is absent.
		"/internal/progress/course-summaries": courseSummariesHandler(40, []string{}),
	})
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-adv/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when required module not passed, got %d", rec.Code)
	}
}

// TestCrossCourseLock_MinScoreOnly_Met verifies a course that only requires a
// minimum score (no specific modules) unlocks when the threshold is reached.
func TestCrossCourseLock_MinScoreOnly_Met(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		"/internal/progress/course-summaries": courseSummariesHandler(50, []string{}),
	})
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/network-basics/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when minScore met, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCrossCourseLock_MinScoreOnly_NotMet verifies that a score below the
// threshold is rejected.
func TestCrossCourseLock_MinScoreOnly_NotMet(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		"/internal/progress/course-summaries": courseSummariesHandler(20, []string{}),
	})
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/network-basics/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when score below threshold, got %d", rec.Code)
	}
}

// TestCrossCourseLock_AnyProgress_Met verifies that a course that only requires
// "any progress" in the prereq is unlocked when totalScore > 0.
func TestCrossCourseLock_AnyProgress_Met(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		"/internal/progress/course-summaries": courseSummariesHandler(1, []string{}),
	})
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/free-course/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when any progress exists, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCrossCourseLock_AnyProgress_NotMet verifies that zero progress on the
// prereq course blocks access even when no specific score/module is required.
func TestCrossCourseLock_AnyProgress_NotMet(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(nil) // returns totalScore=0 by default
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/free-course/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when no progress in prereq, got %d", rec.Code)
	}
}

// ── Cross-course lock: GetModule ──────────────────────────────────────────────

// TestCrossCourseLock_GetModule_Blocked verifies that GetModule also enforces
// cross-course prerequisites (not just ListModules).
func TestCrossCourseLock_GetModule_Blocked(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(nil)
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-adv/modules/0", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on GetModule when prereq not met, got %d", rec.Code)
	}
}

// TestCrossCourseLock_GetModule_Met verifies that GetModule succeeds once the
// cross-course prerequisite is satisfied.
func TestCrossCourseLock_GetModule_Met(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		"/internal/progress/course-summaries": courseSummariesHandler(35, []string{"quiz-bases-linux"}),
	})
	defer mock.Close()

	s := newCrossCourseLockState(t, mock)

	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-adv/modules/0", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GetModule when prereq met, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── ListAdminCourses ──────────────────────────────────────────────────────────

// TestListAdminCourses verifies list admin courses behavior.
func TestListAdminCourses(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/admin/courses", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Courses []CourseResponse `json:"courses"`
		Total   int              `json:"total"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	// All 3 courses including private docker-fundamentals
	if resp.Total != 3 {
		t.Errorf("ListAdminCourses: expected 3 courses, got %d", resp.Total)
	}
}

// TestListAdminCourses_Unauthorized verifies list admin courses in the
// unauthorized scenario.
func TestListAdminCourses_Unauthorized(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/admin/courses", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

// ── Labs ──────────────────────────────────────────────────────────────────────

// TestListLabs verifies list labs behavior.
func TestListLabs(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/labs", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Labs []labResponse `json:"labs"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Labs) != 3 {
		t.Errorf("expected 3 labs, got %d", len(resp.Labs))
	}

	if resp.Labs[0].CourseID != "kubernetes-basics" {
		t.Errorf("expected CourseID=kubernetes-basics, got %q", resp.Labs[0].CourseID)
	}
}

// TestListLabs_CourseNotFound verifies list labs in the course not found
// scenario.
func TestListLabs_CourseNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/no-such-course/labs", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestGetLab_Found verifies get lab in the found scenario.
func TestGetLab_Found(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	// "Architecture" module → slug "architecture"
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/labs/architecture", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lab labResponse `json:"lab"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Lab.Title != "Architecture" {
		t.Errorf("expected title=Architecture, got %q", resp.Lab.Title)
	}

	if resp.Lab.LabType != "interactive" {
		t.Errorf("expected labType=interactive for video, got %q", resp.Lab.LabType)
	}
}

// TestGetLab_TextType verifies get lab in the text type scenario.
func TestGetLab_TextType(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	// "What is K8s" text module → slug "what-is-k8s"
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/labs/what-is-k8s", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for text lab, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lab labResponse `json:"lab"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Lab.LabType != "form" {
		t.Errorf("expected labType=form for text module, got %q", resp.Lab.LabType)
	}
}

// TestGetLab_NotFound verifies get lab in the not found scenario.
func TestGetLab_NotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/labs/no-such-lab", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestGetLab_CourseNotFound verifies get lab in the course not found
// scenario.
func TestGetLab_CourseNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/no-such-course/labs/architecture", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ── GetCourseProgress ─────────────────────────────────────────────────────────

// TestGetCourseProgress verifies get course progress behavior.
func TestGetCourseProgress(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/progress", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["courseId"] != "kubernetes-basics" {
		t.Errorf("expected courseId=kubernetes-basics, got %v", resp["courseId"])
	}

	if resp["total_labs"] == nil {
		t.Error("expected total_labs in response")
	}
}

// TestGetCourseProgress_UnknownCourse verifies get course progress in the
// unknown course scenario.
func TestGetCourseProgress_UnknownCourse(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/unknown-course/progress", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (placeholder), got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	// total_labs = 0 for unknown course
	if resp["total_labs"] != float64(0) {
		t.Errorf("expected total_labs=0 for unknown course, got %v", resp["total_labs"])
	}
}

// ── Cache endpoints ───────────────────────────────────────────────────────────

// TestClearCache verifies clear cache behavior.
func TestClearCache(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ClearCache: expected 200, got %d", rec.Code)
	}
}

// TestClearCourseCache_Found verifies clear course cache in the found
// scenario.
func TestClearCourseCache_Found(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/kubernetes-basics/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ClearCourseCache: expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", resp["status"])
	}
}

// TestClearCourseCache_NotFound verifies clear course cache in the not found
// scenario.
func TestClearCourseCache_NotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/nonexistent/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ClearCourseCache: expected 404 for missing course, got %d", rec.Code)
	}
}

// TestClearModuleCache_Found verifies clear module cache in the found
// scenario.
func TestClearModuleCache_Found(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/kubernetes-basics/modules/0/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ClearModuleCache: expected 200, got %d", rec.Code)
	}
}

// TestClearModuleCache_CourseNotFound verifies clear module cache in the
// course not found scenario.
func TestClearModuleCache_CourseNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/nosuchcourse/modules/0/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ClearModuleCache: expected 404, got %d", rec.Code)
	}
}

// TestClearModuleCache_InvalidIndex verifies clear module cache in the
// invalid index scenario.
func TestClearModuleCache_InvalidIndex(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/kubernetes-basics/modules/99/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ClearModuleCache: expected 404 for invalid index, got %d", rec.Code)
	}
}

// ── ServeUpload ───────────────────────────────────────────────────────────────

// TestServeUpload_InvalidFilename verifies serve upload in the invalid
// filename scenario.
func TestServeUpload_InvalidFilename(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/uploads/../../etc/passwd", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Chi router sanitizes path traversal, resulting in /uploads/etc/passwd
	// which doesn't exist → 404, OR the handler catches ".." → 400
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Fatalf("ServeUpload traversal: expected 400 or 404, got %d", rec.Code)
	}
}

// TestServeUpload_ValidFile verifies serve upload in the valid file
// scenario.
func TestServeUpload_ValidFile(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/uploads/sample.txt", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// sample.txt exists in testdata, so we expect 200
	if rec.Code != http.StatusOK {
		t.Fatalf("ServeUpload valid file: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── GetLesson ─────────────────────────────────────────────────────────────────

// TestGetLesson_Found verifies get lesson in the found scenario.
func TestGetLesson_Found(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	// "Architecture" video module
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/lessons/architecture", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetLesson: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lesson lessonDetail `json:"lesson"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Lesson.Title != "Architecture" {
		t.Errorf("GetLesson: expected title=Architecture, got %q", resp.Lesson.Title)
	}
}

// TestGetLesson_NotFound verifies get lesson in the not found scenario.
func TestGetLesson_NotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/lessons/no-such-lesson", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GetLesson: expected 404, got %d", rec.Code)
	}
}

// TestGetLesson_CourseNotFound verifies get lesson in the course not found
// scenario.
func TestGetLesson_CourseNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/no-course/lessons/architecture", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GetLesson course not found: expected 404, got %d", rec.Code)
	}
}

// TestGetLesson_NonPublicEnrolled verifies get lesson in the non public
// enrolled scenario.
func TestGetLesson_NonPublicEnrolled(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(enrollmentOverrides(true))
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	// docker-fundamentals is non-public, no modules → lesson not found
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals/lessons/architecture", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GetLesson non-public enrolled: expected 404, got %d", rec.Code)
	}
}

// TestGetLesson_NonPublicNotEnrolled verifies get lesson in the non public
// not enrolled scenario.
func TestGetLesson_NonPublicNotEnrolled(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(enrollmentOverrides(false))
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals/lessons/architecture", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("GetLesson non-public unenrolled: expected 403, got %d", rec.Code)
	}
}

// ── ListLessons for non-public course ─────────────────────────────────────────

// TestListLessons_NonPublicEnrolled verifies list lessons in the non public
// enrolled scenario.
func TestListLessons_NonPublicEnrolled(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(enrollmentOverrides(true))
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals/lessons", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListLessons enrolled: expected 200, got %d", rec.Code)
	}
}

// TestListLessons_NonPublicNotEnrolled verifies list lessons in the non
// public not enrolled scenario.
func TestListLessons_NonPublicNotEnrolled(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(enrollmentOverrides(false))
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals/lessons", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("ListLessons not enrolled: expected 403, got %d", rec.Code)
	}
}

// ── SubmitModule ──────────────────────────────────────────────────────────────

// TestSubmitModule_NotAQuiz verifies submit module in the not a quiz
// scenario.
func TestSubmitModule_NotAQuiz(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	// Module 0 is text (not quiz)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/kubernetes-basics/modules/0/submit", strings.NewReader(`{}`))
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("SubmitModule non-quiz: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSubmitModule_CourseNotFound verifies submit module in the course not
// found scenario.
func TestSubmitModule_CourseNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/no-such/modules/0/submit", strings.NewReader(`{}`))
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("SubmitModule course not found: expected 404, got %d", rec.Code)
	}
}

// TestSubmitModule_InvalidIndex verifies submit module in the invalid index
// scenario.
func TestSubmitModule_InvalidIndex(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/kubernetes-basics/modules/99/submit", strings.NewReader(`{}`))
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("SubmitModule invalid index: expected 404, got %d", rec.Code)
	}
}

// ── Utility function unit tests ───────────────────────────────────────────────

// ── Additional GetModule paths ────────────────────────────────────────────────

// TestGetModule_CourseNotFound verifies get module in the course not found
// scenario.
func TestGetModule_CourseNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/nonexistent/modules/0", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// TestGetModule_PrivateNotEnrolled verifies get module in the private not
// enrolled scenario.
func TestGetModule_PrivateNotEnrolled(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(enrollmentOverrides(false))
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals/modules/0", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for private not enrolled, got %d", rec.Code)
	}
}

// TestGetModule_ImageType verifies get module in the image type scenario.
func TestGetModule_ImageType(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	// Module index 2 is image type in kubernetes-basics
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/modules/2", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp moduleResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Type != "image" {
		t.Errorf("expected type=image, got %q", resp.Type)
	}
}

// ── Additional ListModules paths ──────────────────────────────────────────────

// TestListModules_PrivateNotEnrolled verifies list modules in the private
// not enrolled scenario.
func TestListModules_PrivateNotEnrolled(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(enrollmentOverrides(false))
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// ── Additional MarkLessonComplete paths ──────────────────────────────────────

// TestMarkLessonComplete_PrivateNotEnrolled verifies mark lesson complete in
// the private not enrolled scenario.
func TestMarkLessonComplete_PrivateNotEnrolled(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(enrollmentOverrides(false))
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/docker-fundamentals/lessons/intro/complete", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// TestMarkLessonComplete_LessonNotFound verifies mark lesson complete in the
// lesson not found scenario.
func TestMarkLessonComplete_LessonNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/kubernetes-basics/lessons/nonexistent/complete", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// TestMarkLessonComplete_UserServiceError verifies mark lesson complete in
// the user service error scenario.
func TestMarkLessonComplete_UserServiceError(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		"/internal/progress/complete": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/kubernetes-basics/lessons/what-is-k8s/complete", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 from user-service error, got %d", rec.Code)
	}
}

// ── ListLessons over a course's modules ──────────────────────────────────────

// TestListLessons_FromModules verifies that a course's lessons are derived
// from its modules, in display order.
func TestListLessons_FromModules(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	seed := []*content.Course{
		{
			Slug:     "lesson-course",
			Title:    "Lesson Course",
			IsPublic: true,
			Modules: []content.Module{
				{Name: "Introduction", Type: "text"},
				{Name: "Advanced", Type: "text"},
			},
		},
	}

	cfg := &config.Config{
		JWTSecret:  "test-secret",
		JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, seed...), mock.URL, "")
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/lesson-course/lessons", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lessons []lessonSummary `json:"lessons"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Lessons) != 2 {
		t.Errorf("expected 2 lessons, got %d", len(resp.Lessons))
	}
}

// ── GetLesson additional paths ────────────────────────────────────────────────

// TestGetLesson_QuizModule verifies get lesson in the quiz module scenario.
func TestGetLesson_QuizModule(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	// Module index 0 of quiz-course is a quiz type - GetLesson should redirect it
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/quiz-course/lessons/inline-quiz", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for quiz module via lessons, got %d", rec.Code)
	}
}

// TestGetLesson_FromModule verifies that a module-backed lesson is served
// by its derived slug.
func TestGetLesson_FromModule(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	seed := []*content.Course{
		{
			Slug:     "lesson-course2",
			Title:    "Lesson Course 2",
			IsPublic: true,
			Modules: []content.Module{
				{Name: "Intro", Type: "text", InlineContent: "## Intro\n\nHello"},
			},
		},
	}

	cfg := &config.Config{
		JWTSecret:  "test-secret",
		JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, seed...), mock.URL, "")
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/lesson-course2/lessons/intro", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── NewState with credential path ─────────────────────────────────────────────

// TestNewState_InvalidCredentialsPath verifies new state in the invalid
// credentials path scenario.
func TestNewState_InvalidCredentialsPath(t *testing.T) {
	t.Parallel()

	seed := []*content.Course{}
	cfg := &config.Config{
		JWTSecret:          "test-secret",
		GitCredentialsPath: "/nonexistent/git-creds.json"}
	// Should not panic even when credentials file doesn't exist
	s := newStateWith(cfg, seed...)
	if s == nil {
		t.Error("expected non-nil State")
	}
}

// ── GetModule with course prerequisites not met ───────────────────────────────

// TestGetModule_PrerequisitesNotMet verifies get module in the prerequisites
// not met scenario.
func TestGetModule_PrerequisitesNotMet(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMockWith(map[string]http.HandlerFunc{
		"/internal/progress/course-summary": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"totalScore":    0,
				"viewedCount":   0,
				"passedModules": []string{},
			})
		},
	})
	defer mock.Close()

	seed := []*content.Course{
		{
			Slug:     "prereq-course",
			Title:    "Prereq Course",
			IsPublic: true,
			Prerequisites: []content.CoursePrerequisite{
				{Course: "basic-course", MinScore: 100},
			},
			Modules: []content.Module{
				{Name: "Advanced Lesson", Type: "text"},
			},
		},
	}

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, seed...), mock.URL, "")
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/prereq-course/modules/0", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 when prerequisites not met, got %d", rec.Code)
	}
}

// TestTokenForRepo_NoCredentials verifies token for repo in the no
// credentials scenario.
func TestTokenForRepo_NoCredentials(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	// GitCreds is nil (no credentials path set) → returns config.GitToken
	tok := s.tokenForRepo("https://github.com/org/repo")
	if tok != "" {
		t.Errorf("expected empty token (GitToken not set), got %q", tok)
	}
}

// TestTokenForRepo_WithConfigToken verifies token for repo in the with
// config token scenario.
func TestTokenForRepo_WithConfigToken(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	s.Config.GitToken = "my-git-token"

	tok := s.tokenForRepo("https://github.com/org/repo")
	if tok != "my-git-token" {
		t.Errorf("expected my-git-token, got %q", tok)
	}
}

// TestDecode_ValidJSON verifies decode in the valid JSON scenario.
func TestDecode_ValidJSON(t *testing.T) {
	t.Parallel()

	var body struct{ Name string }

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"name":"test"}`))

	err := decode(req, &body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Name != "test" {
		t.Errorf("expected name=test, got %q", body.Name)
	}
}

// TestDecode_InvalidJSON verifies decode in the invalid JSON scenario.
func TestDecode_InvalidJSON(t *testing.T) {
	t.Parallel()

	var body struct{ Name string }

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader("not-json"))

	err := decode(req, &body)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestSanitizeQuestions_NonAdmin verifies sanitize questions in the non
// admin scenario.
func TestSanitizeQuestions_NonAdmin(t *testing.T) {
	t.Parallel()

	qq := []content.Question{
		{
			ID:   "q1",
			Type: "single",
			Answers: []content.Answer{
				{ID: "a1", Text: "Right answer", Correct: true},
				{ID: "a2", Text: "Wrong answer", Correct: false},
			},
		},
	}

	out := sanitizeQuestions(qq, false)
	if len(out) != 1 {
		t.Fatalf("expected 1 question, got %d", len(out))
	}

	pq, ok := out[0].(publicQuestion)
	if !ok {
		t.Fatal("expected publicQuestion type")
	}

	for _, a := range pq.Answers {
		if a.Correct {
			t.Error("non-admin should not see Correct=true answers")
		}
	}
}

// TestSanitizeQuestions_Admin verifies sanitize questions in the admin
// scenario.
func TestSanitizeQuestions_Admin(t *testing.T) {
	t.Parallel()

	qq := []content.Question{
		{
			ID:   "q1",
			Type: "single",
			Answers: []content.Answer{
				{ID: "a1", Text: "Right answer", Correct: true},
				{ID: "a2", Text: "Wrong answer", Correct: false},
			},
		},
	}
	out := sanitizeQuestions(qq, true)

	pq, ok := out[0].(publicQuestion)
	if !ok {
		t.Fatal("expected publicQuestion type")
	}

	found := false

	for _, a := range pq.Answers {
		if a.Correct {
			found = true
		}
	}

	if !found {
		t.Error("admin should see at least one Correct=true answer")
	}
}

// TestSanitizeQuestions_Empty verifies sanitize questions in the empty
// scenario.
func TestSanitizeQuestions_Empty(t *testing.T) {
	t.Parallel()

	out := sanitizeQuestions(nil, false)
	if len(out) != 0 {
		t.Errorf("expected empty slice, got %d", len(out))
	}
}

// TestCompletedSlugs_ViewedAndPassed verifies completed slugs in the viewed
// and passed scenario.
func TestCompletedSlugs_ViewedAndPassed(t *testing.T) {
	t.Parallel()

	modules := []content.Module{
		{Name: "Intro", Type: "text"},    // slug: intro
		{Name: "Quiz One", Type: "quiz"}, // slug: quiz-one
	}
	viewedMap := map[string]bool{"intro": true}
	progressMap := map[int]moduleProgressData{
		1: {Passed: true, BestScore: 80, MaxScore: 100},
	}

	result := completedSlugs(modules, viewedMap, progressMap)
	if !result["intro"] {
		t.Error("expected 'intro' in completed slugs from viewedMap")
	}

	if !result["quiz-one"] {
		t.Error("expected 'quiz-one' in completed slugs from progressMap")
	}
}

// TestCompletedSlugs_NotPassed verifies completed slugs in the not passed
// scenario.
func TestCompletedSlugs_NotPassed(t *testing.T) {
	t.Parallel()

	modules := []content.Module{
		{Name: "Quiz One", Type: "quiz"},
	}
	progressMap := map[int]moduleProgressData{
		0: {Passed: false, BestScore: 40},
	}

	result := completedSlugs(modules, map[string]bool{}, progressMap)
	if result["quiz-one"] {
		t.Error("failed quiz should not appear in completed slugs")
	}
}

// TestCompletedSlugs_Empty verifies completed slugs in the empty scenario.
func TestCompletedSlugs_Empty(t *testing.T) {
	t.Parallel()

	result := completedSlugs(nil, map[string]bool{}, nil)
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

// TestIsLocked_AllPrereqsDone verifies is locked in the all prereqs done
// scenario.
func TestIsLocked_AllPrereqsDone(t *testing.T) {
	t.Parallel()

	done := map[string]bool{"intro": true, "basics": true}
	if isLocked([]string{"intro", "basics"}, done) {
		t.Error("expected not locked when all prereqs done")
	}
}

// TestIsLocked_SomePrereqsMissing verifies is locked in the some prereqs
// missing scenario.
func TestIsLocked_SomePrereqsMissing(t *testing.T) {
	t.Parallel()

	done := map[string]bool{"intro": true}
	if !isLocked([]string{"intro", "basics"}, done) {
		t.Error("expected locked when some prereqs are missing")
	}
}

// TestIsLocked_EmptyPrereqs verifies is locked in the empty prereqs
// scenario.
func TestIsLocked_EmptyPrereqs(t *testing.T) {
	t.Parallel()

	if isLocked(nil, map[string]bool{}) {
		t.Error("expected not locked for empty prereqs")
	}
}

// ── GetModule additional paths ────────────────────────────────────────────────

// newTestStateWithQuiz builds a State exposing a quiz course with a quiz,
// text and lab module.
func newTestStateWithQuiz(t *testing.T, mock *httptest.Server) *State {
	t.Helper()

	seed := []*content.Course{
		{
			Slug:     "quiz-course",
			Title:    "Quiz Course",
			IsPublic: true,
			Modules: []content.Module{
				{
					Name: "Inline Quiz", Type: "quiz",
					PassingScore: 50,
					Questions: []content.Question{
						{
							ID: "q1", Type: "single", Points: 1,
							Answers: []content.Answer{
								{ID: "a1", Text: "Right", Correct: true},
								{ID: "a2", Text: "Wrong", Correct: false},
							},
						},
					},
				},
				{Name: "Text Module", Type: "text"}, // no git content → returns empty string
				{Name: "Lab Module", Type: "lab", Src: "https://labs.example.com/lab1"},
			},
		},
		{
			Slug:     "quiz-course-no-questions",
			Title:    "Empty Quiz",
			IsPublic: true,
			Modules: []content.Module{
				{Name: "Empty Quiz", Type: "quiz"}, // no questions, no git
			},
		},
	}

	cfg := &config.Config{
		JWTSecret:  "test-secret",
		JWTExpiryH: 24,
		UploadsDir: "./testdata"}

	return withPeers(newStateWith(cfg, seed...), mock.URL, "")
}

// TestGetModule_TextInline verifies get module in the text inline scenario.
func TestGetModule_TextInline(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/quiz-course/modules/1", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp moduleResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Type != "text" {
		t.Errorf("expected type=text, got %q", resp.Type)
	}
}

// TestGetModule_LabType verifies get module in the lab type scenario.
func TestGetModule_LabType(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/quiz-course/modules/2", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp moduleResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Type != "lab" {
		t.Errorf("expected type=lab, got %q", resp.Type)
	}

	if resp.Content != "https://labs.example.com/lab1" {
		t.Errorf("expected lab URL, got %q", resp.Content)
	}
}

// TestGetModule_QuizInline verifies get module in the quiz inline scenario.
func TestGetModule_QuizInline(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/quiz-course/modules/0", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Type          string `json:"type"`
		QuestionCount int    `json:"questionCount"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Type != "quiz" {
		t.Errorf("expected type=quiz, got %q", resp.Type)
	}
}

// ── SubmitModule additional paths ─────────────────────────────────────────────

// TestSubmitModule_InlineQuiz_Success verifies submit module in the inline
// quiz / success scenario.
func TestSubmitModule_InlineQuiz_Success(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	body := `{"answers":{"q1":{"selected_id":"a1"}}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/quiz-course/modules/0/submit", strings.NewReader(body))
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSubmitModule_InlineQuiz_InvalidJSON verifies submit module in the
// inline quiz / invalid JSON scenario.
func TestSubmitModule_InlineQuiz_InvalidJSON(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/quiz-course/modules/0/submit", strings.NewReader("not-json"))
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestSubmitModule_NoQuestions verifies submit module in the no questions
// scenario.
func TestSubmitModule_NoQuestions(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	body := `{"answers":{}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/quiz-course-no-questions/modules/0/submit", strings.NewReader(body))
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSubmitModule_InvalidBody verifies submit module in the invalid body
// scenario.
func TestSubmitModule_InvalidBody(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/courses/quiz-course/modules/0/submit", strings.NewReader("not-json"))
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON body, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── GetCourse private course paths ────────────────────────────────────────────

// TestGetCourse_PrivateNoAuth verifies get course in the private no auth
// scenario.
func TestGetCourse_PrivateNoAuth(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for private course with no auth, got %d", rec.Code)
	}
}

// TestGetCourse_PrivateEnrolled verifies get course in the private enrolled
// scenario.
func TestGetCourse_PrivateEnrolled(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/docker-fundamentals", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for private course enrolled, got %d", rec.Code)
	}
}

// TestGetCourse_WithPrerequisites verifies get course in the with
// prerequisites scenario.
func TestGetCourse_WithPrerequisites(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	seed := []*content.Course{
		{
			Slug:     "advanced-course",
			Title:    "Advanced Course",
			IsPublic: true,
			Prerequisites: []content.CoursePrerequisite{
				{Course: "basic-course", MinScore: 80, Modules: []string{"intro", "basics"}},
			},
		},
	}

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, seed...)
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/advanced-course", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp CourseResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Prerequisites) != 1 {
		t.Errorf("expected 1 prereq, got %d", len(resp.Prerequisites))
	}
}

// ── GetLab additional paths ───────────────────────────────────────────────────

// TestGetLab_VideoType verifies get lab in the video type scenario.
func TestGetLab_VideoType(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	// Module "Architecture" → slug "architecture", type "video" → labType "interactive"
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/labs/architecture", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lab struct {
			LabType string `json:"labType"`
		} `json:"lab"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Lab.LabType != "interactive" {
		t.Errorf("expected labType=interactive for video module, got %q", resp.Lab.LabType)
	}
}

// TestListLabs_Success verifies list labs in the success scenario.
func TestListLabs_Success(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/labs", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Labs []labResponse `json:"labs"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Labs) != 3 {
		t.Errorf("expected 3 labs, got %d", len(resp.Labs))
	}
}

// ── ClearCourseCache and ClearModuleCache ─────────────────────────────────────

// TestClearCourseCache_Success verifies clear course cache in the success
// scenario.
func TestClearCourseCache_Success(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/kubernetes-basics/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestClearModuleCache_Success verifies clear module cache in the success
// scenario.
func TestClearModuleCache_Success(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/kubernetes-basics/modules/0/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// TestClearModuleCache_NotFound verifies clear module cache in the not found
// scenario.
func TestClearModuleCache_NotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/kubernetes-basics/modules/99/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// ── ServeUpload ───────────────────────────────────────────────────────────────

// TestServeUpload_PathTraversal verifies serve upload in the path traversal
// scenario.
func TestServeUpload_PathTraversal(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/uploads/..%2Fetc%2Fpasswd", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d", rec.Code)
	}
}

// TestClearCourseCache_WithGitModules verifies clear course cache in the
// with git modules scenario.
func TestClearCourseCache_WithGitModules(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	seed := []*content.Course{
		{
			Slug:     "git-course",
			Title:    "Git Course",
			IsPublic: true,
			Modules: []content.Module{
				{Name: "Lesson 1", Type: "text", Src: "https://github.com/org/repo", Ref: "main", Path: "lesson1.md"},
				{Name: "Lesson 2", Type: "text", Src: "https://github.com/org/repo", Ref: "main", Path: "lesson2.md"},
			},
		},
	}

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, seed...)
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/git-course/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ReposCleared int `json:"reposCleared"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.ReposCleared != 1 {
		t.Errorf("expected 1 repo cleared (deduped), got %d", resp.ReposCleared)
	}
}

// TestClearModuleCache_WithGitModule verifies clear module cache in the with
// git module scenario.
func TestClearModuleCache_WithGitModule(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	seed := []*content.Course{
		{
			Slug:     "git-course2",
			Title:    "Git Course 2",
			IsPublic: true,
			Modules: []content.Module{
				{Name: "Lesson 1", Type: "text", Src: "https://github.com/org/repo2", Ref: "main", Path: "l1.md"},
			},
		},
	}

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, seed...)
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/admin/courses/git-course2/modules/0/cache/clear", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		ReposCleared int `json:"reposCleared"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.ReposCleared != 1 {
		t.Errorf("expected reposCleared=1, got %d", resp.ReposCleared)
	}
}

// TestGetLab_LabModuleType verifies get lab in the lab module type scenario.
func TestGetLab_LabModuleType(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	seed := []*content.Course{
		{
			Slug:     "lab-type-course",
			Title:    "Lab Type Course",
			IsPublic: true,
			Modules: []content.Module{
				{Name: "Hands-on Lab", Type: "lab", Src: "http://lab.example.com"},
			},
		},
	}

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, seed...), mock.URL, "")
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/lab-type-course/labs/hands-on-lab", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Lab struct {
			LabType string `json:"labType"`
		} `json:"lab"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Lab.LabType != "interactive" {
		t.Errorf("expected labType=interactive for lab module, got %q", resp.Lab.LabType)
	}
}

// ── visibleModules with admin ─────────────────────────────────────────────────

// TestListModules_AdminSeesHidden verifies list modules in the admin sees
// hidden scenario.
func TestListModules_AdminSeesHidden(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	seed := []*content.Course{
		{
			Slug:     "hidden-course",
			Title:    "Hidden Course",
			IsPublic: true,
			Modules: []content.Module{
				{Name: "Visible Module", Type: "text"},
				{Name: "Hidden Module", Type: "text", Hidden: true},
			},
		},
	}

	cfg := &config.Config{
		JWTSecret:  "test-secret",
		JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, seed...), mock.URL, "")
	r := BuildRouter(s, cfg, false)

	// Admin sees hidden modules
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/hidden-course/modules", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var adminResp struct {
		Modules []moduleResponse `json:"modules"`
	}
	json.NewDecoder(rec.Body).Decode(&adminResp)

	if len(adminResp.Modules) != 2 {
		t.Errorf("admin: expected 2 modules (including hidden), got %d", len(adminResp.Modules))
	}

	// Student doesn't see hidden modules
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/hidden-course/modules", http.NoBody)
	req2.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))
	r.ServeHTTP(rec2, req2)

	var studentResp struct {
		Modules []moduleResponse `json:"modules"`
	}
	json.NewDecoder(rec2.Body).Decode(&studentResp)

	if len(studentResp.Modules) != 1 {
		t.Errorf("student: expected 1 module (non-hidden), got %d", len(studentResp.Modules))
	}
}

// ── ListCourses search filter ─────────────────────────────────────────────────

// TestListCourses_SearchFilter verifies list courses in the search filter
// scenario.
func TestListCourses_SearchFilter(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses?search=kubernetes", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Total int `json:"total"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 kubernetes courses, got %d", resp.Total)
	}
}

// TestListCourses_DifficultyFilter verifies list courses in the difficulty
// filter scenario.
func TestListCourses_DifficultyFilter(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses?difficulty=advanced", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Total int `json:"total"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 1 {
		t.Errorf("expected 1 advanced course, got %d", resp.Total)
	}
}

// ── GetCourseProgress ─────────────────────────────────────────────────────────

// TestGetCourseProgress_Success verifies get course progress in the success
// scenario.
func TestGetCourseProgress_Success(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/kubernetes-basics/progress", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── tokenForRepo with matching credentials ───────────────────────────────────

// TestTokenForRepo_WithMatchingCredentials verifies token for repo in the
// with matching credentials scenario.
func TestTokenForRepo_WithMatchingCredentials(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)

	var creds *content.GitCredentialStore

	tmp, err := os.CreateTemp(t.TempDir(), "creds-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	tmp.WriteString("credentials:\n  - url: \"github.com/org/*\"\n    token: \"my-credential-token\"\n")
	tmp.Close()

	creds, err = content.LoadCredentials(tmp.Name())
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}

	s.GitCreds = creds

	tok := s.tokenForRepo("https://github.com/org/repo")
	if tok != "my-credential-token" {
		t.Errorf("expected my-credential-token, got %q", tok)
	}
}

// ── GetModule with inline quiz ───────────────────────────────────────────────

// newStateWithQuiz builds a State exposing a quiz course with both a regular
// and an inline quiz module.
func newStateWithQuiz(t *testing.T, mock *httptest.Server) *State {
	t.Helper()

	seed := []*content.Course{
		{
			Slug:     "quiz-course",
			Title:    "Quiz Course",
			IsPublic: true,
			Modules: []content.Module{
				{Name: "Intro", Type: "text"},
				{
					Name:         "Knowledge Check",
					Type:         "quiz",
					PassingScore: 50,
					Questions: []content.Question{
						{
							ID:     "q1",
							Type:   "single",
							Points: 10,
							Answers: []content.Answer{
								{ID: "a", Text: "Right", Correct: true},
								{ID: "b", Text: "Wrong", Correct: false},
							},
							Feedback: content.Feedback{Correct: "yes", Wrong: "no"},
						},
					},
				},
				{
					Name:         "Inline Check",
					Type:         "quiz",
					Inline:       true,
					PassingScore: 50,
					Questions: []content.Question{
						{
							ID:       "iq1",
							Type:     "boolean",
							Points:   5,
							Feedback: content.Feedback{Correct: "yes", Wrong: "no"},
						},
					},
				},
			},
		},
	}

	cfg := &config.Config{
		JWTSecret:  "test-secret",
		JWTExpiryH: 24}

	return withPeers(newStateWith(cfg, seed...), mock.URL, "")
}

// TestGetModule_InlineQuiz verifies get module in the inline quiz scenario.
func TestGetModule_InlineQuiz(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/quiz-course/modules/1", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp moduleResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Type != "quiz" {
		t.Errorf("expected type=quiz, got %q", resp.Type)
	}

	if len(resp.Questions) == 0 {
		t.Error("expected questions in quiz module")
	}

	if resp.InlineQuiz == nil {
		t.Error("expected inline quiz in response (next module is inline)")
	}
}

// TestGetModule_QuizModuleCount verifies get module in the quiz module count
// scenario.
func TestGetModule_QuizModuleCount(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newStateWithQuiz(t, mock)
	cfg := &config.Config{JWTSecret: "test-secret"}
	r := BuildRouter(s, cfg, false)

	// Module 0 is text with no inline next quiz (next is quiz at index 1, not inline)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/quiz-course/modules/0", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, cfg.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp moduleResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Type != "text" {
		t.Errorf("expected type=text, got %q", resp.Type)
	}
}
