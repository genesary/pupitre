package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// listModulesAs issues a module-list request for the given course with the
// supplied Authorization header value.
func listModulesAs(t *testing.T, s *State, courseSlug, auth string) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/courses/"+courseSlug+"/modules", http.NoBody)
	req.Header.Set("Authorization", auth)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// getModuleAs issues a single-module request with the supplied Authorization
// header value.
func getModuleAs(t *testing.T, s *State, courseSlug string, idx int, auth string) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/courses/"+courseSlug+"/modules/"+strconv.Itoa(idx), http.NoBody)
	req.Header.Set("Authorization", auth)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// checkMetaCourse seeds a course whose modules exercise every check-metadata
// branch of buildModuleListEntry / buildModuleDetailResponse: a git-backed
// lab, a local (Tauri) check module, a GitLab step-based lab, an inline quiz,
// and a git-backed quiz.
func checkMetaCourse() *content.Course {
	return &content.Course{
		Slug: "meta-course", Title: "Meta Course", IsPublic: true,
		Modules: []content.Module{
			{
				Name: "Git Lab", Type: "lab",
				Src: "https://example.test/lab.git", Ref: "main", Path: "lab.md",
				LabURL: "https://gitlab.test/student/lab",
			},
			{
				Name: "Local Check", Type: "lab",
				CheckProvider: "local", CheckType: "command",
				CheckParams: map[string]any{"cmd": "true"},
				Steps:       []content.CheckStep{{Title: "step-1", CheckType: "command"}},
			},
			{
				Name: "GitLab Steps", Type: "lab",
				CheckProvider: "gitlab",
				Steps: []content.CheckStep{
					{Title: "s1", CheckType: "pipeline"},
					{Title: "s2", CheckType: "pipeline"},
				},
			},
			{
				Name: "Inline Quiz", Type: "quiz",
				Questions: []content.Question{
					{ID: "q1", Type: "single", Question: "pick", Points: 1, Answers: []content.Answer{
						{ID: "a", Text: "A", Correct: true},
						{ID: "b", Text: "B"},
					}},
				},
			},
			{
				Name: "Git Quiz", Type: "quiz",
				Src: "https://example.test/quiz.git", Ref: "main", Path: "quiz.yaml",
			},
		},
	}
}

