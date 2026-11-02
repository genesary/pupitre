//go:build integration

// crud_integration_test.go: broad CRUD round-trip coverage for every
// Gorm*Repository, exercised against a live PostgreSQL instance. Behind the
// `"`"`integration`"`"` build tag; run with TEST_DATABASE_URL set (see
// integration_test.go for the docker one-liner).

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

// crudDB connects, migrates and wipes every table these tests touch.
func crudDB(t *testing.T) *gorm.DB {
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

	err = gdb.WithContext(t.Context()).Exec(`TRUNCATE
		users, enrollments, lesson_progress, module_progress, path_enrollments,
		user_xp_events, user_badges, user_skill_levels, session_bookings,
		platform_settings, markdown_patterns, groups, user_groups,
		group_enrollments, group_role_mappings, export_logs
		RESTART IDENTITY CASCADE`).Error
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return gdb
}

// mkUser inserts a user and returns its id string.
func mkUser(t *testing.T, gdb *gorm.DB) string {
	t.Helper()

	id := uuid.New()
	name := "u-" + id.String()[:8]

	err := gdb.WithContext(t.Context()).Create(&models.User{
		ID: id, Username: name, Email: name + "@t.test", Role: "student",
	}).Error
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	return id.String()
}

// TestSettingRepository_CRUD round-trips the platform_settings key/value
// store.
func TestSettingRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormSettingRepository(gdb)
	ctx := t.Context()

	_, ok, err := repo.Get(ctx, "missing")
	if err != nil || ok {
		t.Fatalf("Get(missing) = ok:%v err:%v", ok, err)
	}

	err = repo.Upsert(ctx, "registration_enabled", "false")
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	err = repo.Upsert(ctx, "registration_enabled", "true") // update path
	if err != nil {
		t.Fatalf("Upsert update: %v", err)
	}

	val, ok, err := repo.Get(ctx, "registration_enabled")
	if err != nil || !ok || val != "true" {
		t.Fatalf("Get = %q ok:%v err:%v", val, ok, err)
	}

	all, err := repo.List(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("List = %d rows, err %v", len(all), err)
	}
}

// TestEnrollmentRepository_CRUD round-trips course enrollments and the
// aggregate counts.
func TestEnrollmentRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormEnrollmentRepository(gdb)
	ctx := t.Context()
	uid := mkUser(t, gdb)

	err := repo.Create(ctx, uid, "linux-intro")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = repo.Create(ctx, uid, "linux-intro") // idempotent
	if err != nil {
		t.Fatalf("Create dup: %v", err)
	}

	ok, err := repo.Exists(ctx, uid, "linux-intro")
	if err != nil || !ok {
		t.Fatalf("Exists = %v, %v", ok, err)
	}

	rows, err := repo.MyEnrollments(ctx, uid)
	if err != nil || len(rows) != 1 {
		t.Fatalf("MyEnrollments = %d, %v", len(rows), err)
	}

	total, err := repo.CountAll(ctx)
	if err != nil || total != 1 {
		t.Fatalf("CountAll = %d, %v", total, err)
	}

	distinct, err := repo.CountDistinctCourses(ctx)
	if err != nil || distinct != 1 {
		t.Fatalf("CountDistinctCourses = %d, %v", distinct, err)
	}

	byCourse, err := repo.ListByCourse(ctx, "linux-intro")
	if err != nil || len(byCourse) != 1 {
		t.Fatalf("ListByCourse = %d, %v", len(byCourse), err)
	}

	err = repo.Delete(ctx, uid, "linux-intro")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	ok, _ = repo.Exists(ctx, uid, "linux-intro")
	if ok {
		t.Fatal("Exists after Delete should be false")
	}
}

// TestLessonProgressRepository_CRUD round-trips lesson completion and the
// viewed-slug queries.
func TestLessonProgressRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormLessonProgressRepository(gdb)
	ctx := t.Context()
	uid := mkUser(t, gdb)

	err := repo.MarkComplete(ctx, uid, "linux-intro", "intro")
	if err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}

	err = repo.MarkComplete(ctx, uid, "linux-intro", "intro") // idempotent
	if err != nil {
		t.Fatalf("MarkComplete dup: %v", err)
	}

	slugs, err := repo.ViewedSlugs(ctx, uid, "linux-intro")
	if err != nil || len(slugs) != 1 || slugs[0] != "intro" {
		t.Fatalf("ViewedSlugs = %v, %v", slugs, err)
	}

	n, err := repo.CountViewed(ctx, uid, "linux-intro")
	if err != nil || n != 1 {
		t.Fatalf("CountViewed = %d, %v", n, err)
	}

	keys, err := repo.ViewedKeys(ctx, uid)
	if err != nil || len(keys) != 1 {
		t.Fatalf("ViewedKeys = %v, %v", keys, err)
	}

	done, err := repo.CompletedCourseSlugs(ctx, uid, []string{"linux-intro"})
	if err != nil {
		t.Fatalf("CompletedCourseSlugs: %v", err)
	}

	_ = done // the __complete__ sentinel is written elsewhere; just exercising the query
}

