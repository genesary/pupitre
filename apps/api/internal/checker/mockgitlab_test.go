package checker

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockGitLab is a minimal in-memory GitLab API server covering only the
// endpoints the checker uses. Each field is optional; unset endpoints return
// 404 so error paths can be exercised too.
type mockGitLab struct {
	t *testing.T

	project      map[string]any   // GET /projects/:id
	branches     []map[string]any // GET /projects/:id/repository/branches
	openMRs      []map[string]any // GET /projects/:id/merge_requests?state=opened
	mergedMRs    []map[string]any // GET /projects/:id/merge_requests?state=merged
	mrByIID      map[string]any   // GET /projects/:id/merge_requests/:iid  (single canned MR, iid ignored)
	mrCommits    []map[string]any // GET /projects/:id/merge_requests/:iid/commits
	files        map[string]string
	compareDiffs []map[string]any
	compareCmts  []map[string]any

	projectStatus int // if non-zero, GET /projects/:id returns this status

	// paginate, when true, splits list responses across two pages: page 1
	// returns the first element with an X-Next-Page header, page 2 the rest.
	paginate bool
}

// newMockGitLab returns a mockGitLab bound to the test.
func newMockGitLab(t *testing.T) *mockGitLab {
	t.Helper()

	return &mockGitLab{t: t, files: map[string]string{}}
}

// fetcher spins up the httptest server and returns a GitLabFetcher wired to
// it.
func (m *mockGitLab) fetcher() (*GitLabFetcher, *httptest.Server) {
	srv := httptest.NewServer(http.HandlerFunc(m.route))

	f, err := NewFetcher("test-token", srv.URL)
	if err != nil {
		m.t.Fatalf("NewFetcher: %v", err)
	}

	return f, srv
}

// writeJSON encodes v as the JSON body of the mock response.
func (m *mockGitLab) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		m.t.Fatalf("encode mock response: %v", err)
	}
}

// writeList encodes a slice, splitting it across two pages when paginate is
// set so the client's NextPage loop is exercised.
func (m *mockGitLab) writeList(w http.ResponseWriter, r *http.Request, items []map[string]any) {
	w.Header().Set("Content-Type", "application/json")

	if !m.paginate || len(items) < 2 {
		m.encode(w, items)

		return
	}

	if r.URL.Query().Get("page") == "2" {
		m.encode(w, items[1:])

		return
	}

	w.Header().Set("X-Next-Page", "2")
	w.Header().Set("X-Total-Pages", "2")
	m.encode(w, items[:1])
}

// encode writes v as JSON, failing the test on an encode error.
func (m *mockGitLab) encode(w http.ResponseWriter, v []map[string]any) {
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		m.t.Fatalf("encode mock list: %v", err)
	}
}

// route dispatches an incoming GitLab API request to the canned response for
// the matching endpoint.
func (m *mockGitLab) route(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path

	switch {
	case strings.Contains(p, "/repository/branches"):
		m.writeList(w, r, m.branches)

	case strings.Contains(p, "/repository/compare"):
		m.writeJSON(w, map[string]any{"diffs": m.compareDiffs, "commits": m.compareCmts})

	case strings.Contains(p, "/repository/files/"):
		m.serveFile(w, r, p)

	case strings.Contains(p, "/merge_requests/") && strings.HasSuffix(p, "/commits"):
		m.writeList(w, r, m.mrCommits)

	case strings.Contains(p, "/merge_requests/"):
		if m.mrByIID == nil {
			http.Error(w, "no MR", http.StatusNotFound)

			return
		}

		m.writeJSON(w, m.mrByIID)

	case strings.HasSuffix(p, "/merge_requests"):
		if r.URL.Query().Get("state") == "merged" {
			m.writeList(w, r, m.mergedMRs)

			return
		}

		m.writeList(w, r, m.openMRs)

	case strings.Contains(p, "/projects/"):
		if m.projectStatus != 0 {
			http.Error(w, "boom", m.projectStatus)

			return
		}

		m.writeJSON(w, m.project)

	default:
		http.Error(w, "unhandled: "+p, http.StatusNotFound)
	}
}

// serveFile returns the base64-wrapped contents of a repository file, or 404
// when the mock has no such file.
func (m *mockGitLab) serveFile(w http.ResponseWriter, r *http.Request, path string) {
	_, name, _ := strings.Cut(path, "/repository/files/")

	content, ok := m.files[name]
	if !ok {
		http.Error(w, "file not found", http.StatusNotFound)

		return
	}

	m.writeJSON(w, map[string]any{
		"file_name": name,
		"file_path": name,
		"ref":       r.URL.Query().Get("ref"),
		"content":   base64.StdEncoding.EncodeToString([]byte(content)),
		"encoding":  "base64",
	})
}
