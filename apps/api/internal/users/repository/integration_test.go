//go:build integration

// Package repository's integration tests run the real schema and the real
// SQL against a live PostgreSQL instance. They are behind the `integration`
// build tag so the default `go test ./...` stays hermetic.
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pupitre -e POSTGRES_USER=pupitre \
//	  -e POSTGRES_DB=pupitre -p 55433:5432 postgres:16-alpine
//	TEST_DATABASE_URL=postgres://pupitre:pupitre@localhost:55433/pupitre?sslmode=disable \
//	  go test -tags=integration ./internal/repository/
package repository_test

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/db"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// newTestDB connects to TEST_DATABASE_URL, applies the real migrations, and
// clears the tables these tests touch.
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

	err = db.RunMigrations(t.Context(), gdb)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	err = gdb.WithContext(t.Context()).
		Exec("TRUNCATE users, enrollments, lesson_progress, module_progress, " +
			"user_badges, user_groups, groups, user_xp_events RESTART IDENTITY CASCADE").Error
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return gdb
}

// newLearner inserts a user and returns its ID, since enrollments carry a
// foreign key to users.
func newLearner(t *testing.T, gdb *gorm.DB) string {
	t.Helper()

	id := uuid.New()
	name := "learner-" + id.String()[:8]

	err := gdb.WithContext(t.Context()).Create(&models.User{
		ID: id, Username: name, Email: name + "@example.test", Role: "student",
	}).Error
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	return id.String()
}

// enrol records an enrollment for userID in courseSlug.
func enrol(t *testing.T, gdb *gorm.DB, userID, courseSlug string) {
	t.Helper()

	err := gdb.WithContext(t.Context()).Create(&models.Enrollment{
		UserID: userID, CourseSlug: courseSlug,
	}).Error
	if err != nil {
		t.Fatalf("enrol: %v", err)
	}
}

// viewLesson records that userID has viewed a module as a lesson.
func viewLesson(t *testing.T, gdb *gorm.DB, userID, courseSlug, slug string) {
	t.Helper()

	err := gdb.WithContext(t.Context()).Create(&models.LessonProgress{
		UserID: userID, CourseSlug: courseSlug, LessonSlug: slug,
	}).Error
	if err != nil {
		t.Fatalf("view lesson %s: %v", slug, err)
	}
}

// passModule records that userID has passed a module.
func passModule(t *testing.T, gdb *gorm.DB, userID, courseSlug, slug string, index int) {
	t.Helper()

	err := gdb.WithContext(t.Context()).Create(&models.ModuleProgress{
		UserID: userID, CourseSlug: courseSlug, ModuleIndex: index,
		ModuleSlug: &slug, Passed: true, BestScore: 10, MaxScore: 10,
	}).Error
	if err != nil {
		t.Fatalf("pass module %s: %v", slug, err)
	}
}

