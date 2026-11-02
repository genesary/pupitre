//go:build integration

// user_integration_test.go covers the UserRepository methods not exercised by
// integration_test.go: password hashes, SSO identity linking, the admin
// projections, search, leaderboard and the admin/mock bootstrap helpers.
// Behind the `integration` build tag; run with TEST_DATABASE_URL set.

package repository_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// TestUserRepository_PasswordHash covers the get/update password-hash pair
// and the active-by-email lookup.
func TestUserRepository_PasswordHash(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormUserRepository(gdb)
	ctx := t.Context()

	id := uuid.New()
	hash := "bcrypt-hash"

	err := repo.Create(ctx, &models.User{
		ID: id, Username: "carol", Email: "carol@t.test", Role: "student",
		AuthProvider: "local", PasswordHash: &hash, IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetPasswordHash(ctx, id)
	if err != nil || got == nil || *got != hash {
		t.Fatalf("GetPasswordHash = %v, %v", got, err)
	}

	err = repo.UpdatePasswordHash(ctx, id, "new-hash")
	if err != nil {
		t.Fatalf("UpdatePasswordHash: %v", err)
	}

	got, _ = repo.GetPasswordHash(ctx, id)
	if got == nil || *got != "new-hash" {
		t.Errorf("hash not updated: %v", got)
	}

	active, err := repo.FindByEmailActive(ctx, "carol@t.test")
	if err != nil || active.ID != id {
		t.Fatalf("FindByEmailActive = %+v, %v", active, err)
	}

	taken, err := repo.ExistsByUsername(ctx, "carol")
	if err != nil || !taken {
		t.Fatalf("ExistsByUsername = %v, %v", taken, err)
	}
}

// TestUserRepository_SSOLinking covers the OIDC/LDAP identity link and
// profile-refresh flow.
func TestUserRepository_SSOLinking(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormUserRepository(gdb)
	ctx := t.Context()

	id := uuid.New()

	err := repo.Create(ctx, &models.User{
		ID: id, Username: "dave", Email: "dave@t.test", Role: "student",
		AuthProvider: "local", IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	byEmail, err := repo.FindByEmail(ctx, "dave@t.test")
	if err != nil || byEmail.ID != id {
		t.Fatalf("FindByEmail = %+v, %v", byEmail, err)
	}

	avatar := "https://cdn/dave.png"
	bio := "hi"

	linked, err := repo.LinkProviderIdentity(ctx, id, "oidc", "oidc-123", &avatar, &bio)
	if err != nil || linked.ProviderUserID == nil || *linked.ProviderUserID != "oidc-123" {
		t.Fatalf("LinkProviderIdentity = %+v, %v", linked, err)
	}

	found, err := repo.FindByProviderIdentity(ctx, "oidc", "oidc-123")
	if err != nil || found.ID != id {
		t.Fatalf("FindByProviderIdentity = %+v, %v", found, err)
	}

	// RefreshSSOProfile keeps a non-blank bio (bio-if-blank semantics) and
	// COALESCEs the avatar, so a nil avatar leaves the linked one in place.
	newBio := "should-not-overwrite"

	refreshed, err := repo.RefreshSSOProfile(ctx, id, nil, &newBio)
	if err != nil {
		t.Fatalf("RefreshSSOProfile: %v", err)
	}

	if refreshed.Bio == nil || *refreshed.Bio != "hi" {
		t.Errorf("bio should be preserved as %q, got %v", "hi", refreshed.Bio)
	}

	if refreshed.AvatarURL == nil || *refreshed.AvatarURL != avatar {
		t.Errorf("avatar should be preserved as %q, got %v", avatar, refreshed.AvatarURL)
	}

	// UpdateSSORole overwrites the stored role (group-admin sync is
	// authoritative) and returns the updated row.
	promoted, err := repo.UpdateSSORole(ctx, id, "admin")
	if err != nil || promoted.Role != "admin" {
		t.Fatalf("UpdateSSORole = %+v, %v", promoted, err)
	}

	reloaded, err := repo.FindByEmail(ctx, "dave@t.test")
	if err != nil || reloaded.Role != "admin" {
		t.Fatalf("role not persisted: %+v, %v", reloaded, err)
	}

	_, err = repo.UpdateSSORole(ctx, uuid.New(), "admin")
	if !errors.Is(err, repository.ErrUserNotFound) {
		t.Errorf("UpdateSSORole unknown id = %v, want ErrUserNotFound", err)
	}
}

// TestUserRepository_AdminProjections covers the admin list/detail/update
// queries and the provider breakdown.
func TestUserRepository_AdminProjections(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormUserRepository(gdb)
	ctx := t.Context()

	local := uuid.New()
	oidc := uuid.New()
	providerID := "p-1"

	err := repo.Create(ctx, &models.User{
		ID: local, Username: "erin", Email: "erin@t.test", Role: "student",
		AuthProvider: "local", IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create local: %v", err)
	}

	err = repo.Create(ctx, &models.User{
		ID: oidc, Username: "frank", Email: "frank@t.test", Role: "manager",
		AuthProvider: "oidc", ProviderUserID: &providerID, IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create oidc: %v", err)
	}

	providers, err := repo.ListAuthProviders(ctx)
	if err != nil || len(providers) != 2 {
		t.Fatalf("ListAuthProviders = %+v, %v", providers, err)
	}

	all, err := repo.ListForAdmin(ctx, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("ListForAdmin(all) = %d, %v", len(all), err)
	}

	onlyOIDC, err := repo.ListForAdmin(ctx, "oidc")
	if err != nil || len(onlyOIDC) != 1 {
		t.Fatalf("ListForAdmin(oidc) = %d, %v", len(onlyOIDC), err)
	}

	detail, err := repo.GetForAdmin(ctx, oidc)
	if err != nil || detail == nil {
		t.Fatalf("GetForAdmin = %+v, %v", detail, err)
	}

	newRole := "admin"
	inactive := false

	updated, err := repo.UpdateAdminFields(ctx, local, nil, nil, nil, &inactive, &newRole)
	if err != nil || updated.Role != "admin" || updated.IsActive {
		t.Fatalf("UpdateAdminFields = %+v, %v", updated, err)
	}

	admins, err := repo.CountByRole(ctx, "admin")
	if err != nil || admins != 1 {
		t.Fatalf("CountByRole(admin) = %d, %v", admins, err)
	}
}

// TestUserRepository_SearchAndLeaderboard covers the two cross-aggregate
// projection queries.
func TestUserRepository_SearchAndLeaderboard(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormUserRepository(gdb)
	ctx := t.Context()

	for _, name := range []string{"searchable-alice", "searchable-bob", "unrelated"} {
		err := repo.Create(ctx, &models.User{
			ID: uuid.New(), Username: name, Email: name + "@t.test",
			Role: "student", AuthProvider: "local", IsActive: true,
		})
		if err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}

	hits, err := repo.Search(ctx, "searchable")
	if err != nil || len(hits) != 2 {
		t.Fatalf("Search(searchable) = %d, %v", len(hits), err)
	}

	board, err := repo.Leaderboard(ctx)
	if err != nil {
		t.Fatalf("Leaderboard: %v", err)
	}

	if len(board) != 3 {
		t.Errorf("Leaderboard rows = %d, want 3", len(board))
	}
}

// TestUserRepository_AdminAndMockBootstrap covers the idempotent admin
// bootstrap helpers and the mock-student upsert.
func TestUserRepository_AdminAndMockBootstrap(t *testing.T) { //nolint:paralleltest // shared db
	gdb := crudDB(t)
	repo := repository.NewGormUserRepository(gdb)
	ctx := t.Context()

	err := repo.CreateAdminIfAbsent(ctx, "root", "root@t.test", "h1")
	if err != nil {
		t.Fatalf("CreateAdminIfAbsent: %v", err)
	}

	err = repo.CreateAdminIfAbsent(ctx, "root", "root@t.test", "h1") // no-op
	if err != nil {
		t.Fatalf("CreateAdminIfAbsent again: %v", err)
	}

	count, err := repo.CountByRole(ctx, "admin")
	if err != nil || count != 1 {
		t.Fatalf("CountByRole(admin) = %d, %v", count, err)
	}

	err = repo.UpsertAdminPassword(ctx, "root", "root@t.test", "h2")
	if err != nil {
		t.Fatalf("UpsertAdminPassword: %v", err)
	}

	sid, err := repo.UpsertMockStudent(ctx, "mock1", "mock1@t.test", "h3")
	if err != nil || sid == uuid.Nil {
		t.Fatalf("UpsertMockStudent = %v, %v", sid, err)
	}

	sid2, err := repo.UpsertMockStudent(ctx, "mock1", "mock1@t.test", "h4") // idempotent
	if err != nil || sid2 != sid {
		t.Fatalf("UpsertMockStudent second = %v (want %v), %v", sid2, sid, err)
	}
}
