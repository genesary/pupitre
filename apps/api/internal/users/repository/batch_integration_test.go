//go:build integration

package repository_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// These tests cover the batched repository reads the handlers rely on to
// stay at a fixed query count. Each is a SQL expression the in-memory fakes
// cannot stand in for — array parameters, uuid-to-text casts, grouping —
// so the real database is the only place their behavior is pinned.

// TestViewedSlugsByCourses_KeysEachCourse verifies one query answers for
// several courses at once, and that a course with no progress is simply
// absent rather than reported empty for someone else's rows.
func TestViewedSlugsByCourses_KeysEachCourse(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormLessonProgressRepository(gdb)

	learner := newLearner(t, gdb)
	other := newLearner(t, gdb)

	viewLesson(t, gdb, learner, "linux", "intro")
	viewLesson(t, gdb, learner, "linux", "shell")
	viewLesson(t, gdb, learner, "docker", "images")
	viewLesson(t, gdb, other, "linux", "not-mine")

	byCourse, err := repo.ViewedSlugsByCourses(ctx, learner, []string{"linux", "docker", "kubernetes"})
	if err != nil {
		t.Fatalf("ViewedSlugsByCourses: %v", err)
	}

	linux := byCourse["linux"]
	slices.Sort(linux)

	if !slices.Equal(linux, []string{"intro", "shell"}) {
		t.Errorf("linux: want [intro shell], got %v", linux)
	}

	if got := byCourse["docker"]; len(got) != 1 || got[0] != "images" {
		t.Errorf("docker: want [images], got %v", got)
	}

	if got, present := byCourse["kubernetes"]; present {
		t.Errorf("a course with no progress should be absent, got %v", got)
	}
}

// TestListByUserCourses_KeysEachCourse verifies the batched module-progress
// read groups rows by course and excludes other learners'.
func TestListByUserCourses_KeysEachCourse(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormModuleProgressRepository(gdb)

	learner := newLearner(t, gdb)
	other := newLearner(t, gdb)

	passModule(t, gdb, learner, "linux", "quiz-1", 0)
	passModule(t, gdb, learner, "linux", "quiz-2", 1)
	passModule(t, gdb, learner, "docker", "quiz-1", 0)
	passModule(t, gdb, other, "linux", "quiz-1", 0)

	byCourse, err := repo.ListByUserCourses(ctx, learner, []string{"linux", "docker"})
	if err != nil {
		t.Fatalf("ListByUserCourses: %v", err)
	}

	if len(byCourse["linux"]) != 2 {
		t.Errorf("linux: want 2 rows, got %d", len(byCourse["linux"]))
	}

	if len(byCourse["docker"]) != 1 {
		t.Errorf("docker: want 1 row, got %d", len(byCourse["docker"]))
	}

	// Ordered by module index, which the overview endpoint reports as-is.
	if got := byCourse["linux"]; len(got) == 2 && got[0].ModuleIndex != 0 {
		t.Errorf("want rows ordered by module index, got %d first", got[0].ModuleIndex)
	}
}

// TestCompletedCourseSlugsByUsers_AnswersForACohort verifies the cohort
// completion query, whose uuid-to-text cast against a text array parameter
// is the part no fake can stand in for.
func TestCompletedCourseSlugsByUsers_AnswersForACohort(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormLessonProgressRepository(gdb)

	finished := newLearner(t, gdb)
	partial := newLearner(t, gdb)
	absent := newLearner(t, gdb)

	viewLesson(t, gdb, finished, "linux", "__complete__")
	viewLesson(t, gdb, finished, "docker", "__complete__")
	viewLesson(t, gdb, partial, "linux", "intro") // viewed, not completed

	byUser, err := repo.CompletedCourseSlugsByUsers(ctx,
		[]string{finished, partial, absent}, []string{"linux", "docker"})
	if err != nil {
		t.Fatalf("CompletedCourseSlugsByUsers: %v", err)
	}

	done := byUser[finished]
	slices.Sort(done)

	if !slices.Equal(done, []string{"docker", "linux"}) {
		t.Errorf("finished learner: want both courses, got %v", done)
	}

	if got := byUser[partial]; len(got) != 0 {
		t.Errorf("a viewed-but-unfinished course must not count as complete, got %v", got)
	}

	if got, present := byUser[absent]; present {
		t.Errorf("a learner with no progress should be absent, got %v", got)
	}
}

