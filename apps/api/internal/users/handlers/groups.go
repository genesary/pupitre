package handlers

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// Platform role names used when deriving a user's role from group
// membership synced from an IdP.
const (
	groupsRoleAdmin   = "admin"
	groupsRoleManager = "manager"
	groupsRoleStudent = "student"
)

// JSON response field/message literals reused across group-related handlers.
const (
	groupsRespKeyMessage = "message"
	groupsRespKeyGroups  = "groups"

	groupsMsgCreated       = "Group created"
	groupsMsgDeleted       = "Group deleted"
	groupsMsgMemberAdded   = "Member added"
	groupsMsgMemberRemoved = "Member removed"
)

// syncGroupEnrollments ensures the user is enrolled in every course
// that any of their groups are enrolled in. Called after every login.
func syncGroupEnrollments(ctx context.Context, groups repository.GroupRepository, userID string) {
	_ = groups.SyncEnrollments(ctx, userID)
}

// addToDefaultGroup ensures the user belongs to the platform default group.
// Called after every login regardless of auth method.
func addToDefaultGroup(ctx context.Context, groups repository.GroupRepository, userID string) {
	_ = groups.AddToDefault(ctx, userID)
}

// syncGroupsAndDeriveRole upserts groups from an IdP into the groups table,
// updates the user's group memberships, and returns the highest platform
// role found in group_role_mappings ('admin' beats 'student'). Defaults to
// the user's current role, falling back to 'student'.
func syncGroupsAndDeriveRole(
	ctx context.Context, groups repository.GroupRepository, userID string, groupNames []string, source string,
) (string, error) {
	role, err := groups.SyncGroupsAndDeriveRole(ctx, userID, groupNames, source)
	if err != nil {
		return role, fmt.Errorf("sync groups and derive role: %w", err)
	}

	return role, nil
}

// ── Admin group handlers ───────────────────────────────────────────────────────

// CreateGroup godoc
// @Summary   Create a local group manually
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     body  body  object  true  "name"
// @Success   201  {object}  map[string]string
// @Failure   400  {object}  map[string]string
// @Failure   409  {object}  map[string]string
// @Router    /api/admin/groups [post].
func (s *State) CreateGroup(writer http.ResponseWriter, req *http.Request) {
	var body struct {
		Name string `json:"name"`
	}

	err := decode(req, &body)
	if err != nil || body.Name == "" {
		s.Error(writer, http.StatusBadRequest, "name required")

		return
	}

	groupID, err := s.Repos.Groups.Create(req.Context(), body.Name)
	if err != nil || groupID == "" {
		s.Error(writer, http.StatusConflict, "A group with this name already exists")

		return
	}

	s.JSON(writer, http.StatusCreated, map[string]string{"id": groupID, groupsRespKeyMessage: groupsMsgCreated})
}

// DeleteGroup godoc
// @Summary   Delete a local group
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Produce   json
// @Param     groupId  path  string  true  "Group UUID"
// @Success   200  {object}  map[string]string
// @Failure   404  {object}  map[string]string
// @Failure   409  {object}  map[string]string
// @Router    /api/admin/groups/{groupId} [delete].
func (s *State) DeleteGroup(writer http.ResponseWriter, req *http.Request) {
	groupID := chi.URLParam(req, "groupId")

	hasChildren, err := s.Repos.Groups.HasChildren(req.Context(), groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if hasChildren {
		s.Error(writer, http.StatusConflict, "Cannot delete a group that has subgroups")

		return
	}

	hasMembers, err := s.Repos.Groups.HasMembers(req.Context(), groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if hasMembers {
		s.Error(writer, http.StatusConflict, "Cannot delete a group that has members")

		return
	}

	deleted, err := s.Repos.Groups.Delete(req.Context(), groupID)
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

// UpdateGroup godoc
// @Summary   Update a local group's name
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     groupId  path    string  true  "Group UUID"
// @Param     body     body  object  true    "name"
// @Success   200  {object}  map[string]string
// @Failure   400  {object}  map[string]string
// @Failure   404  {object}  map[string]string
// @Failure   409  {object}  map[string]string
// @Router    /api/admin/groups/{groupId} [put].
func (s *State) UpdateGroup(writer http.ResponseWriter, req *http.Request) {
	groupID := chi.URLParam(req, "groupId")

	var body struct {
		Name string `json:"name"`
	}

	decodeErr := decode(req, &body)
	if decodeErr != nil || body.Name == "" {
		s.Error(writer, http.StatusBadRequest, "name required")

		return
	}

	err := s.Repos.Groups.UpdateGroup(req.Context(), groupID, body.Name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.Error(writer, http.StatusNotFound, "Group not found or not a local group")

			return
		}

		s.Error(writer, http.StatusConflict, "A group with this name already exists")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "Group updated"})
}

// CreateSubgroup godoc
// @Summary   Create a subgroup under an existing group
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     groupId  path    string  true  "Parent group UUID"
// @Param     body     body  object  true    "name"
// @Success   201  {object}  map[string]string
// @Failure   400  {object}  map[string]string
// @Failure   409  {object}  map[string]string
// @Router    /api/admin/groups/{groupId}/subgroups [post].
func (s *State) CreateSubgroup(writer http.ResponseWriter, req *http.Request) {
	parentID := chi.URLParam(req, "groupId")

	var body struct {
		Name string `json:"name"`
	}

	decodeErr := decode(req, &body)
	if decodeErr != nil || body.Name == "" {
		s.Error(writer, http.StatusBadRequest, "name required")

		return
	}

	newID, err := s.Repos.Groups.CreateSubgroup(req.Context(), body.Name, parentID)
	if err != nil {
		s.Error(writer, http.StatusConflict, err.Error())

		return
	}

	s.JSON(writer, http.StatusCreated, map[string]string{"id": newID, groupsRespKeyMessage: groupsMsgCreated})
}

// ListSubgroups godoc
// @Summary   List direct subgroups of a group
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Produce   json
// @Param     groupId  path  string  true  "Group UUID"
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/groups/{groupId}/subgroups [get].
func (s *State) ListSubgroups(writer http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "groupId")

	rows, err := s.Repos.Groups.GetSubgroups(r.Context(), id)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{groupsRespKeyGroups: rows})
}

