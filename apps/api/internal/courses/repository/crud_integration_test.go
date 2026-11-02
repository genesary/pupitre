//go:build integration

// crud_integration_test.go adds lab-check and path CRUD coverage on top of
// the round-trip tests in integration_test.go. Behind the `integration`
// build tag; run with TEST_DATABASE_URL set.

package repository_test

import (
	"testing"
	"time"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// TestLabCheckRepository_CRUD writes lab checks and reads them back through
// every query the export handlers use.
func TestLabCheckRepository_CRUD(t *testing.T) { //nolint:paralleltest // shares one database
	gdb := newTestDB(t)
	repo := repository.NewGormLabCheckRepository(gdb)
	ctx := t.Context()

	base := time.Date(2026, time.June, 1, 9, 0, 0, 0, time.UTC)

	pass := &models.LabCheck{
		Username: "alice", CourseSlug: "k8s", ModuleIndex: 1, ModuleName: "Pods",
		Allow: true, Violations: []string{}, Verified: true, CheckedAt: base,
	}
	fail := &models.LabCheck{
		Username: "bob", CourseSlug: "docker", ModuleIndex: 0, ModuleName: "Intro",
		Allow: false, Violations: []string{"missing Dockerfile"}, Verified: false,
		CheckedAt: base.Add(24 * time.Hour),
	}

	for _, c := range []*models.LabCheck{pass, fail} {
		err := repo.Create(ctx, c)
		if err != nil {
			t.Fatalf("Create(%s): %v", c.Username, err)
		}
	}

	all, err := repo.List(ctx, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("List(all) = %d, %v", len(all), err)
	}

	k8s, err := repo.List(ctx, "k8s")
	if err != nil || len(k8s) != 1 || k8s[0].Username != "alice" {
		t.Fatalf("List(k8s) = %+v, %v", k8s, err)
	}

	allowTrue := true

	rows, total, err := repo.ListExport(ctx, repository.LabCheckFilter{Allow: &allowTrue}, 10)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("ListExport(allow=true) = rows:%d total:%d err:%v", len(rows), total, err)
	}

	from := base.Add(12 * time.Hour)

	_, total, err = repo.ListExport(ctx, repository.LabCheckFilter{From: &from}, 0)
	if err != nil || total != 1 {
		t.Fatalf("ListExport(from) total = %d, %v", total, err)
	}

	var streamed int

	err = repo.StreamExport(ctx, repository.LabCheckFilter{}, func(_ *models.LabCheck) error {
		streamed++

		return nil
	})
	if err != nil || streamed != 2 {
		t.Fatalf("StreamExport streamed %d, err %v", streamed, err)
	}
}

// TestPathRepository_ListAndDelete exercises the path list pagination and
// delete paths not already covered by integration_test.go.
func TestPathRepository_ListAndDelete(t *testing.T) { //nolint:paralleltest // shares one database
	gdb := newTestDB(t)
	repo := repository.NewGormPathRepository(gdb)
	ctx := t.Context()

	for _, slug := range []string{"path-a", "path-b", "path-c"} {
		err := repo.Create(ctx, &content.Path{
			Slug: slug, Title: slug, Kind: "course", Courses: []string{"c1", "c2"},
		})
		if err != nil {
			t.Fatalf("Create(%s): %v", slug, err)
		}
	}

	all, err := repo.List(ctx, 0, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("List(all) = %d, %v", len(all), err)
	}

	page, err := repo.List(ctx, 2, 1)
	if err != nil || len(page) != 2 {
		t.Fatalf("List(limit 2, offset 1) = %d, %v", len(page), err)
	}

	got, err := repo.Get(ctx, "path-b")
	if err != nil || got.Slug != "path-b" || len(got.Courses) != 2 {
		t.Fatalf("Get(path-b) = %+v, %v", got, err)
	}

	slugs, err := repo.SlugsContainingCourse(ctx, "c1")
	if err != nil || len(slugs) != 3 {
		t.Fatalf("SlugsContainingCourse(c1) = %v, %v", slugs, err)
	}

	err = repo.Delete(ctx, "path-b")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.Get(ctx, "path-b")
	if err == nil {
		t.Fatal("Get(path-b) after Delete should error")
	}
}

