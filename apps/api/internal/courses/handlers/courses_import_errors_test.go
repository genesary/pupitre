package handlers

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// importErrRouter returns an admin router whose CourseRepository fails on
// every write.
func importErrRouter(seed ...*content.Course) http.Handler {
	c := fake.NewCourseRepository(seed...)
	c.Err = errors.New("course db down")

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	state := NewState(cfg, &repository.Repositories{
		Courses:      c,
		Paths:        fake.NewPathRepository(),
		QuizAttempts: fake.NewQuizAttemptRepository(),
		LabChecks:    fake.NewLabCheckRepository(),
	})

	return BuildRouter(state, cfg, false)
}

// TestImportCourseMarkdown_CreateDBError maps a create write failure to 500.
func TestImportCourseMarkdown_CreateDBError(t *testing.T) {
	t.Parallel()

	r := importErrRouter()

	rec := adminRequest(t, r, http.MethodPost, "/api/admin/courses/import",
		importBody(t, importDoc, "create", "h2", ""))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestExportCourseMarkdown_RoundTrips exports a seeded course as markdown.
func TestExportCourseMarkdown_RoundTrips(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryH: 24}
	state := newStateWith(cfg, &content.Course{
		Slug: "linux-intro", Title: "Linux Intro", Category: "linux", IsPublic: true,
		Modules: []content.Module{
			{Name: "Intro", Type: "text", InlineContent: "Bonjour."},
			{Name: "Suite", Type: "text", InlineContent: "Encore."},
		},
	})
	r := BuildRouter(state, cfg, false)

	rec := adminRequest(t, r, http.MethodGet, "/api/admin/courses/linux-intro/export/markdown?split=h2", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Linux Intro") || !strings.Contains(body, "Intro") || !strings.Contains(body, "Bonjour.") {
		t.Errorf("exported markdown missing expected content:\n%s", body)
	}
}

// TestExportCourseMarkdown_NotFound is a 404 for an unknown course.
func TestExportCourseMarkdown_NotFound(t *testing.T) {
	t.Parallel()

	_, r := newAdminState(t)

	rec := adminRequest(t, r, http.MethodGet, "/api/admin/courses/absent/export/markdown", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// TestPreviewCourseMarkdownImport_DBError surfaces a lookup failure during
// planImport (append against a course the repo cannot read).
func TestPreviewCourseMarkdownImport_DBError(t *testing.T) {
	t.Parallel()

	r := importErrRouter()

	rec := adminRequest(t, r, http.MethodPost, "/api/admin/courses/import/preview",
		importBody(t, importDoc, "append", "h2", "linux-intro"))
	if rec.Code < 400 {
		t.Errorf("want an error status, got %d", rec.Code)
	}
}
