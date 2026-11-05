package server

import (
	"net/http"
	"testing"

	"github.com/genesary/pupitre/apps/api/internal/courses/content"
)

// TestWiring_PrivateCourseSessionCapacity is a regression test:
// the capacity of a session of a
// private course is enforced, as for a public course.
func TestWiring_PrivateCourseSessionCapacity(t *testing.T) {
	t.Parallel()

	app := newWiredApp(t)
	ctx := t.Context()

	err := app.domains.Courses.Repos.Courses.Upsert(ctx, &content.Course{
		Slug: "private-k8s", Title: "Private Kubernetes", IsPublic: false,
		Modules:  []content.Module{{Name: "Intro", Type: "text", InlineContent: "Hello"}},
		Sessions: []content.Session{{ID: "s1", Title: "Atelier", Date: "2026-11-02", Capacity: 1}},
	})
	if err != nil {
		t.Fatalf("seed the private course: %v", err)
	}

	// The only seat is already taken by someone else.
	err = app.usersRepos.SessionBookings.Book(ctx, "22222222-2222-2222-2222-222222222222", "private-k8s", "s1")
	if err != nil {
		t.Fatalf("seed a booking: %v", err)
	}

	rec := app.do(t, http.MethodPost, "/api/session-bookings/private-k8s/s1")
	if rec.Code != http.StatusConflict {
		t.Errorf("booking a full session of a private course: %d %s, want 409 Session is full", rec.Code, rec.Body.String())
	}
}

// TestWiring_BookUnknownCourse verifies that a session of a course that does
// not exist cannot be booked.
func TestWiring_BookUnknownCourse(t *testing.T) {
	t.Parallel()

	app := newWiredApp(t)

	rec := app.do(t, http.MethodPost, "/api/session-bookings/no-such-course/s1")
	if rec.Code != http.StatusNotFound {
		t.Errorf("booking a session of an unknown course: %d %s, want 404", rec.Code, rec.Body.String())
	}

	count, err := app.usersRepos.SessionBookings.CountBySession(t.Context(), "no-such-course", "s1")
	if err != nil {
		t.Fatalf("CountBySession: %v", err)
	}

	if count != 0 {
		t.Errorf("seats booked = %d, want 0", count)
	}
}
