//go:build integration

package repository_test

import (
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// TestXPAward_IsIdempotent is the regression test for XP inflation.
//
// Award() has always upserted with ON CONFLICT DO NOTHING, but the table had
// no unique index for it to conflict against, so every repeat call inserted
// another row: re-submitting a quiz already passed, or completing a lesson in
// a course already marked finished, paid the learner again each time.
//
// The in-memory fake deduplicated on (userID, source, sourceSlug) all along —
// modelling the constraint the real schema was missing — which is why the
// handler tests never caught this. Only a real database can.
func TestXPAward_IsIdempotent(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormXPRepository(gdb)

	learner := newLearner(t, gdb)

	for range 5 {
		err := repo.Award(ctx, learner, repository.XPSourceLesson, "linux/intro", repository.XPAmountLesson)
		if err != nil {
			t.Fatalf("Award: %v", err)
		}
	}

	total, err := repo.Total(ctx, learner)
	if err != nil {
		t.Fatalf("Total: %v", err)
	}

	if total != repository.XPAmountLesson {
		t.Errorf("five awards of the same lesson must total %d XP, got %d",
			repository.XPAmountLesson, total)
	}
}

// TestXPAward_DistinctSourcesAccumulate confirms the unique index constrains
// only repeats of the same thing, not different earnings.
func TestXPAward_DistinctSourcesAccumulate(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormXPRepository(gdb)

	learner := newLearner(t, gdb)

	awards := []struct {
		source, slug string
		amount       int
	}{
		{repository.XPSourceLesson, "linux/intro", repository.XPAmountLesson},
		{repository.XPSourceLesson, "linux/shell", repository.XPAmountLesson},
		{repository.XPSourceModule, "linux/quiz-1", repository.XPAmountModule},
		{repository.XPSourceCourse, "linux", repository.XPAmountCourse},
	}

	want := 0

	for _, award := range awards {
		err := repo.Award(ctx, learner, award.source, award.slug, award.amount)
		if err != nil {
			t.Fatalf("Award %s/%s: %v", award.source, award.slug, err)
		}

		want += award.amount
	}

	total, err := repo.Total(ctx, learner)
	if err != nil {
		t.Fatalf("Total: %v", err)
	}

	if total != want {
		t.Errorf("want %d XP across four distinct earnings, got %d", want, total)
	}
}
