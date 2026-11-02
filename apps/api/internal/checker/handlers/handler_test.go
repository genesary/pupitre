package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/checker"
	"github.com/jojo91550/pupitre2/apps/api/internal/checker/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/httperr"
)

// baseConfig returns a Config with sane defaults for handler tests.
func baseConfig() *config.Config {
	return &config.Config{
		GitLabToken:            "tok",
		GitLabBaseURL:          "http://gitlab.local",
		RateLimitRequests:      0,
		RateLimitWindowSeconds: 60,
	}
}

// TestRemoteIP splits host:port and falls back to the raw address.
func TestRemoteIP(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = "203.0.113.5:44444"

	got, err := remoteIP(r)
	if err != nil || got != "203.0.113.5" {
		t.Errorf("remoteIP = %q, %v; want 203.0.113.5", got, err)
	}

	r.RemoteAddr = "no-port-here"

	got, err = remoteIP(r)
	if err != nil || got != "no-port-here" {
		t.Errorf("remoteIP fallback = %q, %v", got, err)
	}
}

// TestHealth serves a JSON ok body on /health.
func TestHealth(t *testing.T) {
	t.Parallel()

	router := New(baseConfig()).BuildRouter()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body map[string]string

	err := json.NewDecoder(w.Body).Decode(&body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("body = %v", body)
	}
}

// TestDecodeEvaluateRequest validates the JSON body of POST /evaluate.
func TestDecodeEvaluateRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"bad json", "{", false},
		{"missing username", `{"project":"g/p","policySrc":"x","policyRef":"main","policyPath":"c.rego"}`, false},
		{"missing policy fields", `{"username":"u","project":"g/p"}`, false},
		{"valid", `{"username":"u","project":"g/p","policySrc":"x","policyRef":"main","policyPath":"c.rego"}`, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/evaluate", strings.NewReader(tc.body))

			_, ok := decodeEvaluateRequest(w, r)
			if ok != tc.ok {
				t.Errorf("ok = %v, want %v (status %d)", ok, tc.ok, w.Code)
			}
		})
	}
}

