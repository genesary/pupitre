package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// gitFixture creates a throwaway local git repo on branch main containing the
// given path→content files, and returns its on-disk path (usable as a clone
// URL by go-git).
func gitFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Point HEAD at main before the first commit so it lands on refs/heads/main
	// and a SingleBranch clone of "main" succeeds.
	err = repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main"))
	if err != nil {
		t.Fatalf("set HEAD: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	for rel, body := range files {
		full := filepath.Join(dir, rel)

		err = os.MkdirAll(filepath.Dir(full), 0o750)
		if err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		err = os.WriteFile(full, []byte(body), 0o600)
		if err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}

		_, err = wt.Add(rel)
		if err != nil {
			t.Fatalf("git add %s: %v", rel, err)
		}
	}

	_, err = wt.Commit("fixture", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t.test", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return dir
}

// TestCheckModule_FullFlow clones the policy fixture, parses check.yaml,
// calls the mock checker-service and persists the verdict.
func TestCheckModule_FullFlow(t *testing.T) {
	t.Parallel()

	repoDir := gitFixture(t, map[string]string{
		"labs/mr/module.md": "# lab",
		"labs/mr/check.yaml": "provider: opa\nproject: \"e-learning/{{ .Username }}\"\n" +
			"files: [\"lab.py\"]\npolicy: policy.rego\n",
		"labs/mr/policy.rego": "package checker.lab\n\ndefault allow := true\n",
	})

	var checkerBody map[string]any

	checker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&checkerBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"allow":true,"violations":[]}`))
	}))
	defer checker.Close()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, &content.Course{
		Slug: "devops-101", Title: "DevOps 101", IsPublic: true,
		Modules: []content.Module{
			{
				Name: "MR lab", Type: "lab",
				Src: repoDir, Ref: "main", Path: "labs/mr/module.md",
			},
		},
	}), "", checker.URL)

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/courses/devops-101/modules/0/check", strings.NewReader("{}"))
	req.Header.Set("Authorization", authHeader(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp CheckResponse

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.Allow {
		t.Errorf("Allow = false, want true")
	}

	// The checker request must carry the resolved policy path and project.
	if checkerBody["policyPath"] != "labs/mr/policy.rego" {
		t.Errorf("policyPath = %v", checkerBody["policyPath"])
	}

	if p, _ := checkerBody["project"].(string); !strings.HasPrefix(p, "e-learning/") {
		t.Errorf("project template not resolved: %v", checkerBody["project"])
	}

	// The verdict must have been persisted.
	checks, err := s.Repos.LabChecks.List(t.Context(), "devops-101")
	if err != nil || len(checks) != 1 || !checks[0].Verified {
		t.Errorf("lab check not stored: %+v (err %v)", checks, err)
	}
}

// TestCheckModule_NoCheckYAML returns 404 when the git dir has no check.yaml.
func TestCheckModule_NoCheckYAML(t *testing.T) {
	t.Parallel()

	repoDir := gitFixture(t, map[string]string{"labs/mr/module.md": "# lab"})

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, &content.Course{
		Slug: "devops-101", IsPublic: true,
		Modules: []content.Module{
			{Name: "MR lab", Type: "lab", Src: repoDir, Ref: "main", Path: "labs/mr/module.md"},
		},
	}), "", "http://unused")

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/courses/devops-101/modules/0/check", strings.NewReader("{}"))
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCheckModule_NotALab rejects a check on a text module.
func TestCheckModule_NotALab(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "devops-101", IsPublic: true,
		Modules: []content.Module{{Name: "Reading", Type: "text", InlineContent: "x"}},
	})

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/courses/devops-101/modules/0/check", strings.NewReader("{}"))
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestCheckModule_NoGitContent rejects a lab module missing src/ref/path.
func TestCheckModule_NoGitContent(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := newStateWith(cfg, &content.Course{
		Slug: "devops-101", IsPublic: true,
		Modules: []content.Module{{Name: "Lab", Type: "lab", Src: "https://x/y"}},
	})

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/courses/devops-101/modules/0/check", strings.NewReader("{}"))
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestCheckModule_NoIdentityProviderLogin is the regression test for defect
// B11: without a login from the identity provider, a lab check is refused
// rather than run against the editable profile username.
func TestCheckModule_NoIdentityProviderLogin(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	course := labStepCourse()
	s := newStateWith(cfg, course)
	s.Users = identityUsers{UserProgress: noUsers{}, name: ""}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/courses/"+course.Slug+"/modules/0/steps/0/check", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	BuildRouter(s, cfg, false).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("lab check without an identity-provider login: %d %s, want 403", rec.Code, rec.Body.String())
	}
}
