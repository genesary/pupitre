package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// importDoc is a two-module course document used across the import tests.
const importDoc = "---\nslug: linux-intro\ntitle: Linux Intro\ncategory: linux\n---\n\n## Intro\n\nBonjour.\n\n## Suite\n\nEncore.\n"

// importBody builds an import request body around a document.
func importBody(t *testing.T, document, mode, split, slug string) string {
	t.Helper()

	payload := map[string]string{"markdown": document, "mode": mode, "split": split, "slug": slug}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal import body: %v", err)
	}

	return string(encoded)
}

// TestImportCourseMarkdownCreates checks that a create import stores the
// document's course and refuses to overwrite an existing slug.
func TestImportCourseMarkdownCreates(t *testing.T) {
	t.Parallel()

	state, router := newAdminState(t)

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses/import", importBody(t, importDoc, "create", "h2", ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	course, err := state.Repos.Courses.Get(t.Context(), "linux-intro")
	if err != nil {
		t.Fatalf("course was not stored: %v", err)
	}

	if course.Title != "Linux Intro" || len(course.Modules) != 2 {
		t.Fatalf("stored course = %+v", course)
	}

	if course.Modules[0].InlineContent != "Bonjour." {
		t.Errorf("module content = %q", course.Modules[0].InlineContent)
	}

	// A second create on the same slug must not overwrite it.
	rec = adminRequest(t, router, http.MethodPost, "/api/admin/courses/import", importBody(t, importDoc, "create", "h2", ""))
	if rec.Code != http.StatusConflict {
		t.Fatalf("second create status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

// TestImportCourseMarkdownAppendsAndReplaces checks that append only adds
// modules and that replace without frontmatter keeps the stored metadata.
func TestImportCourseMarkdownAppendsAndReplaces(t *testing.T) {
	t.Parallel()

	state, router := newAdminState(t)

	adminRequest(t, router, http.MethodPost, "/api/admin/courses/import", importBody(t, importDoc, "create", "h2", ""))

	appended := "## Troisième\n\nEt de trois.\n"

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses/import", importBody(t, appended, "append", "h2", "linux-intro"))
	if rec.Code != http.StatusOK {
		t.Fatalf("append status = %d, body = %s", rec.Code, rec.Body.String())
	}

	course, err := state.Repos.Courses.Get(t.Context(), "linux-intro")
	if err != nil {
		t.Fatalf("get course: %v", err)
	}

	if len(course.Modules) != 3 || course.Title != "Linux Intro" {
		t.Fatalf("append changed more than the module list: %+v", course)
	}

	// Replacing with a document that carries no frontmatter keeps the
	// stored metadata and swaps only the modules.
	rec = adminRequest(t, router, http.MethodPost, "/api/admin/courses/import", importBody(t, appended, "replace", "h2", "linux-intro"))
	if rec.Code != http.StatusOK {
		t.Fatalf("replace status = %d, body = %s", rec.Code, rec.Body.String())
	}

	course, err = state.Repos.Courses.Get(t.Context(), "linux-intro")
	if err != nil {
		t.Fatalf("get course: %v", err)
	}

	if len(course.Modules) != 1 || course.Title != "Linux Intro" || course.Category != "linux" {
		t.Fatalf("replace without frontmatter dropped metadata: %+v", course)
	}
}

// TestImportCourseMarkdownRejectsUnknownCourse checks that append and
// replace 404 rather than creating the course they name.
func TestImportCourseMarkdownRejectsUnknownCourse(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses/import", importBody(t, "## A\n\na\n", "append", "h2", "nope"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

// TestImportCourseMarkdownRejectsBadRequests checks the client errors an
// unusable import request comes back with.
func TestImportCourseMarkdownRejectsBadRequests(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	cases := map[string]string{
		"empty document": importBody(t, "   ", "create", "", ""),
		"unknown mode":   importBody(t, importDoc, "merge", "", ""),
		"unknown split":  importBody(t, importDoc, "create", "h9", ""),
		"no slug":        importBody(t, "just text\n", "create", "", ""),
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses/import", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestPreviewCourseMarkdownImportWritesNothing checks that a preview
// reports what would be stored without storing any of it.
func TestPreviewCourseMarkdownImportWritesNothing(t *testing.T) {
	t.Parallel()

	state, router := newAdminState(t)

	rec := adminRequest(t, router, http.MethodPost, "/api/admin/courses/import/preview", importBody(t, importDoc, "create", "h2", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var preview importPreviewResponse

	err := json.Unmarshal(rec.Body.Bytes(), &preview)
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	if preview.Slug != "linux-intro" || len(preview.Modules) != 2 {
		t.Fatalf("preview = %+v", preview)
	}

	if preview.Modules[0].Bytes != len("Bonjour.") {
		t.Errorf("module bytes = %d", preview.Modules[0].Bytes)
	}

	_, err = state.Repos.Courses.Get(t.Context(), "linux-intro")
	if err == nil {
		t.Error("preview stored the course")
	}
}

// TestExportCourseMarkdownRoundTrips checks that an exported course comes
// back through the import endpoint unchanged.
func TestExportCourseMarkdownRoundTrips(t *testing.T) {
	t.Parallel()

	_, router := newAdminState(t)

	adminRequest(t, router, http.MethodPost, "/api/admin/courses/import", importBody(t, importDoc, "create", "h2", ""))

	rec := adminRequest(t, router, http.MethodGet, "/api/admin/courses/linux-intro/export/markdown", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if got := rec.Header().Get("Content-Type"); got != markdownContentType {
		t.Errorf("content type = %q", got)
	}

	if !strings.Contains(rec.Header().Get("Content-Disposition"), `filename="linux-intro.md"`) {
		t.Errorf("disposition = %q", rec.Header().Get("Content-Disposition"))
	}

	document := rec.Body.String()
	if !strings.Contains(document, "title: Linux Intro") || !strings.Contains(document, "Bonjour.") {
		t.Fatalf("exported document = %s", document)
	}

	// The export must import back into the same two modules.
	rec = adminRequest(t, router, http.MethodPost, "/api/admin/courses/import", importBody(t, document, "replace", "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("re-import status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp importResponse

	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ModuleCount != 2 {
		t.Errorf("round trip produced %d modules", resp.ModuleCount)
	}
}
