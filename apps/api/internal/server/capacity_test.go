package server

import (
	"net/http"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// TestWiring_PrivateCourseSessionCapacity is the regression test of defect
// B24 (docs/specs/190-backend-defects.md): the capacity of a session of a
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
