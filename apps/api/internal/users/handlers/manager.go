package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// defaultGroupName is the platform-wide group every user belongs to;
// it must not count as a manager scope group.
const defaultGroupName = "everyone"

// groupInfo holds the id, name and member list of a manager's scope group.
type groupInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	MemberIDs []string `json:"memberIds"`
}

// managerGroupRow is the per-group response shape for ManagerListGroups.
type managerGroupRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ParentID    string `json:"parentId"`
	Depth       int    `json:"depth"`
	MemberCount int64  `json:"memberCount"`
}

// managerCreateGroupBody is the request body for ManagerCreateGroup.
type managerCreateGroupBody struct {
	Name string `json:"name"`
}

// managerAddMemberBody is the request body for ManagerAddGroupMember.
type managerAddMemberBody struct {
	UserID string `json:"userId"`
}

// managerScopeIDs returns the IDs of a manager's groups, excluding the
// platform-wide default group which should not define scope.
func managerScopeIDs(groups []repository.GroupRow) []string {
	ids := make([]string, 0, len(groups))

	for _, grp := range groups {
		if grp.Name != defaultGroupName {
			ids = append(ids, grp.ID)
		}
	}

	return ids
}

// callerScope returns the users a caller may act on: nil, meaning
// everyone, for an admin; for a manager, the set of members of the groups
// they manage (docs/specs/190-backend-defects.md B6). One query whatever the
// number of groups.
func (s *State) callerScope(ctx context.Context, claims *middleware.Claims) (map[string]bool, error) {
	if claims.Role != groupsRoleManager {
		return nil, nil //nolint:nilnil // nil scope means unrestricted
	}

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("load manager groups: %w", err)
	}

	membersByGroup, err := s.Repos.Groups.ListMemberIDsByGroups(ctx, managerScopeIDs(groups))
	if err != nil {
		return nil, fmt.Errorf("load manager scope: %w", err)
	}

	scope := map[string]bool{}

	for _, memberIDs := range membersByGroup {
		for _, id := range memberIDs {
			scope[id] = true
		}
	}

	return scope, nil
}

