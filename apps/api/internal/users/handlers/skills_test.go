package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// ── skillIsCompleted ──────────────────────────────────────────────────────────

// TestSkillIsCompleted_AllQuizzesPassed verifies the skill is completed
// when every quiz module has a matching passed key.
func TestSkillIsCompleted_AllQuizzesPassed(t *testing.T) {
	t.Parallel()

	modules := []SkillModule{
		{Slug: "quiz-1", Type: skillModuleTypeQuiz, CourseSlug: "golang"},
		{Slug: "quiz-2", Type: skillModuleTypeQuiz, CourseSlug: "golang"},
	}
	passed := map[string]struct{}{"golang/quiz-1": {}, "golang/quiz-2": {}}

	if !skillIsCompleted(modules, passed, nil) {
		t.Error("expected skill completed when all quizzes passed")
	}
}

// TestSkillIsCompleted_QuizNotPassed verifies the skill is incomplete when
// a quiz module has not been passed.
func TestSkillIsCompleted_QuizNotPassed(t *testing.T) {
	t.Parallel()

	modules := []SkillModule{
		{Slug: "quiz-1", Type: skillModuleTypeQuiz, CourseSlug: "golang"},
		{Slug: "quiz-2", Type: skillModuleTypeQuiz, CourseSlug: "golang"},
	}
	passed := map[string]struct{}{"golang/quiz-1": {}}

	if skillIsCompleted(modules, passed, nil) {
		t.Error("expected skill incomplete when a quiz is not passed")
	}
}

// TestSkillIsCompleted_LabViewed verifies a lab module is satisfied when
// the user has a viewed key for it.
func TestSkillIsCompleted_LabViewed(t *testing.T) {
	t.Parallel()

	modules := []SkillModule{
		{Slug: "lab-1", Type: skillModuleTypeLab, CourseSlug: "docker"},
	}
	viewed := map[string]struct{}{"docker/lab-1": {}}

	if !skillIsCompleted(modules, nil, viewed) {
		t.Error("expected skill completed when lab is viewed")
	}
}

// TestSkillIsCompleted_NoAssessableModules verifies a skill with only text
// modules is never considered completed.
func TestSkillIsCompleted_NoAssessableModules(t *testing.T) {
	t.Parallel()

	modules := []SkillModule{
		{Slug: "intro", Type: "text", CourseSlug: "golang"},
	}

	if skillIsCompleted(modules, nil, nil) {
		t.Error("expected skill not completed when no assessable modules")
	}
}

// TestSkillIsCompleted_EmptyModules verifies empty module list returns false.
func TestSkillIsCompleted_EmptyModules(t *testing.T) {
	t.Parallel()

	if skillIsCompleted(nil, nil, nil) {
		t.Error("expected false for empty module list")
	}
}

// ── InternalMarkCourseComplete ────────────────────────────────────────────────

