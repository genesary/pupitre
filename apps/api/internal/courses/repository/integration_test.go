//go:build integration

// Package repository's integration tests run the real schema against a real
// PostgreSQL instance. They are behind the `integration` build tag so the
// default `go test ./...` stays hermetic.
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pupitre -e POSTGRES_USER=pupitre \
//	  -e POSTGRES_DB=pupitre -p 55432:5432 postgres:16-alpine
//	TEST_DATABASE_URL=postgres://pupitre:pupitre@localhost:55432/pupitre?sslmode=disable \
//	  go test -tags=integration ./internal/repository/
package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	coursedb "github.com/jojo91550/pupitre2/apps/api/internal/courses/db"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// newTestDB connects to TEST_DATABASE_URL, applies the real migrations, and
// truncates every table so each test starts from a known state.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	gdb, err := gorm.Open(postgres.Open(url), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(gormlogger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	ctx := t.Context()

	err = coursedb.RunMigrations(ctx, gdb)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	err = gdb.WithContext(ctx).Exec(`TRUNCATE
		courses, course_modules, course_prerequisites, course_sessions,
		paths, path_courses, path_skills, quiz_question_attempts, lab_checks
		RESTART IDENTITY CASCADE`).Error
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return gdb
}

// richIntegrationCourse is a course exercising every column and jsonb
// payload that has to survive a real write/read cycle.
func richIntegrationCourse() *content.Course {
	maxAttempts := 3

	return &content.Course{
		Slug: "kubernetes-basics", Title: "Kubernetes Basics", Description: "Intro to K8s",
		Category: "kubernetes", Difficulty: "beginner", IsPublic: true,
		Scope: "team-a", XPRequired: 120, InPerson: true,
		Badge: &content.Badge{Name: "Operator", Icon: "\U0001F680"},
		Prerequisites: []content.CoursePrerequisite{
			{Course: "linux-intro", MinScore: 30, Modules: []string{"quiz-bases"}},
		},
		Sessions: []content.Session{
			{ID: "sess-1", Title: "Cohort 1", Date: "2026-09-01T10:00:00Z", Location: "Room A", Capacity: 12},
		},
		Modules: []content.Module{
			{
				Name: "What is K8s", Type: "text",
				Src: "https://github.com/org/repo", Ref: "main", Path: "lessons/intro.md",
				Prerequisites: []string{"welcome"}, Skills: []string{"kubernetes"},
			},
			{
				Name: "Quiz", Type: "quiz", PassingScore: 80,
				MaxAttemptsPerQuestion: &maxAttempts, LockOnMaxAttempts: true,
				Cooldown: content.CooldownSpec{Strategy: "linear", BaseSeconds: 15},
				Questions: []content.Question{
					{Type: "single", Question: "Q?", Answers: []content.Answer{{ID: "a", Text: "A", Correct: true}}},
				},
				Skills: []string{"kubernetes", "ops"},
			},
			{
				Name: "Lab", Type: "lab", CheckProvider: "gitlab", CheckType: "mr-open",
				CheckParams: map[string]any{"project": "e-learning/{{ .Username }}"},
				Steps: []content.CheckStep{
					{Title: "Open an MR", CheckType: "mr-open", CheckParams: map[string]any{"target": "main"}},
				},
			},
		},
	}
}

