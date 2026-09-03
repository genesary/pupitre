package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"github.com/genesary/pupitre/user-service/fake"
	"github.com/genesary/pupitre/user-service/internal/config"
)

// pkceRecorder captures the form a token endpoint was called with, so a test
// can assert on the code_verifier that reached the provider.
type pkceRecorder struct {
	mu   sync.Mutex
	form url.Values
}

// record stores the form posted to the token endpoint.
func (p *pkceRecorder) record(values url.Values) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.form = values
}

// get returns the most recently recorded form, or nil if the token endpoint
// was never called.
func (p *pkceRecorder) get() url.Values {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.form
}

// githubRecordingStub wraps githubStub, capturing the token exchange form on
// its way through.
type githubRecordingStub struct {
	inner    githubStub
	recorder *pkceRecorder
}

// RoundTrip implements [http.RoundTripper].
func (g githubRecordingStub) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body != nil && strings.Contains(r.URL.Path, "/login/oauth/access_token") {
		raw, err := io.ReadAll(r.Body)
		if err == nil {
			values, parseErr := url.ParseQuery(string(raw))
			if parseErr == nil {
				g.recorder.record(values)
			}
		}
	}

	return g.inner.RoundTrip(r)
}

// pkceCookieFrom returns the PKCE cookie an /authorize response set, failing
// the test when there is none.
func pkceCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name == pkceCookieName {
			return c
		}
	}

	t.Fatalf("no %s cookie on the authorize response", pkceCookieName)

	return nil
}

// authURLFrom decodes the authorization URL from an /authorize response.
func authURLFrom(t *testing.T, rec *httptest.ResponseRecorder) *url.URL {
	t.Helper()

	var resp map[string]string

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode authorize response: %v", err)
	}

	parsed, err := url.Parse(resp[oauthJSONKeyURL])
	if err != nil {
		t.Fatalf("parse authorize URL %q: %v", resp[oauthJSONKeyURL], err)
	}

	return parsed
}

// ── /authorize issues the challenge and keeps the verifier in the browser ────

// TestOAuthAuthorize_GitHubPKCEChallenge checks that the hand-rolled GitHub
// authorization URL carries an S256 challenge derived from the verifier the
// browser was handed, and nothing more.
func TestOAuthAuthorize_GitHubPKCEChallenge(t *testing.T) {
	t.Parallel()

	rec := htDo(t, githubRouter(), http.MethodGet, "/api/auth/oauth/github/authorize", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	cookie := pkceCookieFrom(t, rec)
	query := authURLFrom(t, rec).Query()

	if got := query.Get(pkceParamMethod); got != pkceChallengeMethod {
		t.Errorf("%s = %q, want %q", pkceParamMethod, got, pkceChallengeMethod)
	}

	want := oauth2.S256ChallengeFromVerifier(cookie.Value)
	if got := query.Get(pkceParamChallenge); got != want {
		t.Errorf("%s = %q, want the S256 challenge of the cookie verifier", pkceParamChallenge, got)
	}

	if query.Has(pkceParamVerifier) {
		t.Error("the verifier must never be sent to the authorization endpoint")
	}
}

// TestOAuthAuthorize_PKCECookieAttributes pins the cookie attributes the
// whole scheme rests on: the verifier must be unreadable from JavaScript and
// scoped to the endpoints that issue and consume it.
func TestOAuthAuthorize_PKCECookieAttributes(t *testing.T) {
	t.Parallel()

	cookie := pkceCookieFrom(t, htDo(t, githubRouter(), http.MethodGet, "/api/auth/oauth/github/authorize", "", ""))

	if !cookie.HttpOnly {
		t.Error("PKCE cookie must be HttpOnly")
	}

	if cookie.Path != pkceCookiePath {
		t.Errorf("PKCE cookie path = %q, want %q", cookie.Path, pkceCookiePath)
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("PKCE cookie SameSite = %v, want Lax", cookie.SameSite)
	}

	if cookie.MaxAge <= 0 {
		t.Errorf("PKCE cookie MaxAge = %d, want a positive lifetime", cookie.MaxAge)
	}
}

// TestOAuthAuthorize_PKCECookieSecureOverHTTPS checks that the Secure
// attribute follows the redirect base rather than the inbound request, which
// arrives as plain HTTP behind a TLS-terminating ingress.
func TestOAuthAuthorize_PKCECookieSecureOverHTTPS(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
		OAuthStateSecret:  "oauth-state-secret",
		OAuthRedirectBase: "https://pupitre.example.com",
		Providers: []config.ProviderConfig{
			{ID: "github", Name: "GitHub", ClientID: "cid", ClientSecret: "csecret"},
		},
	}
	s := &State{Repos: fake.NewRepositories(), Config: cfg}

	rec := htDo(t, BuildRouter(s, cfg, false), http.MethodGet, "/api/auth/oauth/github/authorize", "", "")

	if !pkceCookieFrom(t, rec).Secure {
		t.Error("PKCE cookie must be Secure when the redirect base is https")
	}
}

