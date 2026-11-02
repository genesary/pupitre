//go:build integration

// group_integration_test.go covers the GroupRepository — admin CRUD, the
// hierarchy queries, member management, mappings and the manager scope
// queries — against a live PostgreSQL instance. Behind the `integration`
// build tag; run with TEST_DATABASE_URL set.

package repository_test

import (
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// TestGroupRepository_AdminCRUD covers create, list, rename and delete for
// local groups plus the role-mapping table.
func TestGroupRepository_AdminCRUD(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormGroupRepository(gdb)
	ctx := t.Context()

	id, err := repo.Create(ctx, "team-rh")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	groups, err := repo.List(ctx)
	if err != nil || len(groups) != 1 || groups[0].Name != "team-rh" {
		t.Fatalf("List = %+v, %v", groups, err)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil || got.Name != "team-rh" {
		t.Fatalf("GetByID = %+v, %v", got, err)
	}

	err = repo.UpdateGroup(ctx, id, "team-hr")
	if err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}

	got, _ = repo.GetByID(ctx, id)
	if got.Name != "team-hr" {
		t.Errorf("rename not applied: %q", got.Name)
	}

	err = repo.UpsertMapping(ctx, "team-hr", "student")
	if err != nil {
		t.Fatalf("UpsertMapping: %v", err)
	}

	err = repo.UpsertMapping(ctx, "team-hr", "admin") // update path
	if err != nil {
		t.Fatalf("UpsertMapping update: %v", err)
	}

	maps, err := repo.ListMappings(ctx)
	if err != nil || len(maps) != 1 {
		t.Fatalf("ListMappings = %+v, %v", maps, err)
	}

	removed, err := repo.DeleteMapping(ctx, "team-hr")
	if err != nil || !removed {
		t.Fatalf("DeleteMapping = %v, %v", removed, err)
	}

	gone, err := repo.Delete(ctx, id)
	if err != nil || !gone {
		t.Fatalf("Delete = %v, %v", gone, err)
	}
}

// TestGroupRepository_Hierarchy covers subgroup creation and the ancestor /
// descendant / children queries.
func TestGroupRepository_Hierarchy(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormGroupRepository(gdb)
	ctx := t.Context()

	rootID, err := repo.Create(ctx, "root")
	if err != nil {
		t.Fatalf("Create root: %v", err)
	}

	childID, err := repo.CreateSubgroup(ctx, "child", rootID)
	if err != nil {
		t.Fatalf("CreateSubgroup: %v", err)
	}

	grandID, err := repo.CreateSubgroup(ctx, "grandchild", childID)
	if err != nil {
		t.Fatalf("CreateSubgroup grand: %v", err)
	}

	subs, err := repo.GetSubgroups(ctx, rootID)
	if err != nil || len(subs) != 1 || subs[0].ID != childID {
		t.Fatalf("GetSubgroups(root) = %+v, %v", subs, err)
	}

	hasChildren, err := repo.HasChildren(ctx, rootID)
	if err != nil || !hasChildren {
		t.Fatalf("HasChildren(root) = %v, %v", hasChildren, err)
	}

	ancestors, err := repo.GetAncestors(ctx, grandID)
	if err != nil || len(ancestors) != 2 {
		t.Fatalf("GetAncestors(grand) = %+v, %v", ancestors, err)
	}

	descendants, err := repo.GetDescendants(ctx, rootID)
	if err != nil || len(descendants) != 2 {
		t.Fatalf("GetDescendants(root) = %+v, %v", descendants, err)
	}

	// Re-parent the grandchild directly under root.
	err = repo.MoveGroup(ctx, grandID, &rootID)
	if err != nil {
		t.Fatalf("MoveGroup: %v", err)
	}

	subs, _ = repo.GetSubgroups(ctx, rootID)
	if len(subs) != 2 {
		t.Errorf("after MoveGroup root should have 2 direct children, got %d", len(subs))
	}
}

// TestGroupRepository_Members covers member add/remove and the scope queries
// managers rely on.
func TestGroupRepository_Members(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormGroupRepository(gdb)
	ctx := t.Context()

	gid, err := repo.Create(ctx, "team")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	uid := mkUser(t, gdb)

	err = repo.AddMember(ctx, gid, uid)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	err = repo.AddMember(ctx, gid, uid) // idempotent
	if err != nil {
		t.Fatalf("AddMember dup: %v", err)
	}

	hasMembers, err := repo.HasMembers(ctx, gid)
	if err != nil || !hasMembers {
		t.Fatalf("HasMembers = %v, %v", hasMembers, err)
	}

	ids, err := repo.ListMemberIDs(ctx, gid)
	if err != nil || len(ids) != 1 || ids[0] != uid {
		t.Fatalf("ListMemberIDs = %v, %v", ids, err)
	}

	members, err := repo.ListMembers(ctx, gid)
	if err != nil || len(members) != 1 {
		t.Fatalf("ListMembers = %d, %v", len(members), err)
	}

	byUser, err := repo.GetGroupsByUserID(ctx, uid)
	if err != nil || len(byUser) != 1 || byUser[0].ID != gid {
		t.Fatalf("GetGroupsByUserID = %+v, %v", byUser, err)
	}

	inAny, err := repo.UserInAnyGroup(ctx, uid, []string{gid, "other"})
	if err != nil || !inAny {
		t.Fatalf("UserInAnyGroup = %v, %v", inAny, err)
	}

	notInAny, err := repo.UserInAnyGroup(ctx, uid, []string{"other"})
	if err != nil || notInAny {
		t.Fatalf("UserInAnyGroup(other) = %v, %v", notInAny, err)
	}

	err = repo.RemoveMember(ctx, gid, uid)
	if err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	hasMembers, _ = repo.HasMembers(ctx, gid)
	if hasMembers {
		t.Error("HasMembers should be false after RemoveMember")
	}
}

// TestGroupRepository_CourseEnrollmentCascade covers the group→course
// enrollment fan-out used by the admin cascade.
func TestGroupRepository_CourseEnrollmentCascade(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormGroupRepository(gdb)
	ctx := t.Context()

	gid, err := repo.Create(ctx, "cohort")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	u1 := mkUser(t, gdb)
	u2 := mkUser(t, gdb)

	for _, u := range []string{u1, u2} {
		addErr := repo.AddMember(ctx, gid, u)
		if addErr != nil {
			t.Fatalf("AddMember: %v", addErr)
		}
	}

	n, err := repo.EnrollCourse(ctx, gid, "linux-intro")
	if err != nil {
		t.Fatalf("EnrollCourse: %v", err)
	}

	if n < 1 {
		t.Errorf("EnrollCourse affected %d rows, want >= 1", n)
	}

	rows, err := repo.ListCourseEnrollments(ctx, "linux-intro")
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListCourseEnrollments = %+v, %v", rows, err)
	}

	courses, err := repo.GetGroupCourses(ctx, gid)
	if err != nil || len(courses) != 1 || courses[0] != "linux-intro" {
		t.Fatalf("GetGroupCourses = %v, %v", courses, err)
	}

	// SyncEnrollments fans the group's course enrollments out to each member.
	err = repo.SyncEnrollments(ctx, u1)
	if err != nil {
		t.Fatalf("SyncEnrollments: %v", err)
	}

	var enrolled int64

	err = gdb.WithContext(ctx).Table("enrollments").
		Where("userid = ?::uuid AND courseslug = ?", u1, "linux-intro").Count(&enrolled).Error
	if err != nil || enrolled != 1 {
		t.Fatalf("SyncEnrollments did not create the enrollment row: count=%d err=%v", enrolled, err)
	}

	err = repo.SyncEnrollments(ctx, u1) // idempotent (ON CONFLICT DO NOTHING)
	if err != nil {
		t.Fatalf("SyncEnrollments (second run): %v", err)
	}

	err = repo.UnenrollCourse(ctx, gid, "linux-intro")
	if err != nil {
		t.Fatalf("UnenrollCourse: %v", err)
	}

	courses, _ = repo.GetGroupCourses(ctx, gid)
	if len(courses) != 0 {
		t.Errorf("GetGroupCourses after unenroll = %v", courses)
	}
}

