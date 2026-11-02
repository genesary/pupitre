package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// allErrRepos returns repositories where every fake fails on every call.
func allErrRepos() *repository.Repositories {
	c := fake.NewCourseRepository(&content.Course{Slug: "c", Title: "c", IsPublic: true})
	c.Err = errors.New("course db down")

	p := fake.NewPathRepository(&content.Path{Slug: "p", Title: "p"})
	p.Err = errors.New("path db down")

	l := fake.NewLabCheckRepository()
	l.Err = errors.New("lab db down")

	return &repository.Repositories{
		Courses: c, Paths: p, LabChecks: l, QuizAttempts: fake.NewQuizAttemptRepository(),
	}
}

// TestHandlers_RepoErrorsAreServerErrors sweeps the public/admin read and
// list routes with every repository failing and asserts each returns an
// error status rather than a panic or a leaked 2xx.
func TestHandlers_RepoErrorsAreServerErrors(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := NewState(cfg, allErrRepos())
	r := BuildRouter(s, cfg, false)

	student := authHeader(t, "test-secret")
	admin := adminAuthHeader(t, "test-secret")

	cases := []struct {
		name, method, path, body, auth string
	}{
		{"list courses", http.MethodGet, "/api/courses", "", ""},
		{"get course", http.MethodGet, "/api/courses/c", "", ""},
		{"course progress", http.MethodGet, "/api/courses/c/progress", "", student},
		{"list paths", http.MethodGet, "/api/paths", "", ""},
		{"get path", http.MethodGet, "/api/paths/p", "", ""},
		{"list modules", http.MethodGet, "/api/courses/c/modules", "", student},
		{"admin courses", http.MethodGet, "/api/admin/courses", "", admin},
		{"admin lab-checks", http.MethodGet, "/api/admin/lab-checks", "", admin},
		{"admin lab export preview", http.MethodPost, "/api/admin/exports/lab-checks/preview", `{}`, admin},
		{"admin get definition", http.MethodGet, "/api/admin/courses/c/definition", "", admin},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var body *strings.Reader
			if tc.body == "" {
				body = strings.NewReader("")
			} else {
				body = strings.NewReader(tc.body)
			}

			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, body)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}

			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			// A DB failure must never read as success. Some routes surface it
			// as 4xx (e.g. treated as "not found"); most as 5xx. Either is
			// acceptable — a 2xx or a panic is not.
			if rec.Code >= 200 && rec.Code < 400 {
				t.Errorf("%s %s: got %d on a total DB failure (%s)",
					tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}