// TestModuleProgressRepository_CRUD round-trips module scores and the
// passed-module queries.
func TestModuleProgressRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormModuleProgressRepository(gdb)
	ctx := t.Context()
	uid := mkUser(t, gdb)

	err := repo.RecordProgress(ctx, uid, "linux-intro", 0, "quiz-1", "quiz", 8, 10, true)
	if err != nil {
		t.Fatalf("RecordProgress: %v", err)
	}

	err = repo.RecordProgress(ctx, uid, "linux-intro", 0, "quiz-1", "quiz", 10, 10, true) // update
	if err != nil {
		t.Fatalf("RecordProgress update: %v", err)
	}

	score, err := repo.TotalScore(ctx, uid, "linux-intro")
	if err != nil || score != 10 {
		t.Fatalf("TotalScore = %d, %v", score, err)
	}

	passed, err := repo.PassedModuleSlugs(ctx, uid, "linux-intro")
	if err != nil || len(passed) != 1 {
		t.Fatalf("PassedModuleSlugs = %v, %v", passed, err)
	}

	list, err := repo.ListByUserCourse(ctx, uid, "linux-intro")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByUserCourse = %d, %v", len(list), err)
	}

	keys, err := repo.PassedKeys(ctx, uid)
	if err != nil || len(keys) != 1 {
		t.Fatalf("PassedKeys = %v, %v", keys, err)
	}
}

// TestPathEnrollmentRepository_CRUD round-trips path enrollments with and
// without member courses.
func TestPathEnrollmentRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormPathEnrollmentRepository(gdb)
	ctx := t.Context()
	uid := mkUser(t, gdb)

	err := repo.Enroll(ctx, uid, "devops-path")
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	err = repo.EnrollWithCourses(ctx, uid, "sec-path", []string{"a", "b"})
	if err != nil {
		t.Fatalf("EnrollWithCourses: %v", err)
	}

	in, err := repo.EnrolledInAny(ctx, uid, []string{"devops-path", "x"})
	if err != nil || !in {
		t.Fatalf("EnrolledInAny = %v, %v", in, err)
	}

	rows, err := repo.MyEnrollments(ctx, uid, nil, 0)
	if err != nil || len(rows) != 2 {
		t.Fatalf("MyEnrollments = %d, %v", len(rows), err)
	}

	users, err := repo.ListBySlug(ctx, "devops-path")
	if err != nil || len(users) != 1 {
		t.Fatalf("ListBySlug = %d, %v", len(users), err)
	}

	err = repo.Unenroll(ctx, uid, "devops-path")
	if err != nil {
		t.Fatalf("Unenroll: %v", err)
	}

	in, _ = repo.EnrolledInAny(ctx, uid, []string{"devops-path"})
	if in {
		t.Fatal("EnrolledInAny after Unenroll should be false")
	}
}

// TestXPRepository_CRUD round-trips XP events and the per-user total and
// history.
func TestXPRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormXPRepository(gdb)
	ctx := t.Context()
	uid := mkUser(t, gdb)

	err := repo.Award(ctx, uid, "course", "linux-intro", 50)
	if err != nil {
		t.Fatalf("Award: %v", err)
	}

	err = repo.Award(ctx, uid, "quiz", "quiz-1", 20)
	if err != nil {
		t.Fatalf("Award 2: %v", err)
	}

	total, err := repo.Total(ctx, uid)
	if err != nil || total != 70 {
		t.Fatalf("Total = %d, %v", total, err)
	}

	hist, err := repo.History(ctx, uid)
	if err != nil || len(hist) != 2 {
		t.Fatalf("History = %d, %v", len(hist), err)
	}
}

