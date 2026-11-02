package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	apimiddleware "github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
)

// managerAuthHeader returns a Bearer token header for a manager user.
func managerAuthHeader(t *testing.T, secret string) string {
	t.Helper()

	token, err := apimiddleware.CreateToken(
		"00000000-0000-0000-0000-000000000002",
		"manager@test.com",
		"manager",
		"testmanager",
		secret,
		24,
	)
	if err != nil {
		t.Fatalf("create manager token: %v", err)
	}

	return "Bearer " + token
}

// ── generateSessionID ─────────────────────────────────────────────────────────

// TestGenerateSessionID verifies that generated IDs have the expected format.
func TestGenerateSessionID(t *testing.T) {
	t.Parallel()

	id, err := generateSessionID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(id, "sess-") {
		t.Errorf("expected id to start with sess-, got %q", id)
	}

	// "sess-" (5) + 16 hex chars (8 bytes) = 21 total
	if len(id) != 21 {
		t.Errorf("expected length 21, got %d (%q)", len(id), id)
	}

	hex := id[5:]

	for _, c := range hex {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("id contains non-hex char %q in %q", c, id)
		}
	}
}

// TestGenerateSessionID_Unique verifies two consecutive calls return
// different IDs.
func TestGenerateSessionID_Unique(t *testing.T) {
	t.Parallel()

	first, err := generateSessionID()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	second, err := generateSessionID()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if first == second {
		t.Errorf("expected unique IDs, got %q twice", first)
	}
}

// ── Middleware enforcement ────────────────────────────────────────────────────

// TestCreateSession_RequiresAuth verifies that unauthenticated requests are
// rejected with 401.
func TestCreateSession_RequiresAuth(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/admin/courses/kubernetes-basics/sessions", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", rec.Code)
	}
}

// TestCreateSession_StudentForbidden verifies that a student cannot create
// sessions.
func TestCreateSession_StudentForbidden(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/admin/courses/kubernetes-basics/sessions", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for student, got %d", rec.Code)
	}
}

// TestCreateSession_AdminPassesMiddleware verifies that an admin is not blocked
// by the middleware, and that the session is actually created.
func TestCreateSession_AdminPassesMiddleware(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	body := `{"title":"Test","date":"2026-09-01T10:00:00Z","location":"Room A","capacity":10}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/admin/courses/kubernetes-basics/sessions", strings.NewReader(body))
	req.Header.Set("Authorization", adminAuthHeader(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("admin should be able to create a session, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateSession_ManagerPassesMiddleware verifies that a manager is not
// blocked by the middleware.
func TestCreateSession_ManagerPassesMiddleware(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	body := `{"title":"Test","date":"2026-09-01T10:00:00Z","location":"Room A","capacity":10}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/admin/courses/kubernetes-basics/sessions", strings.NewReader(body))
	req.Header.Set("Authorization", managerAuthHeader(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("manager should be able to create a session, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── Input validation ──────────────────────────────────────────────────────────

// TestCreateSession_BadJSON verifies that malformed JSON returns 400.
func TestCreateSession_BadJSON(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/admin/courses/kubernetes-basics/sessions", strings.NewReader(`{bad json`))
	req.Header.Set("Authorization", adminAuthHeader(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rec.Code)
	}
}

// TestCreateSession_MissingTitle verifies that a missing title returns 400.
func TestCreateSession_MissingTitle(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/admin/courses/kubernetes-basics/sessions",
		strings.NewReader(`{"date":"2026-09-01T10:00:00Z","location":"Room A","capacity":10}`))
	req.Header.Set("Authorization", adminAuthHeader(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing title, got %d", rec.Code)
	}
}

// TestCreateSession_MissingDate verifies that a missing date returns 400.
func TestCreateSession_MissingDate(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/admin/courses/kubernetes-basics/sessions",
		strings.NewReader(`{"title":"Session","location":"Room A","capacity":10}`))
	req.Header.Set("Authorization", adminAuthHeader(t, "test-secret"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing date, got %d", rec.Code)
	}
}

// TestUpdateSession_RequiresAuth verifies that unauthenticated requests are
// rejected with 401.
func TestUpdateSession_RequiresAuth(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut,
		"/api/admin/courses/kubernetes-basics/sessions/sess-abc", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", rec.Code)
	}
}

// TestUpdateSession_StudentForbidden verifies that a student cannot update
// sessions.
func TestUpdateSession_StudentForbidden(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut,
		"/api/admin/courses/kubernetes-basics/sessions/sess-abc", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for student, got %d", rec.Code)
	}
}

// TestDeleteSession_RequiresAuth verifies that unauthenticated requests are
// rejected with 401.
func TestDeleteSession_RequiresAuth(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete,
		"/api/admin/courses/kubernetes-basics/sessions/sess-abc", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", rec.Code)
	}
}

// TestDeleteSession_StudentForbidden verifies that a student cannot delete
// sessions.
func TestDeleteSession_StudentForbidden(t *testing.T) {
	t.Parallel()

	mock := newUserServiceMock()
	defer mock.Close()

	s := newTestState(t, mock)
	r := BuildRouter(s, &config.Config{JWTSecret: "test-secret"}, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete,
		"/api/admin/courses/kubernetes-basics/sessions/sess-abc", http.NoBody)
	req.Header.Set("Authorization", authHeader(t, "test-secret"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for student, got %d", rec.Code)
	}
}
