package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/genesary/pupitre/user-service/fake"
	"github.com/genesary/pupitre/user-service/internal/config"
	"github.com/genesary/pupitre/user-service/internal/models"
)

// mockIdP is a minimal OIDC provider: discovery, JWKS, a token endpoint that
// returns a signed id_token carrying idTokenClaims, and a UserInfo endpoint
// that returns userInfoClaims (when non-nil).
type mockIdP struct {
	server         *httptest.Server
	key            *rsa.PrivateKey
	kid            string
	idTokenClaims  jwt.MapClaims
	userInfoClaims map[string]any

	// mu guards tokenForm, which the token endpoint fills from the
	// server's goroutine and PKCE tests read from the test's.
	mu        sync.Mutex
	tokenForm url.Values
}

// lastTokenForm returns the form posted to the token endpoint by the most
// recent exchange, or nil if none has happened.
func (m *mockIdP) lastTokenForm() url.Values {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.tokenForm
}

// newMockIdP starts the provider. The caller sets idTokenClaims (at least
// iss/aud are filled in automatically) before triggering the token exchange.
func newMockIdP(t *testing.T) *mockIdP {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}

	idp := &mockIdP{key: key, kid: "test-key-1", idTokenClaims: jwt.MapClaims{}}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		base := idp.server.URL
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                base,
			"authorization_endpoint":                base + "/authorize",
			"token_endpoint":                        base + "/token",
			"jwks_uri":                              base + "/jwks",
			"userinfo_endpoint":                     base + "/userinfo",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, _ *http.Request) {
		if idp.userInfoClaims == nil {
			http.Error(w, "no userinfo", http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(idp.userInfoClaims)
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		pub := idp.key.PublicKey
		n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": idp.kid, "n": n, "e": e},
			},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()

		idp.mu.Lock()
		idp.tokenForm = r.Form
		idp.mu.Unlock()

		claims := jwt.MapClaims{
			"iss": idp.server.URL,
			"aud": "test-client",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}
		maps.Copy(claims, idp.idTokenClaims)

		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = idp.kid

		signed, err := tok.SignedString(idp.key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "mock-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     signed,
		})
	})

	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)

	return idp
}

// oidcTestState builds a State whose settings enable OIDC against idp.
func oidcTestState(idp *mockIdP) *State {
	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: "oidc_enabled", Value: "true"},
		models.PlatformSetting{Key: "oidc_provider_url", Value: idp.server.URL},
		models.PlatformSetting{Key: "oidc_client_id", Value: "test-client"},
		models.PlatformSetting{Key: "oidc_client_secret", Value: "test-secret"},
		models.PlatformSetting{Key: "oidc_group_claim", Value: "groups"},
		models.PlatformSetting{Key: "oidc_scopes", Value: "openid email profile groups"},
		models.PlatformSetting{Key: "oidc_redirect_base", Value: "http://localhost:3000"},
	)

	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
		InternalSecret: htInternalSecret, OAuthStateSecret: "oauth-state-secret-key",
		OAuthRedirectBase: "http://localhost:3000",
	}

	return &State{Repos: repos, Config: cfg}
}

