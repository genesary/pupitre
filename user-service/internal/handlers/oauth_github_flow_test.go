package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/genesary/pupitre/internal/httpx"
	"github.com/genesary/pupitre/user-service/fake"
	"github.com/genesary/pupitre/user-service/internal/config"
)

// githubStub is an [http.RoundTripper] that answers GitHub's OAuth token and
// user/email endpoints with canned JSON.
type githubStub struct {
	token       string
	profile     string
	emails      string
	failToken   bool
	failProfile bool
}

// RoundTrip implements [http.RoundTripper].
func (g githubStub) RoundTrip(r *http.Request) (*http.Response, error) {
	body := "{}"
	status := http.StatusOK

	switch {
	case strings.Contains(r.URL.Host, "github.com") && strings.Contains(r.URL.Path, "/login/oauth/access_token"):
		if g.failToken {
			status = http.StatusBadGateway
		}

		body = g.token
	case strings.HasSuffix(r.URL.Path, "/user/emails"):
		body = g.emails
	case strings.HasSuffix(r.URL.Path, "/user"):
		if g.failProfile {
			status = http.StatusUnauthorized
		}

		body = g.profile
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

// withStubTransport swaps the transport of the shared outbound client for
// the duration of the test, so the GitHub calls are answered by rt.
func withStubTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()

	client := httpx.Client()
	prev := client.Transport
	client.Transport = rt

	t.Cleanup(func() { client.Transport = prev })
}

// githubRouter builds a router with a single configured "github" provider.
func githubRouter() http.Handler {
	repos := fake.NewRepositories()
	cfg := &config.Config{
		JWTSecret: htSecret, JWTExpiryH: htExpiry, CORSOrigins: []string{"*"},
		OAuthStateSecret:  "oauth-state-secret",
		OAuthRedirectBase: "http://localhost:3000",
		Providers: []config.ProviderConfig{
			{ID: "github", Name: "GitHub", ClientID: "cid", ClientSecret: "csecret"},
		},
	}
	s := &State{Repos: repos, Config: cfg}

	return BuildRouter(s, cfg, false)
}

// TestOAuthCallback_GitHubFlow completes a GitHub login where the email
// comes from the profile payload directly.
func TestOAuthCallback_GitHubFlow(t *testing.T) { //nolint:paralleltest // swaps the shared outbound transport
	withStubTransport(t, githubStub{
		token:   `{"access_token":"gho_abc","token_type":"bearer"}`,
		profile: `{"id":42,"login":"octocat","name":"The Octocat","email":"octo@example.com","avatarUrl":"https://a/o.png","bio":"cat"}`,
	})

	r := githubRouter()
	s := &State{Config: &config.Config{OAuthStateSecret: "oauth-state-secret"}}

	state, pkce := htPKCE(t, "github", s.oauthStateSecret())

	rec := htDoPKCE(t, r, "/api/auth/oauth/callback",
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

	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Token == "" || resp.User.Email != "octo@example.com" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

// TestOAuthCallback_GitHubEmailFromAPI completes the flow when the profile
// has no email and it must be fetched from /user/emails.
func TestOAuthCallback_GitHubEmailFromAPI(t *testing.T) { //nolint:paralleltest // swaps the shared outbound transport
	withStubTransport(t, githubStub{
		token:   `{"access_token":"gho_abc"}`,
		profile: `{"id":7,"login":"noemail","name":"No Email"}`,
		emails:  `[{"email":"primary@example.com","primary":true,"verified":true}]`,
	})

	r := githubRouter()
	s := &State{Config: &config.Config{OAuthStateSecret: "oauth-state-secret"}}
	state, pkce := htPKCE(t, "github", s.oauthStateSecret())

	rec := htDoPKCE(t, r, "/api/auth/oauth/callback",
		`{"code":"any","state":"`+state+`"}`, pkce)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestOAuthCallback_GitHubNoToken fails when GitHub returns no access token.
func TestOAuthCallback_GitHubNoToken(t *testing.T) { //nolint:paralleltest // swaps the shared outbound transport
	withStubTransport(t, githubStub{token: `{"error":"bad_verification_code"}`})

	r := githubRouter()
	s := &State{Config: &config.Config{OAuthStateSecret: "oauth-state-secret"}}
	state, pkce := htPKCE(t, "github", s.oauthStateSecret())

	rec := htDoPKCE(t, r, "/api/auth/oauth/callback",
		`{"code":"any","state":"`+state+`"}`, pkce)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestOAuthCallback_GitHubNoVerifiedEmail fails when neither the profile nor
// the emails API yields a usable address.
func TestOAuthCallback_GitHubNoVerifiedEmail(t *testing.T) { //nolint:paralleltest // swaps the shared outbound transport
	withStubTransport(t, githubStub{
		token:   `{"access_token":"gho_abc"}`,
		profile: `{"id":9,"login":"ghost"}`,
		emails:  `[]`,
	})

	r := githubRouter()
	s := &State{Config: &config.Config{OAuthStateSecret: "oauth-state-secret"}}
	state, pkce := htPKCE(t, "github", s.oauthStateSecret())

	rec := htDoPKCE(t, r, "/api/auth/oauth/callback",
		`{"code":"any","state":"`+state+`"}`, pkce)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}
