package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
)

// TestManagerHandlers_GroupsDBErrorAre5xx sweeps every manager route with a
// failing Groups repository (every manager handler starts with a group-scope
// query) and asserts a server error rather than a panic or a leaked 2xx.
func TestManagerHandlers_GroupsDBErrorAre5xx(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("groups db down")}
	r := newTestRouterWithRepos(repos)

	auth := htAuthHeaderForSubject(t, "manager", htManagerID)

	cases := []struct {
		name, method, path, body string
	}{
		{"list users", http.MethodGet, "/api/manager/users", ""},
		{"user enrollments", http.MethodGet, "/api/manager/users/" + htStudentID + "/enrollments", ""},
		{"user path-enrollments", http.MethodGet, "/api/manager/users/" + htStudentID + "/path-enrollments", ""},
		{"enroll user", http.MethodPost, "/api/manager/enrollments/c", `{"userId":"` + htStudentID + `"}`},
		{"unenroll user", http.MethodDelete, "/api/manager/enrollments/c/users/" + htStudentID, ""},
		{"list groups", http.MethodGet, "/api/manager/groups", ""},
		{"create group", http.MethodPost, "/api/manager/groups", `{"name":"g"}`},
		{"delete group", http.MethodDelete, "/api/manager/groups/" + htGroupID, ""},
		{"list group members", http.MethodGet, "/api/manager/groups/" + htGroupID + "/members", ""},
		{"add group member", http.MethodPost, "/api/manager/groups/" + htGroupID + "/members", `{"userId":"` + htStudentID + `"}`},
		{"remove group member", http.MethodDelete, "/api/manager/groups/" + htGroupID + "/members/" + htStudentID, ""},
		{"list path enrollments", http.MethodGet, "/api/manager/paths/p/enrollments", ""},
		{"create subgroup", http.MethodPost, "/api/manager/groups/" + htGroupID + "/subgroups", `{"name":"s"}`},
		{"enroll user path", http.MethodPost, "/api/manager/paths/p/enrollments", `{"userId":"` + htStudentID + `"}`},
		{"unenroll user path", http.MethodDelete, "/api/manager/paths/p/enrollments/" + htStudentID, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reader *strings.Reader
			if tc.body == "" {
				reader = strings.NewReader("")
			} else {
				reader = strings.NewReader(tc.body)
			}

			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, reader)
			req.Header.Set("Authorization", auth)

			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code < 400 {
				t.Errorf("%s %s: want an error status on a DB failure, got %d (%s)",
					tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}