// ManagerListUsers godoc
// @Summary   List users in the manager's groups
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/manager/users [get].
func (s *State) ManagerListUsers(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groupIDs := managerScopeIDs(groups)

	users, err := s.Repos.Users.ListByGroupIDs(ctx, groupIDs)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	// One membership query for every scope group, not one per group.
	membersByGroup, err := s.Repos.Groups.ListMemberIDsByGroups(ctx, groupIDs)
	if err != nil {
		zap.L().Error("failed to list scope group members", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	scopeGroups := make([]groupInfo, 0, len(groups))

	for _, grp := range groups {
		if grp.Name == defaultGroupName {
			continue
		}

		memberIDs := membersByGroup[grp.ID]
		if memberIDs == nil {
			memberIDs = []string{}
		}

		scopeGroups = append(scopeGroups, groupInfo{ID: grp.ID, Name: grp.Name, MemberIDs: memberIDs})
	}

	s.JSON(writer, http.StatusOK, map[string]any{
		adminJSONKeyUsers:   users,
		adminJSONKeyTotal:   len(users),
		groupsRespKeyGroups: scopeGroups,
	})
}

// ManagerEnrollUser godoc
// @Summary   Enroll a user in a course (manager, scope-restricted)
// @Tags      Manager
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     slug  path    string  true  "Course slug"
// @Param     body  body  object  true    "userId (UUID)"
// @Success   200   {object}  map[string]string
// @Failure   400   {object}  map[string]string
// @Failure   403   {object}  map[string]string
// @Router    /api/manager/courses/{slug}/enrollments [post].
func (s *State) ManagerEnrollUser(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	slug := param(request, "slug")
	claims := s.claims(request)

	var body struct {
		UserID string `json:"userId"`
	}

	err := decode(request, &body)
	if err != nil || body.UserID == "" {
		s.Error(writer, http.StatusBadRequest, "userId is required")

		return
	}

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groupIDs := managerScopeIDs(groups)

	inScope, err := s.Repos.Groups.UserInAnyGroup(ctx, body.UserID, groupIDs)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !inScope {
		s.Error(writer, http.StatusForbidden, "User not in manager scope")

		return
	}

	err = s.Repos.Enrollments.Create(ctx, body.UserID, slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: adminMsgUserEnrolled})
}

// ManagerGetUserEnrollments godoc
// @Summary   Get enrollments of a user in the manager's scope
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Param     userId  path  string  true  "User UUID"
// @Success   200     {object}  map[string]interface{}
// @Failure   403     {object}  map[string]string
// @Router    /api/manager/users/{userId}/enrollments [get].
func (s *State) ManagerGetUserEnrollments(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	userID := param(request, "userId")
	claims := s.claims(request)

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groupIDs := managerScopeIDs(groups)

	inScope, err := s.Repos.Groups.UserInAnyGroup(ctx, userID, groupIDs)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !inScope {
		s.Error(writer, http.StatusForbidden, "User not in manager scope")

		return
	}

	enrollments, err := s.Repos.Enrollments.MyEnrollments(ctx, userID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{adminJSONKeyEnrollments: enrollments})
}

// managerOwnsGroup reports whether the manager identified by managerID is a
// member of groupID (and thus allowed to manage it). The manager must not
// rely solely on the "everyone" default group.
func (s *State) managerOwnsGroup(request *http.Request, managerID, groupID string) (bool, error) {
	groups, err := s.Repos.Groups.GetGroupsByUserID(request.Context(), managerID)
	if err != nil {
		return false, fmt.Errorf("get manager groups: %w", err)
	}

	for _, grp := range groups {
		if grp.ID == groupID && grp.Name != defaultGroupName {
			return true, nil
		}
	}

	return false, nil
}

// ManagerListGroups godoc
// @Summary   List the manager's own groups
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/manager/groups [get].
func (s *State) ManagerListGroups(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	result := make([]managerGroupRow, 0, len(groups))

	for _, grp := range groups {
		if grp.Name == defaultGroupName {
			continue
		}

		result = append(result, managerGroupRow{
			ID: grp.ID, Name: grp.Name,
			ParentID: grp.ParentID, Depth: grp.Depth,
			MemberCount: grp.MemberCount,
		})
	}

	s.JSON(writer, http.StatusOK, map[string]any{groupsRespKeyGroups: result})
}

// ManagerCreateGroup godoc
// @Summary   Create a local group and auto-add the manager as member
// @Tags      Manager
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     body  body  object  true  "name"
// @Success   201  {object}  map[string]string
// @Failure   400  {object}  map[string]string
// @Failure   409  {object}  map[string]string
// @Router    /api/manager/groups [post].
func (s *State) ManagerCreateGroup(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)

	var body managerCreateGroupBody

	err := decode(request, &body)
	if err != nil || body.Name == "" {
		s.Error(writer, http.StatusBadRequest, "name required")

		return
	}

	groupID, err := s.Repos.Groups.CreateAndJoin(ctx, body.Name, claims.Subject)
	if err != nil || groupID == "" {
		s.Error(writer, http.StatusConflict, "A group with this name already exists")

		return
	}

	s.JSON(writer, http.StatusCreated, map[string]string{"id": groupID, groupsRespKeyMessage: groupsMsgCreated})
}

// ManagerCreateSubgroup godoc
// @Summary   Create a subgroup under a group the manager owns
// @Tags      Manager
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     groupId  path    string  true  "Parent group UUID"
// @Param     body     body  object  true    "name"
// @Success   201  {object}  map[string]string
// @Failure   400  {object}  map[string]string
// @Failure   403  {object}  map[string]string
// @Failure   409  {object}  map[string]string
// @Router    /api/manager/groups/{groupId}/subgroups [post].
func (s *State) ManagerCreateSubgroup(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)
	parentID := param(request, "groupId")

	owns, err := s.managerOwnsGroup(request, claims.Subject, parentID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !owns {
		s.Error(writer, http.StatusForbidden, "Group not in manager scope")

		return
	}

	var body managerCreateGroupBody

	decodeErr := decode(request, &body)
	if decodeErr != nil || body.Name == "" {
		s.Error(writer, http.StatusBadRequest, "name required")

		return
	}

	// The manager becomes a member of the subgroup, so that it stays in
	// their scope (docs/specs/190-backend-defects.md B1).
	newID, err := s.Repos.Groups.CreateSubgroup(ctx, body.Name, parentID, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusConflict, err.Error())

		return
	}

	s.JSON(writer, http.StatusCreated, map[string]string{"id": newID, groupsRespKeyMessage: groupsMsgCreated})
}