// TestMyEnrollments_CountsEachModuleOnce is the regression test for the
// /my-courses progress bar reporting more than 100%.
//
// A quiz module a learner both opened (lesson_progress) and passed
// (module_progress) is one completed module, not two. Counting the two
// tables separately and adding them double-counted it, so a single-module
// course rendered as 2/1 — 200%.
func TestMyEnrollments_CountsEachModuleOnce(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	repo := repository.NewGormEnrollmentRepository(gdb)
	userID := newLearner(t, gdb)

	enrol(t, gdb, userID, "one-module-course")

	// The same module, both viewed and passed.
	viewLesson(t, gdb, userID, "one-module-course", "the-quiz")
	passModule(t, gdb, userID, "one-module-course", "the-quiz", 0)

	rows, err := repo.MyEnrollments(t.Context(), userID)
	if err != nil {
		t.Fatalf("MyEnrollments: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("want 1 enrollment, got %d", len(rows))
	}

	if rows[0].CompletedLabs != 1 {
		t.Errorf("a module that was both viewed and passed must count once: got %d",
			rows[0].CompletedLabs)
	}

	if rows[0].TotalScore != 10 {
		t.Errorf("want total score 10, got %d", rows[0].TotalScore)
	}
}

// TestMyEnrollments_IgnoresCompletionSentinel is the second half of the
// >100% regression: lesson_progress also stores a course-level
// "__complete__" sentinel, which is not a module and must not be counted
// as one.
func TestMyEnrollments_IgnoresCompletionSentinel(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	repo := repository.NewGormEnrollmentRepository(gdb)
	userID := newLearner(t, gdb)

	enrol(t, gdb, userID, "one-module-course")

	viewLesson(t, gdb, userID, "one-module-course", "the-only-module")
	// Finishing the course writes the sentinel alongside the real lesson.
	viewLesson(t, gdb, userID, "one-module-course", "__complete__")

	rows, err := repo.MyEnrollments(t.Context(), userID)
	if err != nil {
		t.Fatalf("MyEnrollments: %v", err)
	}

	if rows[0].CompletedLabs != 1 {
		t.Errorf("the completion sentinel must not count as a module: got %d",
			rows[0].CompletedLabs)
	}
}

// TestMyEnrollments_CountsDistinctModules verifies the aggregate over a
// realistic mix: modules only viewed, modules only passed, and modules
// both.
func TestMyEnrollments_CountsDistinctModules(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	repo := repository.NewGormEnrollmentRepository(gdb)
	userID := newLearner(t, gdb)

	enrol(t, gdb, userID, "mixed-course")

	viewLesson(t, gdb, userID, "mixed-course", "intro")        // viewed only
	viewLesson(t, gdb, userID, "mixed-course", "quiz-one")     // viewed…
	passModule(t, gdb, userID, "mixed-course", "quiz-one", 1)  // …and passed
	passModule(t, gdb, userID, "mixed-course", "quiz-two", 2)  // passed only
	viewLesson(t, gdb, userID, "mixed-course", "__complete__") // sentinel, not a module

	rows, err := repo.MyEnrollments(t.Context(), userID)
	if err != nil {
		t.Fatalf("MyEnrollments: %v", err)
	}

	// intro, quiz-one, quiz-two → 3 distinct completed modules.
	if rows[0].CompletedLabs != 3 {
		t.Errorf("want 3 distinct completed modules, got %d", rows[0].CompletedLabs)
	}
}

// TestMyEnrollments_FailedModuleNotCounted verifies an attempted-but-failed
// module is not counted as completed.
func TestMyEnrollments_FailedModuleNotCounted(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	repo := repository.NewGormEnrollmentRepository(gdb)
	userID := newLearner(t, gdb)

	enrol(t, gdb, userID, "hard-course")

	slug := "tough-quiz"

	err := gdb.WithContext(t.Context()).Create(&models.ModuleProgress{
		UserID: userID, CourseSlug: "hard-course", ModuleIndex: 0,
		ModuleSlug: &slug, Passed: false, BestScore: 3, MaxScore: 10,
	}).Error
	if err != nil {
		t.Fatalf("record attempt: %v", err)
	}

	rows, err := repo.MyEnrollments(t.Context(), userID)
	if err != nil {
		t.Fatalf("MyEnrollments: %v", err)
	}

	if rows[0].CompletedLabs != 0 {
		t.Errorf("a failed module must not count as completed, got %d", rows[0].CompletedLabs)
	}

	if rows[0].TotalScore != 3 {
		t.Errorf("want the attempt's score to still be counted, got %d", rows[0].TotalScore)
	}
}

// TestMyEnrollments_NoProgress verifies a fresh enrollment reports zero
// rather than being dropped from the listing.
func TestMyEnrollments_NoProgress(t *testing.T) { //nolint:paralleltest // shares one database, truncated between tests
	gdb := newTestDB(t)
	repo := repository.NewGormEnrollmentRepository(gdb)
	userID := newLearner(t, gdb)

	enrol(t, gdb, userID, "fresh-course")

	rows, err := repo.MyEnrollments(t.Context(), userID)
	if err != nil {
		t.Fatalf("MyEnrollments: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("want the enrollment listed, got %d rows", len(rows))
	}

	if rows[0].CompletedLabs != 0 || rows[0].TotalScore != 0 {
		t.Errorf("want zeroed progress, got completed=%d score=%d",
			rows[0].CompletedLabs, rows[0].TotalScore)
	}
}
