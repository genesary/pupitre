package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
)

// TestManagerGetUserPathEnrollments_InScope returns the target user's path
// enrollments when they share a group with the manager.
func TestManagerGetUserPathEnrollments_InScope(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/manager/users/"+htStudentID+"/path-enrollments",
		"", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, ok := resp["pathEnrollments"]; !ok {
		t.Errorf("expected pathEnrollments key, got %v", resp)
	}
}

// TestManagerGetUserPathEnrollments_OutOfScope is forbidden for a user
// outside the manager's groups.
func TestManagerGetUserPathEnrollments_OutOfScope(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/manager/users/"+htOutsideID+"/path-enrollments",
		"", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerEnrollUserPath_InScope enrolls a scoped user in a path.
func TestManagerEnrollUserPath_InScope(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/manager/paths/devops-path/enrollments",
		`{"userId":"`+htStudentID+`"}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// The enrollment must now be visible.
	enrolled, err := repos.Paths.EnrolledInAny(t.Context(), htStudentID, []string{"devops-path"})
	if err != nil {
		t.Fatalf("EnrolledInAny: %v", err)
	}

	if !enrolled {
		t.Error("student was not enrolled in the path")
	}
}

// TestManagerEnrollUserPath_MissingUserID rejects a body with no userId.
func TestManagerEnrollUserPath_MissingUserID(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/manager/paths/devops-path/enrollments",
		`{}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestManagerEnrollUserPath_OutOfScope is forbidden for a non-scoped user.
func TestManagerEnrollUserPath_OutOfScope(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/manager/paths/devops-path/enrollments",
		`{"userId":"`+htOutsideID+`"}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerUnenrollUserPath_InScope unenrolls a scoped user from a path.
func TestManagerUnenrollUserPath_InScope(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)

	err := repos.Paths.EnrollWithCourses(t.Context(), htStudentID, "devops-path", nil)
	if err != nil {
		t.Fatalf("seed enrollment: %v", err)
	}

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodDelete, "/api/manager/paths/devops-path/enrollments/"+htStudentID,
		"", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	enrolled, err := repos.Paths.EnrolledInAny(t.Context(), htStudentID, []string{"devops-path"})
	if err != nil {
		t.Fatalf("EnrolledInAny: %v", err)
	}

	if enrolled {
		t.Error("student is still enrolled after unenroll")
	}
}

// TestManagerUnenrollUserPath_OutOfScope is forbidden for a non-scoped user.
func TestManagerUnenrollUserPath_OutOfScope(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodDelete, "/api/manager/paths/devops-path/enrollments/"+htOutsideID,
		"", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerUnenrollUserPath_DBError surfaces a repository failure as 500.
func TestManagerUnenrollUserPath_DBError(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	paths := fake.NewPathEnrollmentRepository()
	paths.Err = errors.New("db down")
	repos.Paths = paths
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodDelete, "/api/manager/paths/devops-path/enrollments/"+htStudentID,
		"", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestManagerCreateSubgroup_OwnedParent creates a subgroup under a group the
// manager belongs to.
func TestManagerCreateSubgroup_OwnedParent(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/manager/groups/"+htGroupID+"/subgroups",
		`{"name":"sub-team"}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["id"] == "" {
		t.Fatal("expected an id in the response")
	}

	// Regression test of defect B1 (docs/specs/190-backend-defects.md): the
	// manager is a member of the new subgroup, so it stays in their scope.
	rec = htDo(t, r, http.MethodGet, "/api/manager/groups", "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if !strings.Contains(rec.Body.String(), resp["id"]) {
		t.Errorf("the new subgroup is not in the manager's groups: %s", rec.Body.String())
	}
}

// TestManagerCreateSubgroup_UnownedParent is forbidden when the manager does
// not belong to the parent group.
func TestManagerCreateSubgroup_UnownedParent(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	other := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

	rec := htDo(t, r, http.MethodPost, "/api/manager/groups/"+other+"/subgroups",
		`{"name":"sub"}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerCreateSubgroup_MissingName rejects an empty name.
func TestManagerCreateSubgroup_MissingName(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodPost, "/api/manager/groups/"+htGroupID+"/subgroups",
		`{}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestManagerListPathEnrollments_ScopeFiltered returns only enrolled users
// who fall within the manager's group scope.
func TestManagerListPathEnrollments_ScopeFiltered(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)

	// The scoped student and the out-of-scope user are both enrolled.
	for _, id := range []string{htStudentID, htOutsideID} {
		err := repos.Paths.EnrollWithCourses(t.Context(), id, "devops-path", nil)
		if err != nil {
			t.Fatalf("seed enrollment: %v", err)
		}
	}

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/manager/paths/devops-path/enrollments",
		"", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string][]map[string]any

	err := json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	users := resp["users"]
	if len(users) != 1 {
		t.Fatalf("want 1 in-scope user, got %d: %+v", len(users), users)
	}
}

// TestManagerGetUserPathEnrollments_DBError surfaces a path repo failure.
func TestManagerGetUserPathEnrollments_DBError(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	paths := fake.NewPathEnrollmentRepository()
	paths.Err = errors.New("db down")
	repos.Paths = paths
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/manager/users/"+htStudentID+"/path-enrollments",
		"", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestManagerListPathEnrollments_DBError surfaces a path repo failure.
func TestManagerListPathEnrollments_DBError(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	paths := fake.NewPathEnrollmentRepository()
	paths.Err = errors.New("db down")
	repos.Paths = paths
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/manager/paths/devops-path/enrollments",
		"", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestManagerGetUserPathEnrollments_GroupsDBError surfaces a groups repo
// failure on the scope query.
func TestManagerGetUserPathEnrollments_GroupsDBError(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	repos.Groups = &fake.GroupRepository{Err: errors.New("groups down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, http.MethodGet, "/api/manager/users/"+htStudentID+"/path-enrollments",
		"", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}
