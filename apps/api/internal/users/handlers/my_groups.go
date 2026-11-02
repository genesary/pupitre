package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// groupRow is the response shape for a single group in the my-groups endpoint.
type groupRow struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	ParentID    string   `json:"parentId"`
	Depth       int      `json:"depth"`
	MappedRole  string   `json:"mappedRole,omitempty"`
	CourseSlugs []string `json:"courseSlugs"`
}

// MyGroups godoc
// @Summary   List the authenticated user's group memberships with their courses
// @Tags      My
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/my/groups [get].
func (s *State) MyGroups(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)

	rows, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groupIDs := make([]string, 0, len(rows))

	for _, row := range rows {
		if row.Name != defaultGroupName {
			groupIDs = append(groupIDs, row.ID)
		}
	}

	// One enrollment query for every group the user belongs to, rather than
	// one per group.
	coursesByGroup, err := s.Repos.Groups.GetGroupCoursesByGroups(ctx, groupIDs)
	if err != nil {
		zap.L().Error("failed to list group courses", zap.String("userID", claims.Subject), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groups := make([]groupRow, 0, len(rows))

	for _, row := range rows {
		if row.Name == defaultGroupName {
			continue
		}

		slugs := coursesByGroup[row.ID]
		if slugs == nil {
			slugs = []string{}
		}

		groups = append(groups, groupRow{
			ID:          row.ID,
			Name:        row.Name,
			Source:      row.Source,
			ParentID:    row.ParentID,
			Depth:       row.Depth,
			MappedRole:  row.MappedRole,
			CourseSlugs: slugs,
		})
	}

	s.JSON(writer, http.StatusOK, map[string]any{groupsRespKeyGroups: groups, adminJSONKeyTotal: len(groups)})
}

// MyGroupMembers godoc
// @Summary   List members of a group the authenticated user belongs to
// @Tags      My
// @Security  BearerAuth
// @Produce   json
// @Param     groupId  path  string  true  "Group UUID"
// @Success   200  {object}  map[string]interface{}
// @Failure   403  {object}  map[string]string
// @Router    /api/my/groups/{groupId}/members [get].
func (s *State) MyGroupMembers(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	claims := s.claims(request)
	groupID := chi.URLParam(request, "groupId")

	inGroup, err := s.Repos.Groups.UserInAnyGroup(ctx, claims.Subject, []string{groupID})
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	if !inGroup {
		s.Error(writer, http.StatusForbidden, "Not a member of this group")

		return
	}

	members, err := s.Repos.Groups.ListMembers(ctx, groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{adminJSONKeyMembers: members, adminJSONKeyTotal: len(members)})
}