// TestInternalMarkCourseComplete_NoSkills verifies a successful course
// completion without skills returns 200.
func TestInternalMarkCourseComplete_NoSkills(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newTestRouterWithRepos(repos)

	body := `{"userId":"11111111-1111-1111-1111-111111111111","courseSlug":"golang-intro"}`
	rec := htDoInternal(t, r, http.MethodPost, "/internal/progress/course-complete", body)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestInternalMarkCourseComplete_WithSkills verifies skill levels are
// upserted when the body includes skills and a difficulty.
func TestInternalMarkCourseComplete_WithSkills(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newTestRouterWithRepos(repos)

	body := `{
		"userId":"11111111-1111-1111-1111-111111111111",
		"courseSlug":"golang-intro",
		"skills":{"golang":{},"ci-cd":{}},
		"difficulty":"beginner",
		"skillTotalCourses":{"golang":3,"ci-cd":2}
	}`
	rec := htDoInternal(t, r, http.MethodPost, "/internal/progress/course-complete", body)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	levels, err := repos.SkillLevels.ListByUser(t.Context(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}

	if len(levels) != 2 {
		t.Errorf("want 2 skill levels, got %d", len(levels))
	}
}

// TestInternalMarkCourseComplete_DBError verifies a DB failure returns 500.
func TestInternalMarkCourseComplete_DBError(t *testing.T) {
	t.Parallel()

	lp := fake.NewLessonProgressRepository()
	lp.Err = errors.New("db down")

	repos := fake.NewRepositories()
	repos.LessonProgress = lp
	r := newTestRouterWithRepos(repos)

	body := `{"userId":"11111111-1111-1111-1111-111111111111","courseSlug":"golang-intro"}`
	rec := htDoInternal(t, r, http.MethodPost, "/internal/progress/course-complete", body)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestInternalMarkCourseComplete_SkillUpsertError verifies a skill upsert
// failure is logged but does not prevent a 200 response.
func TestInternalMarkCourseComplete_SkillUpsertError(t *testing.T) {
	t.Parallel()

	sl := fake.NewSkillLevelRepository()
	sl.Err = errors.New("upsert failed")

	repos := fake.NewRepositories()
	repos.SkillLevels = sl
	r := newTestRouterWithRepos(repos)

	body := `{
		"userId":"11111111-1111-1111-1111-111111111111",
		"courseSlug":"golang-intro",
		"skills":{"golang":{}},
		"difficulty":"beginner",
		"skillTotalCourses":{"golang":1}
	}`
	rec := htDoInternal(t, r, http.MethodPost, "/internal/progress/course-complete", body)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 despite upsert error, got %d", rec.Code)
	}
}

// TestInternalMarkCourseComplete_InvalidBody verifies malformed JSON
// returns 400.
func TestInternalMarkCourseComplete_InvalidBody(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDoInternal(t, r, http.MethodPost, "/internal/progress/course-complete", "not-json")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// ── MySkills ──────────────────────────────────────────────────────────────────

// TestMySkills_Empty verifies an empty skill list returns an empty array.
func TestMySkills_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDo(t, r, http.MethodGet, "/api/my/skills", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	skills, ok := resp["skills"].([]any)
	if !ok || len(skills) != 0 {
		t.Errorf("want empty skills array, got %v", resp["skills"])
	}
}

// TestMySkills_WithLevels verifies seeded skill levels are returned.
func TestMySkills_WithLevels(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()

	err := repos.SkillLevels.Upsert(t.Context(), htUserID, "golang", "beginner", 3)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/my/skills", "", htAuthHeaderForSubject(t, "student", htUserID))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	skills := htSliceField(t, resp, "skills")
	if len(skills) != 1 {
		t.Errorf("want 1 skill level, got %d", len(skills))
	}
}

// TestMySkills_DBError verifies a DB failure returns 500.
func TestMySkills_DBError(t *testing.T) {
	t.Parallel()

	sl := fake.NewSkillLevelRepository()
	sl.Err = errors.New("db down")

	repos := fake.NewRepositories()
	repos.SkillLevels = sl

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/my/skills", "", htAuthHeaderForSubject(t, "student", htUserID))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── MySkillModules ────────────────────────────────────────────────────────────

// newSkillModulesRouter builds a router whose state points at courseServiceURL.
func newSkillModulesRouter(t *testing.T, repos *repository.Repositories, courseServiceURL string) http.Handler {
	t.Helper()

	cfg := &config.Config{
		JWTSecret:   htSecret,
		JWTExpiryH:  htExpiry,
		CORSOrigins: []string{"*"},
	}
	s := &State{Repos: repos, Config: cfg, Courses: courseSourceAt(courseServiceURL)}

	return BuildRouter(s, cfg, false)
}

// TestMySkillModules_InvalidSlug verifies an invalid skill slug returns 400.
func TestMySkillModules_InvalidSlug(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDo(t, r, http.MethodGet, "/api/my/skills/INVALID_SLUG!", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestMySkillModules_CourseServiceDown verifies 502 when unreachable.
func TestMySkillModules_CourseServiceDown(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newSkillModulesRouter(t, repos, "http://127.0.0.1:0")

	rec := htDo(t, r, http.MethodGet, "/api/my/skills/golang", "", htAuthHeaderForSubject(t, "student", htUserID))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("want 502, got %d", rec.Code)
	}
}

// TestMySkillModules_CourseServiceError verifies a non-200 returns 502.
func TestMySkillModules_CourseServiceError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	repos := fake.NewRepositories()
	r := newSkillModulesRouter(t, repos, srv.URL)
	rec := htDo(t, r, http.MethodGet, "/api/my/skills/golang", "", htAuthHeaderForSubject(t, "student", htUserID))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("want 502, got %d", rec.Code)
	}
}

// TestMySkillModules_LessonProgressError verifies that a LessonProgress
// DB error is logged but does not prevent a 200 response.
func TestMySkillModules_LessonProgressError(t *testing.T) {
	t.Parallel()

	srv, _ := newCatalogServer(t, catalogFixture{Skills: map[string][]SkillModule{
		"golang": {{Slug: "intro", Type: "text", CourseSlug: "golang", Index: 0}},
	}})

	lp := fake.NewLessonProgressRepository()
	lp.Err = errors.New("db down")

	repos := fake.NewRepositories()
	repos.LessonProgress = lp

	r := newSkillModulesRouter(t, repos, srv.URL)
	rec := htDo(t, r, http.MethodGet, "/api/my/skills/golang", "", htAuthHeaderForSubject(t, "student", htUserID))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 despite LessonProgress error, got %d", rec.Code)
	}
}

// TestMySkillModules_ModuleProgressError verifies that a ModuleProgress
// DB error is logged but does not prevent a 200 response.
func TestMySkillModules_ModuleProgressError(t *testing.T) {
	t.Parallel()

	srv, _ := newCatalogServer(t, catalogFixture{Skills: map[string][]SkillModule{
		"golang": {{Slug: "quiz-1", Type: "quiz", CourseSlug: "golang", Index: 0}},
	}})

	mp := fake.NewModuleProgressRepository()
	mp.Err = errors.New("db down")

	repos := fake.NewRepositories()
	repos.ModuleProgress = mp

	r := newSkillModulesRouter(t, repos, srv.URL)
	rec := htDo(t, r, http.MethodGet, "/api/my/skills/golang", "", htAuthHeaderForSubject(t, "student", htUserID))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 despite ModuleProgress error, got %d", rec.Code)
	}
}

// TestMySkillModules_HappyPath verifies modules are returned with statuses.
func TestMySkillModules_HappyPath(t *testing.T) {
	t.Parallel()

	srv, _ := newCatalogServer(t, catalogFixture{Skills: map[string][]SkillModule{
		"golang": {
			{Slug: "quiz-1", Type: "quiz", CourseSlug: "golang", CourseTitle: "Go", Index: 0},
			{Slug: "quiz-2", Type: "quiz", CourseSlug: "golang", CourseTitle: "Go", Index: 1},
		},
	}})

	modSlug := "quiz-1"
	mp := fake.NewModuleProgressRepository(models.ModuleProgress{
		UserID:      htUserID,
		CourseSlug:  "golang",
		ModuleSlug:  &modSlug,
		ModuleIndex: 0,
		Passed:      true,
	})

	repos := fake.NewRepositories()
	repos.ModuleProgress = mp

	r := newSkillModulesRouter(t, repos, srv.URL)
	rec := htDo(t, r, http.MethodGet, "/api/my/skills/golang", "", htAuthHeaderForSubject(t, "student", htUserID))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any

	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	modules := htSliceField(t, resp, "modules")
	if len(modules) != 2 {
		t.Errorf("want 2 modules, got %d", len(modules))
	}

	first, ok := modules[0].(map[string]any)
	if !ok {
		t.Fatalf("modules[0] is not a map")
	}

	if first["status"] != pathStatusCompleted {
		t.Errorf("quiz-1: want completed, got %v", first["status"])
	}

	second, ok := modules[1].(map[string]any)
	if !ok {
		t.Fatalf("modules[1] is not a map")
	}

	if second["status"] != pathStatusAvailable {
		t.Errorf("quiz-2: want available, got %v", second["status"])
	}
}