// TestMigrationsCreateSchema verifies AutoMigrate actually produces every
// table, the composite unique index modules are ordered by, and the jsonb
// column types the module payloads rely on.
func TestMigrationsCreateSchema(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()

	for _, table := range []string{
		"courses", "course_modules", "course_prerequisites", "course_sessions",
		"paths", "path_courses", "path_skills", "quiz_question_attempts", "lab_checks",
	} {
		if !gdb.Migrator().HasTable(table) {
			t.Errorf("table %s was not created", table)
		}
	}

	if !gdb.Migrator().HasIndex("course_modules", "course_modules_pos_idx") {
		t.Error("course_modules is missing its (course_slug, position) unique index")
	}

	var jsonbColumns int

	err := gdb.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'course_modules'
		  AND column_name IN ('questions', 'steps', 'check_params')
		  AND data_type = 'jsonb'`).Scan(&jsonbColumns).Error
	if err != nil {
		t.Fatalf("inspect columns: %v", err)
	}

	if jsonbColumns != 3 {
		t.Errorf("want 3 jsonb payload columns on course_modules, got %d", jsonbColumns)
	}
}

// TestCourseRepository_RoundTrip writes a fully-populated course and reads
// it back through the real SQL path.
func TestCourseRepository_RoundTrip(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormCourseRepository(gdb)

	err := repo.Create(ctx, richIntegrationCourse())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	dupErr := repo.Create(ctx, richIntegrationCourse())
	if !errors.Is(dupErr, repository.ErrConflict) {
		t.Errorf("want ErrConflict on duplicate slug, got %v", dupErr)
	}

	got, err := repo.Get(ctx, "kubernetes-basics")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if len(got.Modules) != 3 || got.Modules[0].Name != "What is K8s" {
		t.Fatalf("modules not stored in order: %+v", got.Modules)
	}

	if len(got.Modules[1].Questions) != 1 || got.Modules[1].Questions[0].ID != "q-1" {
		t.Errorf("quiz jsonb did not round-trip: %+v", got.Modules[1].Questions)
	}

	if got.Modules[2].CheckParams["project"] != "e-learning/{{ .Username }}" {
		t.Errorf("check params jsonb did not round-trip: %+v", got.Modules[2].CheckParams)
	}

	if len(got.Prerequisites) != 1 || len(got.Sessions) != 1 {
		t.Errorf("children not stored: prereqs=%+v sessions=%+v", got.Prerequisites, got.Sessions)
	}

	if got.Badge == nil || got.Badge.Icon != "🚀" {
		t.Errorf("badge not stored: %+v", got.Badge)
	}

	// A second write of the same slug replaces the definition wholesale.
	updated := richIntegrationCourse()
	updated.Title = "Kubernetes Basics v2"
	updated.Modules = updated.Modules[:1]

	err = repo.Upsert(ctx, updated)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err = repo.Get(ctx, "kubernetes-basics")
	if err != nil {
		t.Fatalf("get after upsert: %v", err)
	}

	if got.Title != "Kubernetes Basics v2" || len(got.Modules) != 1 {
		t.Errorf("upsert did not replace the definition: title=%q modules=%d", got.Title, len(got.Modules))
	}

	err = repo.Delete(ctx, "kubernetes-basics")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// The FK cascade must take the module rows with it.
	var orphans int64

	err = gdb.WithContext(ctx).Table("course_modules").
		Where("course_slug = ?", "kubernetes-basics").Count(&orphans).Error
	if err != nil {
		t.Fatalf("count orphans: %v", err)
	}

	if orphans != 0 {
		t.Errorf("delete left %d orphaned module rows", orphans)
	}
}

// seedFilterCatalogue writes the small fixture catalogue the filter and
// listing tests query.
func seedFilterCatalogue(t *testing.T, ctx context.Context, repo repository.CourseRepository) {
	t.Helper()

	seed := []*content.Course{
		{
			Slug: "linux-intro", Title: "Linux Intro", Description: "Shell basics",
			Category: "linux", Difficulty: "beginner", IsPublic: true,
			Modules: []content.Module{
				{Name: "Shell", Skills: []string{"Linux"}},
				{Name: "Files", Skills: []string{"linux"}},
			},
		},
		{
			Slug: "docker-deep", Title: "Docker Deep Dive", Description: "Containers",
			Category: "docker", Difficulty: "advanced", IsPublic: true,
			Modules: []content.Module{{Name: "Images", Skills: []string{"docker"}}},
		},
		{
			Slug: "secret-course", Title: "Secret", Category: "linux",
			Difficulty: "beginner", IsPublic: false,
		},
	}

	for _, course := range seed {
		err := repo.Create(ctx, course)
		if err != nil {
			t.Fatalf("seed %s: %v", course.Slug, err)
		}
	}
}

// TestCourseRepository_ListFilters verifies the catalogue query filters in
// SQL, reports module counts, and never loads module rows.
func TestCourseRepository_ListFilters(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormCourseRepository(gdb)

	seedFilterCatalogue(t, ctx, repo)

	all, err := repo.List(ctx, repository.CourseFilter{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("admin listing should see private courses, got %d", len(all))
	}

	for _, course := range all {
		if course.Modules != nil {
			t.Errorf("catalogue listing loaded module rows for %s", course.Slug)
		}
	}

	public, err := repo.List(ctx, repository.CourseFilter{PublicOnly: true})
	if err != nil {
		t.Fatalf("list public: %v", err)
	}

	if len(public) != 2 {
		t.Fatalf("want 2 public courses, got %d", len(public))
	}

	// Ordered by title: "Docker Deep Dive" before "Linux Intro".
	if public[0].Slug != "docker-deep" {
		t.Errorf("catalogue not ordered by title: %+v", public[0].Slug)
	}

	if public[1].ModuleCount != 2 {
		t.Errorf("want moduleCount 2 for linux-intro, got %d", public[1].ModuleCount)
	}

	totals, err := repo.SkillTotals(ctx, []string{"linux", "docker"})
	if err != nil {
		t.Fatalf("skill totals: %v", err)
	}

	// linux-intro declares "Linux" and "linux"; AggregateSkills keeps both,
	// so only the exact tag is counted here.
	if totals["docker"] != 1 {
		t.Errorf("want 1 course teaching docker, got %d", totals["docker"])
	}

	bySkill, err := repo.ModulesBySkills(ctx, []string{"docker"})
	if err != nil {
		t.Fatalf("modules by skill: %v", err)
	}

	docker := bySkill["docker"]
	if len(docker) != 1 || docker[0].CourseSlug != "docker-deep" {
		t.Errorf("want the docker module, got %+v", docker)
	}
}

// TestCourseRepository_ModulesBySkills_BatchesEverySkill verifies the
// batched lookup attributes each module to every requested skill it carries,
// in one query — the property the per-skill loop it replaced could not have.
func TestCourseRepository_ModulesBySkills_BatchesEverySkill(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormCourseRepository(gdb)

	err := repo.Upsert(ctx, &content.Course{
		Slug: "multi", Title: "Multi", IsPublic: true,
		Modules: []content.Module{
			{Name: "Both", Type: "text", InlineContent: "x", Skills: []string{"alpha", "beta"}},
			{Name: "Alpha only", Type: "text", InlineContent: "x", Skills: []string{"alpha"}},
		},
	})
	if err != nil {
		t.Fatalf("seed course: %v", err)
	}

	bySkill, err := repo.ModulesBySkills(ctx, []string{"alpha", "beta", "absent"})
	if err != nil {
		t.Fatalf("modules by skills: %v", err)
	}

	if len(bySkill["alpha"]) != 2 {
		t.Errorf("want both alpha modules, got %+v", bySkill["alpha"])
	}

	if len(bySkill["beta"]) != 1 || bySkill["beta"][0].Name != "Both" {
		t.Errorf("want the shared module attributed to beta too, got %+v", bySkill["beta"])
	}

	if _, present := bySkill["absent"]; present {
		t.Errorf("a skill nothing teaches should be absent, got %+v", bySkill["absent"])
	}
}

// TestCourseRepository_Filters exercises each catalogue filter clause
// against a real database, since they are SQL expressions the fake cannot
// stand in for.
func TestCourseRepository_Filters(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormCourseRepository(gdb)

	seedFilterCatalogue(t, ctx, repo)

	cases := []struct {
		name   string
		filter repository.CourseFilter
		want   []string
	}{
		{"category", repository.CourseFilter{PublicOnly: true, Category: "LINUX"}, []string{"linux-intro"}},
		{"difficulty", repository.CourseFilter{PublicOnly: true, Difficulty: "Advanced"}, []string{"docker-deep"}},
		{"search title", repository.CourseFilter{PublicOnly: true, Search: "deep"}, []string{"docker-deep"}},
		{"search description", repository.CourseFilter{PublicOnly: true, Search: "shell"}, []string{"linux-intro"}},
		{"skill case-insensitive", repository.CourseFilter{PublicOnly: true, Skill: "LINUX"}, []string{"linux-intro"}},
		{"no match", repository.CourseFilter{PublicOnly: true, Category: "ruby"}, nil},
	}

	for _, tc := range cases { //nolint:paralleltest // subtests share the parent's database state
		t.Run(tc.name, func(t *testing.T) {
			got, listErr := repo.List(ctx, tc.filter)
			if listErr != nil {
				t.Fatalf("list: %v", listErr)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("want %v, got %d rows", tc.want, len(got))
			}

			for i, slug := range tc.want {
				if got[i].Slug != slug {
					t.Errorf("row %d: want %s, got %s", i, slug, got[i].Slug)
				}
			}
		})
	}
}

// TestCourseRepository_Sessions verifies session writes are keyed so a
// retry overwrites rather than duplicates.
func TestCourseRepository_Sessions(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormCourseRepository(gdb)

	err := repo.Create(ctx, &content.Course{Slug: "workshop", Title: "Workshop"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	session := content.Session{ID: "sess-1", Title: "Cohort 1", Date: "2026-09-01T10:00:00Z", Capacity: 10}

	for range 2 {
		err = repo.PutSession(ctx, "workshop", session)
		if err != nil {
			t.Fatalf("put session: %v", err)
		}
	}

	course, err := repo.Get(ctx, "workshop")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if len(course.Sessions) != 1 {
		t.Fatalf("repeated write should overwrite, got %d sessions", len(course.Sessions))
	}

	exists, err := repo.SessionExists(ctx, "workshop", "sess-1")
	if err != nil || !exists {
		t.Errorf("SessionExists: got %v, %v", exists, err)
	}

	ghostErr := repo.PutSession(ctx, "ghost", session)
	if !errors.Is(ghostErr, repository.ErrNotFound) {
		t.Errorf("want ErrNotFound for an unknown course, got %v", ghostErr)
	}

	missingErr := repo.DeleteSession(ctx, "workshop", "nope")
	if !errors.Is(missingErr, repository.ErrNotFound) {
		t.Errorf("want ErrNotFound for an unknown session, got %v", missingErr)
	}

	err = repo.DeleteSession(ctx, "workshop", "sess-1")
	if err != nil {
		t.Fatalf("delete session: %v", err)
	}
}

// TestPathRepository_RoundTrip verifies path members keep their order and
// the reverse lookup by course works off the index.
func TestPathRepository_RoundTrip(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()

	courses := repository.NewGormCourseRepository(gdb)
	paths := repository.NewGormPathRepository(gdb)

	err := courses.Create(ctx, &content.Course{
		Slug: "linux-intro", Title: "Linux", IsPublic: true,
		Modules: []content.Module{{Name: "Shell", Skills: []string{"linux", "shell"}}},
	})
	if err != nil {
		t.Fatalf("seed course: %v", err)
	}

	err = paths.Create(ctx, &content.Path{
		Slug: "devops", Title: "DevOps", Kind: "course",
		Courses: []string{"linux-intro", "docker-deep", "k8s"},
	})
	if err != nil {
		t.Fatalf("create path: %v", err)
	}

	dupErr := paths.Create(ctx, &content.Path{Slug: "devops"})
	if !errors.Is(dupErr, repository.ErrConflict) {
		t.Errorf("want ErrConflict on duplicate slug, got %v", dupErr)
	}

	got, err := paths.Get(ctx, "devops")
	if err != nil {
		t.Fatalf("get path: %v", err)
	}

	want := []string{"linux-intro", "docker-deep", "k8s"}
	for i, slug := range want {
		if got.Courses[i] != slug {
			t.Fatalf("member order not preserved: got %v, want %v", got.Courses, want)
		}
	}

	containing, err := paths.SlugsContainingCourse(ctx, "docker-deep")
	if err != nil {
		t.Fatalf("slugs containing: %v", err)
	}

	if len(containing) != 1 || containing[0] != "devops" {
		t.Errorf("want [devops], got %v", containing)
	}

	skills, err := paths.SkillsOfCourses(ctx, []string{"linux-intro"})
	if err != nil {
		t.Fatalf("skills of courses: %v", err)
	}

	if len(skills) != 2 {
		t.Errorf("want the 2 aggregated skills, got %v", skills)
	}

	_, missingErr := paths.Get(ctx, "ghost")
	if !errors.Is(missingErr, repository.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", missingErr)
	}
}

// TestQuizAttemptRepository verifies the batched attempt upsert: counters
// increment atomically, cooldown deadlines are written, and clearing a
// question forgets it.
func TestQuizAttemptRepository(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormQuizAttemptRepository(gdb)

	key := repository.AttemptKey{Username: "alice", CourseSlug: "linux-intro", ModuleIndex: 2}
	spec := content.CooldownSpec{Strategy: "fixed", BaseSeconds: 60}

	// Duplicate IDs in one call must not break the multi-row upsert.
	results, err := repo.RecordFailures(ctx, key, []string{"q1", "q2", "q1"}, spec, nil, false)
	if err != nil {
		t.Fatalf("record failures: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("want 2 distinct questions recorded, got %+v", results)
	}

	if results["q1"].Attempts != 1 || results["q1"].Remaining != 60*time.Second {
		t.Errorf("unexpected first-attempt state: %+v", results["q1"])
	}

	results, err = repo.RecordFailures(ctx, key, []string{"q1"}, spec, nil, false)
	if err != nil {
		t.Fatalf("second record: %v", err)
	}

	if results["q1"].Attempts != 2 {
		t.Errorf("attempt counter did not increment: %+v", results["q1"])
	}

	states, err := repo.States(ctx, key, []string{"q1", "q2", "q3"})
	if err != nil {
		t.Fatalf("states: %v", err)
	}

	if len(states) != 2 {
		t.Errorf("only recorded questions should be reported, got %+v", states)
	}

	if states["q1"].Remaining <= 0 {
		t.Errorf("cooldown deadline was not persisted: %+v", states["q1"])
	}

	// A different learner must not see alice's attempts.
	other, err := repo.States(ctx, repository.AttemptKey{Username: "bob", CourseSlug: "linux-intro", ModuleIndex: 2}, []string{"q1"})
	if err != nil {
		t.Fatalf("states for other user: %v", err)
	}

	if len(other) != 0 {
		t.Errorf("attempt state leaked across users: %+v", other)
	}

	err = repo.Clear(ctx, key, []string{"q1"})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}

	states, err = repo.States(ctx, key, []string{"q1", "q2"})
	if err != nil {
		t.Fatalf("states after clear: %v", err)
	}

	if _, still := states["q1"]; still {
		t.Error("cleared question is still recorded")
	}

	if _, ok := states["q2"]; !ok {
		t.Error("clearing q1 also dropped q2")
	}
}

// TestQuizAttemptRepository_LockOnMaxAttempts verifies the attempt cap
// reports locked instead of handing out another cooldown.
func TestQuizAttemptRepository_LockOnMaxAttempts(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormQuizAttemptRepository(gdb)

	key := repository.AttemptKey{Username: "alice", CourseSlug: "quiz-course", ModuleIndex: 0}
	spec := content.CooldownSpec{Strategy: "fixed", BaseSeconds: 30}
	maxAttempts := 2

	_, err := repo.RecordFailures(ctx, key, []string{"q1"}, spec, &maxAttempts, true)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	results, err := repo.RecordFailures(ctx, key, []string{"q1"}, spec, &maxAttempts, true)
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if !results["q1"].Locked {
		t.Errorf("want locked at the attempt cap, got %+v", results["q1"])
	}
}

// TestRepositoryNotFound verifies missing rows are reported as
// ErrNotFound rather than a zero value, so handlers can tell them apart
// from an outage.
func TestRepositoryNotFound(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := context.Background()

	courses := repository.NewGormCourseRepository(gdb)

	_, getErr := courses.Get(ctx, "ghost")
	if !errors.Is(getErr, repository.ErrNotFound) {
		t.Errorf("Get: want ErrNotFound, got %v", getErr)
	}

	deleteErr := courses.Delete(ctx, "ghost")
	if !errors.Is(deleteErr, repository.ErrNotFound) {
		t.Errorf("Delete: want ErrNotFound, got %v", deleteErr)
	}
}

// TestSeedDevCourses_AgainstPostgres runs the real dev seeder against a
// real database: every embedded course must land, re-running must not
// duplicate them, and the deepest seeded content must survive the round
// trip through jsonb and text[] columns.
func TestSeedDevCourses_AgainstPostgres(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	ctx := t.Context()
	repo := repository.NewGormCourseRepository(gdb)

	err := coursedb.SeedDevCourses(ctx, repo, coursedb.SeedDevCoursesMissing)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	seeded, err := repo.List(ctx, repository.CourseFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(seeded) == 0 {
		t.Fatal("seeding produced no courses")
	}

	t.Logf("seeded %d dev courses", len(seeded))

	for _, course := range seeded {
		if course.Title == "" || course.ModuleCount == 0 {
			t.Errorf("%s seeded incompletely: title=%q modules=%d",
				course.Slug, course.Title, course.ModuleCount)
		}
	}

	// Re-running must be idempotent.
	err = coursedb.SeedDevCourses(ctx, repo, coursedb.SeedDevCoursesMissing)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	again, err := repo.List(ctx, repository.CourseFilter{})
	if err != nil {
		t.Fatalf("list after re-seed: %v", err)
	}

	if len(again) != len(seeded) {
		t.Errorf("re-seeding changed the catalogue size: %d -> %d", len(seeded), len(again))
	}

	// A local edit survives the default mode...
	edited := *seeded[0]
	edited.Title = "Locally edited"
	edited.Modules = []content.Module{{Name: "Only module", Type: "text"}}

	err = repo.Upsert(ctx, &edited)
	if err != nil {
		t.Fatalf("local edit: %v", err)
	}

	err = coursedb.SeedDevCourses(ctx, repo, coursedb.SeedDevCoursesMissing)
	if err != nil {
		t.Fatalf("re-seed after edit: %v", err)
	}

	kept, err := repo.Get(ctx, edited.Slug)
	if err != nil {
		t.Fatalf("get after re-seed: %v", err)
	}

	if kept.Title != "Locally edited" {
		t.Errorf("default mode clobbered a local edit: %q", kept.Title)
	}

	// ...and overwrite mode restores the seed content.
	err = coursedb.SeedDevCourses(ctx, repo, coursedb.SeedDevCoursesOverwrite)
	if err != nil {
		t.Fatalf("overwrite seed: %v", err)
	}

	restored, err := repo.Get(ctx, edited.Slug)
	if err != nil {
		t.Fatalf("get after overwrite: %v", err)
	}

	if restored.Title == "Locally edited" {
		t.Error("overwrite mode did not restore the seed content")
	}

	// Spot-check the richest seeded course end to end.
	linux, err := repo.Get(ctx, "linux-intro")
	if err != nil {
		t.Fatalf("get linux-intro: %v", err)
	}

	if linux.Badge == nil || linux.Badge.Icon != "🐧" {
		t.Errorf("badge did not round-trip: %+v", linux.Badge)
	}

	if len(linux.Modules) == 0 || len(linux.Modules[0].InlineContent) < 100 {
		t.Errorf("markdown module body did not round-trip: %d modules", len(linux.Modules))
	}

	// And the catalogue query must serve them.
	public, err := repo.List(ctx, repository.CourseFilter{PublicOnly: true, Category: "linux"})
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}

	if len(public) == 0 {
		t.Error("seeded linux courses are not reachable through the catalogue filter")
	}
}
