package server

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	checkerconfig "github.com/jojo91550/pupitre2/apps/api/internal/checker/config"
	coursesconfig "github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	coursesrepo "github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
	usersconfig "github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	usersfake "github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	usersmodels "github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	usersrepo "github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// studentID is the student of TestRouter_Guards.
const studentID = "00000000-0000-0000-0000-000000000001"

// testJWTSecret signs the tokens of these tests.
const testJWTSecret = "router-test-secret-long-enough-for-hs256"

// inventoryRoute is one row of the route inventory of spec 103.
type inventoryRoute struct {
	method, path, access, limit string
}

// loadInventory reads testdata/routes.tsv, generated from the route table
// of docs/specs/103-backend-http-api.md (every route but the removed ones).
func loadInventory(t *testing.T) []inventoryRoute {
	t.Helper()

	file, err := os.Open("testdata/routes.tsv")
	if err != nil {
		t.Fatalf("open inventory: %v", err)
	}
	defer func() { _ = file.Close() }()

	var routes []inventoryRoute

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("malformed inventory line %q", line)
		}

		routes = append(routes, inventoryRoute{method: fields[0], path: fields[1], access: fields[2], limit: fields[3]})
	}

	return routes
}

// testRouter builds the monolith router on empty repositories: building it
// and running the guards never reaches the database.
func testRouter(t *testing.T) *chi.Mux {
	t.Helper()

	cfg := &config.Config{
		Port:    8080,
		Users:   &usersconfig.Config{JWTSecret: testJWTSecret, AuthRateLimitRequests: 1000, AuthRateLimitWindowSeconds: 60},
		Courses: &coursesconfig.Config{JWTSecret: testJWTSecret},
		Checker: &checkerconfig.Config{},
	}

	// The guards check the session of the student of TestRouter_Guards.
	usersRepos := &usersrepo.Repositories{Users: usersfake.NewUserRepository(usersmodels.User{
		ID: uuid.MustParse(studentID), Username: "student", Email: "s@example.com", Role: "student", IsActive: true,
	})}

	domains := Wire(cfg, usersRepos, &coursesrepo.Repositories{})

	return BuildRouter(domains, cfg, false)
}

// TestRouter_MatchesInventory checks that the monolith serves exactly the
// kept routes of spec 103: none missing, none extra, no internal route.
func TestRouter_MatchesInventory(t *testing.T) {
	t.Parallel()

	want := map[string]bool{}
	for _, route := range loadInventory(t) {
		want[route.method+" "+route.path] = true
	}

	got := map[string]bool{}

	err := chi.Walk(testRouter(t), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+strings.TrimSuffix(route, "/")] = true

		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}

	var missing, extra []string

	for key := range want {
		if !got[key] {
			missing = append(missing, key)
		}
	}

	for key := range got {
		if !want[key] {
			extra = append(extra, key)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 {
		t.Errorf("routes of spec 103 not served: %v", missing)
	}

	if len(extra) > 0 {
		t.Errorf("routes served but not in spec 103: %v", extra)
	}
}

// concretePath fills every {param} of a route pattern with a placeholder.
func concretePath(pattern string) string {
	parts := strings.Split(pattern, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") {
			parts[i] = "x"
		}
	}

	return strings.Join(parts, "/")
}

// TestRouter_Guards checks the guard of every route against spec 103: a
// guarded route refuses an anonymous request with 401, and a role-guarded
// route refuses a student with 403. Public routes are not requested.
func TestRouter_Guards(t *testing.T) {
	t.Parallel()

	router := testRouter(t)

	studentToken, err := middleware.CreateToken(studentID, "s@example.com", "student", "student", testJWTSecret, 1)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	for _, route := range loadInventory(t) {
		if strings.HasPrefix(route.access, "public") {
			continue
		}

		path := concretePath(route.path)

		anonymous := httptest.NewRecorder()
		router.ServeHTTP(anonymous, httptest.NewRequestWithContext(t.Context(), route.method, path, http.NoBody))

		if anonymous.Code != http.StatusUnauthorized {
			t.Errorf("%s %s (%s) anonymous: status %d, want 401", route.method, route.path, route.access, anonymous.Code)
		}

		if route.access == "signed in" {
			continue
		}

		req := httptest.NewRequestWithContext(t.Context(), route.method, path, http.NoBody)
		req.Header.Set("Authorization", "Bearer "+studentToken)

		student := httptest.NewRecorder()
		router.ServeHTTP(student, req)

		if student.Code != http.StatusForbidden {
			t.Errorf("%s %s (%s) as student: status %d, want 403", route.method, route.path, route.access, student.Code)
		}
	}
}

// TestRouter_Health checks the single health route.
func TestRouter_Health(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("health: %d %s", rec.Code, rec.Body.String())
	}
}
