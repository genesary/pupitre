// Package handlers implements the HTTP handlers for the user-service API.
package handlers

import (
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// Repeated literals used across the admin handlers, pulled out to satisfy
// goconst. Role and "message"/"groups" literals are shared with groups.go
// (groupsRoleAdmin, groupsRoleStudent, groupsRespKeyMessage,
// groupsRespKeyGroups) to avoid duplicate constants in the package.
const (
	adminJSONKeyUsers       = "users"
	adminJSONKeyProviders   = "providers"
	adminJSONKeyEnrolled    = "enrolled"
	adminJSONKeyTotal       = "total"
	adminJSONKeyEnrollments = "enrollments"
	adminJSONKeyMembers     = "members"
	adminMsgUserEnrolled    = "User enrolled"
	adminMsgUserUnenrolled  = "User unenrolled"
)

// AdminStats godoc
// @Summary   Get platform statistics (admin)
// @Tags      Admin - Stats
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/stats [get].
func (s *State) AdminStats(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()

	totalUsers, _ := s.Repos.Users.CountByRole(ctx, groupsRoleStudent)
	totalEnrollments, _ := s.Repos.Enrollments.CountAll(ctx)
	totalCourses, _ := s.Repos.Enrollments.CountDistinctCourses(ctx)

	s.JSON(writer, http.StatusOK, map[string]any{
		"total_users":       totalUsers,
		"total_enrollments": totalEnrollments,
		"totalCourses":      totalCourses,
		"total_labs":        0,
		"total_submissions": 0,
		"success_rate":      "0",
	})
}

// ListAuthProviders godoc
// @Summary   List distinct auth providers in use (admin)
// @Tags      Admin - Users
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/users/providers [get].
func (s *State) ListAuthProviders(writer http.ResponseWriter, request *http.Request) {
	counts, err := s.Repos.Users.ListAuthProviders(request.Context())
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	type providerCount struct {
		Provider string `json:"provider"`
		Count    int64  `json:"count"`
	}

	list := make([]providerCount, 0, len(counts))
	for _, c := range counts {
		list = append(list, providerCount{Provider: c.Provider, Count: c.Count})
	}

	s.JSON(writer, http.StatusOK, map[string]any{adminJSONKeyProviders: list})
}

// userAdminRow is a single user row for the admin listing endpoint.
type userAdminRow struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`

	IsActive bool `json:"isActive"`

	AvatarURL *string `json:"avatarUrl"`
	Bio       *string `json:"bio"`

	AuthProvider string `json:"authProvider"`

	CreatedAt string `json:"createdAt"`

	EnrolledCourses int64 `json:"enrolledCourses"`
}

// userAdminRowFromRepo converts a repository row into its DTO representation.
func userAdminRowFromRepo(r repository.AdminUserRow) userAdminRow {
	return userAdminRow{
		ID: r.ID, Username: r.Username, Email: r.Email, Role: r.Role, IsActive: r.IsActive,
		AvatarURL: r.AvatarURL, Bio: r.Bio, AuthProvider: r.AuthProvider, CreatedAt: r.CreatedAt,
		EnrolledCourses: r.EnrolledCourses,
	}
}

// ListUsers godoc
// @Summary   List all users (admin)
// @Tags      Admin - Users
// @Security  BearerAuth
// @Produce   json
// @Param     provider  query  string  false  "Filter by auth provider"
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/users [get].
func (s *State) ListUsers(writer http.ResponseWriter, request *http.Request) {
	provider := request.URL.Query().Get("provider")

	rows, err := s.Repos.Users.ListForAdmin(request.Context(), provider)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	users := make([]userAdminRow, 0, len(rows))
	for _, r := range rows {
		users = append(users, userAdminRowFromRepo(r))
	}

	s.JSON(writer, http.StatusOK, map[string]any{adminJSONKeyUsers: users, adminJSONKeyTotal: len(users)})
}

// GetUser godoc
// @Summary   Get a user by ID (admin)
// @Tags      Admin - Users
// @Security  BearerAuth
// @Produce   json
// @Param     userId  path  string  true  "User UUID"
// @Success   200   {object}  map[string]interface{}
// @Failure   404   {object}  map[string]string
// @Router    /api/admin/users/{userId} [get].
func (s *State) GetUser(writer http.ResponseWriter, request *http.Request) {
	userID := param(request, "userId")

	type userDetailRow struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`

		IsActive bool `json:"isActive"`

		AvatarURL *string `json:"avatarUrl"`
		Bio       *string `json:"bio"`

		AuthProvider string `json:"authProvider"`

		CreatedAt string `json:"createdAt"`

		EnrolledCourses int64 `json:"enrolledCourses"`
		ViewedLessons   int64 `json:"viewedLessons"`
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "User not found")

		return
	}

	row, err := s.Repos.Users.GetForAdmin(request.Context(), userUUID)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "User not found")

		return
	}

	s.JSON(writer, http.StatusOK, userDetailRow{
		ID: row.ID, Username: row.Username, Email: row.Email, Role: row.Role, IsActive: row.IsActive,
		AvatarURL: row.AvatarURL, Bio: row.Bio, AuthProvider: row.AuthProvider, CreatedAt: row.CreatedAt,
		EnrolledCourses: row.EnrolledCourses, ViewedLessons: row.ViewedLessons,
	})
}

