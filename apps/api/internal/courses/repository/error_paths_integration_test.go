//go:build integration

// error_paths_integration_test.go drives every repository method with a
// cancelled context so each query's error-wrapping branch is exercised.
// Behind the `integration` build tag; run with TEST_DATABASE_URL set.

package repository_test

import (
	"context"
	"io"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// wantErr fails the test unless err is non-nil.
func wantErr(t *testing.T, name string, err error) {
	t.Helper()

	if err == nil {
		t.Errorf("%s: expected an error on a cancelled context", name)
	}
}

// TestRepository_CancelledContext runs every course-service repository method
// against a context that is already cancelled.
//
//nolint:paralleltest // shares one database, truncated between tests
func TestRepository_CancelledContext(t *testing.T) {
	gdb := newTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	courses := repository.NewGormCourseRepository(gdb)
	_, err := courses.List(ctx, repository.CourseFilter{})
	wantErr(t, "Courses.List", err)
	_, err = courses.Get(ctx, "c")
	wantErr(t, "Courses.Get", err)
	_, err = courses.Modules(ctx, "c")
	wantErr(t, "Courses.Modules", err)
	wantErr(t, "Courses.Create", courses.Create(ctx, &content.Course{Slug: "c", Title: "c"}))
	wantErr(t, "Courses.Upsert", courses.Upsert(ctx, &content.Course{Slug: "c", Title: "c"}))
	wantErr(t, "Courses.Delete", courses.Delete(ctx, "c"))
	_, err = courses.SkillTotals(ctx, []string{"s"})
	wantErr(t, "Courses.SkillTotals", err)
	_, err = courses.ModulesBySkills(ctx, []string{"s"})
	wantErr(t, "Courses.ModulesBySkills", err)
	wantErr(t, "Courses.PutSession", courses.PutSession(ctx, "c", content.Session{ID: "s"}))
	wantErr(t, "Courses.DeleteSession", courses.DeleteSession(ctx, "c", "s"))
	_, err = courses.SessionExists(ctx, "c", "s")
	wantErr(t, "Courses.SessionExists", err)

	paths := repository.NewGormPathRepository(gdb)
	_, err = paths.List(ctx, 0, 0)
	wantErr(t, "Paths.List", err)
	_, err = paths.ListBySlugs(ctx, []string{"p"})
	wantErr(t, "Paths.ListBySlugs", err)
	_, err = paths.SkillsByCourse(ctx, []string{"c"})
	wantErr(t, "Paths.SkillsByCourse", err)
	_, err = paths.Get(ctx, "p")
	wantErr(t, "Paths.Get", err)
	_, err = paths.SlugsContainingCourse(ctx, "c")
	wantErr(t, "Paths.SlugsContainingCourse", err)
	_, err = paths.SkillsOfCourses(ctx, []string{"c"})
	wantErr(t, "Paths.SkillsOfCourses", err)
	wantErr(t, "Paths.Create", paths.Create(ctx, &content.Path{Slug: "p", Title: "p"}))
	wantErr(t, "Paths.Upsert", paths.Upsert(ctx, &content.Path{Slug: "p", Title: "p"}))
	wantErr(t, "Paths.Delete", paths.Delete(ctx, "p"))

	labs := repository.NewGormLabCheckRepository(gdb)
	_, err = labs.List(ctx, "c")
	wantErr(t, "LabChecks.List", err)
	wantErr(t, "LabChecks.Create", labs.Create(ctx, &models.LabCheck{Username: "u", CourseSlug: "c"}))
	_, _, err = labs.ListExport(ctx, repository.LabCheckFilter{}, 10)
	wantErr(t, "LabChecks.ListExport", err)
	wantErr(t, "LabChecks.StreamExport",
		labs.StreamExport(ctx, repository.LabCheckFilter{}, func(*models.LabCheck) error { return nil }))

	quiz := repository.NewGormQuizAttemptRepository(gdb)
	_, err = quiz.States(ctx, repository.AttemptKey{Username: "u", CourseSlug: "c", ModuleIndex: 0}, []string{"q"})
	wantErr(t, "QuizAttempts.States", err)
	wantErr(t, "QuizAttempts.Clear",
		quiz.Clear(ctx, repository.AttemptKey{Username: "u", CourseSlug: "c", ModuleIndex: 0}, []string{"q"}))

	_ = io.Discard
}
