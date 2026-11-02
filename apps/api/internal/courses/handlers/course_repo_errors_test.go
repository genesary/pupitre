package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// errCourseState returns a State whose CourseRepository fails every call.
func errCourseState() *State {
	courses := fake.NewCourseRepository()
	courses.Err = errors.New("course db down")

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}

	return NewState(cfg, &repository.Repositories{
		Courses:      courses,
		Paths:        fake.NewPathRepository(),
		QuizAttempts: fake.NewQuizAttemptRepository(),
		LabChecks:    fake.NewLabCheckRepository(),
	})
}

// TestCourseHandlers_DBErrorsAre5xx sweeps the read/list/detail routes with a
// failing course repository and asserts each returns a server error rather
// than panicking or leaking a 200.
func TestCourseHandlers_DBErrorsAre5xx(t *testing.T) {
	t.Parallel()

	s := errCourseState()
	r := BuildRouter(s, s.Config, false)

	studentAuth := authHeader(t, "test-secret")
	adminAuth := adminAuthHeader(t, "test-secret")

	cases := []struct {
		name, method, path, body, auth string
	}{
		{"list courses", http.MethodGet, "/api/courses", "", ""},
		{"get course", http.MethodGet, "/api/courses/some-course", "", ""},
		{"list modules", http.MethodGet, "/api/courses/some-course/modules", "", studentAuth},
		{"get module", http.MethodGet, "/api/courses/some-course/modules/0", "", studentAuth},
		{"submit module", http.MethodPost, "/api/courses/some-course/modules/0/submit", `{"answers":{}}`, studentAuth},
		{"admin list courses", http.MethodGet, "/api/admin/courses", "", adminAuth},
		{"admin create session", http.MethodPost, "/api/admin/courses/some-course/sessions",
			`{"title":"t","date":"2026-01-01T00:00:00Z"}`, adminAuth},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reader *strings.Reader
			if tc.body == "" {
				reader = strings.NewReader("")
			} else {
				reader = strings.NewReader(tc.body)
			}

			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, reader)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}

			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code < 500 || rec.Code > 599 {
				t.Errorf("%s %s: want 5xx on a DB failure, got %d (%s)",
					tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}
