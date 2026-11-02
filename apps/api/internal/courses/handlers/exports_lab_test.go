package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// seededLabState returns a State whose LabCheck repo holds one passing and
// one failing check across two courses.
func seededLabState(t *testing.T) *State {
	t.Helper()

	mock := newUserServiceMock()
	t.Cleanup(mock.Close)

	s := newTestState(t, mock)
	s.Repos.LabChecks = fake.NewLabCheckRepository(
		models.LabCheck{
			Username: "alice", CourseSlug: "kubernetes-basics", ModuleIndex: 1, ModuleName: "Pods",
			Allow: true, Violations: []string{}, Verified: true,
			CheckedAt: time.Date(2026, time.June, 1, 9, 0, 0, 0, time.UTC),
		},
		models.LabCheck{
			Username: "bob", CourseSlug: "docker-fundamentals", ModuleIndex: 0, ModuleName: "Intro",
			Allow: false, Violations: []string{"missing Dockerfile", "no CI"}, Verified: false,
			CheckedAt: time.Date(2026, time.June, 2, 9, 0, 0, 0, time.UTC),
		},
	)

	return s
}

// labExportReq issues an authenticated admin request to a lab-export route.
func labExportReq(t *testing.T, s *State, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := BuildRouter(s, s.Config, false)

	var rdr *strings.Reader
	if body == "" {
		rdr = strings.NewReader("")
	} else {
		rdr = strings.NewReader(body)
	}

	req := httptest.NewRequestWithContext(t.Context(), method, path, rdr)
	req.Header.Set("Authorization", adminAuthHeader(t, s.Config.JWTSecret))

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	return rec
}

// TestExportLabCategories returns the lab_checks category descriptor.
func TestExportLabCategories(t *testing.T) {
	t.Parallel()

	s := seededLabState(t)

	rec := labExportReq(t, s, http.MethodGet, "/api/admin/exports/lab-checks/categories", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp struct {
		Categories []struct {
			ID            string   `json:"id"`
			DefaultFields []string `json:"defaultFields"`
			Fields        []struct {
				ID string `json:"id"`
			} `json:"fields"`
			Filters []struct {
				ID string `json:"id"`
			} `json:"filters"`
		} `json:"categories"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Categories) != 1 || resp.Categories[0].ID != "lab_checks" {
		t.Fatalf("unexpected categories: %+v", resp.Categories)
	}

	if len(resp.Categories[0].Fields) == 0 || len(resp.Categories[0].Filters) == 0 {
		t.Errorf("expected fields and filters to be populated: %+v", resp.Categories[0])
	}
}

// TestExportLabPreview_Default returns every row with the default field set.
func TestExportLabPreview_Default(t *testing.T) {
	t.Parallel()

	s := seededLabState(t)

	rec := labExportReq(t, s, http.MethodPost, "/api/admin/exports/lab-checks/preview", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Headers    []string            `json:"headers"`
		Rows       []map[string]string `json:"rows"`
		TotalCount int                 `json:"totalCount"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.TotalCount != 2 || len(resp.Rows) != 2 {
		t.Fatalf("want 2 rows, got total=%d rows=%d", resp.TotalCount, len(resp.Rows))
	}

	if len(resp.Headers) == 0 {
		t.Error("expected default headers")
	}
}

// TestExportLabPreview_FilteredByCourseAndResult narrows to the failing
// docker check.
func TestExportLabPreview_FilteredByCourseAndResult(t *testing.T) {
	t.Parallel()

	s := seededLabState(t)

	body := `{"fields":["username","courseslug","allow","violations"],` +
		`"filters":{"course_slug":"docker-fundamentals","allow":"false"}}`

	rec := labExportReq(t, s, http.MethodPost, "/api/admin/exports/lab-checks/preview", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Rows       []map[string]string `json:"rows"`
		TotalCount int                 `json:"totalCount"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.TotalCount != 1 || len(resp.Rows) != 1 {
		t.Fatalf("want 1 row, got total=%d rows=%d", resp.TotalCount, len(resp.Rows))
	}

	row := resp.Rows[0]
	if row["username"] != "bob" || row["courseslug"] != "docker-fundamentals" {
		t.Errorf("unexpected row: %+v", row)
	}

	if !strings.Contains(row["violations"], "missing Dockerfile") {
		t.Errorf("violations not joined: %q", row["violations"])
	}
}

// TestExportLabPreview_DateFilter keeps only checks on or after checked_from.
func TestExportLabPreview_DateFilter(t *testing.T) {
	t.Parallel()

	s := seededLabState(t)

	body := `{"filters":{"checked_from":"2026-06-02","checked_to":"2026-06-03"}}`

	rec := labExportReq(t, s, http.MethodPost, "/api/admin/exports/lab-checks/preview", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp struct {
		TotalCount int `json:"totalCount"`
	}

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.TotalCount != 1 {
		t.Errorf("want 1 row after date filter, got %d", resp.TotalCount)
	}
}

// TestExportLabPreview_BadJSON rejects a malformed body.
func TestExportLabPreview_BadJSON(t *testing.T) {
	t.Parallel()

	s := seededLabState(t)

	rec := labExportReq(t, s, http.MethodPost, "/api/admin/exports/lab-checks/preview", "{not json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestExportLabDownload streams a BOM-prefixed semicolon CSV with a header
// row and one line per check.
func TestExportLabDownload(t *testing.T) {
	t.Parallel()

	s := seededLabState(t)

	rec := labExportReq(t, s, http.MethodPost, "/api/admin/exports/lab-checks/download",
		`{"fields":["username","course_slug","allow"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	body := rec.Body.Bytes()
	if len(body) < 3 || body[0] != 0xEF || body[1] != 0xBB || body[2] != 0xBF {
		t.Fatal("expected UTF-8 BOM prefix")
	}

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q", ct)
	}

	text := string(body[3:])
	lines := strings.Split(strings.TrimSpace(text), "\n")

	if len(lines) != 3 {
		t.Fatalf("want header + 2 rows, got %d lines: %q", len(lines), text)
	}

	if !strings.Contains(lines[0], ";") {
		t.Errorf("expected semicolon-delimited header, got %q", lines[0])
	}

	if !strings.Contains(text, "alice") || !strings.Contains(text, "bob") {
		t.Errorf("CSV missing usernames: %q", text)
	}
}

// TestExportLabDownload_BadJSON rejects a malformed body.
func TestExportLabDownload_BadJSON(t *testing.T) {
	t.Parallel()

	s := seededLabState(t)

	rec := labExportReq(t, s, http.MethodPost, "/api/admin/exports/lab-checks/download", "nope")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}
