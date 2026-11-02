package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/fake"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// htManagerID is a fixed UUID used as the manager's subject in manager tests.
const htManagerID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

// htStudentID is a fixed UUID used as a scope-member student in manager tests.
const htStudentID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

// htOutsideID is a fixed UUID for a user outside the manager's scope.
const htOutsideID = "cccccccc-cccc-cccc-cccc-cccccccccccc"

// htGroupID is a fixed group UUID used in manager tests.
const htGroupID = "dddddddd-dddd-dddd-dddd-dddddddddddd"

// seedManagerRepos builds repos with a manager, a scoped student, an outside
// user, and a group containing both the manager and the student.
func seedManagerRepos(t *testing.T) *repository.Repositories {
	t.Helper()

	repos := fake.NewRepositories()

	managerUUID := uuid.MustParse(htManagerID)
	studentUUID := uuid.MustParse(htStudentID)
	outsideUUID := uuid.MustParse(htOutsideID)
	groupUUID := uuid.MustParse(htGroupID)

	repos.Users = fake.NewUserRepository(
		models.User{ID: managerUUID, Username: "manager", Email: "mgr@test.com", Role: "manager", IsActive: true, AuthProvider: "local"},
		models.User{ID: studentUUID, Username: "student", Email: "stu@test.com", Role: "student", IsActive: true, AuthProvider: "local"},
		models.User{ID: outsideUUID, Username: "outside", Email: "out@test.com", Role: "student", IsActive: true, AuthProvider: "local"},
	)

	groups := fake.NewGroupRepository(models.Group{ID: groupUUID, Name: "team-rh", Source: "local"})
	groups.UserGroup = []models.UserGroup{
		{UserID: htManagerID, GroupID: groupUUID},
		{UserID: htStudentID, GroupID: groupUUID},
	}
	repos.Groups = groups

	return repos
}

// ── ManagerListUsers ──────────────────────────────────────────────────────────