// UpdateUser godoc
// @Summary   Update a user (admin)
// @Tags      Admin - Users
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     userId  path  string  true  "User UUID"
// @Param     body  body  object  true  "Optional fields to update"
// @Success   200   {object}  userPublicRow
// @Failure   404   {object}  map[string]string
// @Router    /api/admin/users/{userId} [put].
func (s *State) UpdateUser(writer http.ResponseWriter, request *http.Request) {
	userID := param(request, "userId")

	var body struct {
		Username *string `json:"username"`
		Bio      *string `json:"bio"`

		AvatarURL *string `json:"avatarUrl"`

		IsActive *bool   `json:"isActive"`
		Role     *string `json:"role"`
	}

	err := decode(request, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	allowedRoles := []string{groupsRoleAdmin, groupsRoleManager, groupsRoleStudent}

	if body.Role != nil && !slices.Contains(allowedRoles, *body.Role) {
		s.Error(writer, http.StatusBadRequest, "Role must be one of: "+strings.Join(allowedRoles, ", "))

		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "User not found")

		return
	}

	user, err := s.Repos.Users.UpdateAdminFields(
		request.Context(), userUUID, body.Username, body.Bio, body.AvatarURL, body.IsActive, body.Role)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "User not found")

		return
	}

	// A deactivation ends the user's sessions; a role change takes effect
	// at once anyway, the guards reading the role of the row
	// (docs/specs/190-backend-defects.md B7).
	if body.IsActive != nil && !*body.IsActive {
		err = s.revokeSessions(request.Context(), userUUID)
		if err != nil {
			zap.L().Error("revoke sessions after deactivation", zap.Error(err))
		}
	}

	s.JSON(writer, http.StatusOK, toUserPublicRow(user))
}

// DeleteUser godoc
// @Summary   Delete a user (admin)
// @Tags      Admin - Users
// @Security  BearerAuth
// @Produce   json
// @Param     userId  path  string  true  "User UUID"
// @Success   200   {object}  map[string]string
// @Router    /api/admin/users/{userId} [delete].
func (s *State) DeleteUser(writer http.ResponseWriter, request *http.Request) {
	userID := param(request, "userId")

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	err = s.Repos.Users.Delete(request.Context(), userUUID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "User deleted"})
}

// SearchUsers godoc
// @Summary   Search users by username or email (admin)
// @Tags      Admin - Users
// @Security  BearerAuth
// @Produce   json
// @Param     q  query  string  true  "Search query"
// @Success   200   {object}  map[string]interface{}
// @Router    /api/admin/users/search [get].
func (s *State) SearchUsers(writer http.ResponseWriter, request *http.Request) {
	q := strings.ToLower(request.URL.Query().Get("q"))

	rows, err := s.Repos.Users.Search(request.Context(), q)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	type result struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}

	users := make([]result, 0, len(rows))
	for _, r := range rows {
		users = append(users, result{ID: r.ID, Username: r.Username, Email: r.Email})
	}

	s.JSON(writer, http.StatusOK, map[string]any{adminJSONKeyUsers: users})
}

