package fake_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// TestUserRepository_AdminAndMockHelpers covers the seed-oriented fake user
// helpers (CreateAdminIfAbsent / UpsertAdminPassword / UpsertMockStudent)
// that only db.Seed* would otherwise exercise.
func TestUserRepository_AdminAndMockHelpers(t *testing.T) {
	t.Parallel()

	repo := fake.NewUserRepository()
	ctx := t.Context()

	err := repo.CreateAdminIfAbsent(ctx, "root", "root@t.test", "h1")
	if err != nil {
		t.Fatalf("CreateAdminIfAbsent: %v", err)
	}

	err = repo.CreateAdminIfAbsent(ctx, "root", "root@t.test", "h2") // no-op
	if err != nil {
		t.Fatalf("CreateAdminIfAbsent again: %v", err)
	}

	n, err := repo.CountByRole(ctx, "admin")
	if err != nil || n != 1 {
		t.Fatalf("CountByRole(admin) = %d, %v", n, err)
	}

	err = repo.UpsertAdminPassword(ctx, "root", "root@t.test", "h3") // update existing
	if err != nil {
		t.Fatalf("UpsertAdminPassword existing: %v", err)
	}

	err = repo.UpsertAdminPassword(ctx, "root2", "root2@t.test", "h4") // create
	if err != nil {
		t.Fatalf("UpsertAdminPassword new: %v", err)
	}

	n, _ = repo.CountByRole(ctx, "admin")
	if n != 2 {
		t.Errorf("admin count = %d, want 2", n)
	}

	id1, err := repo.UpsertMockStudent(ctx, "mock", "mock@t.test", "h5")
	if err != nil || id1 == uuid.Nil {
		t.Fatalf("UpsertMockStudent = %v, %v", id1, err)
	}

	id2, err := repo.UpsertMockStudent(ctx, "mock-renamed", "mock@t.test", "h6")
	if err != nil || id2 != id1 {
		t.Fatalf("UpsertMockStudent second = %v (want %v), %v", id2, id1, err)
	}
}

// TestGroupRepository_FakeHelpers covers the fake Group helpers not hit by
// the handler tests: GetByID, Enroll cascade and recursive membership.
func TestGroupRepository_FakeHelpers(t *testing.T) {
	t.Parallel()

	gid := uuid.New()
	child := uuid.New()
	memberA := uuid.New()
	memberB := uuid.New()

	repo := fake.NewGroupRepository(
		models.Group{ID: gid, Name: "team", Source: "local"},
		models.Group{ID: child, Name: "team-sub", Source: "local", ParentID: &gid},
	)
	repo.UserGroup = []models.UserGroup{
		{UserID: memberA.String(), GroupID: gid},
		{UserID: memberB.String(), GroupID: child},
	}

	ctx := t.Context()

	got, err := repo.GetByID(ctx, gid.String())
	if err != nil || got.Name != "team" {
		t.Fatalf("GetByID = %+v, %v", got, err)
	}

	_, err = repo.GetByID(ctx, uuid.NewString())
	if err == nil {
		t.Error("GetByID(unknown) should error")
	}

	rec, err := repo.ListMembersRecursive(ctx, gid.String())
	if err != nil {
		t.Fatalf("ListMembersRecursive: %v", err)
	}

	_ = rec // the fake returns an empty slice by design; the call must not error

	affected, err := repo.EnrollCourse(ctx, gid.String(), "linux-intro")
	if err != nil {
		t.Fatalf("EnrollCourse: %v", err)
	}

	if affected < 1 {
		t.Errorf("EnrollCourse affected %d, want >= 1", affected)
	}

	courses, err := repo.GetGroupCourses(ctx, gid.String())
	if err != nil || len(courses) != 1 || courses[0] != "linux-intro" {
		t.Errorf("GetGroupCourses = %v, %v", courses, err)
	}
}

// TestPathEnrollmentRepository_FakeEnroll covers the plain Enroll path on
// the fake, which the handlers reach only through EnrollWithCourses.
func TestPathEnrollmentRepository_FakeEnroll(t *testing.T) {
	t.Parallel()

	repo := fake.NewPathEnrollmentRepository()
	ctx := t.Context()
	uid := uuid.NewString()

	err := repo.Enroll(ctx, uid, "devops-path")
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	err = repo.Enroll(ctx, uid, "devops-path") // idempotent
	if err != nil {
		t.Fatalf("Enroll again: %v", err)
	}

	in, err := repo.EnrolledInAny(ctx, uid, []string{"devops-path"})
	if err != nil || !in {
		t.Errorf("EnrolledInAny = %v, %v", in, err)
	}
}

// TestSessionBookingRepository_FakeExistsByUser covers the fake
// ExistsByUser predicate.
func TestSessionBookingRepository_FakeExistsByUser(t *testing.T) {
	t.Parallel()

	repo := fake.NewSessionBookingRepository()
	ctx := t.Context()
	uid := uuid.NewString()

	present, err := repo.ExistsByUser(ctx, uid, "linux-intro", "sess-1")
	if err != nil || present {
		t.Fatalf("ExistsByUser before booking = %v, %v", present, err)
	}

	err = repo.Book(ctx, uid, "linux-intro", "sess-1")
	if err != nil {
		t.Fatalf("Book: %v", err)
	}

	present, err = repo.ExistsByUser(ctx, uid, "linux-intro", "sess-1")
	if err != nil || !present {
		t.Errorf("ExistsByUser after booking = %v, %v", present, err)
	}
}