// TestListModules_AdminSeesSourceAndCheckMeta drives the admin branch of
// buildModuleListEntry: git source fields plus per-type check metadata.
func TestListModules_AdminSeesSourceAndCheckMeta(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, checkMetaCourse())

	rec := listModulesAs(t, s, "meta-course", adminAuthHeader(t, "test-secret"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Modules []struct {
			Type          string `json:"type"`
			Src           string `json:"src"`
			HasCheck      bool   `json:"hasCheck"`
			CheckProvider string `json:"checkProvider"`
			LabURL        string `json:"labUrl"`
			QuestionCount int    `json:"questionCount"`
		} `json:"modules"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Modules) != 5 {
		t.Fatalf("want 5 modules, got %d", len(resp.Modules))
	}

	if resp.Modules[0].Src != "https://example.test/lab.git" || !resp.Modules[0].HasCheck {
		t.Errorf("git lab entry = %+v, want src set + hasCheck", resp.Modules[0])
	}

	if resp.Modules[1].CheckProvider != "local" || !resp.Modules[1].HasCheck {
		t.Errorf("local check entry = %+v, want checkProvider=local + hasCheck", resp.Modules[1])
	}

	if resp.Modules[3].QuestionCount != 1 {
		t.Errorf("inline quiz questionCount = %d, want 1", resp.Modules[3].QuestionCount)
	}
}

// TestListModules_StudentUnlockedLabExposesURL drives the lab-module,
// non-locked branch of buildModuleListEntry (only reached when the lab is
// available, so it is the first module here).
func TestListModules_StudentUnlockedLabExposesURL(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, checkMetaCourse())

	rec := listModulesAs(t, s, "meta-course", authHeader(t, "test-secret"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Modules []struct {
			LabURL string `json:"labUrl"`
			Src    string `json:"src"`
			Locked bool   `json:"locked"`
		} `json:"modules"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Modules[0].Locked {
		t.Fatalf("first lab module should not be locked for a student")
	}

	if resp.Modules[0].LabURL != "https://gitlab.test/student/lab" ||
		resp.Modules[0].Src != "https://example.test/lab.git" {
		t.Errorf("unlocked lab entry = %+v, want labUrl + src exposed", resp.Modules[0])
	}
}

// TestGetModule_LocalCheckDetail drives the local-check branch of
// buildModuleDetailResponse (checkProvider/checkType/checkParams/steps).
func TestGetModule_LocalCheckDetail(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, checkMetaCourse())

	rec := getModuleAs(t, s, "meta-course", 1, authHeader(t, "test-secret"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		HasCheck      bool                `json:"hasCheck"`
		CheckProvider string              `json:"checkProvider"`
		CheckType     string              `json:"checkType"`
		CheckParams   map[string]any      `json:"checkParams"`
		Steps         []content.CheckStep `json:"steps"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.HasCheck || resp.CheckProvider != "local" || resp.CheckType != "command" {
		t.Errorf("local check detail = %+v", resp)
	}

	if len(resp.Steps) != 1 || resp.CheckParams["cmd"] != "true" {
		t.Errorf("local check params/steps = %+v", resp)
	}
}

// TestGetModule_GitLabStepsDetail drives the GitLab step-based branch of
// buildModuleDetailResponse.
func TestGetModule_GitLabStepsDetail(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, checkMetaCourse())

	rec := getModuleAs(t, s, "meta-course", 2, authHeader(t, "test-secret"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		CheckProvider string              `json:"checkProvider"`
		Steps         []content.CheckStep `json:"steps"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.CheckProvider != "gitlab" || len(resp.Steps) != 2 {
		t.Errorf("gitlab steps detail = %+v", resp)
	}
}

// TestGetModule_VideoContentResolved covers the video/image arm of
// populateModuleContent.
func TestGetModule_VideoContentResolved(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "vid-course", Title: "Vid", IsPublic: true,
		Modules: []content.Module{
			{Name: "Clip", Type: "video", Src: "/uploads/clip.mp4"},
		},
	})

	rec := getModuleAs(t, s, "vid-course", 0, authHeader(t, "test-secret"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Content string `json:"content"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Content != "/uploads/clip.mp4" {
		t.Errorf("video content = %q, want the src URL", resp.Content)
	}
}

// TestGetModule_LabGitFetchFails covers the lab error arm of
// populateModuleContent when the git source cannot be cloned.
func TestGetModule_LabGitFetchFails(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "bad-lab", Title: "Bad Lab", IsPublic: true,
		Modules: []content.Module{
			{
				Name: "Broken", Type: "lab",
				Src: "/nonexistent/repo.git", Ref: "main", Path: "lab.md",
			},
		},
	})

	rec := getModuleAs(t, s, "bad-lab", 0, authHeader(t, "test-secret"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 on unreachable lab git source, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestGetModule_QuizCooldownsSurfaced pre-records a failed attempt so the
// module detail response carries live cooldown state (moduleCooldowns loop
// body).
func TestGetModule_QuizCooldownsSurfaced(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	course := &content.Course{
		Slug: "cd-course", Title: "Cooldowns", IsPublic: true,
		Modules: []content.Module{
			{
				Name: "Quiz", Type: "quiz", PassingScore: 100,
				Cooldown: content.CooldownSpec{Strategy: "fixed", BaseSeconds: 600},
				Questions: []content.Question{
					{ID: "q1", Type: "single", Question: "pick", Points: 1, Answers: []content.Answer{
						{ID: "a", Text: "A", Correct: true},
						{ID: "b", Text: "B"},
					}},
				},
			},
		},
	}
	s := newStateWith(cfg, course)

	key := repository.AttemptKey{
		Username:    "00000000-0000-0000-0000-000000000001",
		CourseSlug:  "cd-course",
		ModuleIndex: 0,
	}

	_, err := s.Repos.QuizAttempts.RecordFailures(t.Context(), key, []string{"q1"},
		content.CooldownSpec{Strategy: "fixed", BaseSeconds: 600}, nil, false)
	if err != nil {
		t.Fatalf("seed cooldown: %v", err)
	}

	rec := getModuleAs(t, s, "cd-course", 0, authHeader(t, "test-secret"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Cooldowns map[string]struct {
			RemainingSeconds int `json:"remainingSeconds"`
			Attempts         int `json:"attempts"`
		} `json:"cooldowns"`
	}

	err = json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	cd, ok := resp.Cooldowns["q1"]
	if !ok || cd.Attempts != 1 || cd.RemainingSeconds <= 0 {
		t.Errorf("cooldown state for q1 = %+v (present=%v), want 1 attempt + positive remaining", cd, ok)
	}
}

// TestGetModule_InlineQuizFromGit embeds the trailing inline quiz in a
// lesson's response, resolving its questions from a git fixture
// (populateInlineQuizQuestions git branch).
func TestGetModule_InlineQuizFromGit(t *testing.T) {
	t.Parallel()

	quizYAML := "" +
		"id: iq\ntitle: Inline Quiz\npassingScore: 50\n" +
		"questions:\n" +
		"  - id: i1\n    type: single\n    question: pick a\n    points: 2\n" +
		"    answers:\n      - id: a\n        text: A\n        correct: true\n      - id: b\n        text: B\n"

	repoDir := gitFixture(t, map[string]string{"iq.yaml": quizYAML})

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "inline-course", Title: "Inline", IsPublic: true,
		Modules: []content.Module{
			{Name: "Lesson", Type: "text", InlineContent: "read me"},
			{
				Name: "Check", Type: "quiz", Inline: true,
				Src: repoDir, Ref: "main", Path: "iq.yaml",
			},
		},
	})

	rec := getModuleAs(t, s, "inline-course", 0, authHeader(t, "test-secret"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		InlineQuiz *struct {
			QuestionCount int   `json:"questionCount"`
			MaxScore      int   `json:"maxScore"`
			Questions     []any `json:"questions"`
		} `json:"inlineQuiz"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.InlineQuiz == nil || resp.InlineQuiz.QuestionCount != 1 || resp.InlineQuiz.MaxScore != 2 {
		t.Fatalf("inline quiz not resolved from git: %+v", resp.InlineQuiz)
	}
}