// TestEarnedCounts_GroupsByCourse verifies the batched badge-earner count.
func TestEarnedCounts_GroupsByCourse(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormBadgeRepository(gdb)

	first := newLearner(t, gdb)
	second := newLearner(t, gdb)

	for _, award := range []struct{ user, course string }{
		{first, "linux"}, {second, "linux"}, {first, "docker"},
	} {
		err := repo.Award(ctx, award.user, award.course)
		if err != nil {
			t.Fatalf("Award: %v", err)
		}
	}

	counts, err := repo.EarnedCounts(ctx, []string{"linux", "docker", "nobody-has-this"})
	if err != nil {
		t.Fatalf("EarnedCounts: %v", err)
	}

	if counts["linux"] != 2 {
		t.Errorf("linux: want 2 earners, got %d", counts["linux"])
	}

	if counts["docker"] != 1 {
		t.Errorf("docker: want 1 earner, got %d", counts["docker"])
	}

	if _, present := counts["nobody-has-this"]; present {
		t.Error("a badge nobody has earned should be absent from the counts")
	}
}

// TestGroupMembershipBatches verifies the two cohort-wide group queries,
// both of which cast uuid columns to text to compare against an array
// parameter.
func TestGroupMembershipBatches(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormGroupRepository(gdb)

	inScope := newLearner(t, gdb)
	alsoInScope := newLearner(t, gdb)
	outOfScope := newLearner(t, gdb)

	scoped := newGroup(t, gdb, "scoped")
	elsewhere := newGroup(t, gdb, "elsewhere")

	joinGroup(t, gdb, inScope, scoped)
	joinGroup(t, gdb, alsoInScope, scoped)
	joinGroup(t, gdb, outOfScope, elsewhere)

	members, err := repo.UsersInAnyGroup(ctx,
		[]string{inScope, alsoInScope, outOfScope}, []string{scoped})
	if err != nil {
		t.Fatalf("UsersInAnyGroup: %v", err)
	}

	if !members[inScope] || !members[alsoInScope] {
		t.Errorf("want both scoped members reported in scope, got %v", members)
	}

	if members[outOfScope] {
		t.Error("a member of another group must not be reported in scope")
	}

	byGroup, err := repo.ListMemberIDsByGroups(ctx, []string{scoped, elsewhere})
	if err != nil {
		t.Fatalf("ListMemberIDsByGroups: %v", err)
	}

	if len(byGroup[scoped]) != 2 {
		t.Errorf("scoped group: want 2 members, got %v", byGroup[scoped])
	}

	if len(byGroup[elsewhere]) != 1 {
		t.Errorf("other group: want 1 member, got %v", byGroup[elsewhere])
	}
}

// newGroup inserts a group and returns its ID.
func newGroup(t *testing.T, gdb *gorm.DB, name string) string {
	t.Helper()

	id := uuid.New()

	err := gdb.WithContext(t.Context()).Create(&models.Group{
		ID: id, Name: name + "-" + id.String()[:8], Source: "local", Path: id.String(),
	}).Error
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	return id.String()
}

// joinGroup records that userID is a member of groupID.
func joinGroup(t *testing.T, gdb *gorm.DB, userID, groupID string) {
	t.Helper()

	err := gdb.WithContext(t.Context()).Create(&models.UserGroup{
		UserID: userID, GroupID: uuid.MustParse(groupID),
	}).Error
	if err != nil {
		t.Fatalf("join group: %v", err)
	}
}
