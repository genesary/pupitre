package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// ldapRouter builds a router whose settings enable LDAP against serverURL.
func ldapRouter(serverURL, bindDN string) http.Handler {
	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: "ldap_enabled", Value: "true"},
		models.PlatformSetting{Key: "ldap_server_url", Value: serverURL},
		models.PlatformSetting{Key: "ldap_bind_dn", Value: bindDN},
		models.PlatformSetting{Key: "ldap_bind_password", Value: "svc-pass"},
		models.PlatformSetting{Key: "ldap_user_base_dn", Value: "ou=people,dc=example,dc=com"},
		models.PlatformSetting{Key: "ldap_user_filter", Value: "(mail=%s)"},
		models.PlatformSetting{Key: "ldap_group_base_dn", Value: "ou=groups,dc=example,dc=com"},
	)

	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
	}
	s := &State{Repos: repos, Config: cfg}

	return BuildRouter(s, cfg, false)
}

// TestLDAPLogin_FullFlow authenticates against the mock directory and gets an
// app JWT back, covering dialLDAPServer, bindLDAPUser and ldapUserGroups.
func TestLDAPLogin_FullFlow(t *testing.T) {
	t.Parallel()

	m := newMockLDAP(t)
	r := ldapRouter(m.url(), m.bindDN)

	rec := htDo(t, r, http.MethodPost, "/api/auth/ldap/login",
		`{"email":"jdoe@example.com","password":"s3cret"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Token string `json:"token"`
		User  struct {
			Email string `json:"email"`
		} `json:"user"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Token == "" || resp.User.Email != "jdoe@example.com" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

// TestLDAPLogin_WrongPassword is rejected when the user bind fails.
func TestLDAPLogin_WrongPassword(t *testing.T) {
	t.Parallel()

	m := newMockLDAP(t)
	r := ldapRouter(m.url(), m.bindDN)

	rec := htDo(t, r, http.MethodPost, "/api/auth/ldap/login",
		`{"email":"jdoe@example.com","password":"wrong"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestLDAPLogin_NoServiceBind works without a configured bind DN (anonymous
// search path).
func TestLDAPLogin_NoServiceBind(t *testing.T) {
	t.Parallel()

	m := newMockLDAP(t)
	r := ldapRouter(m.url(), "") // no ldap_bind_dn

	rec := htDo(t, r, http.MethodPost, "/api/auth/ldap/login",
		`{"email":"jdoe@example.com","password":"s3cret"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestLDAPLogin_BadJSON rejects a malformed body.
func TestLDAPLogin_BadJSON(t *testing.T) {
	t.Parallel()

	r := ldapRouter("ldap://127.0.0.1:1", "")

	rec := htDo(t, r, http.MethodPost, "/api/auth/ldap/login", "{", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestLDAPLogin_MissingFields rejects an empty email or password.
func TestLDAPLogin_MissingFields(t *testing.T) {
	t.Parallel()

	r := ldapRouter("ldap://127.0.0.1:1", "")

	rec := htDo(t, r, http.MethodPost, "/api/auth/ldap/login", `{"email":"a@b.com"}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestLDAPLogin_Disabled returns 400 when LDAP is not enabled.
func TestLDAPLogin_Disabled(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	cfg := &config.Config{JWTSecret: htSecret, JWTExpiryH: htExpiry}
	s := &State{Repos: repos, Config: cfg}
	r := BuildRouter(s, cfg, false)

	rec := htDo(t, r, http.MethodPost, "/api/auth/ldap/login",
		`{"email":"a@b.com","password":"x"}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestLDAPLogin_DialFailure returns 500 when the directory is unreachable.
func TestLDAPLogin_DialFailure(t *testing.T) {
	t.Parallel()

	r := ldapRouter("ldap://127.0.0.1:1", "cn=svc,dc=example,dc=com")

	rec := htDo(t, r, http.MethodPost, "/api/auth/ldap/login",
		`{"email":"a@b.com","password":"x"}`, "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}