// TestOIDCCallback_FullFlow drives a complete OIDC login against the mock
// provider: discovery, token exchange, JWKS verification, user upsert and
// group sync, ending in an app JWT.
func TestOIDCCallback_FullFlow(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	idp.idTokenClaims = jwt.MapClaims{
		"sub":     "oidc-user-42",
		"email":   "sso.user@example.com",
		"name":    "SSO User",
		"picture": "https://cdn.example/sso.png",
		"groups":  []any{"engineering", "staff"},
	}

	s := oidcTestState(idp)
	router := BuildRouter(s, s.Config, false)

	state, pkce := htPKCE(t, oidcProviderKey, s.oauthStateSecret())

	body := `{"code":"any-code","state":"` + state + `"}`
	rec := htDoPKCE(t, router, "/api/auth/oidc/callback", body, pkce)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Token string `json:"token"`
		User  struct {
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"user"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Token == "" {
		t.Error("expected an app JWT in the response")
	}

	if resp.User.Email != "sso.user@example.com" {
		t.Errorf("user email = %q", resp.User.Email)
	}
}

// TestOAuthCallback_GenericOIDCFlow drives OAuthCallback for a generic OIDC
// provider (issuerUrl set) against the mock IdP, covering fetchOIDCProvider
// and oauthFinishLogin.
func TestOAuthCallback_GenericOIDCFlow(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	idp.idTokenClaims = jwt.MapClaims{
		"sub":   "kc-user-7",
		"email": "keycloak.user@example.com",
		"name":  "Keycloak User",
	}

	repos := fake.NewRepositories()
	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
		OAuthStateSecret:  "oauth-state-secret-key",
		OAuthRedirectBase: "http://localhost:3000",
		Providers: []config.ProviderConfig{
			{ID: "keycloak", Name: "Keycloak", ClientID: "test-client", ClientSecret: "test-secret", IssuerURL: idp.server.URL},
		},
	}
	s := &State{Repos: repos, Config: cfg}
	router := BuildRouter(s, cfg, false)

	state, pkce := htPKCE(t, "keycloak", s.oauthStateSecret())

	rec := htDoPKCE(t, router, "/api/auth/oauth/callback",
		`{"code":"any","state":"`+state+`"}`, pkce)
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

	if resp.Token == "" || resp.User.Email != "keycloak.user@example.com" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

// TestOAuthCallback_UnknownProvider rejects a state for a provider that is
// not configured.
func TestOAuthCallback_UnknownProvider(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
		OAuthStateSecret: "oauth-state-secret-key",
	}
	s := &State{Repos: repos, Config: cfg}
	router := BuildRouter(s, cfg, false)

	state, pkce := htPKCE(t, "nope", s.oauthStateSecret())
	rec := htDoPKCE(t, router, "/api/auth/oauth/callback",
		`{"code":"x","state":"`+state+`"}`, pkce)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestOIDCCallback_BadState rejects a state token not issued for OIDC.
func TestOIDCCallback_BadState(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	s := oidcTestState(idp)
	router := BuildRouter(s, s.Config, false)

	rec := htDo(t, router, http.MethodPost, "/api/auth/oidc/callback",
		`{"code":"x","state":"not-a-valid-state"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestOIDCCallback_NoEmailClaim rejects an id_token that carries no email.
func TestOIDCCallback_NoEmailClaim(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	idp.idTokenClaims = jwt.MapClaims{"sub": "no-email-user"}

	s := oidcTestState(idp)
	router := BuildRouter(s, s.Config, false)

	state, pkce := htPKCE(t, oidcProviderKey, s.oauthStateSecret())
	rec := htDoPKCE(t, router, "/api/auth/oidc/callback",
		`{"code":"x","state":"`+state+`"}`, pkce)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestOIDCCallback_Disabled returns 400 when OIDC is not configured.
func TestOIDCCallback_Disabled(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
		OAuthStateSecret: "oauth-state-secret-key",
	}
	s := &State{Repos: repos, Config: cfg}
	router := BuildRouter(s, cfg, false)

	state, pkce := htPKCE(t, oidcProviderKey, s.oauthStateSecret())
	rec := htDoPKCE(t, router, "/api/auth/oidc/callback",
		`{"code":"x","state":"`+state+`"}`, pkce)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestOIDCAuthorize_ReturnsProviderURL builds the authorization redirect
// against the mock provider's discovery document.
func TestOIDCAuthorize_ReturnsProviderURL(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	s := oidcTestState(idp)
	router := BuildRouter(s, s.Config, false)

	rec := htDo(t, router, http.MethodGet, "/api/auth/oidc/authorize", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["url"] == "" && resp["authorizeUrl"] == "" && resp["authorize_url"] == "" {
		t.Errorf("expected an authorize URL in %v", resp)
	}
}

// TestOIDCCallback_StringGroupsClaim covers the comma-separated string form
// of oidcGroupsFromClaims and the split-horizon issuer-URL branch of
// oidcContext.
func TestOIDCCallback_StringGroupsClaim(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	idp.idTokenClaims = jwt.MapClaims{
		"sub":    "sg-1",
		"email":  "string.groups@example.com",
		"groups": "engineering, platform ,",
	}

	repos := fake.NewRepositories()
	repos.Settings = fake.NewSettingRepository(
		models.PlatformSetting{Key: "oidc_enabled", Value: "true"},
		models.PlatformSetting{Key: "oidc_provider_url", Value: idp.server.URL},
		models.PlatformSetting{Key: "oidc_issuer_url", Value: idp.server.URL},
		models.PlatformSetting{Key: "oidc_client_id", Value: "test-client"},
		models.PlatformSetting{Key: "oidc_client_secret", Value: "test-secret"},
		models.PlatformSetting{Key: "oidc_group_claim", Value: "groups"},
		models.PlatformSetting{Key: "oidc_redirect_base", Value: "http://localhost:3000"},
	)

	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
		OAuthStateSecret: "oauth-state-secret-key", OAuthRedirectBase: "http://localhost:3000",
	}
	s := &State{Repos: repos, Config: cfg}
	router := BuildRouter(s, cfg, false)

	state, pkce := htPKCE(t, oidcProviderKey, s.oauthStateSecret())
	rec := htDoPKCE(t, router, "/api/auth/oidc/callback",
		`{"code":"x","state":"`+state+`"}`, pkce)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestOidcGroupsFromClaims_Direct covers the missing-claim path.
func TestOidcGroupsFromClaims_Direct(t *testing.T) {
	t.Parallel()

	if got := oidcGroupsFromClaims(map[string]any{}, "groups"); got != nil {
		t.Errorf("missing claim = %v, want nil", got)
	}

	got := oidcGroupsFromClaims(map[string]any{"groups": []any{"a", 3, "b"}}, "groups")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("list claim filtered wrongly: %v", got)
	}
}

// TestOAuthAuthorize_GenericOIDC builds the authorization URL for a generic
// OIDC provider via discovery against the mock IdP.
func TestOAuthAuthorize_GenericOIDC(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)

	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
		OAuthStateSecret:  "oauth-state-secret-key",
		OAuthRedirectBase: "http://localhost:3000",
		Providers: []config.ProviderConfig{
			{ID: "keycloak", Name: "Keycloak", ClientID: "kc", ClientSecret: "sec", IssuerURL: idp.server.URL},
		},
	}
	s := &State{Repos: fake.NewRepositories(), Config: cfg}
	r := BuildRouter(s, cfg, false)

	rec := htDo(t, r, http.MethodGet, "/api/auth/oauth/keycloak/authorize", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["url"] == "" || !strings.Contains(resp["url"], "/authorize") {
		t.Errorf("unexpected authorize URL: %q", resp["url"])
	}
}

// TestOIDCCallback_UserInfoEnrichment resolves the email from the UserInfo
// endpoint when the id_token omits it, covering enrichClaimsFromUserInfo's
// merge path.
func TestOIDCCallback_UserInfoEnrichment(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	idp.idTokenClaims = jwt.MapClaims{"sub": "ui-1"} // no email in the token
	idp.userInfoClaims = map[string]any{
		"sub":   "ui-1",
		"email": "from.userinfo@example.com",
		"name":  "UserInfo Person",
	}

	s := oidcTestState(idp)
	router := BuildRouter(s, s.Config, false)

	state, pkce := htPKCE(t, oidcProviderKey, s.oauthStateSecret())
	rec := htDoPKCE(t, router, "/api/auth/oidc/callback",
		`{"code":"x","state":"`+state+`"}`, pkce)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 (email came from UserInfo), got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}

	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.User.Email != "from.userinfo@example.com" {
		t.Errorf("email = %q, want the UserInfo value", resp.User.Email)
	}
}
