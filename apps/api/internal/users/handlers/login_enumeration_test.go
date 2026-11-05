package handlers

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/genesary/pupitre/apps/api/internal/users/fake"
	"github.com/genesary/pupitre/apps/api/internal/users/models"
)

// TestLogin_DoesNotRevealAccounts is the regression test of defect B14
// (docs/specs/190-backend-defects.md): an unknown email and an SSO account
// get the same answer as a wrong password.
func TestLogin_DoesNotRevealAccounts(t *testing.T) {
	t.Parallel()

	sub := "kc-sub"
	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.New(), Username: "sso-user", Email: "sso@test.com", Role: "student",
		IsActive: true, AuthProvider: oidcProviderKey, ProviderUserID: &sub,
	})
	r := newTestRouterWithRepos(repos)

	unknown := htDo(t, r, http.MethodPost, "/api/auth/login", `{"email":"nobody@test.com","password":"x"}`, "")
	sso := htDo(t, r, http.MethodPost, "/api/auth/login", `{"email":"sso@test.com","password":"x"}`, "")

	if unknown.Code != http.StatusUnauthorized || sso.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 twice, got %d and %d", unknown.Code, sso.Code)
	}

	if unknown.Body.String() != sso.Body.String() {
		t.Errorf("the answers differ, so they tell an SSO account exists:\nunknown: %s\nSSO:     %s",
			unknown.Body.String(), sso.Body.String())
	}
}

// TestSpendPasswordCheckTime_LongPassword verifies a password over bcrypt's
// limit costs the time of a check too: hashing would refuse it at once, while
// comparing it with a stored hash does not, so an unknown email would answer
// faster than a known one.
func TestSpendPasswordCheckTime_LongPassword(t *testing.T) {
	t.Parallel()

	started := time.Now()

	spendPasswordCheckTime(strings.Repeat("x", bcryptMaxPasswordBytes+1))

	if elapsed := time.Since(started); elapsed < 10*time.Millisecond {
		t.Errorf("took %s: the password was not hashed", elapsed)
	}
}
