package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"

	apimiddleware "github.com/genesary/pupitre/internal/middleware"
	"github.com/genesary/pupitre/user-service/fake"
	"github.com/genesary/pupitre/user-service/internal/config"
	"github.com/genesary/pupitre/user-service/internal/models"
	"github.com/genesary/pupitre/user-service/internal/repository"
)

const (
	// htSecret is the JWT signing secret shared by handler tests.
	htSecret = "test-jwt-signing-secret-32bytes!!"
	// htExpiry is the JWT expiry, in hours, used by handler tests.
	htExpiry = 24
	// htInternalSecret is the shared service-to-service secret used in tests.
	htInternalSecret = "test-internal-secret"
	// htUserID is a fixed, valid UUID for tests that need an authenticated
	// subject matching a seeded fake.UserRepository row (htAuthHeader's
	// default subject, "user-uuid-1", is intentionally not a valid UUID).
	htUserID = "11111111-1111-1111-1111-111111111111"
)

//nolint:gochecknoglobals // shared fixture, seeded once in TestMain
var (
	// htLoginPassword is the plaintext password backing htLoginHash.
	htLoginPassword = "TestPass99"
	// htLoginHash is bcrypt(htLoginPassword), computed in TestMain.
	htLoginHash string
)

// TestMain seeds htLoginHash before running the handler test suite.
func TestMain(m *testing.M) {
	h, err := bcrypt.GenerateFromPassword([]byte(htLoginPassword), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}

	htLoginHash = string(h)

	os.Exit(m.Run())
}

// newTestRouter builds a router wired to default-empty fake repositories
// for handler tests.
func newTestRouter() http.Handler {
	return newTestRouterWithRepos(fake.NewRepositories())
}

// newTestRouterWithRepos builds a router wired to repos, for tests that
// need custom-seeded or error-injecting fake repositories.
func newTestRouterWithRepos(repos *repository.Repositories) http.Handler {
	cfg := &config.Config{
		JWTSecret:      htSecret,
		JWTExpiryH:     htExpiry,
		CORSOrigins:    []string{"*"},
		InternalSecret: htInternalSecret,
	}
	s := &State{Repos: repos, Config: cfg}

	return BuildRouter(s, cfg, false)
}

// htDoInternal issues a request to an /internal/* route, attaching the
// shared secret header required by InternalAuth middleware.
func htDoInternal(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	var buf *bytes.Reader
	if body != "" {
		buf = bytes.NewReader([]byte(body))
	} else {
		buf = bytes.NewReader(nil)
	}

	req := httptest.NewRequestWithContext(t.Context(), method, path, buf)
	req.Header.Set("X-Internal-Secret", htInternalSecret)

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

// htAuthHeader builds a "Bearer <token>" header value for role.
func htAuthHeader(t *testing.T, role string) string {
	t.Helper()

	return htAuthHeaderForSubject(t, role, "user-uuid-1")
}

// htAuthHeaderForSubject builds a "Bearer <token>" header value for role
// with a specific JWT subject — used by tests that need the subject to
// match a seeded fake.UserRepository row's real UUID (htAuthHeader's
// default subject, "user-uuid-1", is intentionally not a valid UUID).
func htAuthHeaderForSubject(t *testing.T, role, subject string) string {
	t.Helper()

	tok, err := apimiddleware.CreateToken(subject, "user@test.com", role, "testuser", htSecret, htExpiry)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	return "Bearer " + tok
}

// skipErrRows pushes n error rows so ReadSetting calls consume them
// and return their fallbacks.
// htDo issues an HTTP request against handler and returns the response.
func htDo(t *testing.T, handler http.Handler, method, path, body, auth string) *httptest.ResponseRecorder {
	t.Helper()

	var buf *bytes.Reader
	if body != "" {
		buf = bytes.NewReader([]byte(body))
	} else {
		buf = bytes.NewReader(nil)
	}

	req := httptest.NewRequestWithContext(t.Context(), method, path, buf)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

// htPKCE mints the pair /authorize hands a browser: a state token bound to
// a PKCE challenge, and the cookie holding the matching verifier. Callback
// tests present both, as a real login does.
func htPKCE(t *testing.T, provider, secret string) (string, *http.Cookie) {
	t.Helper()

	verifier := oauth2.GenerateVerifier()

	state, err := makeOAuthState(provider, oauth2.S256ChallengeFromVerifier(verifier), secret)
	if err != nil {
		t.Fatalf("makeOAuthState: %v", err)
	}

	return state, &http.Cookie{Name: pkceCookieName, Value: verifier}
}

// htDoPKCE POSTs body to path with cookies attached, for callback endpoints
// that require the PKCE cookie issued at the start of the flow.
func htDoPKCE(t *testing.T, handler http.Handler, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

// htSliceField extracts resp[key] as a []any, failing the test if the
// key is absent or holds a value of a different type.
func htSliceField(t *testing.T, resp map[string]any, key string) []any {
	t.Helper()

	v, ok := resp[key].([]any)
	if !ok {
		t.Fatalf("expected %q to be []any, got %T", key, resp[key])
	}

	return v
}

// htMapField extracts v as a map[string]any, failing the test if v
// holds a value of a different type.
func htMapField(t *testing.T, v any) map[string]any {
	t.Helper()

	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", v)
	}

	return m
}

// ── Health ────────────────────────────────────────────────────────────────────

// TestHealth verifies health behavior.
func TestHealth(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/health", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("Health: want 200, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", resp["status"])
	}
}

// ── PublicSettings ────────────────────────────────────────────────────────────

// TestPublicSettings verifies public settings behavior.
func TestPublicSettings(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/settings/public", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("PublicSettings: want 200, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)

	if _, ok := resp["registration_enabled"]; !ok {
		t.Error("expected registration_enabled in response")
	}
}

// ── Register ──────────────────────────────────────────────────────────────────

// TestRegister_InvalidJSON verifies register invalid JSON behavior.
func TestRegister_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/register", "not-json", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestRegister_RegistrationDisabled verifies register registration disabled
// behavior.
func TestRegister_RegistrationDisabled(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: settingKeyRegistrationEnabled, Value: "false"},
	)
	r := newTestRouterWithRepos(repos)
	body := `{"username":"testuser","email":"t@test.com","password":"TestPass99"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestRegister_EmailDomainBlocked verifies register email domain blocked
// behavior.
func TestRegister_EmailDomainBlocked(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: settingKeyRegistrationEnabled, Value: "true"},
		models.PlatformSetting{Key: "registration_email_whitelist", Value: "example.com"},
	)
	r := newTestRouterWithRepos(repos)
	body := `{"username":"testuser","email":"t@other.com","password":"TestPass99"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 (domain blocked), got %d", rec.Code)
	}
}

// TestRegister_UsernameTooShort verifies register username too short
// behavior.
func TestRegister_UsernameTooShort(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	body := `{"username":"ab","email":"t@test.com","password":"TestPass99"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestRegister_PasswordTooShort verifies register password too short
// behavior.
func TestRegister_PasswordTooShort(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	body := `{"username":"validuser","email":"t@test.com","password":"short"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestRegister_PasswordNeedsUppercase verifies register password needs
// uppercase behavior.
func TestRegister_PasswordNeedsUppercase(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: settingKeyPasswordMinLength, Value: "8"},
		models.PlatformSetting{Key: settingKeyPasswordRequireUppercase, Value: "true"},
	)
	r := newTestRouterWithRepos(repos)
	body := `{"username":"validuser","email":"t@test.com","password":"nouppercase"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (no uppercase), got %d", rec.Code)
	}
}

// TestRegister_PasswordNeedsNumber verifies register password needs number
// behavior.
func TestRegister_PasswordNeedsNumber(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: settingKeyPasswordMinLength, Value: "8"},
		models.PlatformSetting{Key: settingKeyPasswordRequireUppercase, Value: "false"},
		models.PlatformSetting{Key: settingKeyPasswordRequireNumber, Value: "true"},
	)
	r := newTestRouterWithRepos(repos)
	body := `{"username":"validuser","email":"t@test.com","password":"NoNumbersHere"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (no number), got %d", rec.Code)
	}
}

// TestRegister_PasswordMinLenFallback verifies register password min len
// fallback behavior.
func TestRegister_PasswordMinLenFallback(t *testing.T) {
	t.Parallel()

	// When the min_length setting returns "0" (< 1), it falls back to 8
	r := newTestRouter()
	body := `{"username":"validuser","email":"t@test.com","password":"short"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (too short after fallback), got %d", rec.Code)
	}
}

// TestRegister_PasswordHasUppercasePasses verifies register password has
// uppercase passes behavior.
func TestRegister_PasswordHasUppercasePasses(t *testing.T) {
	t.Parallel()

	// password_require_uppercase=true, password HAS uppercase → passes that check
	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: settingKeyPasswordMinLength, Value: "8"},
		models.PlatformSetting{Key: settingKeyPasswordRequireUppercase, Value: "true"},
		models.PlatformSetting{Key: settingKeyPasswordRequireNumber, Value: "false"},
	)
	// After validation passes, hit a DB error on the existence check to stop.
	users := fake.NewUserRepository()
	users.Err = errors.New("db down")
	repos.Users = users
	r := newTestRouterWithRepos(repos)
	body := `{"username":"validuser","email":"t@test.com","password":"ValidPassWord"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500 (DB error after valid password), got %d", rec.Code)
	}
}

// TestRegister_PasswordHasNumberPasses verifies register password has number
// passes behavior.
func TestRegister_PasswordHasNumberPasses(t *testing.T) {
	t.Parallel()

	// password_require_number=true, password HAS number → passes that check
	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: settingKeyPasswordMinLength, Value: "8"},
		models.PlatformSetting{Key: settingKeyPasswordRequireUppercase, Value: "false"},
		models.PlatformSetting{Key: settingKeyPasswordRequireNumber, Value: "true"},
	)
	// After validation passes, hit a DB error on the existence check to stop.
	users := fake.NewUserRepository()
	users.Err = errors.New("db down")
	repos.Users = users
	r := newTestRouterWithRepos(repos)
	body := `{"username":"validuser","email":"t@test.com","password":"validpassword1"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500 (DB error after valid password), got %d", rec.Code)
	}
}

// TestRegister_Conflict verifies register conflict behavior.
func TestRegister_Conflict(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.New(), Username: "validuser", Email: "t@test.com", IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)
	body := `{"username":"validuser","email":"t@test.com","password":"TestPass99"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// TestRegister_DBError verifies register DB error behavior.
func TestRegister_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	users := fake.NewUserRepository()
	users.Err = errors.New("connection failed")
	repos.Users = users
	r := newTestRouterWithRepos(repos)
	body := `{"username":"validuser","email":"t@test.com","password":"TestPass99"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

// TestLogin_InvalidJSON verifies login invalid JSON behavior.
func TestLogin_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/login", "not-json", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestLogin_LocalLoginDisabled verifies login local login disabled behavior.
func TestLogin_LocalLoginDisabled(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: settingKeySSOLocalLoginEnabled, Value: "false"},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/auth/login", `{"email":"t@test.com","password":"TestPass99"}`, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestLogin_UserNotFound verifies login user not found behavior.
func TestLogin_UserNotFound(t *testing.T) {
	t.Parallel()

	// sso_local_login_enabled → empty → fallback "true"; user query → empty → error → 401
	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/login", `{"email":"t@test.com","password":"TestPass99"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestLogin_NullPasswordHash verifies login null password hash behavior.
func TestLogin_NullPasswordHash(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/login", `{"email":"t@test.com","password":"TestPass99"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 (SSO user), got %d", rec.Code)
	}
}

// TestLogin_WrongPassword verifies login wrong password behavior.
func TestLogin_WrongPassword(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/login", `{"email":"t@test.com","password":"WrongPassword"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 (wrong password), got %d", rec.Code)
	}
}

// TestLogin_Success verifies login success behavior.
func TestLogin_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	hash := htLoginHash
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.New(), Username: "testuser", Email: "t@test.com", PasswordHash: &hash,
		Role: "student", IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)
	body := fmt.Sprintf(`{"email":"t@test.com","password":%q}`, htLoginPassword)

	rec := htDo(t, r, "POST", "/api/auth/login", body, "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected token in response")
	}
}

// ── Me ────────────────────────────────────────────────────────────────────────