// TestManagerListUsers_Unauthorized verifies that missing auth returns 401.
func TestManagerListUsers_Unauthorized(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/manager/users", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestManagerListUsers_ForbiddenStudent verifies that a student is rejected.
func TestManagerListUsers_ForbiddenStudent(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/manager/users", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerListUsers_ForbiddenAdmin verifies that an admin is rejected by the
// manager middleware.
func TestManagerListUsers_ForbiddenAdmin(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/manager/users", "", htAuthHeader(t, "admin"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerListUsers_Empty verifies 200 with empty lists when the manager
// has no groups.
func TestManagerListUsers_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/manager/users", "", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["total"] == nil {
		t.Error("expected total in response")
	}
}

// TestManagerListUsers_WithGroups verifies 200 with users when the manager
// belongs to a group.
func TestManagerListUsers_WithGroups(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/manager/users", "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if _, ok := resp["groups"]; !ok {
		t.Error("expected groups in response")
	}
}

// TestManagerListUsers_DBError verifies 500 when group lookup fails.
func TestManagerListUsers_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/manager/users", "", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── ManagerEnrollUser ─────────────────────────────────────────────────────────

// TestManagerEnrollUser_MissingUserID verifies 400 when userId is absent.
func TestManagerEnrollUser_MissingUserID(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/manager/enrollments/my-course", `{"userId":""}`, htAuthHeader(t, "manager"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestManagerEnrollUser_InvalidJSON verifies 400 for malformed body.
func TestManagerEnrollUser_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/manager/enrollments/my-course", "not-json", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestManagerEnrollUser_OutOfScope verifies 403 when target user is not in any
// of the manager's groups.
func TestManagerEnrollUser_OutOfScope(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	body := `{"userId":"` + htOutsideID + `"}`

	rec := htDo(t, r, "POST", "/api/manager/enrollments/my-course", body, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 (out of scope), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManagerEnrollUser_Success verifies 200 when target user is in scope.
func TestManagerEnrollUser_Success(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	body := `{"userId":"` + htStudentID + `"}`

	rec := htDo(t, r, "POST", "/api/manager/enrollments/my-course", body, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManagerEnrollUser_DBError verifies 500 when group repo fails.
func TestManagerEnrollUser_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	body := `{"userId":"` + htStudentID + `"}`

	rec := htDo(t, r, "POST", "/api/manager/enrollments/my-course", body, htAuthHeader(t, "manager"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── ManagerUnenrollUser ───────────────────────────────────────────────────────

// TestManagerUnenrollUser_OutOfScope verifies 403 when target user is not in
// scope.
func TestManagerUnenrollUser_OutOfScope(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/manager/enrollments/my-course/users/"+htOutsideID, "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerUnenrollUser_Success verifies 200 when target user is in scope.
func TestManagerUnenrollUser_Success(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/manager/enrollments/my-course/users/"+htStudentID, "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManagerUnenrollUser_DBErrorGroups verifies 500 when group lookup fails.
func TestManagerUnenrollUser_DBErrorGroups(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/manager/enrollments/my-course/users/"+htStudentID, "", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// TestManagerUnenrollUser_DBErrorEnrollment verifies 500 when the enrollment
// delete fails.
func TestManagerUnenrollUser_DBErrorEnrollment(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	enrollments := fake.NewEnrollmentRepository()
	enrollments.Err = errors.New("db down")
	repos.Enrollments = enrollments
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/manager/enrollments/my-course/users/"+htStudentID, "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── ManagerGetUserEnrollments ─────────────────────────────────────────────────

// TestManagerGetUserEnrollments_OutOfScope verifies 403 when target user is
// not in scope.
func TestManagerGetUserEnrollments_OutOfScope(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/manager/users/"+htOutsideID+"/enrollments", "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerGetUserEnrollments_Success verifies 200 when target user is in
// scope.
func TestManagerGetUserEnrollments_Success(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/manager/users/"+htStudentID+"/enrollments", "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManagerGetUserEnrollments_DBError verifies 500 when group lookup fails.
func TestManagerGetUserEnrollments_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/manager/users/"+htStudentID+"/enrollments", "", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── ManagerListGroups ─────────────────────────────────────────────────────────

// TestManagerListGroups_Unauthorized verifies 401 without token.
func TestManagerListGroups_Unauthorized(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/manager/groups", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

// TestManagerListGroups_ForbiddenStudent verifies 403 for students.
func TestManagerListGroups_ForbiddenStudent(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/manager/groups", "", htAuthHeader(t, "student"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerListGroups_Empty verifies 200 with an empty list.
func TestManagerListGroups_Empty(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "GET", "/api/manager/groups", "", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if _, ok := resp["groups"]; !ok {
		t.Error("expected groups key in response")
	}
}

// TestManagerListGroups_WithGroups verifies 200 with the manager's groups.
func TestManagerListGroups_WithGroups(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/manager/groups", "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	groups := htSliceField(t, resp, "groups")
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}

// TestManagerListGroups_DBError verifies 500 when the group repo fails.
func TestManagerListGroups_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/manager/groups", "", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── ManagerCreateGroup ────────────────────────────────────────────────────────

// TestManagerCreateGroup_MissingName verifies 400 when name is absent.
func TestManagerCreateGroup_MissingName(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/manager/groups", `{"name":""}`, htAuthHeader(t, "manager"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestManagerCreateGroup_InvalidJSON verifies 400 for malformed body.
func TestManagerCreateGroup_InvalidJSON(t *testing.T) {
	t.Parallel()

	r := newTestRouter()

	rec := htDo(t, r, "POST", "/api/manager/groups", "not-json", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestManagerCreateGroup_Conflict verifies 409 when the group name is taken.
func TestManagerCreateGroup_Conflict(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = fake.NewGroupRepository(models.Group{Name: "existing-group", Source: "local"})
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/manager/groups", `{"name":"existing-group"}`, htAuthHeader(t, "manager"))
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// TestManagerCreateGroup_Success verifies 201 and manager auto-join.
func TestManagerCreateGroup_Success(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/manager/groups", `{"name":"new-team"}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["id"] == nil || resp["id"] == "" {
		t.Error("expected group id in response")
	}
}

// TestManagerCreateGroup_DBError verifies 409 when the repo fails.
func TestManagerCreateGroup_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/manager/groups", `{"name":"new-team"}`, htAuthHeader(t, "manager"))
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// ── ManagerDeleteGroup ────────────────────────────────────────────────────────

// TestManagerDeleteGroup_NotOwner verifies 403 when manager is not a member.
func TestManagerDeleteGroup_NotOwner(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	// htGroupID exists but manager (htManagerID) is a member; use a random UUID
	// that does not exist in any of the manager's groups.
	outsideGroupID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

	rec := htDo(t, r, "DELETE", "/api/manager/groups/"+outsideGroupID, "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerDeleteGroup_NotFound verifies 404 for a non-local group.
func TestManagerDeleteGroup_NotFound(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)

	groups, ok := repos.Groups.(*fake.GroupRepository)
	if !ok {
		t.Fatal("repos.Groups is not *fake.GroupRepository")
	}

	// Make the group a non-local source so Delete returns false.
	for i := range groups.Groups {
		if groups.Groups[i].ID.String() == htGroupID {
			groups.Groups[i].Source = "ldap"
		}
	}

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/manager/groups/"+htGroupID, "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManagerDeleteGroup_Success verifies 200 on successful deletion.
func TestManagerDeleteGroup_Success(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/manager/groups/"+htGroupID, "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManagerDeleteGroup_DBError verifies 500 when group lookup fails.
func TestManagerDeleteGroup_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/manager/groups/"+htGroupID, "", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── ManagerListGroupMembers ───────────────────────────────────────────────────

// TestManagerListGroupMembers_NotOwner verifies 403 for an unowned group.
func TestManagerListGroupMembers_NotOwner(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	outsideGroupID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

	rec := htDo(t, r, "GET", "/api/manager/groups/"+outsideGroupID+"/members", "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerListGroupMembers_Success verifies 200 with the group's members.
func TestManagerListGroupMembers_Success(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/manager/groups/"+htGroupID+"/members", "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if _, ok := resp["members"]; !ok {
		t.Error("expected members key in response")
	}
}

// TestManagerListGroupMembers_DBError verifies 500 when group lookup fails.
func TestManagerListGroupMembers_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "GET", "/api/manager/groups/"+htGroupID+"/members", "", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── ManagerAddGroupMember ─────────────────────────────────────────────────────

// TestManagerAddGroupMember_NotOwner verifies 403 for an unowned group.
func TestManagerAddGroupMember_NotOwner(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	outsideGroupID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

	rec := htDo(t, r, "POST", "/api/manager/groups/"+outsideGroupID+"/members", `{"userId":"`+htStudentID+`"}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerAddGroupMember_MissingUserID verifies 400 when userId is absent.
func TestManagerAddGroupMember_MissingUserID(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/manager/groups/"+htGroupID+"/members", `{"userId":""}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// TestManagerAddGroupMember_Success verifies 200 on member addition.
func TestManagerAddGroupMember_Success(t *testing.T) {
	t.Parallel()

	// A second group of the manager: the student, in scope through the
	// first group, may be added to it.
	repos := seedManagerRepos(t)

	groups, ok := repos.Groups.(*fake.GroupRepository)
	if !ok {
		t.Fatal("seedManagerRepos must use the fake group repository")
	}

	secondGroup := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	groups.Groups = append(groups.Groups, models.Group{ID: secondGroup, Name: "team-b", Source: "local"})
	groups.UserGroup = append(groups.UserGroup, models.UserGroup{UserID: htManagerID, GroupID: secondGroup})

	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/manager/groups/"+secondGroup.String()+"/members", `{"userId":"`+htStudentID+`"}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManagerAddGroupMember_OutOfScope is the regression test for defect B6:
// a manager cannot pull a user from outside their scope into their group.
func TestManagerAddGroupMember_OutOfScope(t *testing.T) {
	t.Parallel()

	r := newTestRouterWithRepos(seedManagerRepos(t))

	rec := htDo(t, r, "POST", "/api/manager/groups/"+htGroupID+"/members", `{"userId":"`+htOutsideID+`"}`, htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManagerAddGroupMember_DBError verifies 500 when group lookup fails.
func TestManagerAddGroupMember_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "POST", "/api/manager/groups/"+htGroupID+"/members", `{"userId":"`+htStudentID+`"}`, htAuthHeader(t, "manager"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

// ── ManagerRemoveGroupMember ──────────────────────────────────────────────────

// TestManagerRemoveGroupMember_NotOwner verifies 403 for an unowned group.
func TestManagerRemoveGroupMember_NotOwner(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	outsideGroupID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

	rec := htDo(t, r, "DELETE", "/api/manager/groups/"+outsideGroupID+"/members/"+htStudentID, "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

// TestManagerRemoveGroupMember_Success verifies 200 on member removal.
func TestManagerRemoveGroupMember_Success(t *testing.T) {
	t.Parallel()

	repos := seedManagerRepos(t)
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/manager/groups/"+htGroupID+"/members/"+htStudentID, "", htAuthHeaderForSubject(t, "manager", htManagerID))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestManagerRemoveGroupMember_DBError verifies 500 when group lookup fails.
func TestManagerRemoveGroupMember_DBError(t *testing.T) {
	t.Parallel()

	repos := fake.NewRepositories()
	repos.Groups = &fake.GroupRepository{Err: errors.New("db down")}
	r := newTestRouterWithRepos(repos)

	rec := htDo(t, r, "DELETE", "/api/manager/groups/"+htGroupID+"/members/"+htStudentID, "", htAuthHeader(t, "manager"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}