// ManagerDeleteGroup godoc
// @Summary   Delete a group the manager owns (is member of)
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Param     groupId  path  string  true  "Group UUID"
// @Success   200  {object}  map[string]string
// @Failure   403  {object}  map[string]string
// @Failure   404  {object}  map[string]string
// @Router    /api/manager/groups/{groupId} [delete].
func (s *State) ManagerDeleteGroup(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)
	groupID := param(request, "groupId")

	owns, err := s.managerOwnsGroup(request, claims.Subject, groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !owns {
		s.Error(writer, http.StatusForbidden, "Group not in manager scope")

		return
	}

	deleted, err := s.Repos.Groups.Delete(ctx, groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !deleted {
		s.Error(writer, http.StatusNotFound, "Group not found or not a local group")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: groupsMsgDeleted})
}

// ManagerListGroupMembers godoc
// @Summary   List members of a group the manager owns
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Param     groupId  path  string  true  "Group UUID"
// @Success   200  {object}  map[string]interface{}
// @Failure   403  {object}  map[string]string
// @Router    /api/manager/groups/{groupId}/members [get].
func (s *State) ManagerListGroupMembers(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)
	groupID := chi.URLParam(request, "groupId")

	owns, err := s.managerOwnsGroup(request, claims.Subject, groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !owns {
		s.Error(writer, http.StatusForbidden, "Group not in manager scope")

		return
	}

	members, err := s.Repos.Groups.ListMembers(ctx, groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{"members": members})
}

// ManagerAddGroupMember godoc
// @Summary   Add a member to a group the manager owns
// @Tags      Manager
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     groupId  path    string  true  "Group UUID"
// @Param     body     body  object  true    "userId"
// @Success   200  {object}  map[string]string
// @Failure   400  {object}  map[string]string
// @Failure   403  {object}  map[string]string
// @Router    /api/manager/groups/{groupId}/members [post].
func (s *State) ManagerAddGroupMember(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)
	groupID := chi.URLParam(request, "groupId")

	owns, err := s.managerOwnsGroup(request, claims.Subject, groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !owns {
		s.Error(writer, http.StatusForbidden, "Group not in manager scope")

		return
	}

	var body managerAddMemberBody

	err = decode(request, &body)
	if err != nil || body.UserID == "" {
		s.Error(writer, http.StatusBadRequest, "userId required")

		return
	}

	// A manager adds users already in their scope: the page offers no
	// one else, and adding anyone would widen the scope at will
	// (docs/specs/190-backend-defects.md B6).
	scope, err := s.callerScope(ctx, claims)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if scope != nil && !scope[body.UserID] {
		s.Error(writer, http.StatusForbidden, "User not in manager scope")

		return
	}

	err = s.Repos.Groups.AddMember(ctx, groupID, body.UserID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: groupsMsgMemberAdded})
}

// ManagerRemoveGroupMember godoc
// @Summary   Remove a member from a group the manager owns
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Param     groupId  path  string  true  "Group UUID"
// @Param     userId   path  string  true  "User UUID"
// @Success   200  {object}  map[string]string
// @Failure   403  {object}  map[string]string
// @Router    /api/manager/groups/{groupId}/members/{userId} [delete].
func (s *State) ManagerRemoveGroupMember(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)
	groupID := chi.URLParam(request, "groupId")
	userID := chi.URLParam(request, "userId")

	owns, err := s.managerOwnsGroup(request, claims.Subject, groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !owns {
		s.Error(writer, http.StatusForbidden, "Group not in manager scope")

		return
	}

	err = s.Repos.Groups.RemoveMember(ctx, groupID, userID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: groupsMsgMemberRemoved})
}

