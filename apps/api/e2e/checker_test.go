//go:build e2e

package e2e_test

import (
	"net/http"
	"testing"
)

// TestAPIHealth checks the single health endpoint of the API.
func TestAPIHealth(t *testing.T) {
	resp := doJSON(t, http.MethodGet, globalCfg.APIURL+"/health", nil, "")
	mustStatus(t, resp, http.StatusOK)

	var out struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}

	decodeBody(t, resp, &out)

	if out.Status != "ok" || out.Service != "pupitre-api" {
		t.Fatalf("health: got %+v, want status ok from pupitre-api", out)
	}
}

// TestLabCheckRequiresAuth checks that a lab check, which runs the checker
// in-process, is refused without a token.
func TestLabCheckRequiresAuth(t *testing.T) {
	for _, path := range []string{
		"/api/courses/any-course/modules/0/check",
		"/api/courses/any-course/modules/0/steps/0/check",
	} {
		resp := doJSON(t, http.MethodPost, globalCfg.APIURL+path, map[string]string{}, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("POST %s without a token: got %d, want 401", path, resp.StatusCode)
		}

		resp.Body.Close()
	}
}

// TestServiceRoutesAreGone checks that the routes the services used to call
// each other are not served (docs/specs/103-backend-http-api.md).
func TestServiceRoutesAreGone(t *testing.T) {
	for _, path := range []string{"/evaluate", "/check-step", "/internal/progress/complete", "/internal/enrollments/auto"} {
		resp := doJSON(t, http.MethodPost, globalCfg.APIURL+path, map[string]string{}, "")
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("POST %s: got %d, want 404", path, resp.StatusCode)
		}

		resp.Body.Close()
	}
}
