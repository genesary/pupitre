package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// newPathTestState builds a State pre-populated with two fixture learning
// paths, for use by the path-listing and path-detail handler tests.
func newPathTestState(t *testing.T) *State {
	t.Helper()

	paths := fake.NewPathRepository(
		&content.Path{
			Slug:        "devops-path",
			Title:       "DevOps Path",
			Description: "From Linux to Kubernetes",
			Courses:     []string{"linux-intro", "docker-fundamentals", "kubernetes-basics"},
		},
		&content.Path{
			Slug:    "python-path",
			Title:   "Python Path",
			Courses: []string{"python-basics", "python-advanced"},
		},
	)

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}

	return NewState(cfg, &repository.Repositories{
		Courses:      fake.NewCourseRepository(),
		Paths:        paths,
		QuizAttempts: fake.NewQuizAttemptRepository(),
		LabChecks:    fake.NewLabCheckRepository(),
	})
}

// TestListPaths verifies the paths list endpoint returns all known paths.
func TestListPaths(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newPathTestState(t)
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/paths", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any

	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	paths, ok := resp["paths"].([]any)
	if !ok {
		t.Fatal("expected paths array")
	}

	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}

// TestGetPath verifies the path-detail endpoint returns the requested path.
func TestGetPath(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newPathTestState(t)
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/paths/devops-path", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var p content.Path

	err := json.NewDecoder(w.Body).Decode(&p)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if p.Slug != "devops-path" {
		t.Errorf("expected slug=devops-path, got %q", p.Slug)
	}

	if len(p.Courses) != 3 {
		t.Errorf("expected 3 courses, got %d", len(p.Courses))
	}
}

// TestGetPathNotFound verifies an unknown path slug yields a 404 response.
func TestGetPathNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newPathTestState(t)
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/paths/nonexistent", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// TestListPathsUnordered verifies ListPaths returns every known path without
// imposing a server-side sort order (sorting was moved to the frontend, since
// sorting server-side on every request does not scale with the path count).
func TestListPathsUnordered(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newPathTestState(t)
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/paths", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]any

	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	paths, ok := resp["paths"].([]any)
	if !ok {
		t.Fatal("expected paths array")
	}

	titles := make(map[string]bool, len(paths))
	for _, p := range paths {
		pm, ok := p.(map[string]any)
		if !ok {
			t.Fatal("expected path object")
		}

		title, ok := pm["title"].(string)
		if !ok {
			t.Fatal("expected string title")
		}

		titles[title] = true
	}

	for _, want := range []string{"DevOps Path", "Python Path"} {
		if !titles[want] {
			t.Errorf("expected path titled %q in response, got %v", want, titles)
		}
	}
}

// TestListPathsPagination verifies the limit/offset query params bound and
// slice the result set.
func TestListPathsPagination(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newPathTestState(t)
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/paths?limit=1", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]any

	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	paths, ok := resp["paths"].([]any)
	if !ok {
		t.Fatal("expected paths array")
	}

	if len(paths) != 1 {
		t.Fatalf("expected 1 path with limit=1, got %d", len(paths))
	}

	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/paths?offset=2", http.NoBody)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp = nil

	err = json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	paths, ok = resp["paths"].([]any)
	if !ok {
		t.Fatal("expected paths array")
	}

	if len(paths) != 0 {
		t.Errorf("expected 0 paths with offset=2 (only 2 paths exist), got %d", len(paths))
	}
}

// TestListPathsPaginationNegativeLimitBypasses verifies a negative limit is
// treated the same as an omitted one — an explicit escape hatch for callers
// that always send a numeric limit but sometimes want everything.
func TestListPathsPaginationNegativeLimitBypasses(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newPathTestState(t)
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/paths?limit=-1", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]any

	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	paths, ok := resp["paths"].([]any)
	if !ok {
		t.Fatal("expected paths array")
	}

	if len(paths) != 2 {
		t.Errorf("expected negative limit to bypass pagination and return all 2 paths, got %d", len(paths))
	}
}
