//go:build integration

// misc_integration_test.go mops up the repository methods not covered by the
// other integration files: group create-and-join, recursive membership,
// login-time enrollment sync, config-driven patterns, ReadSetting and the
// group-scoped admin user list. Behind the `integration` build tag; run with
// TEST_DATABASE_URL set.

package repository_test

import (
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// TestGroupRepository_CreateAndJoin creates a group and joins the owner in
// one call, and exercises the recursive-membership read.
func TestGroupRepository_CreateAndJoin(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormGroupRepository(gdb)
	ctx := t.Context()

	owner := mkUser(t, gdb)

	gid, err := repo.CreateAndJoin(ctx, "founders", owner)
	if err != nil {
		t.Fatalf("CreateAndJoin: %v", err)
	}

	ids, err := repo.ListMemberIDs(ctx, gid)
	if err != nil || len(ids) != 1 || ids[0] != owner {
		t.Fatalf("owner not joined: %v, %v", ids, err)
	}

	child, err := repo.CreateSubgroup(ctx, "founders-sub", gid)
	if err != nil {
		t.Fatalf("CreateSubgroup: %v", err)
	}

	sub := mkUser(t, gdb)

	err = repo.AddMember(ctx, child, sub)
	if err != nil {
		t.Fatalf("AddMember(child): %v", err)
	}

	rec, err := repo.ListMembersRecursive(ctx, gid)
	if err != nil {
		t.Fatalf("ListMembersRecursive: %v", err)
	}

	if len(rec) != 2 {
		t.Errorf("recursive membership = %d, want 2 (owner + subgroup member)", len(rec))
	}
}

// TestGroupRepository_LoginSyncHelpers covers AddToDefault and
// SyncEnrollments, the two login-time fan-out helpers.
func TestGroupRepository_LoginSyncHelpers(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormGroupRepository(gdb)
	ctx := t.Context()

	uid := mkUser(t, gdb)

	err := repo.AddToDefault(ctx, uid)
	if err != nil {
		t.Fatalf("AddToDefault: %v", err)
	}

	groups, err := repo.GetGroupsByUserID(ctx, uid)
	if err != nil || len(groups) == 0 {
		t.Fatalf("AddToDefault did not add the user to any group: %v, %v", groups, err)
	}

	// SyncEnrollments is a no-op when the user's groups have no course
	// enrollments, but it must not error.
	err = repo.SyncEnrollments(ctx, uid)
	if err != nil {
		t.Fatalf("SyncEnrollments: %v", err)
	}
}

// TestPatternRepository_ConfigAndScope covers UpsertFromConfig, the
// course-scoped list and the id+scope delete.
func TestPatternRepository_ConfigAndScope(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormPatternRepository(gdb)
	ctx := t.Context()

	err := repo.UpsertFromConfig(ctx, "note", "Note", "d", "text", "<div/>", "", "", "global")
	if err != nil {
		t.Fatalf("UpsertFromConfig: %v", err)
	}

	err = repo.UpsertFromConfig(ctx, "note", "Note v2", "d2", "text", "<p/>", "", "", "global") // update path
	if err != nil {
		t.Fatalf("UpsertFromConfig update: %v", err)
	}

	author := mkUser(t, gdb)

	scoped, err := repo.Create(ctx, "cbox", "Callout", "d", "text", "<aside/>", "", "", "course:linux-intro", author)
	if err != nil {
		t.Fatalf("Create(scoped): %v", err)
	}

	forCourse, err := repo.ListForCourse(ctx, "linux-intro")
	if err != nil {
		t.Fatalf("ListForCourse: %v", err)
	}

	if len(forCourse) == 0 {
		t.Error("ListForCourse returned nothing for a course with a scoped pattern")
	}

	removed, err := repo.DeleteByIDAndScope(ctx, scoped.ID, "course:linux-intro")
	if err != nil || !removed {
		t.Fatalf("DeleteByIDAndScope = %v, %v", removed, err)
	}

	// Wrong scope must not delete.
	other, err := repo.Create(ctx, "cbox2", "C2", "d", "text", "<aside/>", "", "", "global", author)
	if err != nil {
		t.Fatalf("Create(other): %v", err)
	}

	gone, err := repo.DeleteByIDAndScope(ctx, other.ID, "course:mismatch")
	if err != nil {
		t.Fatalf("DeleteByIDAndScope(mismatch): %v", err)
	}

	if gone {
		t.Error("DeleteByIDAndScope removed a pattern despite a scope mismatch")
	}
}

// TestReadSetting returns the stored value or the fallback.
func TestReadSetting(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormSettingRepository(gdb)
	ctx := t.Context()

	if got := repository.ReadSetting(ctx, repo, "missing_key", "fallback"); got != "fallback" {
		t.Errorf("ReadSetting(missing) = %q, want fallback", got)
	}

	err := repo.Upsert(ctx, "present_key", "stored")
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if got := repository.ReadSetting(ctx, repo, "present_key", "fallback"); got != "stored" {
		t.Errorf("ReadSetting(present) = %q, want stored", got)
	}
}

// TestUserRepository_ListByGroupIDs returns the admin rows for members of
// the given groups.
func TestUserRepository_ListByGroupIDs(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	groups := repository.NewGormGroupRepository(gdb)
	users := repository.NewGormUserRepository(gdb)
	ctx := t.Context()

	gid, err := groups.Create(ctx, "cohort-x")
	if err != nil {
		t.Fatalf("Create group: %v", err)
	}

	member := mkUser(t, gdb)
	mkUser(t, gdb) // a non-member

	err = groups.AddMember(ctx, gid, member)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	rows, err := users.ListByGroupIDs(ctx, []string{gid})
	if err != nil {
		t.Fatalf("ListByGroupIDs: %v", err)
	}

	if len(rows) != 1 || rows[0].ID != member {
		t.Errorf("ListByGroupIDs = %+v, want just the member", rows)
	}

	empty, err := users.ListByGroupIDs(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("ListByGroupIDs(nil) = %+v, %v", empty, err)
	}
}

// TestNewGormRepositories_Wiring builds the full bundle and smoke-tests one
// method per member.
func TestNewGormRepositories_Wiring(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repos := repository.NewGormRepositories(gdb)
	ctx := t.Context()

	if repos.Users == nil || repos.Groups == nil || repos.Settings == nil || repos.Export == nil ||
		repos.Enrollments == nil || repos.Badges == nil || repos.XP == nil || repos.Paths == nil {
		t.Fatal("NewGormRepositories left a member nil")
	}

	_, _, err := repos.Settings.Get(ctx, "anything")
	if err != nil {
		t.Errorf("Settings.Get through the bundle: %v", err)
	}

	n, err := repos.Enrollments.CountAll(ctx)
	if err != nil || n != 0 {
		t.Errorf("Enrollments.CountAll through the bundle = %d, %v", n, err)
	}
}
