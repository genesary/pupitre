package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// newExportRouter builds a router whose State points course-service at
// courseSvcURL, so enrichment paths can be exercised.
func newExportRouter(repos *repository.Repositories, courseSvcURL string) http.Handler {
	cfg := &config.Config{
		JWTSecret:   htSecret,
		JWTExpiryH:  htExpiry,
		CORSOrigins: []string{"*"},
	}
	s := &State{Repos: repos, Config: cfg, Courses: courseSourceAt(courseSvcURL)}

	return BuildRouter(s, cfg, false)
}

// TestExportCategories_OK lists the available categories for an admin.
func TestExportCategories_OK(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/admin/exports/categories", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	cats := htSliceField(t, resp, "categories")
	if len(cats) == 0 {
		t.Error("expected at least one category")
	}
}

// TestExportCategories_StudentForbidden rejects a non-manager/admin caller.
func TestExportCategories_StudentForbidden(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodGet, "/api/admin/exports/categories", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestExportPreview_BadJSON rejects a malformed body.
func TestExportPreview_BadJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/preview", "not-json", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestExportPreview_MissingCategory rejects a body with no category.
func TestExportPreview_MissingCategory(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/preview", `{"fields":["email"]}`, htAuthHeader(t, "manager"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestExportPreview_OK returns headers, rows and the total count.
func TestExportPreview_OK(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	exp := fake.NewExportRepository()
	exp.Headers = []string{"email"}
	exp.Rows = []map[string]string{{"email": "a@x.com"}, {"email": "b@x.com"}}
	exp.Total = 2
	repos.Export = exp

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/preview",
		`{"category":"users","fields":["email"]}`, htAuthHeader(t, "manager"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["totalCount"] != float64(2) {
		t.Errorf("totalCount = %v, want 2", resp["totalCount"])
	}
}

// TestExportPreview_UnknownCategory maps an "unknown ..." repo error to 400.
func TestExportPreview_UnknownCategory(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	exp := fake.NewExportRepository()
	exp.FetchErr = errors.New("unknown category: bogus")
	repos.Export = exp

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/preview",
		`{"category":"bogus","fields":["email"]}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestExportPreview_RepoError maps a generic repo error to 500.
func TestExportPreview_RepoError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	exp := fake.NewExportRepository()
	exp.FetchErr = errors.New("connection reset")
	repos.Export = exp

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/preview",
		`{"category":"users","fields":["email"]}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestExportPreview_VirtualFieldEnrichment fills course_title from
// course-service and drops the helper slug column.
func TestExportPreview_VirtualFieldEnrichment(t *testing.T) {
	t.Parallel()

	courseSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"courses":[{"slug":"linux-intro","title":"Linux Intro"}]}`))
	}))
	defer courseSvc.Close()

	repos := fake.NewRepositories()
	exp := fake.NewExportRepository()
	exp.Rows = []map[string]string{{"courseslug": "linux-intro"}}
	repos.Export = exp

	r := newExportRouter(repos, courseSvc.URL)

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/preview",
		`{"category":"enrollments","fields":["course_title"]}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	rows := htSliceField(t, resp, "rows")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := htMapField(t, rows[0])
	if row["course_title"] != "Linux Intro" {
		t.Errorf("course_title = %v, want Linux Intro", row["course_title"])
	}

	if _, leaked := row["courseslug"]; leaked {
		t.Error("helper field courseslug should have been removed")
	}
}

// TestExportPreview_PathTitleEnrichment fills path_title from course-service
// (covering fetchPathTitles) and drops the helper slug column.
func TestExportPreview_PathTitleEnrichment(t *testing.T) {
	t.Parallel()

	courseSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/paths") {
			_, _ = w.Write([]byte(`{"paths":[{"slug":"devops-path","title":"DevOps Path"}]}`))

			return
		}

		_, _ = w.Write([]byte(`{"courses":[]}`))
	}))
	defer courseSvc.Close()

	repos := fake.NewRepositories()
	exp := fake.NewExportRepository()
	exp.Rows = []map[string]string{{"path_slug": "devops-path"}}
	repos.Export = exp

	r := newExportRouter(repos, courseSvc.URL)

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/preview",
		`{"category":"path_enrollments","fields":["path_title"]}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	rows := htSliceField(t, resp, "rows")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := htMapField(t, rows[0])
	if row["path_title"] != "DevOps Path" {
		t.Errorf("path_title = %v, want DevOps Path", row["path_title"])
	}

	if _, leaked := row["path_slug"]; leaked {
		t.Error("helper field path_slug should have been removed")
	}
}

// TestExportDownload_BadJSON rejects a malformed body.
func TestExportDownload_BadJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/download", "{", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestExportDownload_MissingCategory rejects a body with no category.
func TestExportDownload_MissingCategory(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/download", `{"fields":["email"]}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestExportDownload_OK streams a BOM-prefixed CSV and records the audit log.
func TestExportDownload_OK(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	exp := fake.NewExportRepository()
	exp.Rows = []map[string]string{{"email": "a@x.com"}, {"email": "b@x.com"}}
	repos.Export = exp

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/download",
		`{"category":"users","fields":["email"]}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	body := rec.Body.Bytes()
	if len(body) < 3 || body[0] != 0xEF || body[1] != 0xBB || body[2] != 0xBF {
		t.Error("expected UTF-8 BOM prefix")
	}

	if !strings.Contains(rec.Header().Get("Content-Disposition"), "users-export-") {
		t.Errorf("unexpected Content-Disposition: %q", rec.Header().Get("Content-Disposition"))
	}

	if !strings.Contains(rec.Body.String(), "a@x.com") {
		t.Errorf("CSV missing row data: %q", rec.Body.String())
	}

	if len(exp.Logged) != 1 {
		t.Fatalf("expected 1 audit log entry, got %d", len(exp.Logged))
	}

	if exp.Logged[0].RowCount != 2 {
		t.Errorf("logged RowCount = %d, want 2", exp.Logged[0].RowCount)
	}
}

// TestExportDownload_LogFailureStillSucceeds keeps a 200 when the audit
// insert fails (the error is only logged).
func TestExportDownload_LogFailureStillSucceeds(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	exp := fake.NewExportRepository()
	exp.Rows = []map[string]string{{"email": "a@x.com"}}
	exp.LogErr = errors.New("audit table locked")
	repos.Export = exp

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/download",
		`{"category":"users","fields":["email"]}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200 despite audit failure, got %d", rec.Code)
	}
}

// TestExportDownload_EnrichedCSV writes the enriched dataset as CSV when a
// virtual field is requested.
func TestExportDownload_EnrichedCSV(t *testing.T) {
	t.Parallel()

	courseSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"courses":[{"slug":"linux-intro","title":"Linux Intro"}]}`))
	}))
	defer courseSvc.Close()

	repos := fake.NewRepositories()
	exp := fake.NewExportRepository()
	exp.Rows = []map[string]string{{"courseslug": "linux-intro"}}
	repos.Export = exp

	r := newExportRouter(repos, courseSvc.URL)

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/download",
		`{"category":"enrollments","fields":["course_title"]}`, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Linux Intro") {
		t.Errorf("enriched CSV missing title: %q", rec.Body.String())
	}
}