// ListAncestors godoc
// @Summary   List all ancestor groups (root → parent) of a group
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Produce   json
// @Param     groupId  path  string  true  "Group UUID"
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/groups/{groupId}/ancestors [get].
func (s *State) ListAncestors(writer http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "groupId")

	rows, err := s.Repos.Groups.GetAncestors(r.Context(), id)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{"ancestors": rows})
}

// ListDescendants godoc
// @Summary   List all descendant groups (subtree) of a group
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Produce   json
// @Param     groupId  path  string  true  "Group UUID"
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/groups/{groupId}/descendants [get].
func (s *State) ListDescendants(writer http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "groupId")

	rows, err := s.Repos.Groups.GetDescendants(r.Context(), id)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{"descendants": rows})
}

// MoveGroup godoc
// @Summary   Move a group under a new parent (null parentId = root)
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     groupId  path    string  true  "Group UUID to move"
// @Param     body     body  object  true    "parentId (UUID or null)"
// @Success   200  {object}  map[string]string
// @Failure   400  {object}  map[string]string
// @Router    /api/admin/groups/{groupId}/parent [patch].
func (s *State) MoveGroup(writer http.ResponseWriter, req *http.Request) {
	groupID := chi.URLParam(req, "groupId")

	var body struct {
		ParentID *string `json:"parentId"`
	}

	decodeErr := decode(req, &body)
	if decodeErr != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	err := s.Repos.Groups.MoveGroup(req.Context(), groupID, body.ParentID)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, err.Error())

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "Group moved"})
}

// ListGroups godoc
// @Summary   List all known groups synced from IdP
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/groups [get].
func (s *State) ListGroups(writer http.ResponseWriter, r *http.Request) {
	rows, err := s.Repos.Groups.List(r.Context())
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	type groupRow struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Source      string `json:"source"`
		ParentID    string `json:"parentId"`
		Depth       int    `json:"depth"`
		CreatedAt   string `json:"createdAt"`
		MemberCount int64  `json:"memberCount"`
		MappedRole  string `json:"mappedRole"`
	}

	groups := make([]groupRow, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, groupRow{
			ID: row.ID, Name: row.Name, Source: row.Source,
			ParentID: row.ParentID, Depth: row.Depth,
			CreatedAt: row.CreatedAt, MemberCount: row.MemberCount, MappedRole: row.MappedRole,
		})
	}

	s.JSON(writer, http.StatusOK, map[string]any{groupsRespKeyGroups: groups})
}

// ListGroupMappings godoc
// @Summary   List group → role mappings
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/groups/mappings [get].
func (s *State) ListGroupMappings(writer http.ResponseWriter, r *http.Request) {
	rows, err := s.Repos.Groups.ListMappings(r.Context())
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	type mapping struct {
		GroupName    string `json:"groupName"`
		PlatformRole string `json:"platformRole"`
	}

	mappings := make([]mapping, 0, len(rows))
	for _, row := range rows {
		mappings = append(mappings, mapping{GroupName: row.GroupName, PlatformRole: row.PlatformRole})
	}

	s.JSON(writer, http.StatusOK, map[string]any{"mappings": mappings})
}

// UpsertGroupMapping godoc
// @Summary   Create or update a group → role mapping
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     body  body  object  true  "groupName and platformRole"
// @Success   200   {object}  map[string]string
// @Failure   400   {object}  map[string]string
// @Router    /api/admin/groups/mappings [post].
func (s *State) UpsertGroupMapping(writer http.ResponseWriter, req *http.Request) {
	var body struct {
		GroupName    string `json:"groupName"`
		PlatformRole string `json:"platformRole"`
	}

	err := decode(req, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	if body.GroupName == "" {
		s.Error(writer, http.StatusBadRequest, "groupName required")

		return
	}

	allowedRoles := []string{groupsRoleAdmin, groupsRoleManager, groupsRoleStudent}

	if !slices.Contains(allowedRoles, body.PlatformRole) {
		s.Error(writer, http.StatusBadRequest, "platformRole must be one of: "+strings.Join(allowedRoles, ", "))

		return
	}

	err = s.Repos.Groups.UpsertMapping(req.Context(), body.GroupName, body.PlatformRole)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "Mapping saved"})
}

// DeleteGroupMapping godoc
// @Summary   Delete a group → role mapping
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Produce   json
// @Param     groupName  path  string  true  "Group name"
// @Success   200  {object}  map[string]string
// @Failure   404  {object}  map[string]string
// @Router    /api/admin/groups/mappings/{groupName} [delete].
func (s *State) DeleteGroupMapping(writer http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "groupName")

	deleted, err := s.Repos.Groups.DeleteMapping(r.Context(), name)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !deleted {
		s.Error(writer, http.StatusNotFound, "Mapping not found")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "Mapping deleted"})
}
