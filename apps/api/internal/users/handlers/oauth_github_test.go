package handlers

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// rtFunc adapts a function to the [http.RoundTripper] interface.
type rtFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements [http.RoundTripper].
func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// jsonResponse builds a 200 JSON response carrying body.
func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// TestGithubProfileMedia extracts the avatar and bio pointers when present
// and leaves them nil otherwise.
func TestGithubProfileMedia(t *testing.T) {
	t.Parallel()

	avatar, bio := githubProfileMedia(map[string]any{
		"avatarUrl": "https://avatars.example/u.png",
		"bio":       "builder of things",
	})
	if avatar == nil || *avatar != "https://avatars.example/u.png" {
		t.Errorf("avatar = %v", avatar)
	}

	if bio == nil || *bio != "builder of things" {
		t.Errorf("bio = %v", bio)
	}

	avatar, bio = githubProfileMedia(map[string]any{"avatarUrl": "", "bio": ""})
	if avatar != nil || bio != nil {
		t.Errorf("expected nil pointers for empty fields, got %v / %v", avatar, bio)
	}

	avatar, bio = githubProfileMedia(map[string]any{})
	if avatar != nil || bio != nil {
		t.Errorf("expected nil pointers for absent fields, got %v / %v", avatar, bio)
	}
}

// TestGithubPrimaryEmail prefers the primary, verified address and falls
// back to the first entry.
func TestGithubPrimaryEmail(t *testing.T) {
	t.Parallel()

	primaryVerified := `[
		{"email":"secondary@example.com","primary":false,"verified":true},
		{"email":"primary@example.com","primary":true,"verified":true}
	]`
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Path, "/user/emails") {
			t.Errorf("unexpected request path %q", r.URL.Path)
		}

		return jsonResponse(primaryVerified), nil
	})}

	if got := githubPrimaryEmail(client, "tok"); got != "primary@example.com" {
		t.Errorf("primary verified = %q, want primary@example.com", got)
	}

	// No primary+verified entry -> first address is returned.
	fallback := `[{"email":"only@example.com","primary":false,"verified":false}]`
	client = &http.Client{Transport: rtFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(fallback), nil
	})}

	if got := githubPrimaryEmail(client, "tok"); got != "only@example.com" {
		t.Errorf("fallback = %q, want only@example.com", got)
	}

	// Empty list -> empty string.
	client = &http.Client{Transport: rtFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(`[]`), nil
	})}

	if got := githubPrimaryEmail(client, "tok"); got != "" {
		t.Errorf("empty list = %q, want empty", got)
	}
}
