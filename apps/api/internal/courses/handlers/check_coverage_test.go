package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/fake"
)

// fakeLabChecks type-asserts a State's lab-check repository back to the
// in-memory fake so a test can force it to fail.
func fakeLabChecks(t *testing.T, s *State) *fake.LabCheckRepository {
	t.Helper()

	repo, ok := s.Repos.LabChecks.(*fake.LabCheckRepository)
	if !ok {
		t.Fatalf("LabChecks is %T, not the fake", s.Repos.LabChecks)
	}

	return repo
}

// labCheckFixture returns a State whose single lab module points at a git
// fixture carrying check.yaml + policy, wired to checkerURL.
func labCheckFixture(t *testing.T, checkerURL string) *State {
	t.Helper()

	repoDir := gitFixture(t, map[string]string{
		"labs/mr/module.md": "# lab",
		"labs/mr/check.yaml": "provider: opa\nproject: \"e-learning/x\"\n" +
			"files: [\"lab.py\"]\npolicy: policy.rego\n",
		"labs/mr/policy.rego": "package checker.lab\n\ndefault allow := true\n",
	})

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}

	return withPeers(newStateWith(cfg, &content.Course{
		Slug: "devops-101", Title: "DevOps 101", IsPublic: true,
		Modules: []content.Module{
			{Name: "MR lab", Type: "lab", Src: repoDir, Ref: "main", Path: "labs/mr/module.md"},
		},
	}), "", checkerURL)
}

// postCheck fires the module-check endpoint for module 0 of devops-101.
func postCheck(t *testing.T, s *State) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/courses/devops-101/modules/0/check", strings.NewReader("{}"))
	req.Header.Set("Authorization", authHeader(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// TestCheckModule_CheckerUnreachable maps a dial failure to checker-service
// onto a 500 (callCheckerRoute transport-error branch).
func TestCheckModule_CheckerUnreachable(t *testing.T) {
	t.Parallel()

	// 127.0.0.1:1 is reliably closed.
	s := labCheckFixture(t, "http://127.0.0.1:1")

	rec := postCheck(t, s)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 on unreachable checker, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCheckModule_CheckerReturnsError surfaces checker-service's own error
// body as a 502 (callCheckerRoute non-200 branch).
func TestCheckModule_CheckerReturnsError(t *testing.T) {
	t.Parallel()

	checker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"policy compilation failed"}`))
	}))
	defer checker.Close()

	s := labCheckFixture(t, checker.URL)

	rec := postCheck(t, s)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "policy compilation failed") {
		t.Errorf("checker error message not forwarded: %s", rec.Body.String())
	}
}

// TestCheckModule_CheckerBadJSON maps an undecodable 200 body to a 500
// (callCheckerRoute decode branch).
func TestCheckModule_CheckerBadJSON(t *testing.T) {
	t.Parallel()

	checker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer checker.Close()

	s := labCheckFixture(t, checker.URL)

	rec := postCheck(t, s)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 on garbage checker body, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCheckModule_StoreFailureIsSwallowed keeps the check response a 200 even
// when persisting the verdict fails (storeLabCheck warn branch).
func TestCheckModule_StoreFailureIsSwallowed(t *testing.T) {
	t.Parallel()

	checker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"allow":true,"violations":null}`))
	}))
	defer checker.Close()

	s := labCheckFixture(t, checker.URL)
	fakeLabChecks(t, s).Err = errors.New("lab_checks table is gone")

	rec := postCheck(t, s)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 despite a persistence failure, got %d: %s", rec.Code, rec.Body.String())
	}
}