// TestNewGormRepositories wires a full Repositories bundle and smoke-tests
// one method on each member.
func TestNewGormRepositories(t *testing.T) { //nolint:paralleltest // shares one database
	gdb := newTestDB(t)
	repos := repository.NewGormRepositories(gdb)
	ctx := t.Context()

	if repos.Courses == nil || repos.Paths == nil || repos.QuizAttempts == nil || repos.LabChecks == nil {
		t.Fatal("NewGormRepositories left a member nil")
	}

	checks, err := repos.LabChecks.List(ctx, "")
	if err != nil {
		t.Fatalf("LabChecks.List: %v", err)
	}

	if len(checks) != 0 {
		t.Errorf("expected no lab checks in a fresh db, got %d", len(checks))
	}

	paths, err := repos.Paths.List(ctx, 0, 0)
	if err != nil {
		t.Fatalf("Paths.List: %v", err)
	}

	if len(paths) != 0 {
		t.Errorf("expected no paths in a fresh db, got %d", len(paths))
	}
}

// TestCourseRepository_ModulesAndSkills covers the standalone Modules read,
// the skill aggregation queries and the Upsert (update) path.
func TestCourseRepository_ModulesAndSkills(t *testing.T) { //nolint:paralleltest // shares one database
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormCourseRepository(gdb)

	err := repo.Create(ctx, richIntegrationCourse())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	mods, err := repo.Modules(ctx, "kubernetes-basics")
	if err != nil {
		t.Fatalf("Modules: %v", err)
	}

	if len(mods) != 3 || mods[0].Name != "What is K8s" {
		t.Fatalf("Modules returned %+v", mods)
	}

	totals, err := repo.SkillTotals(ctx, []string{"kubernetes", "ops", "absent"})
	if err != nil {
		t.Fatalf("SkillTotals: %v", err)
	}

	if totals["kubernetes"] == 0 {
		t.Errorf("SkillTotals missing kubernetes: %+v", totals)
	}

	bySkill, err := repo.ModulesBySkills(ctx, []string{"kubernetes"})
	if err != nil {
		t.Fatalf("ModulesBySkills: %v", err)
	}

	if len(bySkill["kubernetes"]) == 0 {
		t.Error("ModulesBySkills(kubernetes) returned nothing")
	}

	// Upsert with a changed module set exercises replaceCourseChildren.
	updated := richIntegrationCourse()
	updated.Title = "Kubernetes Basics v2"
	updated.Modules = updated.Modules[:1]

	err = repo.Upsert(ctx, updated)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	after, err := repo.Get(ctx, "kubernetes-basics")
	if err != nil {
		t.Fatalf("Get after Upsert: %v", err)
	}

	if after.Title != "Kubernetes Basics v2" || len(after.Modules) != 1 {
		t.Errorf("Upsert did not replace children: title=%q modules=%d", after.Title, len(after.Modules))
	}
}

// TestCourseRepository_ListAttachesChildren checks that a catalogue listing
// hydrates each course's prerequisites and sessions in its batched follow-up
// queries (attachChildren / attachPrerequisites / attachSessions).
func TestCourseRepository_ListAttachesChildren(t *testing.T) { //nolint:paralleltest // shares one database
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormCourseRepository(gdb)

	err := repo.Create(ctx, richIntegrationCourse())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A second course with no children, so the attach loops must match rows
	// to the right course rather than blindly appending.
	err = repo.Create(ctx, &content.Course{
		Slug: "bare-course", Title: "Bare", Category: "misc", Difficulty: "beginner", IsPublic: true,
	})
	if err != nil {
		t.Fatalf("create bare: %v", err)
	}

	list, err := repo.List(ctx, repository.CourseFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var rich, bare *content.Course

	for _, c := range list {
		switch c.Slug {
		case "kubernetes-basics":
			rich = c
		case "bare-course":
			bare = c
		}
	}

	if rich == nil || bare == nil {
		t.Fatalf("List did not return both courses: %+v", list)
	}

	if len(rich.Prerequisites) != 1 || rich.Prerequisites[0].Course != "linux-intro" {
		t.Errorf("prerequisites not attached on List: %+v", rich.Prerequisites)
	}

	if len(rich.Sessions) != 1 || rich.Sessions[0].ID != "sess-1" {
		t.Errorf("sessions not attached on List: %+v", rich.Sessions)
	}

	if len(bare.Prerequisites) != 0 || len(bare.Sessions) != 0 {
		t.Errorf("child rows leaked onto the childless course: %+v / %+v", bare.Prerequisites, bare.Sessions)
	}
}