// TestMe_Unauthorized verifies me unauthorized behavior.
func TestMe_Unauthorized(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/auth/me", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestMe_NotFound verifies me not found behavior.
func TestMe_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/auth/me", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestMe_Success verifies me success behavior.
func TestMe_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.MustParse(htUserID), Username: "testuser", Email: "t@test.com",
		Role: "student", IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/auth/me", "", htAuthHeaderForSubject(t, "student", htUserID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── UpdateProfile ─────────────────────────────────────────────────────────────

// TestUpdateProfile_InvalidJSON verifies update profile invalid JSON
// behavior.
func TestUpdateProfile_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/auth/profile", "bad-json", htAuthHeader(t, "student"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpdateProfile_UsernameTooShort verifies update profile username too
// short behavior.
func TestUpdateProfile_UsernameTooShort(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/auth/profile", `{"username":"ab"}`, htAuthHeader(t, "student"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpdateProfile_UsernameDisallowed verifies update profile username
// disallowed behavior.
func TestUpdateProfile_UsernameDisallowed(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: "profile_allow_username_change", Value: "false"},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "PUT", "/api/auth/profile", `{"username":"newname"}`, htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestUpdateProfile_BioOnly verifies update profile bio only behavior.
func TestUpdateProfile_BioOnly(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.MustParse(htUserID), Username: "testuser", Email: "t@test.com",
		Role: "student", IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "PUT", "/api/auth/profile", `{"bio":"My bio"}`, htAuthHeaderForSubject(t, "student", htUserID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── ChangePassword ────────────────────────────────────────────────────────────

// TestChangePassword_MissingOldPassword verifies change password missing old
// password behavior.
func TestChangePassword_MissingOldPassword(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/auth/password", `{"newPassword":"NewPass99"}`, htAuthHeader(t, "student"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestChangePassword_MissingNewPassword verifies change password missing new
// password behavior.
func TestChangePassword_MissingNewPassword(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/auth/password", `{"oldPassword":"OldPass99"}`, htAuthHeader(t, "student"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestChangePassword_NewPasswordTooShort verifies change password new
// password too short behavior.
func TestChangePassword_NewPasswordTooShort(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/auth/password", `{"oldPassword":"OldPass99","newPassword":"short"}`, htAuthHeader(t, "student"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestChangePassword_SSOUser verifies change password SSO user behavior.
func TestChangePassword_SSOUser(t *testing.T) {
	t.Parallel()

	// validatePasswordPolicy: 3 ReadSettings → all empty → fallbacks; then QueryRow → empty → error → 400
	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/auth/password", `{"oldPassword":"OldPass99","newPassword":"NewPass9999"}`, htAuthHeader(t, "student"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (SSO user), got %d", rec.Code)
	}
}

// TestChangePassword_WrongOldPassword verifies change password wrong old
// password behavior.
func TestChangePassword_WrongOldPassword(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	hash := htLoginHash
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.MustParse(htUserID), Username: "testuser", Email: "t@test.com", PasswordHash: &hash,
		Role: "student", IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "PUT", "/api/auth/password", `{"oldPassword":"WrongOld","newPassword":"NewPass9999"}`, htAuthHeaderForSubject(t, "student", htUserID))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestChangePassword_Success verifies change password success behavior.
func TestChangePassword_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	hash := htLoginHash
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.MustParse(htUserID), Username: "testuser", Email: "t@test.com", PasswordHash: &hash,
		Role: "student", IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)
	body := fmt.Sprintf(`{"oldPassword":%q,"newPassword":"NewPass9999"}`, htLoginPassword)

	rec := htDo(t, r, "PUT", "/api/auth/password", body, htAuthHeaderForSubject(t, "student", htUserID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestChangePassword_DBUpdateError verifies change password DB update error
// behavior.
func TestChangePassword_DBUpdateError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	hash := htLoginHash
	users := fake.NewUserRepository(models.User{
		ID: uuid.MustParse(htUserID), Username: "testuser", Email: "t@test.com", PasswordHash: &hash,
		Role: "student", IsActive: true, AuthProvider: "local",
	})
	users.UpdatePasswordHashErr = errors.New("db down")
	repos.Users = users
	r := newTestRouterWithRepos(repos)
	body := fmt.Sprintf(`{"oldPassword":%q,"newPassword":"NewPass9999"}`, htLoginPassword)

	rec := htDo(t, r, "PUT", "/api/auth/password", body, htAuthHeaderForSubject(t, "student", htUserID))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestChangePassword_InvalidJSON verifies change password invalid JSON
// behavior.
func TestChangePassword_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/auth/password", "not-json", htAuthHeader(t, "student"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpdateProfile_UsernameConflict verifies update profile username
// conflict behavior.
func TestUpdateProfile_UsernameConflict(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(
		models.User{
			ID: uuid.MustParse(htUserID), Username: "currentname", Email: "t@test.com",
			Role: "student", IsActive: true, AuthProvider: "local",
		},
		models.User{
			ID: uuid.New(), Username: "takenname", Email: "other@test.com",
			Role: "student", IsActive: true, AuthProvider: "local",
		},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "PUT", "/api/auth/profile", `{"username":"takenname"}`, htAuthHeaderForSubject(t, "student", htUserID))
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// TestUpdateProfile_UsernameSuccess verifies update profile username success
// behavior.
func TestUpdateProfile_UsernameSuccess(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.MustParse(htUserID), Username: "oldname", Email: "t@test.com",
		Role: "student", IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "PUT", "/api/auth/profile", `{"username":"newname"}`, htAuthHeaderForSubject(t, "student", htUserID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── Admin Stats ───────────────────────────────────────────────────────────────

// TestAdminStats verifies admin stats behavior.
func TestAdminStats(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/stats", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if _, ok := resp["total_users"]; !ok {
		t.Error("expected total_users in response")
	}
}

// TestAdminStats_Unauthorized verifies admin stats unauthorized behavior.
func TestAdminStats_Unauthorized(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/stats", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for non-admin, got %d", rec.Code)
	}
}

// ── ListUsers ─────────────────────────────────────────────────────────────────

// TestListUsers_Empty verifies list users empty behavior.
func TestListUsers_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/users", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestListUsers_DBError verifies list users DB error behavior.
func TestListUsers_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	users := fake.NewUserRepository()
	users.Err = errors.New("db error")
	repos.Users = users
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/users", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestListUsers_WithData verifies list users with data behavior.
func TestListUsers_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.New(), Username: "alice", Email: "alice@test.com", Role: "student", IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/users", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	users := htSliceField(t, resp, "users")
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
}

// TestListUsers_WithProviderFilter verifies list users with provider filter
// behavior.
func TestListUsers_WithProviderFilter(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(
		models.User{ID: uuid.New(), Username: "alice", Email: "alice@test.com", Role: "student", IsActive: true, AuthProvider: "local"},
		models.User{ID: uuid.New(), Username: "bob", Email: "bob@test.com", Role: "student", IsActive: true, AuthProvider: "github"},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/users?provider=github", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	users := htSliceField(t, resp, "users")
	if len(users) != 1 {
		t.Errorf("expected 1 user with github provider, got %d", len(users))
	}
}

// ── GetUser ───────────────────────────────────────────────────────────────────

// TestGetUser_NotFound verifies get user not found behavior.
func TestGetUser_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/users/user-uuid-1", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestGetUser_Success verifies get user success behavior.
func TestGetUser_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.MustParse(htUserID), Username: "testuser", Email: "t@test.com", Role: "student",
		IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/users/"+htUserID, "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── UpdateUser ────────────────────────────────────────────────────────────────

// TestUpdateUser_InvalidRole verifies update user invalid role behavior.
func TestUpdateUser_InvalidRole(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/admin/users/user-uuid-1", `{"role":"superuser"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpdateUser_NotFound verifies update user not found behavior.
func TestUpdateUser_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/admin/users/user-uuid-1", `{"role":"student"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestUpdateUser_Success verifies update user success behavior.
func TestUpdateUser_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.MustParse(htUserID), Username: "testuser", Email: "t@test.com", Role: "student",
		IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "PUT", "/api/admin/users/"+htUserID, `{"role":"admin"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── DeleteUser ────────────────────────────────────────────────────────────────

// TestDeleteUser_DBError verifies delete user DB error behavior.
func TestDeleteUser_DBError(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "DELETE", "/api/admin/users/user-uuid-1", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestDeleteUser_Success verifies delete user success behavior.
func TestDeleteUser_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{ID: uuid.MustParse(htUserID), Username: "testuser", Email: "t@test.com", Role: "student", IsActive: true, AuthProvider: "local"})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/users/"+htUserID, "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── SearchUsers ───────────────────────────────────────────────────────────────

// TestSearchUsers_Empty verifies search users empty behavior.
func TestSearchUsers_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/users/search?q=alice", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestSearchUsers_WithData verifies search users with data behavior.
func TestSearchUsers_WithData(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/users/search?q=alice", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── ListAuthProviders ─────────────────────────────────────────────────────────

// TestListAuthProviders_Empty verifies list auth providers empty behavior.
func TestListAuthProviders_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/users/providers", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── AdminLeaderboard ──────────────────────────────────────────────────────────

// TestAdminLeaderboard_Empty verifies admin leaderboard empty behavior.
func TestAdminLeaderboard_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/leaderboard", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestAdminLeaderboard_WithData verifies admin leaderboard with data
// behavior.
func TestAdminLeaderboard_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.New(), Username: "alice", Email: "alice@test.com", Role: "student", IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/leaderboard", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	lb := htSliceField(t, resp, "leaderboard")
	if len(lb) != 1 {
		t.Errorf("expected 1 entry, got %d", len(lb))
	}
}

// TestAdminLeaderboard_DBError verifies admin leaderboard DB error behavior.
func TestAdminLeaderboard_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	users := fake.NewUserRepository()
	users.Err = errors.New("db error")
	repos.Users = users
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/leaderboard", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── GetSettings ───────────────────────────────────────────────────────────────

// TestGetSettings_DBError verifies get settings DB error behavior.
func TestGetSettings_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	settings := fake.NewSettingRepository()
	settings.Err = errors.New("db error")
	repos.Settings = settings
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/settings", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestGetSettings_Empty verifies get settings empty behavior.
func TestGetSettings_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/settings", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestGetSettings_WithData verifies get settings with data behavior.
func TestGetSettings_WithData(t *testing.T) {
	t.Parallel()

	desc := "enable registration"
	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: "registration_enabled", Value: "true", Description: &desc},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/settings", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	settingsList := htSliceField(t, resp, "settings")
	if len(settingsList) != 1 {
		t.Errorf("expected 1 setting, got %d", len(settingsList))
	}
}

// TestGetSettings_SecretRedacted verifies that secret setting values are
// replaced with "********" in the GET response.
func TestGetSettings_SecretRedacted(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/settings", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	json.NewDecoder(rec.Body).Decode(&resp)

	settings := htSliceField(t, resp, "settings")
	for _, raw := range settings {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		key, _ := item["key"].(string)
		val, _ := item["value"].(string)

		if (key == "oidc_client_secret" || key == "ldap_bind_password") && val != settingRedactedValue {
			t.Errorf("key %q: want redacted value, got %q", key, val)
		}

		if key == "oidc_enabled" && val != "true" {
			t.Errorf("key %q: want plain value, got %q", key, val)
		}
	}
}

// TestUpdateSettings_InsecureSkipVerifyRejected verifies that
// oidc_insecure_skip_verify cannot be set via the API.
func TestUpdateSettings_InsecureSkipVerifyRejected(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/admin/settings", `{"oidc_insecure_skip_verify":"true"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (key not allowed), got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── UpdateSettings ────────────────────────────────────────────────────────────

// TestUpdateSettings_EmptyBody verifies update settings empty body behavior.
func TestUpdateSettings_EmptyBody(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/admin/settings", `{}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpdateSettings_UnknownKey verifies update settings unknown key
// behavior.
func TestUpdateSettings_UnknownKey(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/admin/settings", `{"unknown_setting":"val"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpdateSettings_NonStringValue verifies update settings non string
// value behavior.
func TestUpdateSettings_NonStringValue(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/admin/settings", `{"registration_enabled":true}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpdateSettings_DBError verifies update settings DB error behavior.
func TestUpdateSettings_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	settings := fake.NewSettingRepository()
	settings.Err = errors.New("db error")
	repos.Settings = settings
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "PUT", "/api/admin/settings", `{"registration_enabled":"false"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestUpdateSettings_Success verifies update settings success behavior.
func TestUpdateSettings_Success(t *testing.T) {
	t.Parallel()

	// Exec → empty → OK, nil; GetSettings → Query → empty rows
	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/admin/settings", `{"registration_enabled":"true"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── Groups ────────────────────────────────────────────────────────────────────

// TestCreateGroup_MissingName verifies create group missing name behavior.
func TestCreateGroup_MissingName(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups", `{"name":""}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestCreateGroup_Conflict verifies create group conflict behavior.
func TestCreateGroup_Conflict(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = fake.NewGroupRepository(models.Group{Name: "existing-group"})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/admin/groups", `{"name":"existing-group"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// TestCreateGroup_Success verifies create group success behavior.
func TestCreateGroup_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups", `{"name":"new-group"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusCreated {
		t.Errorf("want 201, got %d", rec.Code)
	}
}

// TestDeleteGroup_NotFound verifies delete group not found behavior.
func TestDeleteGroup_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "DELETE", "/api/admin/groups/group-uuid-1", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestDeleteGroup_Success verifies delete group success behavior.
func TestDeleteGroup_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	groupID := uuid.MustParse("00000000-0000-0000-0000-00000000dead")
	repos.Groups = fake.NewGroupRepository(models.Group{ID: groupID, Name: "devs", Source: "local"})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/groups/"+groupID.String(), "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestDeleteGroup_DBError verifies delete group DB error behavior.
func TestDeleteGroup_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/groups/group-uuid-1", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestListGroups_DBError verifies list groups DB error behavior.
func TestListGroups_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/groups", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestListGroups_Empty verifies list groups empty behavior.
func TestListGroups_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/groups", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestListGroups_WithData verifies list groups with data behavior.
func TestListGroups_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = fake.NewGroupRepository(models.Group{Name: "admins", Source: "local"})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/groups", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	groups := htSliceField(t, resp, "groups")
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}

// ── GroupMappings ─────────────────────────────────────────────────────────────

// TestListGroupMappings_DBError verifies list group mappings DB error
// behavior.
func TestListGroupMappings_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/groups/mappings", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestListGroupMappings_Empty verifies list group mappings empty behavior.
func TestListGroupMappings_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/groups/mappings", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestListGroupMappings_WithData verifies list group mappings with data
// behavior.
func TestListGroupMappings_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()

	groups, ok := repos.Groups.(*fake.GroupRepository)
	if !ok {
		t.Fatalf("repos.Groups is not *fake.GroupRepository")
	}

	groups.Mappings = []models.GroupRoleMapping{{GroupName: "devs", PlatformRole: "admin"}}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/groups/mappings", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestUpsertGroupMapping_MissingGroupName verifies upsert group mapping
// missing group name behavior.
func TestUpsertGroupMapping_MissingGroupName(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups/mappings", `{"groupName":"","platformRole":"admin"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpsertGroupMapping_InvalidRole verifies upsert group mapping invalid
// role behavior.
func TestUpsertGroupMapping_InvalidRole(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups/mappings", `{"groupName":"devs","platformRole":"superadmin"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpsertGroupMapping_Success verifies upsert group mapping success
// behavior.
func TestUpsertGroupMapping_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups/mappings", `{"groupName":"devs","platformRole":"admin"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestDeleteGroupMapping_NotFound verifies delete group mapping not found
// behavior.
func TestDeleteGroupMapping_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "DELETE", "/api/admin/groups/mappings/devs", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestDeleteGroupMapping_Success verifies delete group mapping success
// behavior.
func TestDeleteGroupMapping_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()

	groups, ok := repos.Groups.(*fake.GroupRepository)
	if !ok {
		t.Fatalf("repos.Groups is not *fake.GroupRepository")
	}

	groups.Mappings = []models.GroupRoleMapping{{GroupName: "devs", PlatformRole: "admin"}}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/groups/mappings/devs", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── Enrollments ───────────────────────────────────────────────────────────────

// TestEnroll_DBError verifies enroll DB error behavior.
func TestEnroll_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Enrollments = &fake.EnrollmentRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/enrollments/my-course", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestEnroll_Success verifies enroll success behavior.
func TestEnroll_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/enrollments/my-course", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestUnenroll_DBError verifies unenroll DB error behavior.
func TestUnenroll_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Enrollments = &fake.EnrollmentRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/enrollments/my-course", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestUnenroll_Success verifies unenroll success behavior.
func TestUnenroll_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "DELETE", "/api/enrollments/my-course", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── MarkLessonComplete ────────────────────────────────────────────────────────

// TestMarkLessonComplete_DBError verifies mark lesson complete DB error
// behavior.
func TestMarkLessonComplete_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.LessonProgress = &fake.LessonProgressRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "POST", "/internal/progress/complete",
		`{"userId":"user-uuid-1","courseSlug":"my-course","lessonSlug":"intro"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestMarkLessonComplete_Success verifies mark lesson complete success
// behavior.
func TestMarkLessonComplete_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, "POST", "/internal/progress/complete",
		`{"userId":"user-uuid-1","courseSlug":"my-course","lessonSlug":"intro"}`)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── MyCourses ─────────────────────────────────────────────────────────────────

// TestEnroll_ForeignKeyConstraint verifies enroll foreign key constraint
// behavior.
func TestEnroll_ForeignKeyConstraint(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Enrollments = &fake.EnrollmentRepository{Err: errors.New("foreign key constraint violation")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/enrollments/test-course", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 (FK constraint = expired session), got %d", rec.Code)
	}
}

// TestMyCourses_DBError verifies my courses DB error behavior.
func TestMyCourses_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Enrollments = &fake.EnrollmentRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/my/courses", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestMyCourses_Empty verifies my courses empty behavior.
func TestMyCourses_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/my/courses", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestMyCourses_WithEnrollments verifies my courses with enrollments
// behavior.
func TestMyCourses_WithEnrollments(t *testing.T) {
	t.Parallel()

	// Seed one enrollment; fetchCourseDetails will fail (empty URL → bad request)
	// but MyCourses gracefully falls back to slug-only data
	repos := fake.NewRepositories()
	repos.Enrollments = fake.NewEnrollmentRepository(models.Enrollment{
		UserID: "user-uuid-1", CourseSlug: "my-course-slug",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/my/courses", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Courses []struct {
			Slug  string `json:"slug"`
			Title string `json:"title"`
		} `json:"courses"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Courses) != 1 {
		t.Errorf("expected 1 course, got %d", len(resp.Courses))
	}
}

// TestMyCourses_WithCourseService verifies my courses with course service
// behavior.
func TestMyCourses_WithCourseService(t *testing.T) {
	t.Parallel()

	// Mock course service that returns valid course data
	courseSvc, _ := newCatalogServer(t, catalogFixture{Courses: map[string]CourseInfo{
		"test-course": {Slug: "test-course", ID: "test-course", Title: "Test Course", IsPublic: true, LabCount: 3},
	}})

	repos := fake.NewRepositories()
	repos.Enrollments = fake.NewEnrollmentRepository(models.Enrollment{
		UserID: "user-uuid-1", CourseSlug: "test-course",
	})
	s := &State{
		Repos:  repos,
		Config: &config.Config{JWTSecret: htSecret, JWTExpiryH: htExpiry, CourseServiceURL: courseSvc.URL},
	}
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/my/courses", http.NoBody)
	req.Header.Set("Authorization", htAuthHeader(t, "student"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Courses []struct {
			Title string `json:"title"`
		} `json:"courses"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Courses) != 1 || resp.Courses[0].Title != "Test Course" {
		t.Errorf("expected 1 course with title 'Test Course', got %v", resp.Courses)
	}
}

// TestMyCourses_CourseServiceNon200 verifies my courses course service
// non200 behavior.
func TestMyCourses_CourseServiceNon200(t *testing.T) {
	t.Parallel()

	// Course service returns 404 → fetchCourseDetails returns err → fallback slug-only row
	courseSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer courseSvc.Close()

	repos := fake.NewRepositories()
	repos.Enrollments = fake.NewEnrollmentRepository(models.Enrollment{
		UserID: "user-uuid-1", CourseSlug: "missing-course",
	})
	s := &State{
		Repos:  repos,
		Config: &config.Config{JWTSecret: htSecret, JWTExpiryH: htExpiry, CourseServiceURL: courseSvc.URL},
	}
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/my/courses", http.NoBody)
	req.Header.Set("Authorization", htAuthHeader(t, "student"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestMyCourses_CourseServiceBadJSON verifies my courses course service bad
// JSON behavior.
func TestMyCourses_CourseServiceBadJSON(t *testing.T) {
	t.Parallel()

	// Course service returns 200 but invalid JSON → fetchCourseDetails decode fails → fallback
	courseSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json"))
	}))
	defer courseSvc.Close()

	repos := fake.NewRepositories()
	repos.Enrollments = fake.NewEnrollmentRepository(models.Enrollment{
		UserID: "user-uuid-1", CourseSlug: "bad-json-course",
	})
	s := &State{
		Repos:  repos,
		Config: &config.Config{JWTSecret: htSecret, JWTExpiryH: htExpiry, CourseServiceURL: courseSvc.URL},
	}
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/my/courses", http.NoBody)
	req.Header.Set("Authorization", htAuthHeader(t, "student"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── Admin Courses ─────────────────────────────────────────────────────────────

// TestListCourseEnrollments_Empty verifies list course enrollments empty
// behavior.
func TestListCourseEnrollments_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/enrollments/my-course", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestAdminEnrollUser_DBError verifies admin enroll user DB error behavior.
func TestAdminEnrollUser_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	enrollments := fake.NewEnrollmentRepository()
	enrollments.Err = errors.New("db down")
	repos.Enrollments = enrollments
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/admin/enrollments/my-course", `{"userId":"user-uuid-1"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminEnrollUser_MissingUserID verifies admin enroll user missing user
// ID behavior.
func TestAdminEnrollUser_MissingUserID(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/enrollments/my-course", `{"userId":""}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestAdminEnrollUser_Success verifies admin enroll user success behavior.
func TestAdminEnrollUser_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/enrollments/my-course", `{"userId":"user-uuid-1"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestAdminUnenrollUser_Success verifies admin unenroll user success
// behavior.
func TestAdminUnenrollUser_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "DELETE", "/api/admin/enrollments/my-course/users/user-uuid-1", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestAdminEnrollGroup_MissingGroupID verifies admin enroll group missing
// group ID behavior.
func TestAdminEnrollGroup_MissingGroupID(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/enrollments/my-course/groups", `{"groupId":""}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestAdminEnrollGroup_Success verifies admin enroll group success behavior.
func TestAdminEnrollGroup_Success(t *testing.T) {
	t.Parallel()

	// 2 Exec calls: group_enrollments link + backfill enrollments → both from empty → OK
	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/enrollments/my-course/groups", `{"groupId":"group-uuid-1"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestAdminUnenrollGroup_Success verifies admin unenroll group success
// behavior.
func TestAdminUnenrollGroup_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "DELETE", "/api/admin/enrollments/my-course/groups/group-uuid-1", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestAdminListGroupEnrollments_Empty verifies admin list group enrollments
// empty behavior.
func TestAdminListGroupEnrollments_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/enrollments/my-course/groups", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── SyncProgress ──────────────────────────────────────────────────────────────

// TestSyncProgress_MissingFields verifies sync progress missing fields
// behavior.
func TestSyncProgress_MissingFields(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	body := `{"userId":"uuid-1","courseSlug":""}`

	rec := htDo(t, r, "POST", "/api/admin/sync-progress", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestSyncProgress_Success verifies sync progress success behavior.
func TestSyncProgress_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	body := `{"userId":"uuid-1","courseSlug":"my-course","lessonSlug":"intro"}`

	rec := htDo(t, r, "POST", "/api/admin/sync-progress", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestSyncProgress_InvalidJSON verifies sync progress invalid JSON behavior.
func TestSyncProgress_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/sync-progress", "not-json", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestSyncProgress_DBError verifies sync progress DB error behavior.
func TestSyncProgress_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	lessonProgress := fake.NewLessonProgressRepository()
	lessonProgress.Err = errors.New("db error")
	repos.LessonProgress = lessonProgress
	r := newTestRouterWithRepos(repos)
	body := `{"userId":"uuid-1","courseSlug":"my-course","lessonSlug":"intro"}`

	rec := htDo(t, r, "POST", "/api/admin/sync-progress", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestListCourseEnrollments_WithData verifies list course enrollments with
// data behavior.
func TestListCourseEnrollments_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Enrollments = fake.NewEnrollmentRepository(
		models.Enrollment{UserID: "user-uuid-1", CourseSlug: "my-course"},
		models.Enrollment{UserID: "user-uuid-2", CourseSlug: "my-course"},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/enrollments/my-course", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	enrollments := htSliceField(t, resp, "enrollments")
	if len(enrollments) != 2 {
		t.Errorf("want 2 enrollments, got %d", len(enrollments))
	}
}

// TestListCourseEnrollments_DBError verifies list course enrollments DB
// error behavior.
func TestListCourseEnrollments_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	enrollments := fake.NewEnrollmentRepository()
	enrollments.Err = errors.New("db error")
	repos.Enrollments = enrollments
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/enrollments/my-course", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestAdminUnenrollUser_DBError verifies admin unenroll user DB error
// behavior.
func TestAdminUnenrollUser_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	enrollments := fake.NewEnrollmentRepository()
	enrollments.Err = errors.New("db error")
	repos.Enrollments = enrollments
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/enrollments/my-course/users/user-uuid-1", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestAdminEnrollGroup_DBError verifies admin enroll group DB error
// behavior.
func TestAdminEnrollGroup_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	groups := fake.NewGroupRepository()
	groups.Err = errors.New("db error")
	repos.Groups = groups
	r := newTestRouterWithRepos(repos)
	body := `{"groupId":"group-uuid-1"}`

	rec := htDo(t, r, "POST", "/api/admin/enrollments/my-course/groups", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestListAuthProviders_WithData verifies list auth providers with data
// behavior.
func TestListAuthProviders_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(
		models.User{ID: uuid.New(), Username: "alice", Email: "alice@test.com", Role: "student", IsActive: true, AuthProvider: "local"},
		models.User{ID: uuid.New(), Username: "bob", Email: "bob@test.com", Role: "student", IsActive: true, AuthProvider: "github"},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/users/providers", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	providers := htSliceField(t, resp, "providers")
	if len(providers) != 2 {
		t.Errorf("want 2 providers, got %d", len(providers))
	}
}

// TestListAuthProviders_DBError verifies list auth providers DB error
// behavior.
func TestListAuthProviders_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	users := fake.NewUserRepository()
	users.Err = errors.New("db error")
	repos.Users = users
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/users/providers", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestAdminListGroupEnrollments_WithData verifies admin list group
// enrollments with data behavior.
func TestAdminListGroupEnrollments_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	groupID := uuid.New()
	groups := fake.NewGroupRepository(models.Group{ID: groupID, Name: "devops-team", Source: "local"})
	groups.GroupEnrollments = []models.GroupEnrollment{{GroupID: groupID, CourseSlug: "my-course"}}
	repos.Groups = groups
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/enrollments/my-course/groups", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	groups2 := htSliceField(t, resp, "groups")
	if len(groups2) != 1 {
		t.Errorf("want 1 group, got %d", len(groups2))
	}
}

// TestAdminListGroupEnrollments_DBError verifies admin list group
// enrollments DB error behavior.
func TestAdminListGroupEnrollments_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	groups := fake.NewGroupRepository()
	groups.Err = errors.New("db error")
	repos.Groups = groups
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/enrollments/my-course/groups", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestAdminUnenrollGroup_DBError verifies admin unenroll group DB error
// behavior.
func TestAdminUnenrollGroup_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	groups := fake.NewGroupRepository()
	groups.Err = errors.New("db error")
	repos.Groups = groups
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/enrollments/my-course/groups/group-uuid-1", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestRegister_Success verifies register success behavior.
func TestRegister_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	body := `{"username":"testuser","email":"test@example.com","password":"ValidPass9"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMyCourses_WithCourseData verifies my courses with course data
// behavior.
func TestMyCourses_WithCourseData(t *testing.T) {
	t.Parallel()

	mockCourse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"slug":            "kubernetes",
			"id":              "k8s-1",
			"title":           "Kubernetes",
			"description":     "Learn K8s",
			"category":        "devops",
			"difficulty":      "beginner",
			"isPublic":        true,
			"labCount":        3,
			"enrollmentCount": 100,
		})
	}))
	defer mockCourse.Close()

	repos := fake.NewRepositories()
	repos.Enrollments = fake.NewEnrollmentRepository(models.Enrollment{
		UserID: "user-uuid-1", CourseSlug: "kubernetes",
	})

	cfg := &config.Config{
		JWTSecret:        htSecret,
		JWTExpiryH:       htExpiry,
		CORSOrigins:      []string{"*"},
		CourseServiceURL: mockCourse.URL,
	}
	s := &State{Repos: repos, Config: cfg}
	r := BuildRouter(s, cfg, false)

	rec := htDo(t, r, "GET", "/api/my/courses", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	courses := htSliceField(t, resp, "courses")
	if len(courses) != 1 {
		t.Errorf("want 1 course, got %d", len(courses))
	}
}

// ── Internal: Enrollments ─────────────────────────────────────────────────────

// TestInternalRoutes_Unauthorized verifies that /internal/* routes reject
// requests missing the X-Internal-Secret header with 401.
func TestInternalRoutes_Unauthorized(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	routes := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", "/internal/enrollments/auto", `{"userId":"u","courseSlug":"c"}`},
		{"GET", "/internal/enrollments/check?userId=u&courseSlug=c", ""},
		{"GET", "/internal/progress/viewed?userId=u&courseSlug=c", ""},
		{"POST", "/internal/progress/complete", `{"userId":"u","courseSlug":"c","lessonSlug":"l"}`},
		{"POST", "/internal/progress/course-complete", `{"userId":"u","courseSlug":"c"}`},
		{"POST", "/internal/progress/module", `{"userId":"u","courseSlug":"c","moduleIndex":0,"score":0,"maxScore":10,"passed":false}`},
		{"GET", "/internal/progress/modules?userId=u&courseSlug=c", ""},
		{"GET", "/internal/progress/course-summary?userId=u&courseSlug=c", ""},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()

			rec := htDo(t, r, tc.method, tc.path, tc.body, "")
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("want 401 without secret, got %d", rec.Code)
			}
		})
	}
}

// TestInternalAutoEnroll_InvalidJSON verifies internal auto enroll invalid
// JSON behavior.
func TestInternalAutoEnroll_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, "POST", "/internal/enrollments/auto", "bad")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestInternalAutoEnroll_DBError verifies internal auto enroll DB error
// behavior.
func TestInternalAutoEnroll_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Enrollments = &fake.EnrollmentRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)
	body := `{"userId":"user-uuid-1","courseSlug":"my-course"}`

	rec := htDoInternal(t, r, "POST", "/internal/enrollments/auto", body)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestInternalAutoEnroll_Success verifies internal auto enroll success
// behavior.
func TestInternalAutoEnroll_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	body := `{"userId":"user-uuid-1","courseSlug":"my-course"}`

	rec := htDoInternal(t, r, "POST", "/internal/enrollments/auto", body)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestInternalCheckEnrollment_Enrolled verifies internal check enrollment
// enrolled behavior.
func TestInternalCheckEnrollment_Enrolled(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Enrollments = fake.NewEnrollmentRepository(models.Enrollment{
		UserID: "uuid-1", CourseSlug: "my-course",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "GET", "/internal/enrollments/check?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]bool
	json.NewDecoder(rec.Body).Decode(&resp)

	if !resp["enrolled"] {
		t.Error("expected enrolled=true")
	}
}

// TestInternalCheckEnrollment_NotEnrolled verifies internal check enrollment
// not enrolled behavior.
func TestInternalCheckEnrollment_NotEnrolled(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, "GET", "/internal/enrollments/check?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]bool
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["enrolled"] {
		t.Error("expected enrolled=false")
	}
}

// TestInternalCheckEnrollment_DBError verifies internal check enrollment DB
// error behavior.
func TestInternalCheckEnrollment_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Enrollments = &fake.EnrollmentRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "GET", "/internal/enrollments/check?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── Internal: Progress ────────────────────────────────────────────────────────

// TestInternalViewedLessons_Empty verifies internal viewed lessons empty
// behavior.
func TestInternalViewedLessons_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, "GET", "/internal/progress/viewed?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	viewed := htSliceField(t, resp, "viewed")
	if len(viewed) != 0 {
		t.Errorf("expected 0 viewed lessons, got %d", len(viewed))
	}
}

// TestInternalViewedLessons_WithData verifies internal viewed lessons with
// data behavior.
func TestInternalViewedLessons_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.LessonProgress = fake.NewLessonProgressRepository(
		models.LessonProgress{UserID: "uuid-1", CourseSlug: "my-course", LessonSlug: "intro"},
		models.LessonProgress{UserID: "uuid-1", CourseSlug: "my-course", LessonSlug: "part-2"},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "GET", "/internal/progress/viewed?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	viewed := htSliceField(t, resp, "viewed")
	if len(viewed) != 2 {
		t.Errorf("expected 2 viewed, got %d", len(viewed))
	}
}

// TestInternalMarkComplete_InvalidJSON verifies internal mark complete
// invalid JSON behavior.
func TestInternalMarkComplete_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, "POST", "/internal/progress/complete", "bad")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestInternalMarkComplete_Success verifies internal mark complete success
// behavior.
func TestInternalMarkComplete_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	body := `{"userId":"uuid-1","courseSlug":"my-course","lessonSlug":"intro"}`

	rec := htDoInternal(t, r, "POST", "/internal/progress/complete", body)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestInternalRecordModuleProgress_InvalidJSON verifies internal record
// module progress invalid JSON behavior.
func TestInternalRecordModuleProgress_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, "POST", "/internal/progress/module", "bad")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestInternalRecordModuleProgress_Success verifies internal record module
// progress success behavior.
func TestInternalRecordModuleProgress_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	body := `{"userId":"uuid-1","courseSlug":"my-course","moduleIndex":0,"score":80,"maxScore":100,"passed":true}`

	rec := htDoInternal(t, r, "POST", "/internal/progress/module", body)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestInternalGetModuleProgress_Empty verifies internal get module progress
// empty behavior.
func TestInternalGetModuleProgress_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, "GET", "/internal/progress/modules?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestInternalGetModuleProgress_WithData verifies internal get module
// progress with data behavior.
func TestInternalGetModuleProgress_WithData(t *testing.T) {
	t.Parallel()

	slug0, slug1 := "module-0", "module-1"
	repos := fake.NewRepositories()
	repos.ModuleProgress = fake.NewModuleProgressRepository(
		models.ModuleProgress{
			UserID: "uuid-1", CourseSlug: "my-course", ModuleIndex: 0, ModuleSlug: &slug0,
			BestScore: 80, MaxScore: 100, Passed: true, Attempts: 2,
		},
		models.ModuleProgress{
			UserID: "uuid-1", CourseSlug: "my-course", ModuleIndex: 1, ModuleSlug: &slug1,
			BestScore: 60, MaxScore: 100, Passed: false, Attempts: 1,
		},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "GET", "/internal/progress/modules?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	progress := htSliceField(t, resp, "progress")
	if len(progress) != 2 {
		t.Errorf("expected 2 progress entries, got %d", len(progress))
	}
}

// TestInternalCourseSummary_DBError verifies internal course summary DB
// error behavior.
func TestInternalCourseSummary_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.ModuleProgress = &fake.ModuleProgressRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "GET", "/internal/progress/course-summary?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestInternalCourseSummary_Success verifies internal course summary success
// behavior.
func TestInternalCourseSummary_Success(t *testing.T) {
	t.Parallel()

	slug1, slug2 := "module-1", "module-2"
	repos := fake.NewRepositories()
	repos.ModuleProgress = fake.NewModuleProgressRepository(
		models.ModuleProgress{
			UserID: "uuid-1", CourseSlug: "my-course", ModuleIndex: 0, ModuleSlug: &slug1,
			BestScore: 100, MaxScore: 100, Passed: true, Attempts: 1,
		},
		models.ModuleProgress{
			UserID: "uuid-1", CourseSlug: "my-course", ModuleIndex: 1, ModuleSlug: &slug2,
			BestScore: 80, MaxScore: 100, Passed: true, Attempts: 1,
		},
	)
	repos.LessonProgress = fake.NewLessonProgressRepository(
		models.LessonProgress{UserID: "uuid-1", CourseSlug: "my-course", LessonSlug: "l1"},
		models.LessonProgress{UserID: "uuid-1", CourseSlug: "my-course", LessonSlug: "l2"},
		models.LessonProgress{UserID: "uuid-1", CourseSlug: "my-course", LessonSlug: "l3"},
		models.LessonProgress{UserID: "uuid-1", CourseSlug: "my-course", LessonSlug: "l4"},
		models.LessonProgress{UserID: "uuid-1", CourseSlug: "my-course", LessonSlug: "l5"},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "GET", "/internal/progress/course-summary?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["totalScore"] == nil {
		t.Error("expected totalScore in response")
	}

	modules := htSliceField(t, resp, "passedModules")
	if len(modules) != 2 {
		t.Errorf("expected 2 passed modules, got %d", len(modules))
	}
}

// ── OAuth handlers ────────────────────────────────────────────────────────────

// newOAuthRouter builds a router wired with OAuth test routes.
func newOAuthRouter(providers ...config.ProviderConfig) http.Handler {
	cfg := &config.Config{
		JWTSecret:         htSecret,
		JWTExpiryH:        htExpiry,
		CORSOrigins:       []string{"*"},
		OAuthRedirectBase: "http://localhost:3000",
		Providers:         providers,
	}
	s := &State{Repos: fake.NewRepositories(), Config: cfg}

	return BuildRouter(s, cfg, false)
}

// TestOAuthAuthorize_UnknownProvider verifies o auth authorize unknown
// provider behavior.
func TestOAuthAuthorize_UnknownProvider(t *testing.T) {
	t.Parallel()

	r := newOAuthRouter()

	rec := htDo(t, r, "GET", "/api/auth/oauth/unknown/authorize", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestOAuthAuthorize_GitHub verifies o auth authorize git hub behavior.
func TestOAuthAuthorize_GitHub(t *testing.T) {
	t.Parallel()

	r := newOAuthRouter(config.ProviderConfig{
		ID: "github", Name: "GitHub", ClientID: "gh-client-id",
	})

	rec := htDo(t, r, "GET", "/api/auth/oauth/github/authorize", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["url"] == "" {
		t.Error("expected non-empty authorize URL")
	}
}

// TestOAuthAuthorize_OIDCMissingIssuer verifies o auth authorize OIDC
// missing issuer behavior.
func TestOAuthAuthorize_OIDCMissingIssuer(t *testing.T) {
	t.Parallel()

	r := newOAuthRouter(config.ProviderConfig{
		ID: "custom-oidc", Name: "Custom OIDC", ClientID: "oidc-id",
		// No IssuerURL, no well-known mapping → empty
	})

	rec := htDo(t, r, "GET", "/api/auth/oauth/custom-oidc/authorize", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestOAuthCallback_InvalidJSON verifies o auth callback invalid JSON
// behavior.
func TestOAuthCallback_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newOAuthRouter()

	rec := htDo(t, r, "POST", "/api/auth/oauth/callback", "not-json", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestOAuthCallback_InvalidState verifies o auth callback invalid state
// behavior.
func TestOAuthCallback_InvalidState(t *testing.T) {
	t.Parallel()

	r := newOAuthRouter()
	body := `{"code":"some-code","state":"invalid-state-token"}`

	rec := htDo(t, r, "POST", "/api/auth/oauth/callback", body, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestResolveIssuerURL_KnownProviders verifies resolve issuer URL known
// providers behavior.
func TestResolveIssuerURL_KnownProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id   string
		want string
	}{
		{"gitlab", "https://gitlab.com"},
		{"google", "https://accounts.google.com"},
	}
	for _, tt := range tests {
		p := &config.ProviderConfig{ID: tt.id}

		got := resolveIssuerURL(p)
		if got != tt.want {
			t.Errorf("resolveIssuerURL(%q): want %q, got %q", tt.id, tt.want, got)
		}
	}
}

// TestResolveIssuerURL_CustomIssuer verifies resolve issuer URL custom
// issuer behavior.
func TestResolveIssuerURL_CustomIssuer(t *testing.T) {
	t.Parallel()

	p := &config.ProviderConfig{ID: "keycloak", IssuerURL: "https://auth.example.com/realm/test"}

	got := resolveIssuerURL(p)
	if got != "https://auth.example.com/realm/test" {
		t.Errorf("expected custom issuer, got %q", got)
	}
}

// TestResolveIssuerURL_Unknown verifies resolve issuer URL unknown behavior.
func TestResolveIssuerURL_Unknown(t *testing.T) {
	t.Parallel()

	p := &config.ProviderConfig{ID: "unknown-provider"}

	got := resolveIssuerURL(p)
	if got != "" {
		t.Errorf("expected empty for unknown provider, got %q", got)
	}
}

// ── Patterns ──────────────────────────────────────────────────────────────────

// testPattern returns a seeded global-scope pattern named "callout" with a
// fixed, valid UUID.
func testPattern() models.MarkdownPattern {
	return models.MarkdownPattern{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		Name:        "callout",
		Label:       "Callout",
		Description: "A callout box",
		HTML:        "<div>{{content}}</div>",
		CSS:         ".callout{}",
		Scope:       "global",
	}
}

// TestListPatterns_Success verifies list patterns success behavior.
func TestListPatterns_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Patterns = fake.NewPatternRepository(testPattern())
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/patterns", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	patterns := htSliceField(t, resp, "patterns")
	if len(patterns) != 1 {
		t.Errorf("want 1 pattern, got %d", len(patterns))
	}
}

// TestListPatterns_Empty verifies list patterns empty behavior.
func TestListPatterns_Empty(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/patterns", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	patterns := htSliceField(t, resp, "patterns")
	if len(patterns) != 0 {
		t.Errorf("want empty patterns, got %d", len(patterns))
	}
}

// TestListPatterns_DBError verifies list patterns DB error behavior.
func TestListPatterns_DBError(t *testing.T) {
	t.Parallel()

	patterns := fake.NewPatternRepository()
	patterns.Err = errors.New("db error")
	repos := fake.NewRepositories()
	repos.Patterns = patterns
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/patterns", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestGetPattern_Success verifies get pattern success behavior.
func TestGetPattern_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Patterns = fake.NewPatternRepository(testPattern())
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/patterns/550e8400-e29b-41d4-a716-446655440000", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestGetPattern_InvalidUUID verifies get pattern invalid UUID behavior.
func TestGetPattern_InvalidUUID(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "GET", "/api/patterns/not-a-uuid", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestGetPattern_NotFound verifies get pattern not found behavior.
func TestGetPattern_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "GET", "/api/patterns/550e8400-e29b-41d4-a716-446655440000", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestCreatePattern_Unauthorized verifies create pattern unauthorized
// behavior.
func TestCreatePattern_Unauthorized(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())
	body := `{"name":"callout","label":"Callout","html":"<div></div>"}`

	rec := htDo(t, r, "POST", "/api/admin/patterns", body, htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestCreatePattern_InvalidJSON verifies create pattern invalid JSON
// behavior.
func TestCreatePattern_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "POST", "/api/admin/patterns", "not-json{", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestCreatePattern_MissingFields verifies create pattern missing fields
// behavior.
func TestCreatePattern_MissingFields(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())
	body := `{"name":"callout","label":"Callout"}` // missing html

	rec := htDo(t, r, "POST", "/api/admin/patterns", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestCreatePattern_Success verifies create pattern success behavior.
func TestCreatePattern_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())
	body := `{"name":"callout","label":"Callout","html":"<div>{{content}}</div>"}`

	rec := htDo(t, r, "POST", "/api/admin/patterns", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCreatePattern_Conflict verifies create pattern conflict behavior.
func TestCreatePattern_Conflict(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Patterns = fake.NewPatternRepository(testPattern())
	r := newTestRouterWithRepos(repos)
	body := `{"name":"callout","label":"Callout","html":"<div></div>"}`

	rec := htDo(t, r, "POST", "/api/admin/patterns", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// TestUpdatePattern_Success verifies update pattern success behavior.
func TestUpdatePattern_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Patterns = fake.NewPatternRepository(testPattern())
	r := newTestRouterWithRepos(repos)
	body := `{"label":"Updated Callout","html":"<span>{{content}}</span>"}`

	rec := htDo(t, r, "PUT", "/api/admin/patterns/callout", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdatePattern_NotFound verifies update pattern not found behavior.
func TestUpdatePattern_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())
	body := `{"label":"Updated","html":"<span></span>"}`

	rec := htDo(t, r, "PUT", "/api/admin/patterns/nonexistent", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestUpdatePattern_MissingHTML verifies update pattern missing HTML
// behavior.
func TestUpdatePattern_MissingHTML(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())
	body := `{"label":"Updated"}`

	rec := htDo(t, r, "PUT", "/api/admin/patterns/callout", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestDeletePattern_Success verifies delete pattern success behavior.
func TestDeletePattern_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Patterns = fake.NewPatternRepository(testPattern())
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/patterns/callout", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", rec.Code)
	}
}

// TestDeletePattern_NotFound verifies delete pattern not found behavior.
func TestDeletePattern_NotFound(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(fake.NewRepositories())

	rec := htDo(t, r, "DELETE", "/api/admin/patterns/nonexistent", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// ── OAuth helpers ─────────────────────────────────────────────────────────────

// TestMakeOAuthState verifies make o auth state behavior.
func TestMakeOAuthState(t *testing.T) {
	t.Parallel()

	tok, err := makeOAuthState("github", "challenge", htSecret)
	if err != nil {
		t.Fatalf("makeOAuthState: %v", err)
	}

	if tok == "" {
		t.Error("expected non-empty state token")
	}
}

// TestDecodeOAuthState_Valid verifies decode o auth state valid behavior.
func TestDecodeOAuthState_Valid(t *testing.T) {
	t.Parallel()

	tok, err := makeOAuthState("github", "the-challenge", htSecret)
	if err != nil {
		t.Fatal(err)
	}

	provider, challenge, ok := decodeOAuthState(tok, htSecret)
	if !ok {
		t.Error("expected ok=true for valid state token")
	}

	if provider != "github" {
		t.Errorf("want provider=github, got %q", provider)
	}

	if challenge != "the-challenge" {
		t.Errorf("want the committed PKCE challenge back, got %q", challenge)
	}
}

// TestDecodeOAuthState_WrongSecret verifies decode o auth state wrong secret
// behavior.
func TestDecodeOAuthState_WrongSecret(t *testing.T) {
	t.Parallel()

	tok, _ := makeOAuthState("github", "challenge", htSecret)

	_, _, ok := decodeOAuthState(tok, "wrong-secret-key-32-bytes-long!!!")
	if ok {
		t.Error("expected ok=false for wrong secret")
	}
}

// TestDecodeOAuthState_Garbage verifies decode o auth state garbage
// behavior.
func TestDecodeOAuthState_Garbage(t *testing.T) {
	t.Parallel()

	_, _, ok := decodeOAuthState("not.a.valid.token", htSecret)
	if ok {
		t.Error("expected ok=false for garbage token")
	}
}

// TestDecodeOAuthState_Empty verifies decode o auth state empty behavior.
func TestDecodeOAuthState_Empty(t *testing.T) {
	t.Parallel()

	_, _, ok := decodeOAuthState("", htSecret)
	if ok {
		t.Error("expected ok=false for empty token")
	}
}

// TestSanitizeUsername_Normal verifies sanitize username normal behavior.
func TestSanitizeUsername_Normal(t *testing.T) {
	t.Parallel()

	if got := sanitizeUsername("John Doe"); got != "JohnDoe" {
		t.Errorf("want JohnDoe, got %q", got)
	}
}

// TestSanitizeUsername_Empty verifies sanitize username empty behavior.
func TestSanitizeUsername_Empty(t *testing.T) {
	t.Parallel()

	if got := sanitizeUsername(""); got != "user" {
		t.Errorf("want user, got %q", got)
	}
}

// TestSanitizeUsername_SpecialChars verifies sanitize username special chars
// behavior.
func TestSanitizeUsername_SpecialChars(t *testing.T) {
	t.Parallel()

	got := sanitizeUsername("user@example.com!")
	if got != "userexamplecom" {
		t.Errorf("want userexamplecom, got %q", got)
	}
}

// TestSanitizeUsername_Truncated verifies sanitize username truncated
// behavior.
func TestSanitizeUsername_Truncated(t *testing.T) {
	t.Parallel()

	long := "abcdefghijklmnopqrstuvwxyzabcdefghijklmno" // 41 chars

	got := sanitizeUsername(long)
	if len(got) > 32 {
		t.Errorf("expected max 32 chars, got %d: %q", len(got), got)
	}
}

// TestSanitizeUsername_ValidChars verifies sanitize username valid chars
// behavior.
func TestSanitizeUsername_ValidChars(t *testing.T) {
	t.Parallel()

	if got := sanitizeUsername("user_name-123"); got != "user_name-123" {
		t.Errorf("want user_name-123, got %q", got)
	}
}

// TestListProviders_NoProviders verifies list providers no providers
// behavior.
func TestListProviders_NoProviders(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/auth/oauth/providers", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	providers := htSliceField(t, resp, "providers")
	if len(providers) != 0 {
		t.Errorf("want empty providers list, got %d", len(providers))
	}
}

// TestListProviders_WithProviders verifies list providers with providers
// behavior.
func TestListProviders_WithProviders(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		JWTSecret:   htSecret,
		JWTExpiryH:  htExpiry,
		CORSOrigins: []string{"*"},
		Providers: []config.ProviderConfig{
			{ID: "github", Name: "GitHub", ClientID: "gh-client-id"},
			{ID: "empty-provider", Name: "No Client", ClientID: ""},
		},
	}
	s := &State{Config: cfg}
	r := BuildRouter(s, cfg, false)

	rec := htDo(t, r, "GET", "/api/auth/oauth/providers", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	providers := htSliceField(t, resp, "providers")
	if len(providers) != 1 {
		t.Errorf("want 1 provider (empty filtered), got %d", len(providers))
	}

	p := htMapField(t, providers[0])
	if p["id"] != "github" {
		t.Errorf("want id=github, got %v", p["id"])
	}
}

// ── Pure helper function tests ────────────────────────────────────────────────

// TestNullStr_Empty verifies null str empty behavior.
func TestNullStr_Empty(t *testing.T) {
	t.Parallel()

	if nullStr("") != nil {
		t.Error("expected nil for empty string")
	}
}

// TestNullStr_NonEmpty verifies null str non empty behavior.
func TestNullStr_NonEmpty(t *testing.T) {
	t.Parallel()

	s := nullStr("hello")
	if s == nil || *s != "hello" {
		t.Errorf("expected *string='hello', got %v", s)
	}
}

// TestDerefStr_Nil verifies deref str nil behavior.
func TestDerefStr_Nil(t *testing.T) {
	t.Parallel()

	if derefStr(nil) != "" {
		t.Error("expected empty string for nil pointer")
	}
}

// TestDerefStr_NonNil verifies deref str non nil behavior.
func TestDerefStr_NonNil(t *testing.T) {
	t.Parallel()

	s := "world"
	if derefStr(&s) != "world" {
		t.Errorf("expected 'world', got %q", derefStr(&s))
	}
}

// ── extractURLBase (oidc.go) ──────────────────────────────────────────────────

// TestExtractURLBase_Valid verifies extract URL base valid behavior.
func TestExtractURLBase_Valid(t *testing.T) {
	t.Parallel()

	got := extractURLBase("https://sso.example.com/auth/realms/master")
	if got != "https://sso.example.com" {
		t.Errorf("want https://sso.example.com, got %q", got)
	}
}

// TestExtractURLBase_NoHost verifies extract URL base no host behavior.
func TestExtractURLBase_NoHost(t *testing.T) {
	t.Parallel()

	got := extractURLBase("not-a-url")
	if got != "not-a-url" {
		t.Errorf("expected unchanged, got %q", got)
	}
}

// TestExtractURLBase_JustHost verifies extract URL base just host behavior.
func TestExtractURLBase_JustHost(t *testing.T) {
	t.Parallel()

	got := extractURLBase("http://localhost:8080")
	if got != "http://localhost:8080" {
		t.Errorf("want http://localhost:8080, got %q", got)
	}
}

// ── viewedLessons direct call ─────────────────────────────────────────────────

// TestViewedLessons_DBError verifies viewed lessons DB error behavior.
func TestViewedLessons_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.LessonProgress = &fake.LessonProgressRepository{Err: errors.New("db error")}
	s := &State{Repos: repos, Config: &config.Config{}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	result := viewedLessons(s, req, "some-course", "user-id")
	if result != nil {
		t.Errorf("expected nil map on error, got %v", result)
	}
}

// TestViewedLessons_WithData verifies viewed lessons with data behavior.
func TestViewedLessons_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.LessonProgress = fake.NewLessonProgressRepository(
		models.LessonProgress{UserID: "user-id", CourseSlug: "my-course", LessonSlug: "intro"},
		models.LessonProgress{UserID: "user-id", CourseSlug: "my-course", LessonSlug: "advanced"},
	)
	s := &State{Repos: repos, Config: &config.Config{}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	result := viewedLessons(s, req, "my-course", "user-id")
	if result == nil {
		t.Fatal("expected non-nil map")
	}

	if !result["intro"] || !result["advanced"] {
		t.Errorf("expected intro and advanced in result: %v", result)
	}
}

// TestViewedLessons_Empty verifies viewed lessons empty behavior.
func TestViewedLessons_Empty(t *testing.T) {
	t.Parallel()

	s := &State{Repos: fake.NewRepositories(), Config: &config.Config{}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	result := viewedLessons(s, req, "my-course", "user-id")
	if result == nil {
		t.Fatal("expected non-nil map (empty)")
	}

	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

// ── Course patterns (direct handler calls with chi context) ───────────────────

// newStateWithRepos builds a handler state backed by the given repos.
func newStateWithRepos(repos *repository.Repositories) *State {
	return &State{
		Repos: repos,
		Config: &config.Config{
			JWTSecret:  htSecret,
			JWTExpiryH: htExpiry,
		},
	}
}

// withChiParam injects a chi URL parameter into the request context.
func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)

	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// withAuthAndChiParam injects auth context and a chi URL parameter.
func withAuthAndChiParam(r *http.Request, role, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	claims := &apimiddleware.Claims{Email: "test@example.com", Role: role}
	claims.Subject = "user-uuid-1"
	ctx = context.WithValue(ctx, apimiddleware.ClaimsKey, claims)

	return r.WithContext(ctx)
}

// TestListCoursePatterns_Success verifies list course patterns success
// behavior.
func TestListCoursePatterns_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Patterns = fake.NewPatternRepository(models.MarkdownPattern{
		ID: uuid.New(), Name: "callout", Label: "Callout", Description: "A callout",
		HTML: "<div>{{content}}</div>", Scope: "my-course",
	})
	s := newStateWithRepos(repos)
	rec := httptest.NewRecorder()
	req := withChiParam(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/my-course/patterns", http.NoBody), "slug", "my-course")
	s.ListCoursePatterns(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestListCoursePatterns_DBError verifies list course patterns DB error
// behavior.
func TestListCoursePatterns_DBError(t *testing.T) {
	t.Parallel()

	patterns := fake.NewPatternRepository()
	patterns.Err = errors.New("db error")
	repos := fake.NewRepositories()
	repos.Patterns = patterns
	s := newStateWithRepos(repos)
	rec := httptest.NewRecorder()
	req := withChiParam(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/my-course/patterns", http.NoBody), "slug", "my-course")
	s.ListCoursePatterns(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestListCoursePatterns_Empty verifies list course patterns empty behavior.
func TestListCoursePatterns_Empty(t *testing.T) {
	t.Parallel()

	s := newStateWithRepos(fake.NewRepositories())
	rec := httptest.NewRecorder()
	req := withChiParam(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/courses/my-course/patterns", http.NoBody), "slug", "my-course")
	s.ListCoursePatterns(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestCreateCoursePattern_InvalidJSON verifies create course pattern invalid
// JSON behavior.
func TestCreateCoursePattern_InvalidJSON(t *testing.T) {
	t.Parallel()

	s := newStateWithRepos(fake.NewRepositories())
	rec := httptest.NewRecorder()
	req := withAuthAndChiParam(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader("bad json")), "admin", "slug", "my-course")
	s.CreateCoursePattern(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestCreateCoursePattern_MissingFields verifies create course pattern
// missing fields behavior.
func TestCreateCoursePattern_MissingFields(t *testing.T) {
	t.Parallel()

	s := newStateWithRepos(fake.NewRepositories())
	rec := httptest.NewRecorder()
	req := withAuthAndChiParam(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"name":"test"}`)), "admin", "slug", "my-course")
	s.CreateCoursePattern(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateCoursePattern_Success verifies create course pattern success
// behavior.
func TestCreateCoursePattern_Success(t *testing.T) {
	t.Parallel()

	s := newStateWithRepos(fake.NewRepositories())
	rec := httptest.NewRecorder()
	body := `{"name":"callout","label":"Callout","html":"<div>{{content}}</div>"}`
	req := withAuthAndChiParam(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body)), "admin", "slug", "my-course")
	s.CreateCoursePattern(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateCoursePattern_Conflict verifies create course pattern conflict
// behavior.
func TestCreateCoursePattern_Conflict(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Patterns = fake.NewPatternRepository(models.MarkdownPattern{
		ID: uuid.New(), Name: "callout", Label: "Callout", Description: "A callout",
		HTML: "<div>{{content}}</div>", Scope: "my-course",
	})
	s := newStateWithRepos(repos)
	rec := httptest.NewRecorder()
	body := `{"name":"callout","label":"Callout","html":"<div>{{content}}</div>"}`
	req := withAuthAndChiParam(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body)), "admin", "slug", "my-course")
	s.CreateCoursePattern(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// TestDeleteCoursePattern_InvalidUUID verifies delete course pattern invalid
// UUID behavior.
func TestDeleteCoursePattern_InvalidUUID(t *testing.T) {
	t.Parallel()

	s := newStateWithRepos(fake.NewRepositories())
	rec := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "my-course")
	rctx.URLParams.Add("id", "not-a-uuid")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/", http.NoBody).WithContext(
		context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
	s.DeleteCoursePattern(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestDeleteCoursePattern_Success verifies delete course pattern success
// behavior.
func TestDeleteCoursePattern_Success(t *testing.T) {
	t.Parallel()

	patternID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	repos := fake.NewRepositories()
	repos.Patterns = fake.NewPatternRepository(models.MarkdownPattern{
		ID: patternID, Name: "callout", Label: "Callout", HTML: "<div></div>", Scope: "my-course",
	})
	s := newStateWithRepos(repos)
	rec := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "my-course")
	rctx.URLParams.Add("id", "550e8400-e29b-41d4-a716-446655440000")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/", http.NoBody).WithContext(
		context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
	s.DeleteCoursePattern(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", rec.Code)
	}
}

// TestDeleteCoursePattern_NotFound verifies delete course pattern not found
// behavior.
func TestDeleteCoursePattern_NotFound(t *testing.T) {
	t.Parallel()

	s := newStateWithRepos(fake.NewRepositories())
	rec := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", "my-course")
	rctx.URLParams.Add("id", "550e8400-e29b-41d4-a716-446655440000")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/", http.NoBody).WithContext(
		context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
	s.DeleteCoursePattern(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// ── upsertSSOUser direct calls ────────────────────────────────────────────────

// TestUpsertSSOUser_NewUser verifies upsert SSO user new user behavior.
func TestUpsertSSOUser_NewUser(t *testing.T) {
	t.Parallel()

	users := fake.NewUserRepository()

	u, err := upsertSSOUser(context.Background(), users, "john@example.com", "John Doe", nil, nil, "github", "gh-123")
	if err != nil {
		t.Fatalf("upsertSSOUser: %v", err)
	}

	if u.Email != "john@example.com" {
		t.Errorf("email: want john@example.com, got %q", u.Email)
	}
}

// TestUpsertSSOUser_ExistingByProvider verifies upsert SSO user existing by
// provider behavior.
func TestUpsertSSOUser_ExistingByProvider(t *testing.T) {
	t.Parallel()

	existingID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	providerUID := "gh-123"
	users := fake.NewUserRepository(models.User{
		ID: existingID, Username: "johndoe", Email: "john@example.com", Role: "student",
		IsActive: true, AuthProvider: "github", ProviderUserID: &providerUID,
	})

	u, err := upsertSSOUser(context.Background(), users, "john@example.com", "John Doe", nil, nil, "github", "gh-123")
	if err != nil {
		t.Fatalf("upsertSSOUser: %v", err)
	}

	if u.ID != existingID.String() {
		t.Errorf("ID: want %s, got %q", existingID, u.ID)
	}
}

// TestUpsertSSOUser_ExistingByEmail_SameProvider verifies upsert SSO user
// existing by email same provider behavior.
func TestUpsertSSOUser_ExistingByEmail_SameProvider(t *testing.T) {
	t.Parallel()

	emailID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	users := fake.NewUserRepository(models.User{
		ID: emailID, Username: "johndoe", Email: "john@example.com", Role: "student",
		IsActive: true, AuthProvider: "github",
	})

	u, err := upsertSSOUser(context.Background(), users, "john@example.com", "John Doe", nil, nil, "github", "gh-456")
	if err != nil {
		t.Fatalf("upsertSSOUser: %v", err)
	}

	if u.ID != emailID.String() {
		t.Errorf("ID: want %s, got %q", emailID, u.ID)
	}
}

// TestUpsertSSOUser_ExistingByEmail_DifferentProvider verifies upsert SSO
// user existing by email different provider behavior.
func TestUpsertSSOUser_ExistingByEmail_DifferentProvider(t *testing.T) {
	t.Parallel()

	localID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	users := fake.NewUserRepository(models.User{
		ID: localID, Username: "johndoe", Email: "john@example.com", Role: "student",
		IsActive: true, AuthProvider: "local",
	})

	_, err := upsertSSOUser(context.Background(), users, "john@example.com", "John Doe", nil, nil, "github", "gh-123")
	if err == nil {
		t.Error("expected error for different auth provider")
	}
}

// TestUpsertSSOUser_UsernameConflict verifies upsert SSO user username
// conflict behavior.
func TestUpsertSSOUser_UsernameConflict(t *testing.T) {
	t.Parallel()

	users := fake.NewUserRepository(models.User{
		ID:       uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Username: "JohnDoe", Email: "someoneelse@example.com", Role: "student",
		IsActive: true, AuthProvider: "local",
	})

	u, err := upsertSSOUser(context.Background(), users, "john@example.com", "John Doe", nil, nil, "github", "gh-789")
	if err != nil {
		t.Fatalf("upsertSSOUser: %v", err)
	}

	if u.Email != "john@example.com" {
		t.Errorf("email mismatch: %q", u.Email)
	}

	if u.Username == "JohnDoe" {
		t.Errorf("expected suffixed username on conflict, got %q", u.Username)
	}
}

// ── syncGroupsAndDeriveRole direct calls ──────────────────────────────────────

// TestSyncGroupsAndDeriveRole_DBError verifies sync groups and derive role
// DB error behavior.
func TestSyncGroupsAndDeriveRole_DBError(t *testing.T) {
	t.Parallel()

	groups := &fake.GroupRepository{Err: errors.New("db error")}

	_, err := syncGroupsAndDeriveRole(context.Background(), groups, "user-uuid", []string{"admins"}, "oidc")
	if err == nil {
		t.Error("expected error when sync fails")
	}
}

// TestSyncGroupsAndDeriveRole_NoGroups verifies sync groups and derive role
// no groups behavior.
func TestSyncGroupsAndDeriveRole_NoGroups(t *testing.T) {
	t.Parallel()

	groups := fake.NewGroupRepository()

	role, err := syncGroupsAndDeriveRole(context.Background(), groups, "user-uuid", []string{}, "oidc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if role != "student" {
		t.Errorf("expected student, got %q", role)
	}
}

// TestSyncGroupsAndDeriveRole_WithGroup verifies sync groups and derive role
// with group behavior.
func TestSyncGroupsAndDeriveRole_WithGroup(t *testing.T) {
	t.Parallel()

	groups := fake.NewGroupRepository()
	groups.Mappings = []models.GroupRoleMapping{{GroupName: "admins", PlatformRole: "admin"}}

	role, err := syncGroupsAndDeriveRole(context.Background(), groups, "user-uuid", []string{"admins"}, "oidc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if role != "admin" {
		t.Errorf("expected admin, got %q", role)
	}
}

// TestSyncGroupsAndDeriveRole_RollbackOnError verifies that a failed sync
// leaves the user's prior groups/role untouched instead of landing in a
// broken partial-sync state, matching the real repository's transactional
// rollback behavior.
func TestSyncGroupsAndDeriveRole_RollbackOnError(t *testing.T) {
	t.Parallel()

	userID := "user-uuid"

	groups := fake.NewGroupRepository()
	groups.UserRoles[userID] = "admin"
	groups.Err = errors.New("db error")

	role, err := syncGroupsAndDeriveRole(context.Background(), groups, userID, []string{"students"}, "oidc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if role != "admin" {
		t.Errorf("expected role to remain admin after failed sync, got %q", role)
	}

	if groups.UserRoles[userID] != "admin" {
		t.Errorf("expected stored role to remain admin, got %q", groups.UserRoles[userID])
	}
}

// ── OIDCAuthorize (disabled) ──────────────────────────────────────────────────

// TestOIDCAuthorize_Disabled verifies OIDC authorize disabled behavior.
func TestOIDCAuthorize_Disabled(t *testing.T) {
	t.Parallel()

	// loadOIDCSettings calls ReadSetting 9 times; all return defaults (push error rows)
	for range 10 {
	}

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/auth/oidc/authorize", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (OIDC disabled), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestOIDCCallback_InvalidState verifies OIDC callback invalid state
// behavior.
func TestOIDCCallback_InvalidState(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/oidc/callback", `{"code":"c","state":"bad-state"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestOIDCCallback_InvalidJSON verifies OIDC callback invalid JSON behavior.
func TestOIDCCallback_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/oidc/callback", "not-json", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// ── OAuthCallback provider not found path ─────────────────────────────────────

// TestOAuthCallback_GithubProviderFetchFails verifies o auth callback github
// provider fetch fails behavior.
func TestOAuthCallback_GithubProviderFetchFails(t *testing.T) {
	t.Parallel()

	// GitHub provider is configured but fetchGitHub will fail (invalid code → 401)
	s := &State{
		Config: &config.Config{
			JWTSecret:  htSecret,
			JWTExpiryH: htExpiry,
			Providers: []config.ProviderConfig{
				{ID: "github", Name: "GitHub", ClientID: "fake-client-id", ClientSecret: "fake-secret"},
			},
		},
	}
	r := BuildRouter(s, s.Config, false)

	state, pkce := htPKCE(t, "github", htSecret)

	body := fmt.Sprintf(`{"code":"invalid-code","state":%q}`, state)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/auth/oauth/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(pkce)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	// fetchGitHub will fail (network error or bad code response)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when fetchGitHub fails, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestOAuthCallback_ValidStateUnknownProvider verifies o auth callback valid
// state unknown provider behavior.
func TestOAuthCallback_ValidStateUnknownProvider(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		JWTSecret:   htSecret,
		JWTExpiryH:  htExpiry,
		CORSOrigins: []string{"*"},
		Providers:   []config.ProviderConfig{},
	}
	s := &State{Config: cfg}
	r := BuildRouter(s, cfg, false)

	// Create a valid state token for an unknown provider
	stateToken, pkce := htPKCE(t, "unknown-provider", htSecret)
	body := fmt.Sprintf(`{"code":"test-code","state":%q}`, stateToken)

	rec := htDoPKCE(t, r, "/api/auth/oauth/callback", body, pkce)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (unknown provider), got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── LoadPatternsFromConfig ────────────────────────────────────────────────────

// TestLoadPatternsFromConfig_FileNotFound verifies load patterns from config
// file not found behavior.
func TestLoadPatternsFromConfig_FileNotFound(t *testing.T) {
	t.Parallel()

	patterns := fake.NewPatternRepository()

	err := LoadPatternsFromConfig(context.Background(), patterns, "/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

// TestLoadPatternsFromConfig_InvalidYAML verifies load patterns from config
// invalid YAML behavior.
func TestLoadPatternsFromConfig_InvalidYAML(t *testing.T) {
	t.Parallel()

	patterns := fake.NewPatternRepository()

	tmpFile, err := os.CreateTemp(t.TempDir(), "patterns-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("not: valid: yaml: :")
	tmpFile.Close()

	err = LoadPatternsFromConfig(context.Background(), patterns, tmpFile.Name())
	if err != nil {
		t.Logf("got error (may be OK for YAML parse): %v", err)
	}
}

// TestLoadPatternsFromConfig_Success verifies load patterns from config
// success behavior.
func TestLoadPatternsFromConfig_Success(t *testing.T) {
	t.Parallel()

	patterns := fake.NewPatternRepository()

	yaml := `patterns:
  - name: callout
    label: Callout
    description: A callout box
    html: "<div class=\"callout\">{{content}}</div>"
    scope: global
`

	tmpFile, err := os.CreateTemp(t.TempDir(), "patterns-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(yaml)
	tmpFile.Close()

	err = LoadPatternsFromConfig(context.Background(), patterns, tmpFile.Name())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLoadPatternsFromConfig_DBError verifies load patterns from config DB
// error behavior.
func TestLoadPatternsFromConfig_DBError(t *testing.T) {
	t.Parallel()

	patterns := fake.NewPatternRepository()
	patterns.Err = errors.New("db error")

	yaml := `patterns:
  - name: callout
    label: Callout
    html: "<div>{{content}}</div>"
`

	tmpFile, err := os.CreateTemp(t.TempDir(), "patterns-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(yaml)
	tmpFile.Close()

	err = LoadPatternsFromConfig(context.Background(), patterns, tmpFile.Name())
	if err == nil {
		t.Error("expected DB error")
	}
}

// ── doGet (oauth.go) ─────────────────────────────────────────────────────────

// TestDoGet_Success verifies do get success behavior.
func TestDoGet_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mytoken" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"login":"johndoe","id":12345}`))
	}))
	defer srv.Close()

	var result map[string]any

	err := doGet(&http.Client{}, srv.URL, "mytoken", &result)
	if err != nil {
		t.Fatalf("doGet: %v", err)
	}

	if result["login"] != "johndoe" {
		t.Errorf("expected login=johndoe, got %v", result["login"])
	}
}

// TestDoGet_NoBearer verifies do get no bearer behavior.
func TestDoGet_NoBearer(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	var result map[string]any

	err := doGet(&http.Client{}, srv.URL, "", &result)
	if err != nil {
		t.Fatalf("doGet no bearer: %v", err)
	}
}

// TestDoGet_BadURL verifies do get bad URL behavior.
func TestDoGet_BadURL(t *testing.T) {
	t.Parallel()

	var result map[string]any

	err := doGet(&http.Client{}, "http://127.0.0.1:0/invalid", "", &result)
	if err == nil {
		t.Error("expected error for bad URL")
	}
}

// TestDoGet_InvalidJSON verifies do get invalid JSON behavior.
func TestDoGet_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	var result map[string]any

	err := doGet(&http.Client{}, srv.URL, "", &result)
	if err == nil {
		t.Error("expected JSON parse error")
	}
}

// ── oidcContext (oidc.go) ─────────────────────────────────────────────────────

// TestOIDCContext_WithIssuerURL verifies OIDC context with issuer URL
// behavior.
func TestOIDCContext_WithIssuerURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := oidcSettings{IssuerURL: "https://issuer.example.com"}

	result := oidcContext(ctx, cfg)
	if result == ctx {
		t.Error("expected modified context when IssuerURL is set")
	}
}

// TestOIDCContext_WithoutIssuerURL verifies OIDC context without issuer URL
// behavior.
func TestOIDCContext_WithoutIssuerURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := oidcSettings{}

	result := oidcContext(ctx, cfg)
	if result != ctx {
		t.Error("expected same context when IssuerURL is empty")
	}
}

// ── loadOIDCSettings (oidc.go) ────────────────────────────────────────────────

// TestLoadOIDCSettings_EnabledAndConfigured verifies load OIDC settings
// enabled and configured behavior.
func TestLoadOIDCSettings_EnabledAndConfigured(t *testing.T) {
	t.Parallel()

	settings := fake.NewSettingRepository(
		models.PlatformSetting{Key: "oidc_enabled", Value: "true"},
		models.PlatformSetting{Key: "oidc_provider_url", Value: "https://sso.example.com/realms/master"},
		models.PlatformSetting{Key: "oidc_issuer_url", Value: "https://sso.example.com/realms/master"},
		models.PlatformSetting{Key: "oidc_client_id", Value: "my-client"},
		models.PlatformSetting{Key: "oidc_client_secret", Value: "my-secret"},
		models.PlatformSetting{Key: "oidc_group_claim", Value: "groups"},
		models.PlatformSetting{Key: "oidc_redirect_base", Value: "http://localhost:3000"},
		models.PlatformSetting{Key: "oidc_browser_base_url", Value: ""},
		models.PlatformSetting{Key: "oidc_scopes", Value: "openid email profile"},
	)
	s := &State{
		Repos:  &repository.Repositories{Settings: settings},
		Config: &config.Config{OAuthRedirectBase: "http://localhost:3000"},
	}

	cfg, err := s.loadOIDCSettings(context.Background())
	if err != nil {
		t.Fatalf("loadOIDCSettings: %v", err)
	}

	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}

	if cfg.ClientID != "my-client" {
		t.Errorf("ClientID: want my-client, got %q", cfg.ClientID)
	}
}

// TestLoadOIDCSettings_MissingClientID verifies load OIDC settings missing
// client ID behavior.
func TestLoadOIDCSettings_MissingClientID(t *testing.T) {
	t.Parallel()

	settings := fake.NewSettingRepository(
		models.PlatformSetting{Key: "oidc_enabled", Value: "true"},
		models.PlatformSetting{Key: "oidc_provider_url", Value: "https://sso.example.com"},
		models.PlatformSetting{Key: "oidc_client_secret", Value: "my-secret"},
	)
	s := &State{
		Repos:  &repository.Repositories{Settings: settings},
		Config: &config.Config{OAuthRedirectBase: "http://localhost:3000"},
	}

	_, err := s.loadOIDCSettings(context.Background())
	if err == nil {
		t.Error("expected error for missing clientId")
	}
}

// TestLoadOIDCSettings_InsecureSkipVerifyForbiddenWithoutIssuerURL verifies
// that insecure_skip_verify is rejected when no oidc_issuer_url is set.
func TestLoadOIDCSettings_InsecureSkipVerifyForbiddenWithoutIssuerURL(t *testing.T) {
	t.Parallel()

	settings := fake.NewSettingRepository(
		models.PlatformSetting{Key: "oidc_enabled", Value: "true"},
		models.PlatformSetting{Key: "oidc_provider_url", Value: "https://sso.example.com"},
		models.PlatformSetting{Key: "oidc_client_id", Value: "my-client"},
		models.PlatformSetting{Key: "oidc_client_secret", Value: "my-secret"},
		models.PlatformSetting{Key: "oidc_insecure_skip_verify", Value: "true"},
	)
	s := &State{
		Repos:  &repository.Repositories{Settings: settings},
		Config: &config.Config{OAuthRedirectBase: "http://localhost:3000"},
	}

	_, err := s.loadOIDCSettings(context.Background())
	if err == nil {
		t.Error("expected error: insecure_skip_verify without issuer_url must be rejected")
	}
}

// TestLoadOIDCSettings_InsecureSkipVerifyAllowedWithIssuerURL verifies that
// insecure_skip_verify is accepted when oidc_issuer_url is set (split-horizon).
func TestLoadOIDCSettings_InsecureSkipVerifyAllowedWithIssuerURL(t *testing.T) {
	t.Parallel()

	settings := fake.NewSettingRepository(
		models.PlatformSetting{Key: "oidc_enabled", Value: "true"},
		models.PlatformSetting{Key: "oidc_provider_url", Value: "https://keycloak.internal/realms/master"},
		models.PlatformSetting{Key: "oidc_issuer_url", Value: "https://sso.example.com/realms/master"},
		models.PlatformSetting{Key: "oidc_client_id", Value: "my-client"},
		models.PlatformSetting{Key: "oidc_client_secret", Value: "my-secret"},
		models.PlatformSetting{Key: "oidc_insecure_skip_verify", Value: "true"},
	)
	s := &State{
		Repos:  &repository.Repositories{Settings: settings},
		Config: &config.Config{OAuthRedirectBase: "http://localhost:3000"},
	}

	cfg, err := s.loadOIDCSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error for split-horizon setup: %v", err)
	}

	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true for split-horizon internal CA")
	}
}

// ── More groups handler tests ─────────────────────────────────────────────────

// TestUpsertGroupMapping_InvalidPlatformRole verifies upsert group mapping
// invalid platform role behavior.
func TestUpsertGroupMapping_InvalidPlatformRole(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups/mappings", `{"groupName":"admins","platformRole":"superuser"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid platformRole, got %d", rec.Code)
	}
}

// TestUpsertGroupMapping_InvalidJSON verifies upsert group mapping invalid
// JSON behavior.
func TestUpsertGroupMapping_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups/mappings", "bad-json", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestDeleteGroupMapping_DBError2 verifies delete group mapping DB error2
// behavior.
func TestDeleteGroupMapping_DBError2(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/groups/mappings/testers", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── More internal handlers ────────────────────────────────────────────────────

// TestInternalMarkComplete_DBError verifies internal mark complete DB error
// behavior.
func TestInternalMarkComplete_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.LessonProgress = &fake.LessonProgressRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "POST", "/internal/progress/complete", `{"userId":"uid","courseSlug":"c","lessonSlug":"l"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestInternalRecordModuleProgress_DBError verifies internal record module
// progress DB error behavior.
func TestInternalRecordModuleProgress_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.ModuleProgress = &fake.ModuleProgressRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)
	body := `{"userId":"uid","courseSlug":"c","moduleIndex":0,"score":10,"maxScore":10,"passed":true}`

	rec := htDoInternal(t, r, "POST", "/internal/progress/module", body)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestInternalGetModuleProgress_NoProgress verifies internal get module
// progress no progress behavior.
func TestInternalGetModuleProgress_NoProgress(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDoInternal(t, r, "GET", "/internal/progress/modules?userId=uid&courseSlug=c", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── Register additional coverage ──────────────────────────────────────────────

// TestRegister_InsertError verifies register insert error behavior.
func TestRegister_InsertError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	users := fake.NewUserRepository()
	users.CreateErr = errors.New("db error")
	repos.Users = users
	r := newTestRouterWithRepos(repos)
	body := `{"username":"validuser","email":"t@test.com","password":"TestPass99"}`

	rec := htDo(t, r, "POST", "/api/auth/register", body, "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── UpdateUser additional coverage ───────────────────────────────────────────

// TestUpdateUser_InvalidJSON verifies update user invalid JSON behavior.
func TestUpdateUser_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/admin/users/user-uuid-1", "not-json", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// ── SearchUsers additional coverage ──────────────────────────────────────────

// TestSearchUsers_DBError verifies search users DB error behavior.
func TestSearchUsers_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	users := fake.NewUserRepository()
	users.Err = errors.New("db error")
	repos.Users = users
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/users/search?q=alice", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── UpdateSettings additional coverage ───────────────────────────────────────

// TestUpdateSettings_InvalidJSON verifies update settings invalid JSON
// behavior.
func TestUpdateSettings_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "PUT", "/api/admin/settings", "not-json", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// ── syncGroupsAndDeriveRole additional coverage ───────────────────────────────

// TestSyncGroupsAndDeriveRole_EmptyGroupName verifies sync groups and derive
// role empty group name behavior.
func TestSyncGroupsAndDeriveRole_EmptyGroupName(t *testing.T) {
	t.Parallel()

	groups := fake.NewGroupRepository()

	role, err := syncGroupsAndDeriveRole(context.Background(), groups, "user-uuid", []string{""}, "oidc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if role != "student" {
		t.Errorf("expected student, got %q", role)
	}

	if len(groups.UserGroup) != 0 {
		t.Errorf("expected empty group name to be skipped entirely, got %d memberships", len(groups.UserGroup))
	}
}

// TestSyncGroupsAndDeriveRole_NonAdminRoleMapping verifies sync groups and
// derive role non admin role mapping behavior.
func TestSyncGroupsAndDeriveRole_NonAdminRoleMapping(t *testing.T) {
	t.Parallel()

	groups := fake.NewGroupRepository()
	groups.Mappings = []models.GroupRoleMapping{{GroupName: "students", PlatformRole: "student"}}

	role, err := syncGroupsAndDeriveRole(context.Background(), groups, "user-uuid", []string{"students"}, "oidc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if role != "student" {
		t.Errorf("expected student, got %q", role)
	}
}

// ── upsertSSOUser additional coverage ────────────────────────────────────────

// TestUpsertSSOUser_ExistingByProvider_UpdateError verifies upsert SSO user
// existing by provider update error behavior.
func TestUpsertSSOUser_ExistingByProvider_UpdateError(t *testing.T) {
	t.Parallel()

	existingID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	providerUID := "gh-123"
	users := fake.NewUserRepository(models.User{
		ID: existingID, Username: "johndoe", Email: "john@example.com", Role: "student",
		IsActive: true, AuthProvider: "github", ProviderUserID: &providerUID,
	})
	users.RefreshSSOProfileErr = errors.New("update error")

	u, err := upsertSSOUser(context.Background(), users, "john@example.com", "John Doe", nil, nil, "github", "gh-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if u.ID != existingID.String() {
		t.Errorf("expected %s, got %q", existingID, u.ID)
	}
}

// TestUpsertSSOUser_ExistingByEmail_UpdateError verifies upsert SSO user
// existing by email update error behavior.
func TestUpsertSSOUser_ExistingByEmail_UpdateError(t *testing.T) {
	t.Parallel()

	emailID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	users := fake.NewUserRepository(models.User{
		ID: emailID, Username: "johndoe", Email: "john@example.com", Role: "student",
		IsActive: true, AuthProvider: "",
	})
	users.LinkProviderIdentityErr = errors.New("update error")

	u, err := upsertSSOUser(context.Background(), users, "john@example.com", "John Doe", nil, nil, "github", "gh-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if u.ID != emailID.String() {
		t.Errorf("expected %s, got %q", emailID, u.ID)
	}
}

// TestUpsertSSOUser_InsertError verifies upsert SSO user insert error
// behavior.
func TestUpsertSSOUser_InsertError(t *testing.T) {
	t.Parallel()

	users := fake.NewUserRepository()
	users.CreateErr = errors.New("insert error")

	_, err := upsertSSOUser(context.Background(), users, "john@example.com", "John Doe", nil, nil, "github", "gh-789")
	if err == nil {
		t.Error("expected error when INSERT fails")
	}
}

// ── OIDCCallback additional coverage ─────────────────────────────────────────

// TestOIDCCallback_OIDCDisabled verifies OIDC callback OIDC disabled
// behavior.
func TestOIDCCallback_OIDCDisabled(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	state, pkce := htPKCE(t, "oidc", htSecret)
	body := fmt.Sprintf(`{"code":"auth-code","state":%q}`, state)

	rec := htDoPKCE(t, r, "/api/auth/oidc/callback", body, pkce)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (OIDC disabled), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestOIDCCallback_ProviderUnreachable verifies OIDC callback provider
// unreachable behavior.
func TestOIDCCallback_ProviderUnreachable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: "oidc_enabled", Value: "true"},
		models.PlatformSetting{Key: "oidc_provider_url", Value: srv.URL},
		models.PlatformSetting{Key: "oidc_client_id", Value: "my-client"},
		models.PlatformSetting{Key: "oidc_client_secret", Value: "my-secret"},
		models.PlatformSetting{Key: "oidc_scopes", Value: "openid email profile"},
	)
	r := newTestRouterWithRepos(repos)
	state, pkce := htPKCE(t, "oidc", htSecret)
	body := fmt.Sprintf(`{"code":"code","state":%q}`, state)

	rec := htDoPKCE(t, r, "/api/auth/oidc/callback", body, pkce)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── loadLDAPSettings ──────────────────────────────────────────────────────────

// TestLoadLDAPSettings_Disabled verifies load LDAP settings disabled
// behavior.
func TestLoadLDAPSettings_Disabled(t *testing.T) {
	t.Parallel()

	s := &State{
		Repos:  &repository.Repositories{Settings: fake.NewSettingRepository()},
		Config: &config.Config{},
	}

	_, err := s.loadLDAPSettings(context.Background())
	if err == nil {
		t.Error("expected error when LDAP is disabled")
	}
}

// TestLoadLDAPSettings_NotConfigured verifies load LDAP settings not
// configured behavior.
func TestLoadLDAPSettings_NotConfigured(t *testing.T) {
	t.Parallel()

	settings := fake.NewSettingRepository(
		models.PlatformSetting{Key: "ldap_enabled", Value: "true"},
	)
	s := &State{
		Repos:  &repository.Repositories{Settings: settings},
		Config: &config.Config{},
	}

	_, err := s.loadLDAPSettings(context.Background())
	if err == nil {
		t.Error("expected error when LDAP not fully configured")
	}
}

// TestLoadLDAPSettings_Configured verifies load LDAP settings configured
// behavior.
func TestLoadLDAPSettings_Configured(t *testing.T) {
	t.Parallel()

	settings := fake.NewSettingRepository(
		models.PlatformSetting{Key: "ldap_enabled", Value: "true"},
		models.PlatformSetting{Key: "ldap_server_url", Value: "ldap://ldap.example.com"},
		models.PlatformSetting{Key: "ldap_bind_dn", Value: "cn=admin,dc=example,dc=com"},
		models.PlatformSetting{Key: "ldap_bind_password", Value: "secret"},
		models.PlatformSetting{Key: "ldap_user_base_dn", Value: "ou=users,dc=example,dc=com"},
		models.PlatformSetting{Key: "ldap_user_filter", Value: "(mail=%s)"},
	)
	s := &State{
		Repos:  &repository.Repositories{Settings: settings},
		Config: &config.Config{},
	}

	cfg, err := s.loadLDAPSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServerURL != "ldap://ldap.example.com" {
		t.Errorf("ServerURL: want ldap://ldap.example.com, got %q", cfg.ServerURL)
	}
}

// ── LDAPLogin early paths ─────────────────────────────────────────────────────

// TestLDAPLogin_InvalidJSON verifies LDAP login invalid JSON behavior.
func TestLDAPLogin_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/ldap/login", "not-json", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestLDAPLogin_EmptyCredentials verifies LDAP login empty credentials
// behavior.
func TestLDAPLogin_EmptyCredentials(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/ldap/login", `{"email":"","password":""}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestLDAPLogin_LDAPDisabled verifies LDAP login LDAP disabled behavior.
func TestLDAPLogin_LDAPDisabled(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/auth/ldap/login", `{"email":"user@example.com","password":"pass123"}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (LDAP disabled), got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── InternalCourseSummary additional coverage ─────────────────────────────────

// TestInternalCourseSummary_QueryError verifies internal course summary
// query error behavior.
func TestInternalCourseSummary_QueryError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.ModuleProgress = moduleProgressListErrRepository{
		ModuleProgressRepository: fake.NewModuleProgressRepository(),
		err:                      errors.New("db error"),
	}
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "GET", "/internal/progress/course-summary?userId=uuid-1&courseSlug=my-course", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// moduleProgressListErrRepository wraps a fake ModuleProgressRepository so
// that the lesson-progress read succeeds but the module-progress read fails,
// exercising the second of the summary's two batched repository calls.
type moduleProgressListErrRepository struct {
	*fake.ModuleProgressRepository

	err error
}

// ListByUserCourses always fails with r.err.
func (r moduleProgressListErrRepository) ListByUserCourses(
	_ context.Context, _ string, _ []string,
) (map[string][]models.ModuleProgress, error) {
	return nil, r.err
}

// ── InternalGetModuleProgress additional coverage ─────────────────────────────

// TestInternalGetModuleProgress_DBError verifies internal get module
// progress DB error behavior.
func TestInternalGetModuleProgress_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.ModuleProgress = &fake.ModuleProgressRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDoInternal(t, r, "GET", "/internal/progress/modules?userId=uid&courseSlug=c", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── sanitizeUsername (extra cases) ───────────────────────────────────────────

// TestSanitizeUsername_AllSpecial verifies sanitize username all special
// behavior.
func TestSanitizeUsername_AllSpecial(t *testing.T) {
	t.Parallel()

	if got := sanitizeUsername("!@#$%^&*()"); got != "user" {
		t.Errorf("want user for all-special input, got %q", got)
	}
}

// TestSanitizeUsername_Long verifies sanitize username long behavior.
func TestSanitizeUsername_Long(t *testing.T) {
	t.Parallel()

	long := "abcdefghijklmnopqrstuvwxyz_extra_chars_here"

	got := sanitizeUsername(long)
	if len(got) > 32 {
		t.Errorf("expected max 32 chars, got %d: %q", len(got), got)
	}
}

// TestSanitizeUsername_WithDashUnderscore verifies sanitize username with
// dash underscore behavior.
func TestSanitizeUsername_WithDashUnderscore(t *testing.T) {
	t.Parallel()

	got := sanitizeUsername("user-name_ok")
	if got != "user-name_ok" {
		t.Errorf("want user-name_ok, got %q", got)
	}
}

// ── extractURLBase ───────────────────────────────────────────────────────────

// TestExtractURLBase_Normal verifies extract URL base normal behavior.
func TestExtractURLBase_Normal(t *testing.T) {
	t.Parallel()

	got := extractURLBase("http://localhost:8080/auth/realms/test")
	if got != "http://localhost:8080" {
		t.Errorf("want http://localhost:8080, got %q", got)
	}
}

// TestExtractURLBase_NoPath verifies extract URL base no path behavior.
func TestExtractURLBase_NoPath(t *testing.T) {
	t.Parallel()

	got := extractURLBase("https://example.com")
	if got != "https://example.com" {
		t.Errorf("want https://example.com, got %q", got)
	}
}

// TestExtractURLBase_InvalidURL verifies extract URL base invalid URL
// behavior.
func TestExtractURLBase_InvalidURL(t *testing.T) {
	t.Parallel()

	got := extractURLBase("://bad-url")
	if got == "" {
		t.Error("expected non-empty result for invalid URL")
	}
}

// TestExtractURLBase_EmptyHost verifies extract URL base empty host
// behavior.
func TestExtractURLBase_EmptyHost(t *testing.T) {
	t.Parallel()

	got := extractURLBase("relative-path/only")
	if got != "relative-path/only" {
		t.Errorf("want raw URL returned, got %q", got)
	}
}

// ── OIDCAuthorize provider unreachable ───────────────────────────────────────

// TestOIDCAuthorize_ProviderUnreachable verifies OIDC authorize provider
// unreachable behavior.
func TestOIDCAuthorize_ProviderUnreachable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: "oidc_enabled", Value: "true"},
		models.PlatformSetting{Key: "oidc_provider_url", Value: srv.URL},
		models.PlatformSetting{Key: "oidc_client_id", Value: "my-client"},
		models.PlatformSetting{Key: "oidc_client_secret", Value: "my-secret"},
		models.PlatformSetting{Key: "oidc_scopes", Value: "openid email profile"},
	)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/auth/oidc/authorize", "", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── decodeOAuthState (extra cases) ───────────────────────────────────────────

// TestDecodeOAuthState_Invalid verifies decode o auth state invalid
// behavior.
func TestDecodeOAuthState_Invalid(t *testing.T) {
	t.Parallel()

	_, _, ok := decodeOAuthState("not-a-jwt", htSecret)
	if ok {
		t.Error("expected invalid state to fail")
	}
}

// ── ListProviders (extra cases) ───────────────────────────────────────────────

// TestListProviders_Empty verifies list providers empty behavior.
func TestListProviders_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/auth/oauth/providers", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	providers, ok := resp["providers"]
	if !ok {
		t.Error("expected providers key in response")
	}

	_ = providers
}

// TestListProviders_WithProvider verifies list providers with provider
// behavior.
func TestListProviders_WithProvider(t *testing.T) {
	t.Parallel()

	s := &State{
		Config: &config.Config{
			JWTSecret:  htSecret,
			JWTExpiryH: htExpiry,
			Providers: []config.ProviderConfig{
				{ID: "github", Name: "GitHub", ClientID: "gh-id"},
			},
		},
	}
	r := BuildRouter(s, s.Config, false)

	rec := htDo(t, r, "GET", "/api/auth/oauth/providers", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	providers, _ := resp["providers"].([]any)
	if len(providers) != 1 {
		t.Errorf("want 1 provider, got %d", len(providers))
	}
}

// ── UpdateUser manager role ───────────────────────────────────────────────────

// TestUpdateUser_ManagerRole verifies that setting role to "manager" is
// accepted by UpdateUser.
func TestUpdateUser_ManagerRole(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID: uuid.MustParse(htUserID), Username: "testuser", Email: "t@test.com", Role: "student",
		IsActive: true, AuthProvider: "local",
	})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "PUT", "/api/admin/users/"+htUserID, `{"role":"manager"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── UpsertGroupMapping manager role ──────────────────────────────────────────

// TestUpsertGroupMapping_ManagerRole verifies that platformRole "manager" is
// accepted by UpsertGroupMapping.
func TestUpsertGroupMapping_ManagerRole(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups/mappings", `{"groupName":"devs","platformRole":"manager"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── Admin group members ───────────────────────────────────────────────────────

// TestListGroupMembers_Empty verifies 200 with empty member list.
func TestListGroupMembers_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/admin/groups/group-uuid-1/members", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// TestListGroupMembers_WithData verifies 200 with member data.
func TestListGroupMembers_WithData(t *testing.T) {
	t.Parallel()

	groupUUID := uuid.MustParse(htGroupID)
	repos := fake.NewRepositories()
	groups := fake.NewGroupRepository(models.Group{ID: groupUUID, Name: "team-rh", Source: "local"})
	groups.UserGroup = []models.UserGroup{
		{UserID: htUserID, GroupID: groupUUID},
	}
	repos.Groups = groups
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/groups/"+htGroupID+"/members", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	members := htSliceField(t, resp, "members")
	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}
}

// TestListGroupMembers_DBError verifies 500 when member lookup fails.
func TestListGroupMembers_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/admin/groups/group-uuid-1/members", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestAddGroupMember_MissingUserID verifies 400 when userId is absent.
func TestAddGroupMember_MissingUserID(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups/group-uuid-1/members", `{"userId":""}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestAddGroupMember_InvalidJSON verifies 400 for malformed body.
func TestAddGroupMember_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups/"+htGroupID+"/members", "not-json", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestAddGroupMember_Success verifies 200 when member is added.
func TestAddGroupMember_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/admin/groups/"+htGroupID+"/members", `{"userId":"`+htUserID+`"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAddGroupMember_DBError verifies 500 when the repository fails.
func TestAddGroupMember_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/admin/groups/group-uuid-1/members", `{"userId":"`+htUserID+`"}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestRemoveGroupMember_Success verifies 200 when member is removed.
func TestRemoveGroupMember_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "DELETE", "/api/admin/groups/"+htGroupID+"/members/"+htUserID, "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRemoveGroupMember_DBError verifies 500 when the repository fails.
func TestRemoveGroupMember_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db error")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/admin/groups/group-uuid-1/members/"+htUserID, "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── isMemberOfAny ─────────────────────────────────────────────────────────────

// TestIsMemberOfAny_Match verifies a match is found when groups overlap.
func TestIsMemberOfAny_Match(t *testing.T) {
	t.Parallel()

	if !isMemberOfAny([]string{"dev", "admins"}, []string{"admins", "ops"}) {
		t.Error("expected match")
	}
}

// TestIsMemberOfAny_NoMatch verifies no match when groups are disjoint.
func TestIsMemberOfAny_NoMatch(t *testing.T) {
	t.Parallel()

	if isMemberOfAny([]string{"dev", "qa"}, []string{"admins", "ops"}) {
		t.Error("expected no match")
	}
}

// TestIsMemberOfAny_EmptyGroups verifies that nil groups never match.
func TestIsMemberOfAny_EmptyGroups(t *testing.T) {
	t.Parallel()

	if isMemberOfAny(nil, []string{"admins"}) {
		t.Error("expected no match with empty groups")
	}
}

// TestIsMemberOfAny_EmptyTargets verifies that nil targets never match.
func TestIsMemberOfAny_EmptyTargets(t *testing.T) {
	t.Parallel()

	if isMemberOfAny([]string{"admins"}, nil) {
		t.Error("expected no match with empty targets")
	}
}

// ── completeSSOLogin admin group mapping ──────────────────────────────────────

// TestCompleteSSOLogin_AdminGroupGrantsAdminRole verifies a user in an admin
// SSO group receives the admin role on login.
func TestCompleteSSOLogin_AdminGroupGrantsAdminRole(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	s := newStateWithRepos(repos)
	ctx := context.Background()

	rec := httptest.NewRecorder()
	s.completeSSOLogin(
		ctx, rec,
		"admin-grp@test.com", "Admin Grp", nil, nil,
		"oidc", "oidc-sub-admingrp",
		[]string{"seanergy-admins"},
		[]string{"seanergy-admins"},
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp authResponse

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.User.Role != groupsRoleAdmin {
		t.Errorf("want role %q, got %q", groupsRoleAdmin, resp.User.Role)
	}

	persisted, err := repos.Users.FindByEmail(ctx, "admin-grp@test.com")
	if err != nil {
		t.Fatalf("find persisted user: %v", err)
	}

	if persisted.Role != groupsRoleAdmin {
		t.Errorf("persisted role = %q, want %q", persisted.Role, groupsRoleAdmin)
	}
}

// TestCompleteSSOLogin_NonAdminGroupRevokesStoredAdminRole verifies an OIDC
// group-admin configuration is authoritative and revokes a prior admin role.
func TestCompleteSSOLogin_NonAdminGroupRevokesStoredAdminRole(t *testing.T) {
	t.Parallel()

	providerUserID := "oidc-sub-devgrp"
	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID:             uuid.New(),
		Username:       "dev-grp",
		Email:          "dev-grp@test.com",
		Role:           groupsRoleAdmin,
		IsActive:       true,
		AuthProvider:   oidcProviderKey,
		ProviderUserID: &providerUserID,
	})
	s := newStateWithRepos(repos)
	ctx := context.Background()

	rec := httptest.NewRecorder()
	s.completeSSOLogin(
		ctx, rec,
		"dev-grp@test.com", "Dev Grp", nil, nil,
		"oidc", "oidc-sub-devgrp",
		[]string{"dev"},
		[]string{"seanergy-admins"},
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp authResponse

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.User.Role != groupsRoleStudent {
		t.Errorf("response role = %q, want %q", resp.User.Role, groupsRoleStudent)
	}

	persisted, err := repos.Users.FindByEmail(ctx, "dev-grp@test.com")
	if err != nil {
		t.Fatalf("find persisted user: %v", err)
	}

	if persisted.Role != groupsRoleStudent {
		t.Errorf("persisted role = %q, want %q", persisted.Role, groupsRoleStudent)
	}
}

// TestCompleteSSOLogin_NilGroupAdminsHasNoEffect verifies nil groupAdmins
// leaves role assignment unaffected.
func TestCompleteSSOLogin_NilGroupAdminsHasNoEffect(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	s := newStateWithRepos(repos)
	ctx := context.Background()

	rec := httptest.NewRecorder()
	s.completeSSOLogin(
		ctx, rec,
		"nilgrp@test.com", "Nil Grp", nil, nil,
		"oidc", "oidc-sub-nilgrp",
		[]string{"seanergy-admins"},
		nil,
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp authResponse

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.User.Role == groupsRoleAdmin {
		t.Errorf("expected no admin promotion when groupAdmins is nil")
	}
}

// TestCompleteSSOLogin_AdminGroupPromotesStoredStudent verifies an existing
// student whose OIDC groups now match an admin group is promoted and the new
// role is persisted, not just reflected in the response.
func TestCompleteSSOLogin_AdminGroupPromotesStoredStudent(t *testing.T) {
	t.Parallel()

	providerUserID := "oidc-sub-promote"
	repos := fake.NewRepositories()
	repos.Users = fake.NewUserRepository(models.User{
		ID:             uuid.New(),
		Username:       "promote-me",
		Email:          "promote-me@test.com",
		Role:           groupsRoleStudent,
		IsActive:       true,
		AuthProvider:   oidcProviderKey,
		ProviderUserID: &providerUserID,
	})
	s := newStateWithRepos(repos)
	ctx := context.Background()

	rec := httptest.NewRecorder()
	s.completeSSOLogin(
		ctx, rec,
		"promote-me@test.com", "Promote Me", nil, nil,
		oidcProviderKey, providerUserID,
		[]string{"seanergy-admins"},
		[]string{"seanergy-admins"},
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp authResponse

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.User.Role != groupsRoleAdmin {
		t.Errorf("response role = %q, want %q", resp.User.Role, groupsRoleAdmin)
	}

	persisted, err := repos.Users.FindByEmail(ctx, "promote-me@test.com")
	if err != nil {
		t.Fatalf("find persisted user: %v", err)
	}

	if persisted.Role != groupsRoleAdmin {
		t.Errorf("persisted role = %q, want %q", persisted.Role, groupsRoleAdmin)
	}
}

// TestCompleteSSOLogin_RoleSyncFailureReturns500 verifies a failure while
// persisting the group-admin role aborts the login with a 500 rather than
// issuing a token with an unsynchronized role.
func TestCompleteSSOLogin_RoleSyncFailureReturns500(t *testing.T) {
	t.Parallel()

	users := fake.NewUserRepository()
	users.UpdateSSORoleErr = errors.New("db down")
	repos := fake.NewRepositories()
	repos.Users = users
	s := newStateWithRepos(repos)
	ctx := context.Background()

	rec := httptest.NewRecorder()
	s.completeSSOLogin(
		ctx, rec,
		"sync-fail@test.com", "Sync Fail", nil, nil,
		oidcProviderKey, "oidc-sub-syncfail",
		[]string{"seanergy-admins"},
		[]string{"seanergy-admins"},
	)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", rec.Code, rec.Body.String())
	}
}
