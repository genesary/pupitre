package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// TestMyPaths_SkillKind drives MyPaths for a skill-kind path, exercising
// buildSkillStatuses / fetchSkillModules / skillIsCompleted end to end
// against a mock course-service.
func TestMyPaths_SkillKind(t *testing.T) {
	t.Parallel()

	courseSvc, _ := newCatalogServer(t, catalogFixture{
		Paths: map[string]PathInfo{
			"skills-path": {Slug: "skills-path", Title: "Skills Path", Kind: "skill", Skills: []string{"linux", "docker"}},
		},
		Skills: map[string][]SkillModule{
			"linux":  {{Slug: "quiz-linux", Type: "quiz", CourseSlug: "linux-intro"}},
			"docker": {},
		},
	})

	userID := uuid.New()

	repos := fake.NewRepositories()
	repos.Paths = fake.NewPathEnrollmentRepository(models.PathEnrollment{
		UserID: userID, PathSlug: "skills-path", EnrolledAt: time.Now(),
	})
	// The linux quiz is passed → the "linux" skill is completed.
	slug := "quiz-linux"
	repos.ModuleProgress = fake.NewModuleProgressRepository(models.ModuleProgress{
		UserID: userID.String(), CourseSlug: "linux-intro", ModuleIndex: 0,
		ModuleSlug: &slug, Passed: true, BestScore: 10, MaxScore: 10,
	})

	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
	}
	s := &State{Repos: repos, Config: cfg, Courses: courseSourceAt(courseSvc.URL)}
	router := BuildRouter(s, cfg, false)

	rec := htDo(t, router, http.MethodGet, "/api/my/paths", "",
		htAuthHeaderForSubject(t, "student", userID.String()))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Paths []struct {
			Slug    string `json:"slug"`
			Kind    string `json:"kind"`
			Courses []struct {
				Slug   string `json:"slug"`
				Status string `json:"status"`
			} `json:"courses"`
		} `json:"paths"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Paths) != 1 || resp.Paths[0].Kind != "skill" {
		t.Fatalf("unexpected paths: %+v", resp.Paths)
	}

	statuses := resp.Paths[0].Courses
	if len(statuses) != 2 {
		t.Fatalf("want 2 skill statuses, got %+v", statuses)
	}

	if statuses[0].Slug != "linux" || statuses[0].Status != "completed" {
		t.Errorf("linux status = %+v, want completed", statuses[0])
	}

	// docker has no assessable modules → not completed; since linux is done it
	// is the next available skill.
	if statuses[1].Slug != "docker" || statuses[1].Status != "available" {
		t.Errorf("docker status = %+v, want available", statuses[1])
	}
}

// TestMyPaths_SkillKind_ModuleFetchFails marks every skill "locked" when
// course-service cannot return its modules (buildSkillStatuses error branch).
func TestMyPaths_SkillKind_ModuleFetchFails(t *testing.T) {
	t.Parallel()

	// The path resolves, but no skill has any module recorded, so no skill
	// can be reported completed.
	courseSvc, _ := newCatalogServer(t, catalogFixture{
		Paths: map[string]PathInfo{
			"skills-path": {Slug: "skills-path", Title: "Skills", Kind: "skill", Skills: []string{"linux", "docker"}},
		},
	})

	userID := uuid.New()
	repos := fake.NewRepositories()
	repos.Paths = fake.NewPathEnrollmentRepository(models.PathEnrollment{
		UserID: userID, PathSlug: "skills-path", EnrolledAt: time.Now(),
	})

	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
	}
	s := &State{Repos: repos, Config: cfg, Courses: courseSourceAt(courseSvc.URL)}
	router := BuildRouter(s, cfg, false)

	rec := htDo(t, router, http.MethodGet, "/api/my/paths", "",
		htAuthHeaderForSubject(t, "student", userID.String()))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Paths []struct {
			Courses []struct {
				Status string `json:"status"`
			} `json:"courses"`
		} `json:"paths"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Paths) != 1 || len(resp.Paths[0].Courses) != 2 {
		t.Fatalf("unexpected skill statuses: %+v", resp.Paths)
	}

	for _, c := range resp.Paths[0].Courses {
		if c.Status != "locked" {
			t.Errorf("status = %q, want locked when modules cannot be fetched", c.Status)
		}
	}
}
