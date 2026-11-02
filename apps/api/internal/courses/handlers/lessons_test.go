package handlers

import (
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// newSkillCatalogState returns a State seeded with courses that carry skill
// tags, so the skill-total query has something to count.
func newSkillCatalogState(t *testing.T) *State {
	t.Helper()

	mock := newUserServiceMock()
	t.Cleanup(mock.Close)

	cfg := &config.Config{JWTSecret: "test-secret"}

	return withPeers(newStateWith(cfg,
		&content.Course{
			Slug:       "go-basics",
			Title:      "Go Basics",
			Difficulty: "beginner",
			IsPublic:   true,
			Skills:     []string{"golang", "backend"},
		},
		&content.Course{
			Slug:       "go-advanced",
			Title:      "Go Advanced",
			Difficulty: "advanced",
			IsPublic:   true,
			Skills:     []string{"golang"},
		},
		&content.Course{
			Slug:       "docker-intro",
			Title:      "Docker Intro",
			Difficulty: "beginner",
			IsPublic:   true,
			Skills:     []string{"docker", "backend"},
		},
	), mock.URL, "")
}

// TestSkillTotals_NoMatch verifies zero is returned for a skill that no
// course in the catalog covers.
func TestSkillTotals_NoMatch(t *testing.T) {
	t.Parallel()

	s := newSkillCatalogState(t)

	totals, err := s.Repos.Courses.SkillTotals(t.Context(), []string{"kubernetes"})
	if err != nil {
		t.Fatalf("SkillTotals: %v", err)
	}

	if totals["kubernetes"] != 0 {
		t.Errorf("want 0 for unmatched skill, got %d", totals["kubernetes"])
	}
}

// TestSkillTotals_SingleSkill verifies the count is 1 for a skill covered
// by exactly one course.
func TestSkillTotals_SingleSkill(t *testing.T) {
	t.Parallel()

	s := newSkillCatalogState(t)

	totals, err := s.Repos.Courses.SkillTotals(t.Context(), []string{"docker"})
	if err != nil {
		t.Fatalf("SkillTotals: %v", err)
	}

	if totals["docker"] != 1 {
		t.Errorf("want 1 course for docker, got %d", totals["docker"])
	}
}

// TestSkillTotals_MultiSkill verifies each skill is counted independently
// when multiple skills are requested at once.
func TestSkillTotals_MultiSkill(t *testing.T) {
	t.Parallel()

	s := newSkillCatalogState(t)

	totals, err := s.Repos.Courses.SkillTotals(t.Context(), []string{"golang", "backend"})
	if err != nil {
		t.Fatalf("SkillTotals: %v", err)
	}

	if totals["golang"] != 2 {
		t.Errorf("want 2 courses for golang, got %d", totals["golang"])
	}

	if totals["backend"] != 2 {
		t.Errorf("want 2 courses for backend, got %d", totals["backend"])
	}
}

// TestSkillTotals_EmptyInput verifies an empty skill set returns an empty
// map without panicking.
func TestSkillTotals_EmptyInput(t *testing.T) {
	t.Parallel()

	s := newSkillCatalogState(t)

	totals, err := s.Repos.Courses.SkillTotals(t.Context(), nil)
	if err != nil {
		t.Fatalf("SkillTotals: %v", err)
	}

	if len(totals) != 0 {
		t.Errorf("want empty totals, got %v", totals)
	}
}
