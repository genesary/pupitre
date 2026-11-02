package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// newSessionsRouter builds a router whose state points at courseServiceURL.
func newSessionsRouter(t *testing.T, repos *repository.Repositories, courseServiceURL string) http.Handler {
	t.Helper()

	cfg := &config.Config{
		JWTSecret:   htSecret,
		JWTExpiryH:  htExpiry,
		CORSOrigins: []string{"*"},
	}
	s := &State{Repos: repos, Config: cfg, Courses: courseSourceAt(courseServiceURL)}

	return BuildRouter(s, cfg, false)
}

// ── BookSession ───────────────────────────────────────────────────────────────

// TestBookSession_Success verifies that booking a session with no capacity
// limit returns 204 No Content.
func TestBookSession_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDo(t, r, http.MethodPost, "/api/session-bookings/my-course/sess-1", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestBookSession_SessionFull verifies that booking a full session returns 409.
func TestBookSession_SessionFull(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"sessions":[{"id":"sess-1","capacity":1}]}`)
	}))
	defer srv.Close()

	repos := fake.NewRepositories()

	sb := fake.NewSessionBookingRepository()

	err := sb.Book(t.Context(), "other-user", "my-course", "sess-1")
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}

	repos.SessionBookings = sb

	r := newSessionsRouter(t, repos, srv.URL)
	rec := htDo(t, r, http.MethodPost, "/api/session-bookings/my-course/sess-1", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestBookSession_CountError verifies that a DB error on CountBySession
// returns 500.
func TestBookSession_CountError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"sessions":[{"id":"sess-1","capacity":5}]}`)
	}))
	defer srv.Close()

	repos := fake.NewRepositories()
	sb := fake.NewSessionBookingRepository()
	sb.Err = errors.New("db down")
	repos.SessionBookings = sb

	r := newSessionsRouter(t, repos, srv.URL)
	rec := htDo(t, r, http.MethodPost, "/api/session-bookings/my-course/sess-1", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestBookSession_BookError verifies that a DB error on Book returns 500.
func TestBookSession_BookError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	sb := fake.NewSessionBookingRepository()
	sb.Err = errors.New("db down")
	repos.SessionBookings = sb

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodPost, "/api/session-bookings/my-course/sess-1", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestBookSession_CourseServiceDown verifies that a missing course service
// (capacity falls back to 0 = unlimited) still allows booking.
func TestBookSession_CourseServiceDown(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newSessionsRouter(t, repos, "http://127.0.0.1:0")
	rec := htDo(t, r, http.MethodPost, "/api/session-bookings/my-course/sess-1", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusNoContent {
		t.Errorf("want 204 (soft fail on capacity), got %d: %s", rec.Code, rec.Body.String())
	}
}

// ── UnbookSession ─────────────────────────────────────────────────────────────

// TestUnbookSession_Success verifies that cancelling a booking returns 204.
func TestUnbookSession_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDo(t, r, http.MethodDelete, "/api/session-bookings/my-course/sess-1", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUnbookSession_Error verifies that a DB error returns 500.
func TestUnbookSession_Error(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	sb := fake.NewSessionBookingRepository()
	sb.Err = errors.New("db down")
	repos.SessionBookings = sb

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodDelete, "/api/session-bookings/my-course/sess-1", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── MySessionBookings ─────────────────────────────────────────────────────────

// TestMySessionBookings_Empty verifies that a user with no bookings
// receives a 200 with an empty bookings array.
func TestMySessionBookings_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDo(t, r, http.MethodGet, "/api/my/session-bookings", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var body map[string]any

	err := json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}

	bookings, ok := body["bookings"].([]any)
	if !ok || len(bookings) != 0 {
		t.Errorf("want empty bookings array, got %v", body["bookings"])
	}
}

// TestMySessionBookings_WithData verifies that existing bookings are returned.
func TestMySessionBookings_WithData(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()

	sb := fake.NewSessionBookingRepository()

	err := sb.Book(t.Context(), "user-uuid-1", "my-course", "sess-1")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	repos.SessionBookings = sb

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/my/session-bookings", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var body map[string]any

	err = json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}

	bookings, ok := body["bookings"].([]any)
	if !ok || len(bookings) == 0 {
		t.Errorf("want at least one booking, got %v", body["bookings"])
	}
}

// TestMySessionBookings_Error verifies that a DB error returns 500.
func TestMySessionBookings_Error(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	sb := fake.NewSessionBookingRepository()
	sb.Err = errors.New("db down")
	repos.SessionBookings = sb

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/my/session-bookings", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── ListSessionBookings ───────────────────────────────────────────────────────

// TestListSessionBookings_Success verifies that an admin can list participants.
func TestListSessionBookings_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDo(t, r, http.MethodGet, "/api/admin/session-bookings/my-course/sess-1", "", htAuthHeader(t, "admin"))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestListSessionBookings_Error verifies that a DB error returns 500.
func TestListSessionBookings_Error(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	sb := fake.NewSessionBookingRepository()
	sb.Err = errors.New("db down")
	repos.SessionBookings = sb

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/admin/session-bookings/my-course/sess-1", "", htAuthHeader(t, "admin"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── MarkSessionPresence ───────────────────────────────────────────────────────

// TestMarkSessionPresence_Success verifies that an admin can mark presence.
func TestMarkSessionPresence_Success(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDo(t, r, http.MethodPatch,
		"/api/admin/session-bookings/my-course/sess-1/user-uuid-1/presence",
		`{"present":true}`,
		htAuthHeader(t, "admin"),
	)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMarkSessionPresence_BadBody verifies that an invalid body returns 400.
func TestMarkSessionPresence_BadBody(t *testing.T) {
	t.Parallel()

	r := newTestRouter()
	rec := htDo(t, r, http.MethodPatch,
		"/api/admin/session-bookings/my-course/sess-1/user-uuid-1/presence",
		"not-json",
		htAuthHeader(t, "admin"),
	)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestMarkSessionPresence_Error verifies that a DB error returns 500.
func TestMarkSessionPresence_Error(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	sb := fake.NewSessionBookingRepository()
	sb.Err = errors.New("db down")
	repos.SessionBookings = sb

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodPatch,
		"/api/admin/session-bookings/my-course/sess-1/user-uuid-1/presence",
		`{"present":true}`,
		htAuthHeader(t, "admin"),
	)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── SessionBookingCount ───────────────────────────────────────────────────────

// TestSessionBookingCount_Success verifies that the booking count is returned.
func TestSessionBookingCount_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()

	sb := fake.NewSessionBookingRepository()

	err := sb.Book(t.Context(), "user-uuid-1", "my-course", "sess-1")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	repos.SessionBookings = sb

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/session-bookings/my-course/sess-1/count", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var body map[string]any

	err = json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["count"] == nil {
		t.Error("expected count in response")
	}
}

// TestSessionBookingCount_Error verifies that a DB error returns 500.
func TestSessionBookingCount_Error(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	sb := fake.NewSessionBookingRepository()
	sb.Err = errors.New("db down")
	repos.SessionBookings = sb

	r := newTestRouterWithRepos(repos)
	rec := htDo(t, r, http.MethodGet, "/api/session-bookings/my-course/sess-1/count", "", htAuthHeader(t, "student"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}
