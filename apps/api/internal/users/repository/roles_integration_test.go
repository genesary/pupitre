//go:build integration

// roles_integration_test.go covers the manager role in group mappings
// (docs/specs/190-backend-defects.md B20) against a live PostgreSQL
// instance. Behind the `integration` build tag; run with TEST_DATABASE_URL
// set.

package repository_test

import (
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// TestGroupRepository_ManagerMapping saves a group mapped to manager, which
// the database refused before the fix, and checks that a student in that
// group becomes a manager at sign-in.
func TestGroupRepository_ManagerMapping(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormGroupRepository(gdb)
	ctx := t.Context()

	err := repo.UpsertMapping(ctx, "b20-managers", "manager")
	if err != nil {
		t.Fatalf("UpsertMapping manager: %v", err)
	}

	student := mkUser(t, gdb)

	role, err := repo.SyncGroupsAndDeriveRole(ctx, student, []string{"b20-managers"}, "oidc")
	if err != nil {
		t.Fatalf("SyncGroupsAndDeriveRole: %v", err)
	}

	if role != "manager" {
		t.Errorf("student in a manager group: role = %q, want manager", role)
	}

	// An unknown role is still refused by the database.
	err = repo.UpsertMapping(ctx, "b20-bad", "superuser")
	if err == nil {
		t.Error("UpsertMapping accepted an unknown role")
	}
}
