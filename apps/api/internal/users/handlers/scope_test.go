package handlers

import (
	"net/http"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
)

// Regression tests for defect B6 (docs/specs/190-backend-defects.md): a
// manager acts on the members of the groups they manage, no one else.

// TestExport_ManagerIsScoped checks that a manager's export is limited to
// their groups, and an admin's is not.
func TestExport_ManagerIsScoped(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	exports := fake.NewExportRepository()
	repos.Export = exports
	r := newTestRouterWithRepos(repos)

	body := `{"category":"users","fields":["email"]}`

	rec := htDo(t, r, http.MethodPost, "/api/admin/exports/preview", body, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("manager preview: %d %s", rec.Code, rec.Body.String())
	}

	if !exports.LastScope.Restricted || len(exports.LastScope.GroupIDs) != 1 || exports.LastScope.GroupIDs[0] != htGroupID {
		t.Fatalf("manager export scope = %+v, want their group only", exports.LastScope)
	}

	rec = htDo(t, r, http.MethodPost, "/api/admin/exports/preview", body, htAuthHeader(t, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("admin preview: %d %s", rec.Code, rec.Body.String())
	}

	if exports.LastScope.Restricted {
		t.Fatalf("admin export scope = %+v, want unrestricted", exports.LastScope)
	}
}

// TestMarkSessionPresence_ManagerOutOfScope checks that a manager cannot mark
// the attendance of a learner outside their groups, but can inside.
func TestMarkSessionPresence_ManagerOutOfScope(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(seedManagerRepos(t))
	auth := htAuthHeaderForSubject(t, "manager", htManagerID)

	rec := htDo(t, r, http.MethodPatch, "/api/manager/session-bookings/c/s/"+htOutsideID+"/presence", `{"present":true}`, auth)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("out of scope: want 403, got %d %s", rec.Code, rec.Body.String())
	}

	rec = htDo(t, r, http.MethodPatch, "/api/manager/session-bookings/c/s/"+htStudentID+"/presence", `{"present":true}`, auth)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("in scope: got 403 %s", rec.Body.String())
	}
}