// ListCourseEnrollments godoc
// @Summary   List enrollments for a course (admin)
// @Tags      Admin - Courses
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Success   200   {object}  map[string]interface{}
// @Router    /api/admin/courses/{slug}/enrollments [get].
func (s *State) ListCourseEnrollments(writer http.ResponseWriter, request *http.Request) {
	slug := param(request, "slug")

	rows, err := s.Repos.Enrollments.ListByCourse(request.Context(), slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	type enrollment struct {
		UserID   string `json:"userId"`
		Username string `json:"username"`
		Email    string `json:"email"`

		EnrolledAt string `json:"enrolledAt"`
	}

	list := make([]enrollment, 0, len(rows))
	for _, r := range rows {
		list = append(list, enrollment{UserID: r.UserID, Username: r.Username, Email: r.Email, EnrolledAt: r.EnrolledAt})
	}

	s.JSON(writer, http.StatusOK, map[string]any{adminJSONKeyEnrollments: list})
}

// AdminEnrollUser godoc
// @Summary   Enroll a user in a course (admin)
// @Tags      Admin - Courses
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Param     body  body  object  true  "userId (UUID)"
// @Success   200   {object}  map[string]string
// @Router    /api/admin/courses/{slug}/enrollments [post].
func (s *State) AdminEnrollUser(writer http.ResponseWriter, request *http.Request) {
	slug := param(request, "slug")

	var body struct {
		UserID string `json:"userId"`
	}

	err := decode(request, &body)
	if err != nil || body.UserID == "" {
		s.Error(writer, http.StatusBadRequest, "userId is required")

		return
	}

	err = s.Repos.Enrollments.Create(request.Context(), body.UserID, slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: adminMsgUserEnrolled})
}

// AdminUnenrollUser godoc
// @Summary   Unenroll a user from a course (admin)
// @Tags      Admin - Courses
// @Security  BearerAuth
// @Produce   json
// @Param     slug     path  string  true  "Course slug"
// @Param     userId  path  string  true  "User UUID"
// @Success   200   {object}  map[string]string
// @Router    /api/admin/courses/{slug}/enrollments/{userId} [delete].
func (s *State) AdminUnenrollUser(writer http.ResponseWriter, request *http.Request) {
	slug := param(request, "slug")
	userID := param(request, "userId")

	err := s.Repos.Enrollments.Delete(request.Context(), userID, slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: adminMsgUserUnenrolled})
}

// AdminEnrollGroup godoc
// @Summary   Enroll all members of a group in a course (admin)
// @Tags      Admin - Courses
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Param     body  body  object  true  "groupId (UUID)"
// @Success   200   {object}  map[string]interface{}
// @Failure   400   {object}  map[string]string
// @Router    /api/admin/courses/{slug}/enrollments/groups [post].
func (s *State) AdminEnrollGroup(writer http.ResponseWriter, request *http.Request) {
	slug := param(request, "slug")

	var body struct {
		GroupID string `json:"groupId"`
	}

	err := decode(request, &body)
	if err != nil || body.GroupID == "" {
		s.Error(writer, http.StatusBadRequest, "groupId is required")

		return
	}

	backfilled, err := s.Repos.Groups.EnrollCourse(request.Context(), body.GroupID, slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{
		groupsRespKeyMessage: "Group enrolled",
		adminJSONKeyEnrolled: backfilled,
	})
}

// AdminListGroupEnrollments godoc
// @Summary   List groups enrolled in a course (admin)
// @Tags      Admin - Courses
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Success   200   {object}  map[string]interface{}
// @Router    /api/admin/courses/{slug}/enrollments/groups [get].
func (s *State) AdminListGroupEnrollments(writer http.ResponseWriter, request *http.Request) {
	slug := param(request, "slug")

	rows, err := s.Repos.Groups.ListCourseEnrollments(request.Context(), slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	type row struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Source string `json:"source"`

		MemberCount int64 `json:"memberCount"`

		EnrolledAt string `json:"enrolledAt"`
	}

	list := make([]row, 0, len(rows))
	for _, r := range rows {
		list = append(list, row{ID: r.ID, Name: r.Name, Source: r.Source, MemberCount: r.MemberCount, EnrolledAt: r.EnrolledAt})
	}

	s.JSON(writer, http.StatusOK, map[string]any{groupsRespKeyGroups: list})
}

// AdminUnenrollGroup godoc
// @Summary   Remove a group enrollment from a course (admin)
// @Tags      Admin - Courses
// @Security  BearerAuth
// @Produce   json
// @Param     slug      path  string  true  "Course slug"
// @Param     groupId  path  string  true  "Group UUID"
// @Success   200  {object}  map[string]string
// @Router    /api/admin/courses/{slug}/enrollments/groups/{groupId} [delete].
func (s *State) AdminUnenrollGroup(writer http.ResponseWriter, request *http.Request) {
	slug := param(request, "slug")
	groupID := param(request, "groupId")

	err := s.Repos.Groups.UnenrollCourse(request.Context(), groupID, slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "Group enrollment removed"})
}