// TestBadgeRepository_CRUD round-trips badge awards and the
// leaderboard/count queries.
func TestBadgeRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormBadgeRepository(gdb)
	ctx := t.Context()
	uid := mkUser(t, gdb)

	err := repo.Award(ctx, uid, "linux-intro")
	if err != nil {
		t.Fatalf("Award: %v", err)
	}

	err = repo.Award(ctx, uid, "linux-intro") // idempotent
	if err != nil {
		t.Fatalf("Award dup: %v", err)
	}

	mine, err := repo.MyBadges(ctx, uid)
	if err != nil || len(mine) != 1 {
		t.Fatalf("MyBadges = %d, %v", len(mine), err)
	}

	pub, err := repo.UserBadges(ctx, uid)
	if err != nil || len(pub) != 1 {
		t.Fatalf("UserBadges = %d, %v", len(pub), err)
	}

	n, err := repo.EarnedCount(ctx, "linux-intro")
	if err != nil || n != 1 {
		t.Fatalf("EarnedCount = %d, %v", n, err)
	}

	board, err := repo.Leaderboard(ctx, 10)
	if err != nil || len(board) != 1 {
		t.Fatalf("Leaderboard = %d, %v", len(board), err)
	}
}

// TestSkillLevelRepository_CRUD round-trips single and bulk skill-level
// upserts.
func TestSkillLevelRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormSkillLevelRepository(gdb)
	ctx := t.Context()
	uid := mkUser(t, gdb)

	err := repo.Upsert(ctx, uid, "linux", "beginner", 3)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	err = repo.UpsertAll(ctx, uid,
		map[string]struct{}{"linux": {}, "docker": {}},
		"intermediate",
		map[string]int{"linux": 3, "docker": 2},
	)
	if err != nil {
		t.Fatalf("UpsertAll: %v", err)
	}

	rows, err := repo.ListByUser(ctx, uid)
	if err != nil || len(rows) != 2 {
		t.Fatalf("ListByUser = %d, %v", len(rows), err)
	}

	// A second UpsertAll on the same skills exercises the UPDATE path of
	// upsertSkillLevelTx (completed_courses increments, max-difficulty kept).
	err = repo.UpsertAll(ctx, uid,
		map[string]struct{}{"linux": {}, "docker": {}}, "advanced",
		map[string]int{"linux": 3, "docker": 2})
	if err != nil {
		t.Fatalf("UpsertAll (second): %v", err)
	}

	// Invalid difficulty and empty skill set are both no-ops.
	err = repo.UpsertAll(ctx, uid, map[string]struct{}{"linux": {}}, "not-a-level", nil)
	if err != nil {
		t.Errorf("UpsertAll(bad difficulty) = %v, want nil no-op", err)
	}

	err = repo.UpsertAll(ctx, uid, map[string]struct{}{}, "beginner", nil)
	if err != nil {
		t.Errorf("UpsertAll(no skills) = %v, want nil no-op", err)
	}

	err = repo.Upsert(ctx, uid, "invalid-level", "not-a-level", 0)
	if err != nil {
		t.Errorf("Upsert(bad level) = %v, want nil no-op", err)
	}
}

// TestSessionBookingRepository_CRUD round-trips a session booking through
// attendance and cancellation.
func TestSessionBookingRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormSessionBookingRepository(gdb)
	ctx := t.Context()
	uid := mkUser(t, gdb)

	err := repo.Book(ctx, uid, "linux-intro", "sess-1")
	if err != nil {
		t.Fatalf("Book: %v", err)
	}

	ok, err := repo.ExistsByUser(ctx, uid, "linux-intro", "sess-1")
	if err != nil || !ok {
		t.Fatalf("ExistsByUser = %v, %v", ok, err)
	}

	n, err := repo.CountBySession(ctx, "linux-intro", "sess-1")
	if err != nil || n != 1 {
		t.Fatalf("CountBySession = %d, %v", n, err)
	}

	byUser, err := repo.ListByUser(ctx, uid)
	if err != nil || len(byUser) != 1 {
		t.Fatalf("ListByUser = %d, %v", len(byUser), err)
	}

	parts, err := repo.ListBySession(ctx, "linux-intro", "sess-1")
	if err != nil || len(parts) != 1 {
		t.Fatalf("ListBySession = %d, %v", len(parts), err)
	}

	err = repo.MarkPresent(ctx, uid, "linux-intro", "sess-1", true)
	if err != nil {
		t.Fatalf("MarkPresent: %v", err)
	}

	err = repo.Unbook(ctx, uid, "linux-intro", "sess-1")
	if err != nil {
		t.Fatalf("Unbook: %v", err)
	}

	ok, _ = repo.ExistsByUser(ctx, uid, "linux-intro", "sess-1")
	if ok {
		t.Fatal("ExistsByUser after Unbook should be false")
	}
}

