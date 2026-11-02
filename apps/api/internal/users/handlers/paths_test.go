package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// ── buildCourseStatuses ───────────────────────────────────────────────────────

// TestBuildCourseStatuses_Sequential verifies standard linear progression.
func TestBuildCourseStatuses_Sequential(t *testing.T) {
	t.Parallel()

	courses := []string{"a", "b", "c", "d"}
	completed := map[string]struct{}{"a": {}, "b": {}}

	out := buildCourseStatuses(courses, completed)

	want := []string{pathStatusCompleted, pathStatusCompleted, pathStatusAvailable, pathStatusLocked}
	for i, cs := range out {
		if cs.Status != want[i] {
			t.Errorf("course[%d] %q: want %q, got %q", i, courses[i], want[i], cs.Status)
		}
	}
}

// TestBuildCourseStatuses_CrossPathCompletionUnlocksNext verifies a course
// completed in another path unlocks the immediately following course, but
// courses before the gap remain locked.
func TestBuildCourseStatuses_CrossPathCompletionUnlocksNext(t *testing.T) {
	t.Parallel()

	// Simulates security path where "secrets-management" (index 6)
	// completed via DevOps path, but "git-advanced" (index 5) not done.
	courses := []string{"linux-intro", "networking", "cyber", "net-ess", "python", "git-advanced", "secrets-management", "container-security"}
	completed := map[string]struct{}{"secrets-management": {}}

	out := buildCourseStatuses(courses, completed)

	if out[0].Status != pathStatusAvailable {
		t.Errorf("linux-intro: want available, got %q", out[0].Status)
	}

	for i := 1; i < 6; i++ {
		if out[i].Status != pathStatusLocked {
			t.Errorf("%s: want locked, got %q", courses[i], out[i].Status)
		}
	}

	if out[6].Status != pathStatusCompleted {
		t.Errorf("secrets-management: want completed, got %q", out[6].Status)
	}

	// The course immediately after the completed one is unlocked.
	if out[7].Status != pathStatusAvailable {
		t.Errorf("container-security: want available, got %q", out[7].Status)
	}
}

// TestBuildCourseStatuses_PrecedingCourseUnlocks verifies that completing the
// immediately preceding course unlocks the next one even when earlier courses
// in the path are not done (networking-basics done before cybersecurity-intro).
func TestBuildCourseStatuses_PrecedingCourseUnlocks(t *testing.T) {
	t.Parallel()

	// Simulates a user who completed networking-basics before cybersecurity-intro.
	courses := []string{"cybersecurity-intro", "networking-basics", "container-security"}
	completed := map[string]struct{}{"networking-basics": {}}

	out := buildCourseStatuses(courses, completed)

	if out[0].Status != pathStatusAvailable {
		t.Errorf("cybersecurity-intro: want available, got %q", out[0].Status)
	}

	if out[1].Status != pathStatusCompleted {
		t.Errorf("networking-basics: want completed, got %q", out[1].Status)
	}

	if out[2].Status != pathStatusAvailable {
		t.Errorf("container-security: want available, got %q", out[2].Status)
	}
}

// TestBuildCourseStatuses_AllCompleted verifies all-completed courses.
func TestBuildCourseStatuses_AllCompleted(t *testing.T) {
	t.Parallel()

	courses := []string{"a", "b", "c"}
	completed := map[string]struct{}{"a": {}, "b": {}, "c": {}}

	out := buildCourseStatuses(courses, completed)
	for i, cs := range out {
		if cs.Status != pathStatusCompleted {
			t.Errorf("course[%d]: want completed, got %q", i, cs.Status)
		}
	}
}

// TestBuildCourseStatuses_NoneCompleted verifies none-completed courses.
func TestBuildCourseStatuses_NoneCompleted(t *testing.T) {
	t.Parallel()

	courses := []string{"a", "b", "c", "shared"}
	completed := map[string]struct{}{}

	out := buildCourseStatuses(courses, completed)

	if out[0].Status != pathStatusAvailable {
		t.Errorf("a: want available, got %q", out[0].Status)
	}

	if out[1].Status != pathStatusLocked {
		t.Errorf("b: want locked, got %q", out[1].Status)
	}
}