// SyncProgress godoc
// @Summary   Sync lesson progress from Course Service (admin)
// @Tags      Admin - Users
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     body  body  object  true  "userId, courseSlug, lessonSlug"
// @Success   200   {object}  map[string]bool
// @Failure   400   {object}  map[string]string
// @Router    /api/admin/sync-progress [post].
func (s *State) SyncProgress(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		UserID string `json:"userId"`

		CourseSlug string `json:"courseSlug"`

		LessonSlug string `json:"lessonSlug"`
	}

	err := decode(request, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	if body.UserID == "" || body.CourseSlug == "" || body.LessonSlug == "" {
		s.Error(writer, http.StatusBadRequest, "userId, courseSlug, and lessonSlug are required")

		return
	}

	err = s.Repos.LessonProgress.MarkComplete(request.Context(), body.UserID, body.CourseSlug, body.LessonSlug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]bool{"ok": true})
}

// AdminLeaderboard godoc
// @Summary   Get users ranked by total quiz score (admin)
// @Tags      Admin - Stats
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/leaderboard [get].
func (s *State) AdminLeaderboard(writer http.ResponseWriter, request *http.Request) {
	rows, err := s.Repos.Users.Leaderboard(request.Context())
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	type leaderboardRow struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`

		AvatarURL *string `json:"avatarUrl"`

		TotalScore int64 `json:"totalScore"`

		PassedModules int64 `json:"passedModules"`

		EnrolledCourses int64 `json:"enrolledCourses"`
	}

	leaderboard := make([]leaderboardRow, 0, len(rows))
	for _, r := range rows {
		leaderboard = append(leaderboard, leaderboardRow{
			ID: r.ID, Username: r.Username, Email: r.Email, AvatarURL: r.AvatarURL,
			TotalScore: r.TotalScore, PassedModules: r.PassedModules, EnrolledCourses: r.EnrolledCourses,
		})
	}

	s.JSON(writer, http.StatusOK, map[string]any{leaderboardResponseKey: leaderboard})
}

// ListGroupMembers godoc
// @Summary   List members of a group (admin); add ?recursive=true for subtree.
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Produce   json
// @Param     groupId    path   string  true   "Group UUID"
// @Param     recursive  query  bool    false  "Include subgroup members"
// @Success   200  {object}  map[string]interface{}
// @Failure   400  {object}  map[string]string
// @Router    /api/admin/groups/{groupId}/members [get].
func (s *State) ListGroupMembers(writer http.ResponseWriter, request *http.Request) {
	groupID := param(request, "groupId")

	if request.URL.Query().Get("recursive") == "true" {
		members, err := s.Repos.Groups.ListMembersRecursive(request.Context(), groupID)
		if err != nil {
			s.Error(writer, http.StatusInternalServerError, "Database error")

			return
		}

		s.JSON(writer, http.StatusOK, map[string]any{adminJSONKeyMembers: members, adminJSONKeyTotal: len(members)})

		return
	}

	members, err := s.Repos.Groups.ListMembers(request.Context(), groupID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{adminJSONKeyMembers: members, adminJSONKeyTotal: len(members)})
}

// AddGroupMember godoc
// @Summary   Add a user to a group (admin)
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     groupId  path    string  true  "Group UUID"
// @Param     body     body  object  true    "userId (UUID)"
// @Success   200      {object}  map[string]string
// @Failure   400      {object}  map[string]string
// @Router    /api/admin/groups/{groupId}/members [post].
func (s *State) AddGroupMember(writer http.ResponseWriter, request *http.Request) {
	groupID := param(request, "groupId")

	var body struct {
		UserID string `json:"userId"`
	}

	err := decode(request, &body)
	if err != nil || body.UserID == "" {
		s.Error(writer, http.StatusBadRequest, "userId is required")

		return
	}

	err = s.Repos.Groups.AddMember(request.Context(), groupID, body.UserID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: groupsMsgMemberAdded})
}

// RemoveGroupMember godoc
// @Summary   Remove a user from a group (admin)
// @Tags      Admin - Groups
// @Security  BearerAuth
// @Produce   json
// @Param     groupId  path  string  true  "Group UUID"
// @Param     userId   path  string  true  "User UUID"
// @Success   200      {object}  map[string]string
// @Failure   500      {object}  map[string]string
// @Router    /api/admin/groups/{groupId}/members/{userId} [delete].
func (s *State) RemoveGroupMember(writer http.ResponseWriter, request *http.Request) {
	groupID := param(request, "groupId")
	userID := param(request, "userId")

	err := s.Repos.Groups.RemoveMember(request.Context(), groupID, userID)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: groupsMsgMemberRemoved})
}
