package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// lessonCompleteBody is the payload course-service posts when a learner
// finishes a lesson.
const lessonCompletePayload = `{"userId":"user-uuid-1","courseSlug":"linux-intro","lessonSlug":"intro"}`

// TestMarkLessonComplete_AwardsXP marks a lesson complete and confirms
// lesson XP was recorded.
//
// It drives the internal endpoint because that is the one a learner reaches:
// course-service owns the public lesson route and reports the completion
// inwards. user-service used to award lesson XP from a public route of its
// own that no ingress could route to, so the XP was never granted.
func TestMarkLessonComplete_AwardsXP(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, http.MethodPost, "/internal/progress/complete", lessonCompletePayload)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	total, err := repos.XP.Total(t.Context(), "user-uuid-1")
	if err != nil {
		t.Fatalf("XP.Total: %v", err)
	}

	if total == 0 {
		t.Error("expected lesson XP to have been awarded")
	}
}

// TestMarkLessonComplete_XPFailureStillSucceeds keeps a 200 when the XP
// award fails — XP is non-critical, so awardXP only logs its error.
func TestMarkLessonComplete_XPFailureStillSucceeds(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	xp := fake.NewXPRepository()
	xp.Err = errors.New("xp table locked")
	repos.XP = xp
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, http.MethodPost, "/internal/progress/complete", lessonCompletePayload)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200 despite XP failure, got %d", rec.Code)
	}
}

// TestMarkLessonComplete_XPAwardedOnce verifies a repeated completion does
// not pay twice. Award() upserts on (userid, source, source_slug); before
// that index existed the ON CONFLICT clause had nothing to conflict against
// and every repeat call inflated the learner's total.
func TestMarkLessonComplete_XPAwardedOnce(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newTestRouterWithRepos(repos)

	for range 3 {
		rec := htDoInternal(t, r, http.MethodPost, "/internal/progress/complete", lessonCompletePayload)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	total, err := repos.XP.Total(t.Context(), "user-uuid-1")
	if err != nil {
		t.Fatalf("XP.Total: %v", err)
	}

	if total != repository.XPAmountLesson {
		t.Errorf("want the lesson paid once (%d XP), got %d", repository.XPAmountLesson, total)
	}
}
