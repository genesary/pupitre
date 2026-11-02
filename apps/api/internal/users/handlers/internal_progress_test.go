package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// TestInternalViewedLessons_ReturnsSlugs returns the viewed lesson slugs for
// a user in a course.
func TestInternalViewedLessons_ReturnsSlugs(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.LessonProgress = fake.NewLessonProgressRepository(
		models.LessonProgress{UserID: "u1", CourseSlug: "linux-intro", LessonSlug: "intro"},
		models.LessonProgress{UserID: "u1", CourseSlug: "linux-intro", LessonSlug: "shell"},
		models.LessonProgress{UserID: "u1", CourseSlug: "other", LessonSlug: "x"},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, http.MethodGet, "/internal/progress/viewed?userId=u1&courseSlug=linux-intro", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Viewed []string `json:"viewed"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Viewed) != 2 {
		t.Errorf("viewed = %v, want 2 slugs for linux-intro", resp.Viewed)
	}
}

// TestInternalViewedLessons_EmptyIsArray returns a JSON array (not null)
// when the user has viewed nothing.
func TestInternalViewedLessons_EmptyIsArray(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, http.MethodGet, "/internal/progress/viewed?userId=u1&courseSlug=c", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	if got := rec.Body.String(); got == `{"viewed":null}` || got == "{\"viewed\":null}\n" {
		t.Errorf("expected an empty array, got %q", got)
	}
}

// TestInternalViewedLessons_DBError surfaces a repository failure as 500.
func TestInternalViewedLessons_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	lp := fake.NewLessonProgressRepository()
	lp.Err = errors.New("db down")
	repos.LessonProgress = lp
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, http.MethodGet, "/internal/progress/viewed?userId=u1&courseSlug=c", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}
