package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// TestListSkillModules_ReturnsTaggedModules lists every public-course module
// carrying the requested skill tag.
func TestListSkillModules_ReturnsTaggedModules(t *testing.T) {
	t.Parallel()

	s := newStateWith(
		&config.Config{JWTSecret: "test-secret"},
		&content.Course{
			Slug: "linux-intro", Title: "Linux Intro", IsPublic: true,
			Modules: []content.Module{
				{Name: "Shell Basics", Type: "text", Skills: []string{"linux"}},
				{Name: "Networking", Type: "video", Skills: []string{"networking"}},
			},
		},
		&content.Course{
			Slug: "advanced-linux", Title: "Advanced Linux", IsPublic: true,
			Modules: []content.Module{
				{Name: "Systemd", Type: "text", Skills: []string{"linux"}},
			},
		},
		&content.Course{
			Slug: "hidden-linux", Title: "Hidden", IsPublic: false,
			Modules: []content.Module{
				{Name: "Secret", Type: "text", Skills: []string{"linux"}},
			},
		},
	)

	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/skills/linux/modules", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Skill   string `json:"skill"`
		Modules []struct {
			Name       string `json:"name"`
			Slug       string `json:"slug"`
			CourseSlug string `json:"courseSlug"`
		} `json:"modules"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Skill != "linux" {
		t.Errorf("skill = %q", resp.Skill)
	}

	// Two public-course modules tagged "linux"; the hidden course is excluded.
	if len(resp.Modules) != 2 {
		t.Fatalf("want 2 modules, got %d: %+v", len(resp.Modules), resp.Modules)
	}

	for _, m := range resp.Modules {
		if m.CourseSlug == "hidden-linux" {
			t.Errorf("module from a non-public course leaked: %+v", m)
		}

		if m.Slug == "" {
			t.Errorf("module slug not populated: %+v", m)
		}
	}
}

// TestListSkillModules_UnknownSkill returns an empty list, not an error.
func TestListSkillModules_UnknownSkill(t *testing.T) {
	t.Parallel()

	s := newStateWith(&config.Config{JWTSecret: "test-secret"})

	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/skills/nope/modules", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp struct {
		Modules []any `json:"modules"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Modules) != 0 {
		t.Errorf("want no modules, got %d", len(resp.Modules))
	}
}
