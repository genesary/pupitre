package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// ── xpLevel ──────────────────────────────────────────────────────────────────

// TestXPLevel verifies xpLevel returns the correct tier for boundary values.
func TestXPLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		total int
		want  int
	}{
		{0, xpLevelNovice},
		{199, xpLevelNovice},
		{200, xpLevelApprenant},
		{599, xpLevelApprenant},
		{600, xpLevelConfirme},
		{1499, xpLevelConfirme},
		{1500, xpLevelExpert},
		{3499, xpLevelExpert},
		{3500, xpLevelMaitre},
		{9999, xpLevelMaitre},
	}

	for _, tc := range cases {
		got := xpLevel(tc.total)
		if got != tc.want {
			t.Errorf("xpLevel(%d) = %d, want %d", tc.total, got, tc.want)
		}
	}
}

// ── MyXP ─────────────────────────────────────────────────────────────────────

// TestMyXP_Empty verifies a new user gets 0 XP at level 1 (Novice).
func TestMyXP_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDo(t, r, http.MethodGet, "/api/my/xp", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var body map[string]any

	err := json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["total"] != float64(0) {
		t.Errorf("want total=0, got %v", body["total"])
	}

	if body["level"] != float64(xpLevelNovice) {
		t.Errorf("want level=%d, got %v", xpLevelNovice, body["level"])
	}

	if body["levelName"] != levelNames[xpLevelNovice] {
		t.Errorf("want levelName=%s, got %v", levelNames[xpLevelNovice], body["levelName"])
	}
}

// TestMyXP_WithEvents verifies accumulated XP and correct level are returned.
func TestMyXP_WithEvents(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	xp := fake.NewXPRepository()

	err := xp.Award(t.Context(), "user-uuid-1", repository.XPSourceCourse, "my-course", repository.XPAmountCourse)
	if err != nil {
		t.Fatalf("seed xp: %v", err)
	}

	repos.XP = xp

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/my/xp", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var body map[string]any

	err = json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["total"] != float64(repository.XPAmountCourse) {
		t.Errorf("want total=%d, got %v", repository.XPAmountCourse, body["total"])
	}

	if body["level"] != float64(xpLevelApprenant) {
		t.Errorf("want level=%d (Apprenant), got %v", xpLevelApprenant, body["level"])
	}

	history, ok := body["history"].([]any)
	if !ok || len(history) == 0 {
		t.Errorf("want non-empty history, got %v", body["history"])
	}
}

// TestMyXP_TotalError verifies that a DB error on Total returns 500.
func TestMyXP_TotalError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	xp := fake.NewXPRepository()
	xp.Err = errors.New("db down")
	repos.XP = xp

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/my/xp", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestMyXP_HistoryError verifies that a DB error on History returns 500.
func TestMyXP_HistoryError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.XP = &historyErrXP{}

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/my/xp", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── Enroll XP gate ────────────────────────────────────────────────────────────

// newXPGateRouter builds a router pointed at a course-service stub that
// returns xpRequired for the requested course.
func newXPGateRouter(t *testing.T, repos *repository.Repositories, xpRequired int) http.Handler {
	t.Helper()

	srv, _ := newCatalogServer(t, catalogFixture{Courses: map[string]CourseInfo{
		"gated-course": {Slug: "gated-course", ID: "gated-course", XPRequired: xpRequired},
	}})

	cfg := &config.Config{
		JWTSecret:   htSecret,
		JWTExpiryH:  htExpiry,
		CORSOrigins: []string{"*"},
	}
	s := &State{Repos: repos, Config: cfg, Courses: courseSourceAt(srv.URL)}

	return BuildRouter(s, cfg, false)
}

// newDeadCourseRouter builds a router whose course-service URL is unreachable.
func newDeadCourseRouter(t *testing.T, repos *repository.Repositories) http.Handler {
	t.Helper()

	cfg := &config.Config{
		JWTSecret:   htSecret,
		JWTExpiryH:  htExpiry,
		CORSOrigins: []string{"*"},
	}
	s := &State{Repos: repos, Config: cfg, Courses: courseSourceAt("http://127.0.0.1:0")}

	return BuildRouter(s, cfg, false)
}

// TestEnroll_XPGate_Pass verifies enrollment succeeds when
// the user has enough XP.
func TestEnroll_XPGate_Pass(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	xp := fake.NewXPRepository()

	err := xp.Award(t.Context(), "user-uuid-1", repository.XPSourceCourse, "intro", repository.XPAmountCourse)
	if err != nil {
		t.Fatalf("seed xp: %v", err)
	}

	repos.XP = xp

	r := newXPGateRouter(t, repos, repository.XPAmountCourse)
	rec := htDo(t, r, http.MethodPost, "/api/enrollments/gated-course", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestEnroll_XPGate_Fail verifies enrollment is blocked when
// XP is insufficient.
func TestEnroll_XPGate_Fail(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newXPGateRouter(t, repos, repository.XPAmountCourse)
	rec := htDo(t, r, http.MethodPost, "/api/enrollments/gated-course", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestEnroll_XPGate_NoRequirement verifies that enrollment with
// xpRequired=0 succeeds even without any XP.
func TestEnroll_XPGate_NoRequirement(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newXPGateRouter(t, repos, 0)
	rec := htDo(t, r, http.MethodPost, "/api/enrollments/gated-course", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestEnroll_XPGate_CourseServiceDown verifies that a dead
// course-service still allows enrollment — graceful degradation.
func TestEnroll_XPGate_CourseServiceDown(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newDeadCourseRouter(t, repos)
	rec := htDo(t, r, http.MethodPost, "/api/enrollments/any-course", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 (soft-fail), got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── stubs ─────────────────────────────────────────────────────────────────────

// historyErrXP is a minimal XPRepository whose History always fails.
type historyErrXP struct{}

// Award implements XPRepository; always returns nil.
func (f *historyErrXP) Award(_ context.Context, _, _, _ string, _ int) error { return nil }

// Total implements XPRepository; always returns 0.
func (f *historyErrXP) Total(_ context.Context, _ string) (int, error) { return 0, nil }

// History implements XPRepository; always returns an error.
func (f *historyErrXP) History(_ context.Context, _ string) ([]repository.XPEventRow, error) {
	return nil, errors.New("db down")
}
