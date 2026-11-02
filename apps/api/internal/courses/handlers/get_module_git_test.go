package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// getModule issues an authenticated student GET for one module.
func getModule(t *testing.T, s *State, courseSlug string, idx int) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/courses/"+courseSlug+"/modules/"+strconv.Itoa(idx), http.NoBody)
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// TestGetModule_GitText fetches a text module's markdown body from a git
// fixture repo.
func TestGetModule_GitText(t *testing.T) {
	t.Parallel()

	repoDir := gitFixture(t, map[string]string{
		"lessons/intro.md": "# Introduction\n\nWelcome to the course.\n",
	})

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "git-course", Title: "Git Course", IsPublic: true,
		Modules: []content.Module{
			{Name: "Intro", Type: "text", Src: repoDir, Ref: "main", Path: "lessons/intro.md"},
		},
	})

	rec := getModule(t, s, "git-course", 0)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	body, _ := resp["content"].(string)
	if !strings.Contains(body, "Welcome to the course") {
		t.Errorf("module content not fetched from git: %q", body)
	}
}

// TestGetModule_GitQuiz resolves quiz questions from a git-hosted quiz YAML.
func TestGetModule_GitQuiz(t *testing.T) {
	t.Parallel()

	quizYAML := "" +
		"id: q1\n" +
		"title: Basics Quiz\n" +
		"passingScore: 50\n" +
		"questions:\n" +
		"  - id: q-1\n" +
		"    type: single\n" +
		"    question: 2 + 2 ?\n" +
		"    points: 1\n" +
		"    answers:\n" +
		"      - id: a\n" +
		"        text: \"4\"\n" +
		"        correct: true\n" +
		"      - id: b\n" +
		"        text: \"5\"\n"

	repoDir := gitFixture(t, map[string]string{"quizzes/basics.yaml": quizYAML})

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "git-course", Title: "Git Course", IsPublic: true,
		Modules: []content.Module{
			{Name: "Quiz", Type: "quiz", Src: repoDir, Ref: "main", Path: "quizzes/basics.yaml"},
		},
	})

	rec := getModule(t, s, "git-course", 0)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Questions []struct {
			ID       string `json:"id"`
			Question string `json:"question"`
		} `json:"questions"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Questions) != 1 || resp.Questions[0].ID != "q-1" {
		t.Fatalf("quiz questions not populated from git: %+v", resp.Questions)
	}
}

// TestListModules_ExpandsModuleIndex replaces a type:modules entry with the
// modules listed in its git-hosted index file.
func TestListModules_ExpandsModuleIndex(t *testing.T) {
	t.Parallel()

	indexYAML := "" +
		"- name: Lesson One\n" +
		"  type: text\n" +
		"  path: lessons/one.md\n" +
		"- name: Lesson Two\n" +
		"  type: text\n" +
		"  path: lessons/two.md\n"

	repoDir := gitFixture(t, map[string]string{
		"index.yaml":     indexYAML,
		"lessons/one.md": "one",
		"lessons/two.md": "two",
	})

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "git-course", Title: "Git Course", IsPublic: true,
		Modules: []content.Module{
			{Name: "Index", Type: "modules", Src: repoDir, Ref: "main", Path: "index.yaml"},
			{Name: "Standalone", Type: "text", InlineContent: "hi"},
		},
	})

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/courses/git-course/modules", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Modules []struct {
			Name string `json:"name"`
		} `json:"modules"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	names := make([]string, 0, len(resp.Modules))
	for _, m := range resp.Modules {
		names = append(names, m.Name)
	}

	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "Lesson One") || !strings.Contains(joined, "Lesson Two") ||
		!strings.Contains(joined, "Standalone") {
		t.Errorf("module index not expanded: %v", names)
	}
}
