package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
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