// TestPatternRepository_CRUD round-trips a markdown pattern through update
// and delete.
func TestPatternRepository_CRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormPatternRepository(gdb)
	ctx := t.Context()
	author := mkUser(t, gdb)

	created, err := repo.Create(ctx, "callout", "Callout", "desc", "text", "<div></div>", "", "", "global", author)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil || got.Name != "callout" {
		t.Fatalf("Get = %+v, %v", got, err)
	}

	globals, err := repo.ListGlobal(ctx)
	if err != nil || len(globals) != 1 {
		t.Fatalf("ListGlobal = %d, %v", len(globals), err)
	}

	_, err = repo.UpdateByName(ctx, "callout", "callout2", "Callout 2", "d", "text", "<p></p>", "", "", "global")
	if err != nil {
		t.Fatalf("UpdateByName: %v", err)
	}

	removed, err := repo.DeleteByName(ctx, "callout2")
	if err != nil || !removed {
		t.Fatalf("DeleteByName = %v, %v", removed, err)
	}
}

// TestExportRepository_LogDownload exercises Categories and writes one audit
// row.
func TestExportRepository_LogDownload(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormExportRepository(gdb)
	ctx := t.Context()

	cats := repo.Categories()
	if len(cats) == 0 {
		t.Fatal("Categories returned nothing")
	}

	err := repo.LogDownload(ctx, &models.ExportLog{
		UserID: "u1", UserEmail: "u1@t.test", Category: "users", Fields: `["email"]`, RowCount: 3,
	})
	if err != nil {
		t.Fatalf("LogDownload: %v", err)
	}

	var count int64

	err = gdb.WithContext(ctx).Table("export_logs").Count(&count).Error
	if err != nil || count != 1 {
		t.Fatalf("export_logs count = %d, %v", count, err)
	}
}

// TestUserRepository_CoreFlow covers create, lookup, profile update, count
// and delete for users.
func TestUserRepository_CoreFlow(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormUserRepository(gdb)
	ctx := t.Context()

	id := uuid.New()
	u := &models.User{
		ID: id, Username: "alice", Email: "alice@t.test", Role: "student", AuthProvider: "local",
	}

	err := repo.Create(ctx, u)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	exists, err := repo.ExistsByEmailOrUsername(ctx, "alice@t.test", "someone")
	if err != nil || !exists {
		t.Fatalf("ExistsByEmailOrUsername = %v, %v", exists, err)
	}

	byID, err := repo.FindByID(ctx, id)
	if err != nil || byID.Username != "alice" {
		t.Fatalf("FindByID = %+v, %v", byID, err)
	}

	nameTaken, err := repo.ExistsUsernameExcluding(ctx, "alice", uuid.New())
	if err != nil || !nameTaken {
		t.Fatalf("ExistsUsernameExcluding = %v, %v", nameTaken, err)
	}

	newName := "alice2"
	newBio := "hello there"
	newAvatar := "https://cdn/a2.png"

	updated, err := repo.UpdateProfile(ctx, id, &newName, &newBio, &newAvatar)
	if err != nil || updated.Username != "alice2" {
		t.Fatalf("UpdateProfile = %+v, %v", updated, err)
	}

	if updated.Bio == nil || *updated.Bio != "hello there" || updated.AvatarURL == nil {
		t.Errorf("UpdateProfile did not persist bio/avatar: %+v", updated)
	}

	// A no-op update (all nil) must not error and must leave the row intact.
	same, err := repo.UpdateProfile(ctx, id, nil, nil, nil)
	if err != nil || same.Username != "alice2" {
		t.Errorf("UpdateProfile(nil,nil,nil) = %+v, %v", same, err)
	}

	count, err := repo.CountByRole(ctx, "student")
	if err != nil || count != 1 {
		t.Fatalf("CountByRole = %d, %v", count, err)
	}

	// Admin-field update path (this changes the role, so it runs after the
	// student-count assertion above).
	role := "manager"
	inactive := false

	adm, err := repo.UpdateAdminFields(ctx, id, nil, nil, nil, &inactive, &role)
	if err != nil || adm.Role != "manager" || adm.IsActive {
		t.Fatalf("UpdateAdminFields = %+v, %v", adm, err)
	}

	err = repo.TouchLastLogin(ctx, id)
	if err != nil {
		t.Fatalf("TouchLastLogin: %v", err)
	}

	err = repo.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.FindByID(ctx, id)
	if err == nil {
		t.Fatal("FindByID after Delete should error")
	}
}