// TestGroupRepository_SyncGroupsAndDeriveRole covers the login-time group
// sync and its role derivation from the mapping table.
func TestGroupRepository_SyncGroupsAndDeriveRole(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormGroupRepository(gdb)
	ctx := t.Context()

	uid := mkUser(t, gdb)

	err := repo.UpsertMapping(ctx, "staff", "admin")
	if err != nil {
		t.Fatalf("UpsertMapping: %v", err)
	}

	role, err := repo.SyncGroupsAndDeriveRole(ctx, uid, []string{"staff", "everyone"}, "oidc")
	if err != nil {
		t.Fatalf("SyncGroupsAndDeriveRole: %v", err)
	}

	if role != "admin" {
		t.Errorf("derived role = %q, want admin", role)
	}

	groups, err := repo.GetGroupsByUserID(ctx, uid)
	if err != nil || len(groups) != 2 {
		t.Fatalf("GetGroupsByUserID after sync = %+v, %v", groups, err)
	}

	// A second sync with fewer groups replaces the membership set.
	_, err = repo.SyncGroupsAndDeriveRole(ctx, uid, []string{"everyone"}, "oidc")
	if err != nil {
		t.Fatalf("SyncGroupsAndDeriveRole (second): %v", err)
	}

	groups, _ = repo.GetGroupsByUserID(ctx, uid)
	if len(groups) != 1 {
		t.Errorf("after second sync want 1 group, got %d", len(groups))
	}
}
