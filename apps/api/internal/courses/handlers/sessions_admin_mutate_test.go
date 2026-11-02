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

// sessionCourseState returns a State holding one course that already has a
// single scheduled session ("sess-x").
func sessionCourseState() *State {
	return newStateWith(
		&config.Config{JWTSecret: "test-secret"},
		&content.Course{
			Slug:     "kubernetes-basics",
			Title:    "Kubernetes Basics",
			IsPublic: true,
			InPerson: true,
			Sessions: []content.Session{
				{ID: "sess-x", Title: "Cohort 1", Date: "2026-09-01T10:00:00Z", Location: "Room A", Capacity: 10},
			},
		},
	)
}

// adminReq issues an authenticated admin request and returns the recorder.
func adminReq(t *testing.T, s *State, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)

	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Authorization", adminAuthHeader(t, "test-secret"))

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// TestUpdateSession_OK updates an existing session and echoes its ID.
func TestUpdateSession_OK(t *testing.T) {
	t.Parallel()

	s := sessionCourseState()

	rec := adminReq(t, s, http.MethodPut,
		"/api/admin/courses/kubernetes-basics/sessions/sess-x",
		`{"title":"Renamed","date":"2026-10-01T10:00:00Z","location":"Room B","capacity":25}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["id"] != "sess-x" {
		t.Errorf("id = %q, want sess-x", resp["id"])
	}

	// The stored session must reflect the new values.
	got, _ := s.Repos.Courses.Get(t.Context(), "kubernetes-basics")
	if len(got.Sessions) != 1 || got.Sessions[0].Title != "Renamed" || got.Sessions[0].Capacity != 25 {
		t.Errorf("session not updated: %+v", got.Sessions)
	}
}

// TestUpdateSession_BadJSON rejects a malformed body.
func TestUpdateSession_BadJSON(t *testing.T) {
	t.Parallel()

	s := sessionCourseState()

	rec := adminReq(t, s, http.MethodPut,
		"/api/admin/courses/kubernetes-basics/sessions/sess-x", "{not json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestUpdateSession_NotFound returns 404 for an unknown session ID.
func TestUpdateSession_NotFound(t *testing.T) {
	t.Parallel()

	s := sessionCourseState()

	rec := adminReq(t, s, http.MethodPut,
		"/api/admin/courses/kubernetes-basics/sessions/sess-missing",
		`{"title":"x","date":"2026-10-01T10:00:00Z"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestDeleteSession_OK removes the session and reports success.
func TestDeleteSession_OK(t *testing.T) {
	t.Parallel()

	s := sessionCourseState()

	rec := adminReq(t, s, http.MethodDelete,
		"/api/admin/courses/kubernetes-basics/sessions/sess-x", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	got, _ := s.Repos.Courses.Get(t.Context(), "kubernetes-basics")
	if len(got.Sessions) != 0 {
		t.Errorf("session not removed: %+v", got.Sessions)
	}
}

// TestDeleteSession_NotFound returns 404 for an unknown session ID.
func TestDeleteSession_NotFound(t *testing.T) {
	t.Parallel()

	s := sessionCourseState()

	rec := adminReq(t, s, http.MethodDelete,
		"/api/admin/courses/kubernetes-basics/sessions/sess-missing", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestDeleteSession_UnknownCourse returns 404 when the course does not exist.
func TestDeleteSession_UnknownCourse(t *testing.T) {
	t.Parallel()

	s := sessionCourseState()

	rec := adminReq(t, s, http.MethodDelete,
		"/api/admin/courses/no-such-course/sessions/sess-x", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}
