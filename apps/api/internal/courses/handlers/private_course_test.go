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

// TestGetCourse_PrivateViaPathEnrollment lets a learner who is not directly
// enrolled view a private course because a learning path they are enrolled
// in contains it — exercising isEnrolledViaPath and canViewPrivateCourse.
func TestGetCourse_PrivateViaPathEnrollment(t *testing.T) {
	t.Parallel()

	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/internal/enrollments/check":
			_, _ = w.Write([]byte(`{"enrolled":false}`)) // not directly enrolled
		case "/internal/paths/check":
			_, _ = w.Write([]byte(`{"enrolled":true}`)) // but enrolled in a containing path
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer userSrv.Close()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	state := withPeers(NewState(cfg, &repository.Repositories{
		Courses: fake.NewCourseRepository(&content.Course{
			Slug: "secret-course", Title: "Secret", IsPublic: false,
			Modules: []content.Module{{Name: "m", Type: "text", InlineContent: "x"}},
		}),
		Paths: fake.NewPathRepository(&content.Path{
			Slug: "hidden-path", Title: "Hidden Path", Courses: []string{"secret-course"},
		}),
		QuizAttempts: fake.NewQuizAttemptRepository(),
		LabChecks:    fake.NewLabCheckRepository(),
	}), userSrv.URL, "")

	r := BuildRouter(state, cfg, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/secret-course", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 (viewable via path), got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["slug"] != "secret-course" {
		t.Errorf("unexpected course payload: %v", resp)
	}
}

// TestGetCourse_PrivateNoPathHidden keeps a private course hidden from a
// learner with neither a direct nor a path enrollment.
func TestGetCourse_PrivateNoPathHidden(t *testing.T) {
	t.Parallel()

	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"enrolled":false}`))
	}))
	defer userSrv.Close()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	state := withPeers(NewState(cfg, &repository.Repositories{
		Courses: fake.NewCourseRepository(&content.Course{
			Slug: "secret-course", Title: "Secret", IsPublic: false,
		}),
		Paths:        fake.NewPathRepository(), // no path contains it
		QuizAttempts: fake.NewQuizAttemptRepository(),
		LabChecks:    fake.NewLabCheckRepository(),
	}), userSrv.URL, "")

	r := BuildRouter(state, cfg, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/secret-course", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Errorf("want 404/403 for a hidden private course, got %d", rec.Code)
	}
}
