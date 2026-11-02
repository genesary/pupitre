package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/fake"
)

// TestRecordLocalCheck_InsertThenList verifies a client-reported check is
// persisted via the LabCheckRepository and later visible through the admin
// lab-checks listing.
func TestRecordLocalCheck_InsertThenList(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	s.Repos.LabChecks = fake.NewLabCheckRepository()

	r := BuildRouter(s, s.Config, false)

	body, err := json.Marshal(CheckResponse{Allow: true, Violations: []string{"missing README"}})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/courses/kubernetes-basics/modules/0/record", bytes.NewReader(body),
	)
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result CheckResponse

	err = json.NewDecoder(rec.Body).Decode(&result)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result.Allow || len(result.Violations) != 1 {
		t.Fatalf("unexpected echoed result: %+v", result)
	}

	req = httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/api/admin/lab-checks?course=kubernetes-basics", http.NoBody,
	)
	req.Header.Set("Authorization", adminAuthHeader(t, s.Config.JWTSecret))

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var results []labCheckRow

	err = json.NewDecoder(rec.Body).Decode(&results)
	if err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 recorded check, got %d", len(results))
	}

	got := results[0]
	if got.CourseSlug != "kubernetes-basics" || got.ModuleIndex != 0 || got.ModuleName != "What is K8s" {
		t.Errorf("unexpected stored check: %+v", got)
	}

	if !got.Allow || len(got.Violations) != 1 || got.Violations[0] != "missing README" {
		t.Errorf("unexpected violations: %+v", got)
	}

	if got.Verified {
		t.Error("expected Verified=false for a client-reported (RecordLocalCheck) result")
	}
}

// TestRecordLocalCheck_NilRepo_NoError verifies that a client-reported check
// is accepted (fire-and-forget persistence) even when no LabCheckRepository
// is configured — the storeLabCheck nil-guard must not surface as an HTTP
// error.
func TestRecordLocalCheck_NilRepo_NoError(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock) // s.Repos.LabChecks left nil
	r := BuildRouter(s, s.Config, false)

	body, err := json.Marshal(CheckResponse{Allow: false, Violations: []string{"boom"}})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/courses/kubernetes-basics/modules/0/record", bytes.NewReader(body),
	)
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even without a configured repository, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRecordLocalCheck_CourseNotFound verifies 404 for an unknown course.
func TestRecordLocalCheck_CourseNotFound(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/courses/no-such-course/modules/0/record", bytes.NewReader([]byte(`{}`)),
	)
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestRecordLocalCheck_InvalidModuleIndex verifies 400 for an out-of-range
// module index.
func TestRecordLocalCheck_InvalidModuleIndex(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/courses/kubernetes-basics/modules/99/record", bytes.NewReader([]byte(`{}`)),
	)
	req.Header.Set("Authorization", authHeader(t, s.Config.JWTSecret))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