// ManagerUnenrollUser godoc
// @Summary   Unenroll a user from a course (manager, scope-restricted)
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Param     slug    path  string  true  "Course slug"
// @Param     userId  path  string  true  "User UUID"
// @Success   200     {object}  map[string]string
// @Failure   403     {object}  map[string]string
// @Router    /api/manager/courses/{slug}/enrollments/{userId} [delete].
func (s *State) ManagerUnenrollUser(writer http.ResponseWriter, request *http.Request) { //nolint:dupl // scope check identical to ManagerUnenrollUserPath; different repo (Enrollments vs Paths)
	ctx := request.Context()
	slug := param(request, "slug")
	userID := param(request, "userId")
	claims := s.claims(request)

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groupIDs := managerScopeIDs(groups)

	inScope, err := s.Repos.Groups.UserInAnyGroup(ctx, userID, groupIDs)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !inScope {
		s.Error(writer, http.StatusForbidden, "User not in manager scope")

		return
	}

	err = s.Repos.Enrollments.Delete(ctx, userID, slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: adminMsgUserUnenrolled})
}

// ManagerGetUserPathEnrollments godoc
// @Summary   Get path enrollments of a user in the manager's scope
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Param     userId  path  string  true  "User UUID"
// @Success   200     {object}  map[string]interface{}
// @Failure   403     {object}  map[string]string
// @Router    /api/manager/users/{userId}/path-enrollments [get].
func (s *State) ManagerGetUserPathEnrollments(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	userID := param(request, "userId")
	claims := s.claims(request)

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groupIDs := managerScopeIDs(groups)

	inScope, err := s.Repos.Groups.UserInAnyGroup(ctx, userID, groupIDs)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !inScope {
		s.Error(writer, http.StatusForbidden, "User not in manager scope")

		return
	}

	enrollments, err := s.Repos.Paths.MyEnrollments(ctx, userID, nil, 0)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if enrollments == nil {
		enrollments = []repository.PathEnrollmentRow{}
	}

	s.JSON(writer, http.StatusOK, map[string]any{"pathEnrollments": enrollments})
}

// ManagerEnrollUserPath godoc
// @Summary   Enroll a user in a learning path (manager, scope-restricted)
// @Tags      Manager
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     slug  path    string  true  "Path slug"
// @Param     body  body  object  true    "userId (UUID)"
// @Success   200   {object}  map[string]string
// @Failure   400   {object}  map[string]string
// @Failure   403   {object}  map[string]string
// @Router    /api/manager/paths/{slug}/enrollments [post].
func (s *State) ManagerEnrollUserPath(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	slug := param(request, "slug")
	claims := s.claims(request)

	var body struct {
		UserID string `json:"userId"`
	}

	err := decode(request, &body)
	if err != nil || body.UserID == "" {
		s.Error(writer, http.StatusBadRequest, "userId is required")

		return
	}

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groupIDs := managerScopeIDs(groups)

	inScope, err := s.Repos.Groups.UserInAnyGroup(ctx, body.UserID, groupIDs)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !inScope {
		s.Error(writer, http.StatusForbidden, "User not in manager scope")

		return
	}

	detail, detailErr := s.fetchPathDetail(request, slug)
	if detailErr != nil {
		zap.L().Warn("could not fetch path detail for enrollment", zap.String("slug", slug), zap.Error(detailErr))
	}

	var courseSlugs []string
	if detail != nil {
		courseSlugs = detail.Courses
	}

	err = s.Repos.Paths.EnrollWithCourses(ctx, body.UserID, slug, courseSlugs)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: adminMsgUserEnrolled})
}

// ManagerUnenrollUserPath godoc
// @Summary   Unenroll a user from a learning path (manager, scope-restricted)
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Param     slug    path  string  true  "Path slug"
// @Param     userId  path  string  true  "User UUID"
// @Success   200     {object}  map[string]string
// @Failure   403     {object}  map[string]string
// @Router    /api/manager/paths/{slug}/enrollments/{userId} [delete].
func (s *State) ManagerUnenrollUserPath(writer http.ResponseWriter, request *http.Request) { //nolint:dupl // scope check identical to ManagerUnenrollUser; different repo (Paths vs Enrollments)
	ctx := request.Context()
	slug := param(request, "slug")
	userID := param(request, "userId")
	claims := s.claims(request)

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groupIDs := managerScopeIDs(groups)

	inScope, err := s.Repos.Groups.UserInAnyGroup(ctx, userID, groupIDs)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !inScope {
		s.Error(writer, http.StatusForbidden, "User not in manager scope")

		return
	}

	err = s.Repos.Paths.Unenroll(ctx, userID, slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: adminMsgUserUnenrolled})
}
