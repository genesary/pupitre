package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// TestGetLabResults_Filter verifies GetLabResults filters by course slug and
// omits checks belonging to other courses.
func TestGetLabResults_Filter(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	s.Repos.LabChecks = fake.NewLabCheckRepository(
		models.LabCheck{
			Username:    "alice",
			CourseSlug:  "kubernetes-basics",
			ModuleIndex: 0,
			ModuleName:  "What is K8s",
			Allow:       true,
			Violations:  []string{},
			CheckedAt:   time.Now(),
			Verified:    true,
		},
		models.LabCheck{
			Username:    "bob",
			CourseSlug:  "docker-fundamentals",
			ModuleIndex: 0,
			ModuleName:  "Intro",
			Allow:       false,
			Violations:  []string{"missing Dockerfile"},
			CheckedAt:   time.Now(),
			Verified:    false,
		},
	)

	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/api/admin/lab-checks?course=kubernetes-basics", http.NoBody,
	)
	req.Header.Set("Authorization", adminAuthHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var results []labCheckRow

	err := json.NewDecoder(rec.Body).Decode(&results)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 filtered result, got %d", len(results))
	}

	if results[0].Username != "alice" || results[0].CourseSlug != "kubernetes-basics" {
		t.Errorf("unexpected result: %+v", results[0])
	}
}

// TestGetLabResults_Unfiltered verifies GetLabResults returns every recorded
// check when no course filter is supplied.
func TestGetLabResults_Unfiltered(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	s.Repos.LabChecks = fake.NewLabCheckRepository(
		models.LabCheck{Username: "alice", CourseSlug: "kubernetes-basics", CheckedAt: time.Now()},
		models.LabCheck{Username: "bob", CourseSlug: "docker-fundamentals", CheckedAt: time.Now()},
	)

	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/admin/lab-checks", http.NoBody)
	req.Header.Set("Authorization", adminAuthHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var results []labCheckRow

	err := json.NewDecoder(rec.Body).Decode(&results)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}
