package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
)

// testStateNonce is the nonce of the states the tests make.
const testStateNonce = "test-state-nonce"

// makeOAuthState makes a state as startOAuthState does, with testStateNonce;
// htDoOAuth sends the matching cookie.
func makeOAuthState(provider, secret string) (string, error) {
	return makeOAuthStateWithNonce(provider, testStateNonce, secret)
}

// decodeOAuthState verifies a state and returns its provider.
func decodeOAuthState(stateToken, secret string) (string, bool) {
	provider, _, ok := decodeOAuthStateNonce(stateToken, secret)

	return provider, ok
}

// htDoOAuth is htDo for an OAuth or OIDC callback: it sends the state cookie
// of the browser that started the flow, read from the body's state.
func htDoOAuth(t *testing.T, handler http.Handler, method, path, body, auth string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	withStateCookie(req, body)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

// withStateCookie adds the state cookie matching the state of a callback
// body, when it has one.
func withStateCookie(req *http.Request, body string) {
	var payload struct {
		State string `json:"state"`
	}

	if json.Unmarshal([]byte(body), &payload) != nil || payload.State == "" {
		return
	}

	claims := &oauthStateClaims{}

	_, _, err := jwt.NewParser().ParseUnverified(payload.State, claims)
	if err != nil || claims.Nonce == "" {
		return
	}

	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: claims.Nonce})
}

// stateCookie returns the state cookie set by a response.
func stateCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == oauthStateCookie {
			return cookie
		}
	}

	t.Fatalf("no %s cookie in %v", oauthStateCookie, rec.Header().Values("Set-Cookie"))

	return nil
}

// TestOAuthState_BoundToTheBrowser is the regression test of defect B13
// (docs/specs/190-backend-defects.md): a callback accepts a state only from
// the browser that started the flow.
func TestOAuthState_BoundToTheBrowser(t *testing.T) {
	t.Parallel()

	s := &State{Config: &config.Config{OAuthStateSecret: "oauth-state-secret-for-the-tests!!"}}

	start := httptest.NewRecorder()

	state, err := s.startOAuthState(start, "oidc", "https://pupitre.example")
	if err != nil {
		t.Fatalf("startOAuthState: %v", err)
	}

	cookie := stateCookie(t, start)
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/api/auth" {
		t.Errorf("cookie = %+v, want HttpOnly, Secure, SameSite=Lax, Path=/api/auth", cookie)
	}

	callback := func(cookies ...*http.Cookie) (*httptest.ResponseRecorder, bool) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/auth/oidc/callback", http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}

		rec := httptest.NewRecorder()
		provider, ok := s.checkOAuthState(rec, req, state)

		return rec, ok && provider == "oidc"
	}

	_, ok := callback()
	if ok {
		t.Error("a state without its cookie was accepted (login CSRF)")
	}

	_, ok = callback(&http.Cookie{Name: oauthStateCookie, Value: "another-browser"})
	if ok {
		t.Error("a state with another browser's cookie was accepted")
	}

	rec, ok := callback(&http.Cookie{Name: oauthStateCookie, Value: cookie.Value})
	if !ok {
		t.Fatal("the state of this browser was refused")
	}

	if cleared := stateCookie(t, rec); cleared.MaxAge >= 0 {
		t.Errorf("the cookie is not cleared after use: %+v", cleared)
	}

	// A state made without a nonce (as before the fix) is refused.
	old, _ := makeOAuthStateWithNonce("oidc", "", s.oauthStateSecret())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/auth/oidc/callback", http.NoBody)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: ""})

	_, ok = s.checkOAuthState(httptest.NewRecorder(), req, old)
	if ok {
		t.Error("a state without a nonce was accepted")
	}
}

// TestOAuthState_NotSecureOverHTTP keeps the cookie usable in local
// development, served over plain HTTP.
func TestOAuthState_NotSecureOverHTTP(t *testing.T) {
	t.Parallel()

	s := &State{Config: &config.Config{OAuthStateSecret: "oauth-state-secret-for-the-tests!!"}}
	rec := httptest.NewRecorder()

	_, err := s.startOAuthState(rec, "oidc", "http://localhost:3000")
	if err != nil {
		t.Fatalf("startOAuthState: %v", err)
	}

	if stateCookie(t, rec).Secure {
		t.Error("the cookie is Secure over plain HTTP: the browser would not send it back")
	}
}

// TestOAuthCallback_WithoutStateCookie checks the routes: a callback whose
// state is valid but comes without the cookie is refused.
func TestOAuthCallback_WithoutStateCookie(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	state, _ := makeOAuthState("oidc", htSecret)

	for _, path := range []string{"/api/auth/oidc/callback", "/api/auth/oauth/callback"} {
		rec := htDo(t, r, http.MethodPost, path, `{"code":"c","state":"`+state+`"}`, "")
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "state") {
			t.Errorf("%s without the state cookie: %d %s, want 401", path, rec.Code, rec.Body.String())
		}
	}
}
