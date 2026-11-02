package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// labStepCourse returns a course with one GitLab step-check lab module.
func labStepCourse() *content.Course {
	return &content.Course{
		Slug:     "devops-101",
		Title:    "DevOps 101",
		IsPublic: true,
		Modules: []content.Module{
			{
				Name: "Open a Merge Request", Type: "lab",
				Src: "https://git.example.com/org/repo", Ref: "main", Path: "labs/mr/module.md",
				CheckProvider: "gitlab",
				Steps: []content.CheckStep{
					{
						Title:     "Branch exists",
						CheckType: "gitlab_branch",
						CheckParams: map[string]any{
							"project": "e-learning/{{ .Username }}",
							"pattern": "feature/",
						},
					},
				},
			},
			{Name: "Reading", Type: "text", InlineContent: "notes"},
		},
	}
}

// checkStepState wires a State at a mock checker-service.
func checkStepState(checkerURL string) *State {
	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}

	return withPeers(newStateWith(cfg, labStepCourse()), "", checkerURL)
}

// stepCheckReq issues an authenticated student POST to the step-check route.
func stepCheckReq(t *testing.T, s *State, path string) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader("{}"))
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// TestCheckModuleStep_Allow runs a passing step check end to end against the
// mock checker-service.
func TestCheckModuleStep_Allow(t *testing.T) {
	t.Parallel()

	var gotPath string

	checker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"allow":true,"violations":[]}`))
	}))
	defer checker.Close()

	s := checkStepState(checker.URL)

	rec := stepCheckReq(t, s, "/api/courses/devops-101/modules/0/steps/0/check")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if gotPath != "/check-step" {
		t.Errorf("checker was called at %q, want /check-step", gotPath)
	}

	var resp CheckResponse

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.Allow {
		t.Errorf("Allow = false, want true")
	}
}

// TestCheckModuleStep_Deny relays a failing verdict from checker-service.
func TestCheckModuleStep_Deny(t *testing.T) {
	t.Parallel()

	checker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"allow":false,"violations":["no feature/ branch"]}`))
	}))
	defer checker.Close()

	s := checkStepState(checker.URL)

	rec := stepCheckReq(t, s, "/api/courses/devops-101/modules/0/steps/0/check")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp CheckResponse

	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Allow || len(resp.Violations) != 1 {
		t.Errorf("unexpected verdict: %+v", resp)
	}
}

// TestCheckModuleStep_CheckerError maps a checker-service failure to 502.
func TestCheckModuleStep_CheckerError(t *testing.T) {
	t.Parallel()

	checker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"gitlab unreachable"}`))
	}))
	defer checker.Close()

	s := checkStepState(checker.URL)

	rec := stepCheckReq(t, s, "/api/courses/devops-101/modules/0/steps/0/check")
	if rec.Code != http.StatusBadGateway {
		t.Errorf("want 502, got %d", rec.Code)
	}
}

// TestCheckModuleStep_NotALab rejects a step check on a non-lab module.
func TestCheckModuleStep_NotALab(t *testing.T) {
	t.Parallel()

	s := checkStepState("http://unused")

	rec := stepCheckReq(t, s, "/api/courses/devops-101/modules/1/steps/0/check")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestCheckModuleStep_StepOutOfRange returns 404 for an unknown step index.
func TestCheckModuleStep_StepOutOfRange(t *testing.T) {
	t.Parallel()

	s := checkStepState("http://unused")

	rec := stepCheckReq(t, s, "/api/courses/devops-101/modules/0/steps/9/check")
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestCheckModuleStep_MissingProject rejects a step whose params carry no
// project.
func TestCheckModuleStep_MissingProject(t *testing.T) {
	t.Parallel()

	course := labStepCourse()
	course.Modules[0].Steps[0].CheckParams = map[string]any{"pattern": "feature/"}

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	s := withPeers(newStateWith(cfg, course), "", "http://unused")

	rec := stepCheckReq(t, s, "/api/courses/devops-101/modules/0/steps/0/check")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestCheckModuleStep_ModuleIndexOutOfRange returns 404 for an unknown module.
func TestCheckModuleStep_ModuleIndexOutOfRange(t *testing.T) {
	t.Parallel()

	s := checkStepState("http://unused")

	rec := stepCheckReq(t, s, "/api/courses/devops-101/modules/50/steps/0/check")
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}
