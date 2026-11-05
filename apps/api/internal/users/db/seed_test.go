package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/genesary/pupitre/apps/api/internal/users/db"
	"github.com/genesary/pupitre/apps/api/internal/users/fake"
	"github.com/genesary/pupitre/apps/api/internal/users/models"
)

// adminEmail is the email of the account SeedAdmin manages.
const adminEmail = "admin@pupitre.local"

// seededAdmin returns a user repository holding the admin account with the
// password current.
func seededAdmin(t *testing.T, current string) (*fake.UserRepository, uuid.UUID) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(current), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	stored := string(hash)
	id := uuid.New()

	return fake.NewUserRepository(models.User{
		ID: id, Username: "admin", Email: adminEmail, Role: "admin", IsActive: true, PasswordHash: &stored,
	}), id
}

// adminSessionsRevoked reports whether the admin's sessions were revoked.
func adminSessionsRevoked(t *testing.T, users *fake.UserRepository, id uuid.UUID) bool {
	t.Helper()

	state, err := users.SessionState(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	return state.SessionsValidAfter != nil
}

// TestSeedAdmin_NewPasswordEndsSessions verifies that rotating the admin
// password ends the sessions opened with the old one, as a password change
// through the API does.
func TestSeedAdmin_NewPasswordEndsSessions(t *testing.T) {
	t.Parallel()

	users, id := seededAdmin(t, "old-password")

	err := db.SeedAdmin(context.Background(), users, "new-password")
	if err != nil {
		t.Fatal(err)
	}

	if !adminSessionsRevoked(t, users, id) {
		t.Error("a new password must end the admin's sessions")
	}
}

// TestSeedAdmin_NewHashEndsSessions is the same rotation with a pre-computed
// bcrypt hash in ADMIN_PASSWORD.
func TestSeedAdmin_NewHashEndsSessions(t *testing.T) {
	t.Parallel()

	users, id := seededAdmin(t, "old-password")

	next, err := bcrypt.GenerateFromPassword([]byte("new-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	err = db.SeedAdmin(context.Background(), users, string(next))
	if err != nil {
		t.Fatal(err)
	}

	if !adminSessionsRevoked(t, users, id) {
		t.Error("a new hash must end the admin's sessions")
	}
}

// TestSeedAdmin_SamePasswordKeepsSessions verifies that a restart, which seeds
// the same password again, leaves the admin signed in.
func TestSeedAdmin_SamePasswordKeepsSessions(t *testing.T) {
	t.Parallel()

	users, id := seededAdmin(t, "same-password")

	err := db.SeedAdmin(context.Background(), users, "same-password")
	if err != nil {
		t.Fatal(err)
	}

	if adminSessionsRevoked(t, users, id) {
		t.Error("a restart with the same password must keep the admin's sessions")
	}
}

// TestSeedAdmin_SameHashKeepsSessions is the same restart with a pre-computed
// hash.
func TestSeedAdmin_SameHashKeepsSessions(t *testing.T) {
	t.Parallel()

	users, id := seededAdmin(t, "same-password")

	admin, err := users.FindByEmail(context.Background(), adminEmail)
	if err != nil {
		t.Fatal(err)
	}

	err = db.SeedAdmin(context.Background(), users, *admin.PasswordHash)
	if err != nil {
		t.Fatal(err)
	}

	if adminSessionsRevoked(t, users, id) {
		t.Error("a restart with the same hash must keep the admin's sessions")
	}
}

// TestSeedAdmin_NoAdminYet verifies that the first seed, with no account to
// revoke, creates the admin.
func TestSeedAdmin_NoAdminYet(t *testing.T) {
	t.Parallel()

	users := fake.NewUserRepository()

	err := db.SeedAdmin(context.Background(), users, "first-password")
	if err != nil {
		t.Fatal(err)
	}

	_, err = users.FindByEmail(context.Background(), adminEmail)
	if err != nil {
		t.Errorf("admin not created: %v", err)
	}
}

// TestSeedAdmin_FailedRotationIsRetried verifies that a rotation that fails
// leaves the old password in place, so the next start tries it again and ends
// the old sessions: a password stored apart from its revocation would look
// current then, and they would stay open.
func TestSeedAdmin_FailedRotationIsRetried(t *testing.T) {
	t.Parallel()

	users, id := seededAdmin(t, "old-password")
	users.ChangePasswordErr = errors.New("database down")

	err := db.SeedAdmin(context.Background(), users, "new-password")
	if err == nil {
		t.Fatal("SeedAdmin succeeded while the password could not be stored")
	}

	if adminSessionsRevoked(t, users, id) {
		t.Error("sessions revoked by a rotation that failed")
	}

	users.ChangePasswordErr = nil

	err = db.SeedAdmin(context.Background(), users, "new-password")
	if err != nil {
		t.Fatal(err)
	}

	if !adminSessionsRevoked(t, users, id) {
		t.Error("the retried rotation left the old sessions open")
	}
}

// TestSeedAdmin_RenamedAdmin verifies that an admin whose username was changed
// is found by email: inserting a second "admin" would hit the unique email and
// stop the API from starting.
func TestSeedAdmin_RenamedAdmin(t *testing.T) {
	t.Parallel()

	hash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	stored := string(hash)
	id := uuid.New()
	users := fake.NewUserRepository(models.User{
		ID: id, Username: "root", Email: adminEmail, Role: "admin", IsActive: true, PasswordHash: &stored,
	})

	err = db.SeedAdmin(context.Background(), users, "new-password")
	if err != nil {
		t.Fatal(err)
	}

	admin, err := users.FindByEmail(context.Background(), adminEmail)
	if err != nil {
		t.Fatal(err)
	}

	if admin.ID != id || admin.Username != "root" {
		t.Errorf("admin = %s %q, want the existing account", admin.ID, admin.Username)
	}

	if bcrypt.CompareHashAndPassword([]byte(*admin.PasswordHash), []byte("new-password")) != nil {
		t.Error("the new password was not stored")
	}
}