// TestOAuthAuthorize_StateCarriesNoVerifier is the regression test for the
// flaw this feature was reworked to fix: the state parameter travels to the
// identity provider and back through the same front channel as the
// authorization code, so an interceptor who captures the code captures the
// state too. Committing only the challenge is what makes PKCE mean anything
// here.
func TestOAuthAuthorize_StateCarriesNoVerifier(t *testing.T) {
	t.Parallel()

	rec := htDo(t, githubRouter(), http.MethodGet, "/api/auth/oauth/github/authorize", "", "")
	cookie := pkceCookieFrom(t, rec)

	stateToken := authURLFrom(t, rec).Query().Get(oauthStateKey)

	parts := strings.Split(stateToken, ".")
	if len(parts) != 3 {
		t.Fatalf("state is not a JWT: %q", stateToken)
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode state payload: %v", err)
	}

	if strings.Contains(string(payload), cookie.Value) {
		t.Errorf("state payload leaks the code_verifier: %s", payload)
	}

	if !strings.Contains(string(payload), oauth2.S256ChallengeFromVerifier(cookie.Value)) {
		t.Errorf("state payload does not commit to the challenge: %s", payload)
	}
}

// TestOIDCAuthorize_PKCEChallenge covers the same ground for the dedicated
// OIDC flow, whose URL is built by oauth2.Config rather than by hand.
func TestOIDCAuthorize_PKCEChallenge(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	s := oidcTestState(idp)

	rec := htDo(t, BuildRouter(s, s.Config, false), http.MethodGet, "/api/auth/oidc/authorize", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	cookie := pkceCookieFrom(t, rec)
	query := authURLFrom(t, rec).Query()

	if got := query.Get(pkceParamMethod); got != pkceChallengeMethod {
		t.Errorf("%s = %q, want %q", pkceParamMethod, got, pkceChallengeMethod)
	}

	if got, want := query.Get(pkceParamChallenge), oauth2.S256ChallengeFromVerifier(cookie.Value); got != want {
		t.Errorf("%s = %q, want the S256 challenge of the cookie verifier", pkceParamChallenge, got)
	}
}

// ── the verifier reaches the token exchange ──────────────────────────────────

// TestOIDCCallback_SendsCodeVerifier asserts the verifier held by the browser
// is what the token exchange presents to the provider.
func TestOIDCCallback_SendsCodeVerifier(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	idp.idTokenClaims = jwt.MapClaims{"sub": "verifier-user", "email": "verifier.user@example.com"}

	s := oidcTestState(idp)
	state, pkce := htPKCE(t, oidcProviderKey, s.oauthStateSecret())

	rec := htDoPKCE(t, BuildRouter(s, s.Config, false), "/api/auth/oidc/callback",
		`{"code":"x","state":"`+state+`"}`, pkce)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	form := idp.lastTokenForm()
	if form == nil {
		t.Fatal("the token endpoint was never called")
	}

	if got := form.Get(pkceParamVerifier); got != pkce.Value {
		t.Errorf("%s = %q, want the verifier from the browser cookie", pkceParamVerifier, got)
	}
}

// TestOAuthCallback_GitHubSendsCodeVerifier covers the same for GitHub, whose
// token request is built by hand.
func TestOAuthCallback_GitHubSendsCodeVerifier(t *testing.T) { //nolint:paralleltest // swaps the shared outbound transport
	recorder := &pkceRecorder{}
	withStubTransport(t, githubRecordingStub{
		inner: githubStub{
			token:   `{"access_token":"gho_abc"}`,
			profile: `{"id":42,"login":"octocat","email":"octo@example.com"}`,
		},
		recorder: recorder,
	})

	state, pkce := htPKCE(t, "github", "oauth-state-secret")

	rec := htDoPKCE(t, githubRouter(), "/api/auth/oauth/callback",
		`{"code":"any","state":"`+state+`"}`, pkce)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	form := recorder.get()
	if form == nil {
		t.Fatal("GitHub's token endpoint was never called")
	}

	if got := form.Get(pkceParamVerifier); got != pkce.Value {
		t.Errorf("%s = %q, want the verifier from the browser cookie", pkceParamVerifier, got)
	}
}

// ── a callback that cannot produce the verifier is refused ───────────────────

// TestOIDCCallback_MissingPKCECookie is the threat from the issue: an
// interceptor holds a genuine code and state, but not the cookie, and must
// not be able to trade them for a session. No code may be exchanged at all.
func TestOIDCCallback_MissingPKCECookie(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	idp.idTokenClaims = jwt.MapClaims{"sub": "stolen-code-user", "email": "victim@example.com"}

	s := oidcTestState(idp)
	state, _ := htPKCE(t, oidcProviderKey, s.oauthStateSecret())

	rec := htDoPKCE(t, BuildRouter(s, s.Config, false), "/api/auth/oidc/callback",
		`{"code":"intercepted","state":"`+state+`"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without the PKCE cookie, got %d: %s", rec.Code, rec.Body.String())
	}

	if idp.lastTokenForm() != nil {
		t.Error("the code was exchanged despite the missing verifier")
	}
}

// TestOIDCCallback_MismatchedVerifier rejects a cookie from some other flow,
// which is what an attacker who can set a cookie of their own would present.
func TestOIDCCallback_MismatchedVerifier(t *testing.T) {
	t.Parallel()

	idp := newMockIdP(t)
	idp.idTokenClaims = jwt.MapClaims{"sub": "mismatch-user", "email": "victim@example.com"}

	s := oidcTestState(idp)
	state, _ := htPKCE(t, oidcProviderKey, s.oauthStateSecret())
	_, otherFlow := htPKCE(t, oidcProviderKey, s.oauthStateSecret())

	rec := htDoPKCE(t, BuildRouter(s, s.Config, false), "/api/auth/oidc/callback",
		`{"code":"intercepted","state":"`+state+`"}`, otherFlow)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for a verifier from another flow, got %d: %s", rec.Code, rec.Body.String())
	}

	if idp.lastTokenForm() != nil {
		t.Error("the code was exchanged despite the mismatched verifier")
	}
}

// TestOAuthCallback_MissingPKCECookie refuses the GitHub flow the same way.
// GitHub's OAuth apps ignore code_challenge, so this local check is the only
// thing standing between an intercepted code and a session.
func TestOAuthCallback_MissingPKCECookie(t *testing.T) { //nolint:paralleltest // swaps the shared outbound transport
	recorder := &pkceRecorder{}
	withStubTransport(t, githubRecordingStub{
		inner: githubStub{
			token:   `{"access_token":"gho_abc"}`,
			profile: `{"id":42,"login":"octocat","email":"octo@example.com"}`,
		},
		recorder: recorder,
	})

	state, _ := htPKCE(t, "github", "oauth-state-secret")

	rec := htDoPKCE(t, githubRouter(), "/api/auth/oauth/callback",
		`{"code":"intercepted","state":"`+state+`"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without the PKCE cookie, got %d: %s", rec.Code, rec.Body.String())
	}

	if recorder.get() != nil {
		t.Error("the code was exchanged despite the missing verifier")
	}
}

// TestOAuthCallback_ClearsPKCECookie checks the cookie is expired on the way
// out, so a verifier cannot back a second exchange.
func TestOAuthCallback_ClearsPKCECookie(t *testing.T) {
	t.Parallel()

	state, pkce := htPKCE(t, "unknown-provider", "oauth-state-secret")

	rec := htDoPKCE(t, githubRouter(), "/api/auth/oauth/callback",
		`{"code":"x","state":"`+state+`"}`, pkce)

	cleared := pkceCookieFrom(t, rec)
	if cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Errorf("PKCE cookie not cleared: value=%q maxAge=%d", cleared.Value, cleared.MaxAge)
	}
}

// ── consumePKCEVerifier unit cases ───────────────────────────────────────────

// TestConsumePKCEVerifier covers each way the callback's verifier check can
// resolve, including a state token that predates PKCE and therefore commits
// to no challenge at all.
func TestConsumePKCEVerifier(t *testing.T) {
	t.Parallel()

	verifier := oauth2.GenerateVerifier()
	challenge := oauth2.S256ChallengeFromVerifier(verifier)

	cases := []struct {
		name      string
		cookie    string
		challenge string
		want      bool
	}{
		{name: "matching verifier", cookie: verifier, challenge: challenge, want: true},
		{name: "no cookie", cookie: "", challenge: challenge, want: false},
		{name: "wrong verifier", cookie: oauth2.GenerateVerifier(), challenge: challenge, want: false},
		{name: "state commits to no challenge", cookie: verifier, challenge: "", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/auth/oauth/callback", http.NoBody)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: pkceCookieName, Value: tc.cookie})
			}

			rec := httptest.NewRecorder()

			got, ok := consumePKCEVerifier(rec, req, "http://localhost:3000", tc.challenge)
			if ok != tc.want {
				t.Fatalf("ok = %v, want %v", ok, tc.want)
			}

			if tc.want && got != tc.cookie {
				t.Errorf("verifier = %q, want %q", got, tc.cookie)
			}

			if !tc.want && got != "" {
				t.Errorf("verifier = %q, want empty on failure", got)
			}
		})
	}
}