// TestValidateEvaluateRequest enforces the SSRF and path-traversal guards.
func TestValidateEvaluateRequest(t *testing.T) {
	t.Parallel()

	h := New(baseConfig()) // GitLabBaseURL = http://gitlab.local

	cases := []struct {
		name    string
		req     checker.EvaluateRequest
		wantErr bool
	}{
		{
			name: "ok",
			req:  checker.EvaluateRequest{PolicySrc: "http://gitlab.local/g/p", PolicyRef: "main", PolicyPath: "policies/check.rego"},
		},
		{
			name:    "wrong host",
			req:     checker.EvaluateRequest{PolicySrc: "http://evil.example/g/p", PolicyRef: "main", PolicyPath: "check.rego"},
			wantErr: true,
		},
		{
			name:    "ref traversal",
			req:     checker.EvaluateRequest{PolicySrc: "http://gitlab.local/g/p", PolicyRef: "../../etc", PolicyPath: "check.rego"},
			wantErr: true,
		},
		{
			name:    "path escape",
			req:     checker.EvaluateRequest{PolicySrc: "http://gitlab.local/g/p", PolicyRef: "main", PolicyPath: "../../../etc/passwd"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := h.validateEvaluateRequest(tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// TestEvaluate_GitLabTokenMissing returns 500 when no GitLab token is
// configured.
func TestEvaluate_GitLabTokenMissing(t *testing.T) {
	t.Parallel()

	cfg := baseConfig()
	cfg.GitLabToken = ""
	h := New(cfg)

	w := httptest.NewRecorder()
	body := `{"username":"u","project":"g/p","policySrc":"http://gitlab.local/g/p","policyRef":"main","policyPath":"c.rego"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/evaluate", strings.NewReader(body))

	h.Evaluate(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// TestEvaluate_InvalidPolicySrc returns 400 when policySrc points off-
// instance.
func TestEvaluate_InvalidPolicySrc(t *testing.T) {
	t.Parallel()

	h := New(baseConfig())

	w := httptest.NewRecorder()
	body := `{"username":"u","project":"g/p","policySrc":"http://evil/g/p","policyRef":"main","policyPath":"c.rego"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/evaluate", strings.NewReader(body))

	h.Evaluate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestEvaluate_FullSuccess runs a passing policy end to end through the
// handler.
func TestEvaluate_FullSuccess(t *testing.T) {
	t.Parallel()

	policy := "package checker.lab\n\ndefault allow := true\n"

	gl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/repository/files/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"content":  base64.StdEncoding.EncodeToString([]byte(policy)),
				"encoding": "base64",
			})
		case strings.HasSuffix(r.URL.Path, "/merge_requests"):
			_, _ = w.Write([]byte("[]"))
		case strings.Contains(r.URL.Path, "/projects/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 1, "name": "p", "path_with_namespace": "g/p", "default_branch": "main",
			})
		default:
			_, _ = w.Write([]byte("[]"))
		}
	}))

	defer gl.Close()

	cfg := baseConfig()
	cfg.GitLabBaseURL = gl.URL
	h := New(cfg)

	w := httptest.NewRecorder()
	body := `{"username":"u","project":"g/p","policySrc":"` + gl.URL + `/g/p","policyRef":"main","policyPath":"c.rego","files":[]}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/evaluate", strings.NewReader(body))

	h.Evaluate(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	var resp checker.EvaluateResponse

	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.Allow {
		t.Errorf("Allow = false, want true")
	}
}

// TestEvaluate_PolicyFetchFails returns 500 when the policy file cannot be
// fetched.
func TestEvaluate_PolicyFetchFails(t *testing.T) {
	t.Parallel()

	// GitLab server that 404s the policy file request.
	gl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/files/") {
			http.Error(w, "not found", http.StatusNotFound)

			return
		}

		_, _ = w.Write([]byte("{}"))
	}))

	defer gl.Close()

	cfg := baseConfig()
	cfg.GitLabBaseURL = gl.URL
	h := New(cfg)

	w := httptest.NewRecorder()
	body := `{"username":"u","project":"g/p","policySrc":"` + gl.URL + `/g/p","policyRef":"main","policyPath":"c.rego"}`
	h.Evaluate(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/evaluate", strings.NewReader(body)))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// TestEvaluate_StateFetchFails returns 500 when GitLab project state cannot
// be fetched.
func TestEvaluate_StateFetchFails(t *testing.T) {
	t.Parallel()

	policy := "package checker.lab\n\ndefault allow := true\n"

	gl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/repository/files/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"content": base64.StdEncoding.EncodeToString([]byte(policy)), "encoding": "base64",
			})
		case strings.Contains(r.URL.Path, "/projects/"):
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte("[]"))
		}
	}))

	defer gl.Close()

	cfg := baseConfig()
	cfg.GitLabBaseURL = gl.URL
	h := New(cfg)

	w := httptest.NewRecorder()
	body := `{"username":"u","project":"g/p","policySrc":"` + gl.URL + `/g/p","policyRef":"main","policyPath":"c.rego"}`
	h.Evaluate(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/evaluate", strings.NewReader(body)))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// TestCheckStep_FullSuccessViaHandler runs a gitlab_branch step check
// through the handler.
func TestCheckStep_FullSuccessViaHandler(t *testing.T) {
	t.Parallel()

	gl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/repository/branches"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "feature/alice"}})
		default:
			_, _ = w.Write([]byte("[]"))
		}
	}))

	defer gl.Close()

	cfg := baseConfig()
	cfg.GitLabBaseURL = gl.URL
	h := New(cfg)

	w := httptest.NewRecorder()
	body := `{"username":"alice","project":"grp/alice","checkType":"gitlab_branch","checkParams":{"pattern":"feature/"}}`
	h.CheckStep(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/check-step", strings.NewReader(body)))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}

	var resp checker.EvaluateResponse

	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if !resp.Allow {
		t.Errorf("Allow = false, want true")
	}
}

// TestCheckStep_FetcherError returns 500 when the GitLab client cannot be
// built.
func TestCheckStep_FetcherError(t *testing.T) {
	t.Parallel()

	cfg := baseConfig()
	cfg.GitLabBaseURL = "http://exa\x7fmple.com" // malformed -> NewFetcher fails
	h := New(cfg)

	w := httptest.NewRecorder()
	body := `{"username":"alice","project":"grp/alice","checkType":"gitlab_branch"}`
	h.CheckStep(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/check-step", strings.NewReader(body)))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// TestCheckStep_Validation rejects bad bodies, missing fields and missing
// token.
func TestCheckStep_Validation(t *testing.T) {
	t.Parallel()

	h := New(baseConfig())

	// bad JSON
	w := httptest.NewRecorder()
	h.CheckStep(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/check-step", strings.NewReader("{")))

	if w.Code != http.StatusBadRequest {
		t.Errorf("bad json: status = %d, want 400", w.Code)
	}

	// missing fields
	w = httptest.NewRecorder()
	h.CheckStep(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/check-step", strings.NewReader(`{"username":"u"}`)))

	if w.Code != http.StatusBadRequest {
		t.Errorf("missing fields: status = %d, want 400", w.Code)
	}

	// token missing
	cfg := baseConfig()
	cfg.GitLabToken = ""
	w = httptest.NewRecorder()
	New(cfg).CheckStep(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/check-step",
		strings.NewReader(`{"username":"u","project":"g/u","checkType":"gitlab_branch"}`)))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("no token: status = %d, want 500", w.Code)
	}
}

// TestCheckStep_UsernameMismatchViaHandler returns 200 with Allow=false on
// project/username mismatch.
func TestCheckStep_UsernameMismatchViaHandler(t *testing.T) {
	t.Parallel()

	// project last segment != username -> handler returns 200 with Allow=false
	h := New(baseConfig())
	w := httptest.NewRecorder()
	h.CheckStep(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/check-step",
		strings.NewReader(`{"username":"alice","project":"grp/bob","checkType":"gitlab_branch"}`)))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp checker.EvaluateResponse

	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Allow {
		t.Errorf("Allow = true, want false for username mismatch")
	}
}

// TestHTTPErr writes a JSON error body with the given status.
func TestHTTPErr(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	httperr.Write(w, http.StatusTeapot, "nope")

	if w.Code != http.StatusTeapot {
		t.Errorf("code = %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "nope") {
		t.Errorf("body = %q", w.Body.String())
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("content-type = %q", w.Header().Get("Content-Type"))
	}
}

// TestBuildRouter_RateLimitEnabled returns 429 once the configured rate
// limit is exceeded.
func TestBuildRouter_RateLimitEnabled(t *testing.T) {
	t.Parallel()

	cfg := baseConfig()
	cfg.RateLimitRequests = 1
	router := New(cfg).BuildRouter()

	// First health hit ok, a later one should be rate-limited (429).
	var got429 bool

	for range 5 {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
		req.RemoteAddr = "198.51.100.7:5555"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			got429 = true

			break
		}
	}

	if !got429 {
		t.Error("expected a 429 once the rate limit was exceeded")
	}
}