// TestBuildCourseStatuses_SharedCourseUnlocks verifies a shared course
// completed via another path still unlocks the following course.
func TestBuildCourseStatuses_SharedCourseUnlocks(t *testing.T) {
	t.Parallel()

	courses := []string{"a", "b", "shared", "c"}
	completed := map[string]struct{}{"a": {}, "b": {}, "shared": {}}

	out := buildCourseStatuses(courses, completed)

	if out[2].Status != pathStatusCompleted {
		t.Errorf("shared: want completed, got %q", out[2].Status)
	}

	if out[3].Status != pathStatusAvailable {
		t.Errorf("c: want available, got %q", out[3].Status)
	}
}

// ── MyPaths ───────────────────────────────────────────────────────────────────

// TestMyPaths_DBError verifies paths DB error behavior.
func TestMyPaths_DBError(t *testing.T) {
	t.Parallel()

	paths := fake.NewPathEnrollmentRepository()
	paths.Err = errors.New("db down")
	repos := fake.NewRepositories()
	repos.Paths = paths
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/my/paths", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestMyPaths_Empty verifies paths empty behavior.
func TestMyPaths_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "GET", "/api/my/paths", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	paths := htSliceField(t, resp, "paths")
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

// TestMyPaths_CourseServiceError verifies paths course service error
// behavior.
func TestMyPaths_CourseServiceError(t *testing.T) {
	t.Parallel()

	// Enrollment found but course-service unreachable → fallback slug-only entry
	userID := uuid.New()
	repos := fake.NewRepositories()
	repos.Paths = fake.NewPathEnrollmentRepository(models.PathEnrollment{
		UserID: userID, PathSlug: "devops-path", EnrolledAt: time.Now(),
	})

	s := &State{
		Repos:  repos,
		Config: &config.Config{JWTSecret: htSecret, JWTExpiryH: htExpiry}, Courses: courseSourceAt("http://127.0.0.1:0"),
	}
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/my/paths", http.NoBody)
	req.Header.Set("Authorization", htAuthHeaderForSubject(t, "student", userID.String()))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp struct {
		Paths []struct {
			Slug string `json:"slug"`
		} `json:"paths"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Paths) != 1 || resp.Paths[0].Slug != "devops-path" {
		t.Errorf("expected fallback entry for devops-path, got %v", resp.Paths)
	}
}

// TestMyPaths_WithCourseService verifies my paths with course service
// behavior.
func TestMyPaths_WithCourseService(t *testing.T) {
	t.Parallel()

	// Mock course-service returns path detail
	courseSvc, _ := newCatalogServer(t, catalogFixture{Paths: map[string]PathInfo{
		"devops-path": {Slug: "devops-path", Title: "Parcours DevOps", Courses: []string{"lab1", "lab2", "lab3"}},
	}})

	userID := uuid.New()
	repos := fake.NewRepositories()
	repos.Paths = fake.NewPathEnrollmentRepository(models.PathEnrollment{
		UserID: userID, PathSlug: "devops-path", EnrolledAt: time.Now(),
	})
	repos.LessonProgress = fake.NewLessonProgressRepository(models.LessonProgress{
		UserID: userID.String(), CourseSlug: "lab1", LessonSlug: fake.LessonSlugComplete,
	})

	s := &State{
		Repos:  repos,
		Config: &config.Config{JWTSecret: htSecret, JWTExpiryH: htExpiry}, Courses: courseSourceAt(courseSvc.URL),
	}
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/my/paths", http.NoBody)
	req.Header.Set("Authorization", htAuthHeaderForSubject(t, "student", userID.String()))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Paths []struct {
			Slug    string `json:"slug"`
			Title   string `json:"title"`
			Courses []struct {
				Slug   string `json:"slug"`
				Status string `json:"status"`
			} `json:"courses"`
		} `json:"paths"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(resp.Paths))
	}

	p := resp.Paths[0]
	if p.Title != "Parcours DevOps" {
		t.Errorf("expected title=Parcours DevOps, got %q", p.Title)
	}

	if len(p.Courses) != 3 {
		t.Fatalf("expected 3 courses, got %d", len(p.Courses))
	}
	// lab1 completed, lab2 available (previous completed), lab3 locked
	if p.Courses[0].Status != "completed" {
		t.Errorf("lab1: want completed, got %q", p.Courses[0].Status)
	}

	if p.Courses[1].Status != "available" {
		t.Errorf("lab2: want available, got %q", p.Courses[1].Status)
	}

	if p.Courses[2].Status != "locked" {
		t.Errorf("lab3: want locked, got %q", p.Courses[2].Status)
	}
}

// TestMyPaths_RequiresAuth verifies my paths requires auth behavior.
func TestMyPaths_RequiresAuth(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "GET", "/api/my/paths", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// ── AdminListPathEnrollments ──────────────────────────────────────────────────

// TestAdminListPathEnrollments_DBError verifies admin list path enrollments
// DB error behavior.
func TestAdminListPathEnrollments_DBError(t *testing.T) {
	t.Parallel()

	paths := fake.NewPathEnrollmentRepository()
	paths.Err = errors.New("db down")
	repos := fake.NewRepositories()
	repos.Paths = paths
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/paths/devops-path/enrollments", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestAdminListPathEnrollments_Empty verifies admin list path enrollments
// empty behavior.
func TestAdminListPathEnrollments_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "GET", "/api/admin/paths/devops-path/enrollments", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	users := htSliceField(t, resp, "users")
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}

// TestAdminListPathEnrollments_WithUsers verifies admin list path
// enrollments with users behavior.
func TestAdminListPathEnrollments_WithUsers(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	users := fake.NewUserRepository(models.User{ID: userID, Email: "user@test.com", Role: "student"})
	paths := fake.NewPathEnrollmentRepository(models.PathEnrollment{
		UserID: userID, PathSlug: "devops-path", EnrolledAt: time.Now(),
	})
	paths.Users = users
	repos := fake.NewRepositories()
	repos.Users = users
	repos.Paths = paths
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/paths/devops-path/enrollments", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp struct {
		Users []struct {
			Email string `json:"email"`
		} `json:"users"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Users) != 1 || resp.Users[0].Email != "user@test.com" {
		t.Errorf("expected 1 user user@test.com, got %v", resp.Users)
	}
}

// TestAdminListPathEnrollments_RequiresAdmin verifies admin list path
// enrollments requires admin behavior.
func TestAdminListPathEnrollments_RequiresAdmin(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "GET", "/api/admin/paths/devops-path/enrollments", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// ── AdminEnrollUserInPath ─────────────────────────────────────────────────────

// TestAdminEnrollUserInPath_MissingUserID verifies admin enroll user in path
// missing user ID behavior.
func TestAdminEnrollUserInPath_MissingUserID(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "POST", "/api/admin/paths/devops-path/enrollments", `{}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestAdminEnrollUserInPath_DBError verifies admin enroll user in path DB
// error behavior.
func TestAdminEnrollUserInPath_DBError(t *testing.T) {
	t.Parallel()

	paths := fake.NewPathEnrollmentRepository()
	paths.Err = errors.New("db down")
	repos := fake.NewRepositories()
	repos.Paths = paths
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/admin/paths/devops-path/enrollments",
		`{"userId":"11111111-1111-1111-1111-111111111111"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestAdminEnrollUserInPath_Success verifies admin enroll user in path
// success behavior.
func TestAdminEnrollUserInPath_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "POST", "/api/admin/paths/devops-path/enrollments",
		`{"userId":"11111111-1111-1111-1111-111111111111"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminEnrollUserInPath_RequiresAdmin verifies admin enroll user in path
// requires admin behavior.
func TestAdminEnrollUserInPath_RequiresAdmin(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "POST", "/api/admin/paths/devops-path/enrollments",
		`{"userId":"11111111-1111-1111-1111-111111111111"}`, htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// ── AdminUnenrollUserFromPath ─────────────────────────────────────────────────

// TestAdminUnenrollUserFromPath_DBError verifies admin unenroll user from
// path DB error behavior.
func TestAdminUnenrollUserFromPath_DBError(t *testing.T) {
	t.Parallel()

	paths := fake.NewPathEnrollmentRepository()
	paths.Err = errors.New("db down")
	repos := fake.NewRepositories()
	repos.Paths = paths
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/paths/devops-path/enrollments/11111111-1111-1111-1111-111111111111", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestAdminUnenrollUserFromPath_Success verifies admin unenroll user from
// path success behavior.
func TestAdminUnenrollUserFromPath_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "DELETE", "/api/admin/paths/devops-path/enrollments/11111111-1111-1111-1111-111111111111", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestAdminUnenrollUserFromPath_RequiresAdmin verifies admin unenroll user
// from path requires admin behavior.
func TestAdminUnenrollUserFromPath_RequiresAdmin(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "DELETE", "/api/admin/paths/devops-path/enrollments/11111111-1111-1111-1111-111111111111", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// ── fetchPathDetail ───────────────────────────────────────────────────────────

// TestFetchPathDetail_InvalidSlug verifies fetch path detail invalid slug
// behavior.
func TestFetchPathDetail_InvalidSlug(t *testing.T) {
	t.Parallel()

	s := &State{Config: &config.Config{}, Courses: courseSourceAt("http://localhost")}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	cases := []string{"../etc/passwd", "UPPERCASE", "-leading", "with space", "a/b"}
	for _, slug := range cases {
		_, err := s.fetchPathDetail(req, slug)
		if err == nil {
			t.Errorf("expected error for slug %q, got nil", slug)
		}
	}
}

// TestFetchPathDetail_ValidSlug verifies fetch path detail valid slug
// behavior.
func TestFetchPathDetail_ValidSlug(t *testing.T) {
	t.Parallel()

	srv, _ := newCatalogServer(t, catalogFixture{Paths: map[string]PathInfo{
		"my-path": {Slug: "my-path", Title: "My Path"},
	}})

	s := &State{Config: &config.Config{}, Courses: courseSourceAt(srv.URL)}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	detail, err := s.fetchPathDetail(req, "my-path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if detail.Title != "My Path" {
		t.Errorf("want title=My Path, got %q", detail.Title)
	}
}

// ── completedCoursesCtx ───────────────────────────────────────────────────────

// TestCompletedCoursesCtx_EmptySlugs verifies completed courses ctx empty
// slugs behavior.
func TestCompletedCoursesCtx_EmptySlugs(t *testing.T) {
	t.Parallel()

	s := &State{Repos: fake.NewRepositories()}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	if result := s.completedCoursesCtx(req, "user-uuid-1", nil); result != nil {
		t.Errorf("expected nil for empty slugs, got %v", result)
	}
}

// TestCompletedCoursesCtx_DBError verifies completed courses ctx DB error
// behavior.
func TestCompletedCoursesCtx_DBError(t *testing.T) {
	t.Parallel()

	lessonProgress := fake.NewLessonProgressRepository()
	lessonProgress.Err = errors.New("db down")
	repos := fake.NewRepositories()
	repos.LessonProgress = lessonProgress
	s := &State{Repos: repos}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	if result := s.completedCoursesCtx(req, "user-uuid-1", []string{"course-a"}); result != nil {
		t.Errorf("expected nil on DB error, got %v", result)
	}
}

// TestCompletedCoursesCtx_PassedModuleIsNotCompletion verifies that passing
// one module of a course does not report the whole course as completed.
func TestCompletedCoursesCtx_PassedModuleIsNotCompletion(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.ModuleProgress = fake.NewModuleProgressRepository(models.ModuleProgress{
		UserID: "user-uuid-1", CourseSlug: "course-a", Passed: true,
	})
	s := &State{Repos: repos}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	result := s.completedCoursesCtx(req, "user-uuid-1", []string{"course-a"})
	if _, done := result["course-a"]; done {
		t.Error("want course-a incomplete: one passed module is not a finished course")
	}
}

// TestCompletedCoursesCtx_SentinelCompletes verifies that the __complete__
// sentinel — written once every module of a course is done — is what marks a
// course as completed.
func TestCompletedCoursesCtx_SentinelCompletes(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.LessonProgress = fake.NewLessonProgressRepository(models.LessonProgress{
		UserID: "user-uuid-1", CourseSlug: "course-a", LessonSlug: fake.LessonSlugComplete,
	})
	s := &State{Repos: repos}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	result := s.completedCoursesCtx(req, "user-uuid-1", []string{"course-a"})
	if _, done := result["course-a"]; !done {
		t.Errorf("want course-a completed via the sentinel, got %v", result)
	}
}

// ── AdminListPathEnrollments with path detail ─────────────────────────────────

// TestAdminListPathEnrollments_WithDetail verifies admin list path
// enrollments with detail behavior.
func TestAdminListPathEnrollments_WithDetail(t *testing.T) {
	t.Parallel()

	courseSvc, _ := newCatalogServer(t, catalogFixture{Paths: map[string]PathInfo{
		"devops-path": {Slug: "devops-path", Title: "DevOps Path", Courses: []string{"linux-intro", "docker-fundamentals"}},
	}})

	userID := uuid.New()
	users := fake.NewUserRepository(models.User{ID: userID, Email: "user@test.com", Role: "student"})
	paths := fake.NewPathEnrollmentRepository(models.PathEnrollment{
		UserID: userID, PathSlug: "devops-path", EnrolledAt: time.Now(),
	})
	paths.Users = users

	repos := fake.NewRepositories()
	repos.Users = users
	repos.Paths = paths
	repos.LessonProgress = fake.NewLessonProgressRepository(models.LessonProgress{
		UserID: userID.String(), CourseSlug: "linux-intro", LessonSlug: fake.LessonSlugComplete,
	})

	s := &State{
		Repos:  repos,
		Config: &config.Config{JWTSecret: htSecret, JWTExpiryH: htExpiry}, Courses: courseSourceAt(courseSvc.URL),
	}
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/admin/paths/devops-path/enrollments", http.NoBody)
	req.Header.Set("Authorization", htAuthHeader(t, "admin"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Users []struct {
			Email            string `json:"email"`
			TotalCourses     int    `json:"totalCourses"`
			CompletedCourses int    `json:"completedCourses"`
			Courses          []struct {
				Slug   string `json:"slug"`
				Status string `json:"status"`
			} `json:"courses"`
		} `json:"users"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(resp.Users))
	}

	u := resp.Users[0]
	if u.TotalCourses != 2 {
		t.Errorf("want totalCourses=2, got %d", u.TotalCourses)
	}

	if u.CompletedCourses != 1 {
		t.Errorf("want completedCourses=1, got %d", u.CompletedCourses)
	}

	if len(u.Courses) != 2 {
		t.Fatalf("want 2 courses, got %d", len(u.Courses))
	}

	if u.Courses[0].Status != "completed" {
		t.Errorf("linux-intro: want completed, got %q", u.Courses[0].Status)
	}

	if u.Courses[1].Status != "available" {
		t.Errorf("docker-fundamentals: want available, got %q", u.Courses[1].Status)
	}
}

// ── parsePagination ──────────────────────────────────────────────────────────

// TestParsePagination verifies parse pagination behavior.
func TestParsePagination(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		query      string
		wantLimit  *int
		wantOffset int
	}{
		{"omitted means unlimited", "", nil, 0},
		{"zero limit means unlimited", "?limit=0", nil, 0},
		{"negative limit bypasses pagination", "?limit=-1", nil, 0},
		{"negative offset ignored", "?offset=-5", nil, 0},
		{"positive limit and offset applied", "?limit=10&offset=5", new(10), 5},
		{"non-numeric limit ignored", "?limit=abc", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/my/paths"+tc.query, http.NoBody)

			gotLimit, gotOffset := parsePagination(req)
			if (gotLimit == nil) != (tc.wantLimit == nil) || (gotLimit != nil && *gotLimit != *tc.wantLimit) {
				t.Errorf("limit: got %v, want %v", gotLimit, tc.wantLimit)
			}

			if gotOffset != tc.wantOffset {
				t.Errorf("offset: got %d, want %d", gotOffset, tc.wantOffset)
			}
		})
	}
}
