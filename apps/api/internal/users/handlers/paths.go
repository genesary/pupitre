package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// Course-status literals shared by buildCourseStatuses and its callers,
// pulled out to satisfy goconst.
const (
	pathStatusLocked    = "locked"
	pathStatusCompleted = "completed"
	pathStatusAvailable = "available"
	// pathKindSkill identifies a skill-kind learning path.
	pathKindSkill = "skill"
)

// parsePagination reads optional limit/offset query params. A zero,
// negative, or omitted limit maps to a nil *int, passed through to SQL
// as LIMIT NULL (unbounded); pass limit=-1 to request everything.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func parsePagination(request *http.Request) (*int, int) {
	var limit *int

	limitVal, err := strconv.Atoi(request.URL.Query().Get("limit"))
	if err == nil && limitVal > 0 {
		limit = &limitVal
	}

	offset := 0

	offsetVal, err := strconv.Atoi(request.URL.Query().Get("offset"))
	if err == nil && offsetVal > 0 {
		offset = offsetVal
	}

	return limit, offset
}

// courseStatus is a single course's completion state within a path.
type courseStatus struct {
	Slug   string `json:"slug"`
	Status string `json:"status"` // "completed", "available", "locked"
}

// myPath is a learning path the current user is enrolled in, with
// per-course completion status.
type myPath struct {
	Slug        string         `json:"slug"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Kind        string         `json:"kind,omitempty"`
	EnrolledAt  time.Time      `json:"enrolledAt"`
	Courses     []courseStatus `json:"courses,omitempty"`
	Skills      []string       `json:"skills,omitempty"`
}

// enrolledUser represents a user enrolled in a learning path,
// along with their progress across the path's courses.
type enrolledUser struct {
	// UserID is the UUID of the enrolled user.
	UserID string `json:"userId"`
	// Email is the user's email address.
	Email string `json:"email"`
	// Role is the user's platform role (e.g. "student", "admin").
	Role string `json:"role"`
	// EnrolledAt is when the user was enrolled in this path.
	EnrolledAt time.Time `json:"enrolledAt"`
	// CompletedCourses is the count of courses this user finished in the path.
	CompletedCourses int `json:"completedCourses"`
	// TotalCourses is the total number of courses in the path.
	TotalCourses int `json:"totalCourses"`
	// Courses holds per-course completion, populated when detail is available.
	Courses []courseStatus `json:"courses,omitempty"`
}

// buildCourseStatuses derives each course's completion status in an
// ordered path, given the set of courses the user has completed.
//
// Rules:
//   - "completed" if the user finished the course (cross-path counts).
//   - "available" if this is the first course or the immediately preceding
//     course is "completed".
//   - "locked" otherwise.
func buildCourseStatuses(courses []string, completed map[string]struct{}) []courseStatus {
	out := make([]courseStatus, len(courses))

	for idx, slug := range courses {
		_, done := completed[slug]

		switch {
		case done:
			out[idx] = courseStatus{Slug: slug, Status: pathStatusCompleted}
		case idx == 0 || out[idx-1].Status == pathStatusCompleted:
			out[idx] = courseStatus{Slug: slug, Status: pathStatusAvailable}
		default:
			out[idx] = courseStatus{Slug: slug, Status: pathStatusLocked}
		}
	}

	return out
}

// slugRE matches the allowed shape of a course/path slug.
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// fetchPathDetail fetches a learning path's metadata from course-service.
func (s *State) fetchPathDetail(request *http.Request, slug string) (*PathInfo, error) {
	if !slugRE.MatchString(slug) {
		return nil, fmt.Errorf("invalid path slug: %q", slug)
	}

	detail, found := s.catalog().Path(request.Context(), slug)
	if !found {
		return nil, fmt.Errorf("fetch path detail: %q not found", slug)
	}

	return &detail, nil
}

// MyPaths returns learning paths the authenticated user is enrolled in, with
// per-course completion status derived from quiz and lesson progress.
// Supports optional limit/offset pagination; without them, all paths are
// returned.
// @Summary   List learning paths the current user is enrolled in
// @Tags      Paths
// @Security  BearerAuth
// @Produce   json
// @Param     limit   query  int  false  "Max number of paths to return"
// @Param     offset  query  int  false  "Number of paths to skip"
// @Success   200  {object}  map[string]interface{}
// @Router    /api/my/paths [get].
func (s *State) MyPaths(writer http.ResponseWriter, request *http.Request) {
	claims := s.claims(request)
	limit, offset := parsePagination(request)

	rows, err := s.Repos.Paths.MyEnrollments(request.Context(), claims.Subject, limit, offset)
	if err != nil {
		zap.L().Error("failed to query path enrollments", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string][]myPath{"paths": s.resolveEnrollments(request, claims.Subject, rows)})
}

// resolveEnrollments turns a learner's path enrollments into their rendered
// form, with per-course (or per-skill) completion statuses filled in.
//
// Every lookup it needs is done once for the whole page rather than once
// per path: one batched path fetch, one batched skill-modules fetch, one
// completed-courses query over the union of every path's courses, and — for
// skill paths — a single read each of the learner's passed modules and
// viewed lessons. Resolving each enrollment on its own turned a dashboard
// with ten paths into dozens of serial round-trips.
func (s *State) resolveEnrollments(
	req *http.Request, userID string, rows []repository.PathEnrollmentRow,
) []myPath {
	ctx := req.Context()

	pathSlugs := make([]string, 0, len(rows))
	for _, row := range rows {
		pathSlugs = append(pathSlugs, row.Slug)
	}

	details := s.catalog().Paths(ctx, pathSlugs)

	courseSlugs, skillSlugs := pathMemberSlugs(rows, details)

	completed := s.completedCoursesCtx(req, userID, courseSlugs)
	skillStatus := s.skillCompletion(req, userID, skillSlugs)

	result := make([]myPath, 0, len(rows))

	for _, row := range rows {
		detail, found := details[row.Slug]
		if !found {
			zap.L().Warn("failed to fetch path detail", zap.String("slug", row.Slug))

			result = append(result, myPath{
				Slug:       row.Slug,
				Title:      row.Slug,
				EnrolledAt: row.EnrolledAt,
				Courses:    []courseStatus{},
			})

			continue
		}

		members := buildCourseStatuses(detail.Courses, completed)
		if detail.Kind == pathKindSkill {
			members = orderedSkillStatuses(detail.Skills, skillStatus)
		}

		result = append(result, myPath{
			Slug:        detail.Slug,
			Title:       detail.Title,
			Description: detail.Description,
			Kind:        detail.Kind,
			EnrolledAt:  row.EnrolledAt,
			Courses:     members,
			Skills:      detail.Skills,
		})
	}

	return result
}

// pathMemberSlugs collects the deduplicated union of the courses and the
// skills named by every resolved path, so each set can be resolved once.
//
//nolint:gocritic // unnamedResult conflicts with the project's nonamedreturns policy
func pathMemberSlugs(rows []repository.PathEnrollmentRow, details map[string]PathInfo) ([]string, []string) {
	seenCourses := make(map[string]struct{})
	seenSkills := make(map[string]struct{})

	var courses, skills []string

	for _, row := range rows {
		detail, found := details[row.Slug]
		if !found {
			continue
		}

		for _, slug := range detail.Courses {
			if _, dup := seenCourses[slug]; !dup {
				seenCourses[slug] = struct{}{}
				courses = append(courses, slug)
			}
		}

		if detail.Kind != pathKindSkill {
			continue
		}

		for _, skill := range detail.Skills {
			if _, dup := seenSkills[skill]; !dup {
				seenSkills[skill] = struct{}{}
				skills = append(skills, skill)
			}
		}
	}

	return courses, skills
}

// skillState is what one skill of a skill-kind path contributes to the
// listing: whether the learner has finished it, and whether its module list
// could be resolved at all.
type skillState struct {
	completed bool
	// unavailable marks a skill whose modules course-service did not
	// return. It is reported locked rather than available, so an outage
	// cannot silently unlock the rest of a path.
	unavailable bool
}

// skillCompletion reports, for each of skills, whether the learner has
// completed every assessable module teaching it.
//
// The learner's passed modules and viewed lessons are read once, and every
// skill's module list comes back in one batched call.
func (s *State) skillCompletion(req *http.Request, userID string, skills []string) map[string]skillState {
	states := make(map[string]skillState, len(skills))
	if len(skills) == 0 {
		return states
	}

	passed := s.passedModulesCtx(req, userID)
	viewed := s.viewedLessonsCtx(req, userID)
	modulesBySkill := s.catalog().SkillModules(req.Context(), skills)

	for _, skill := range skills {
		modules, resolved := modulesBySkill[skill]
		if !resolved {
			zap.L().Warn("failed to fetch skill modules for path", zap.String("skill", skill))

			states[skill] = skillState{unavailable: true}

			continue
		}

		states[skill] = skillState{completed: skillIsCompleted(modules, passed, viewed)}
	}

	return states
}

// orderedSkillStatuses applies the sequential unlock rule to a skill path:
// a completed skill is "completed", the one after the last completed skill
// is "available", and the rest — along with any skill whose modules could
// not be resolved — are "locked".
func orderedSkillStatuses(skills []string, states map[string]skillState) []courseStatus {
	out := make([]courseStatus, 0, len(skills))
	prevCompleted := true // the first skill has no prerequisite

	for _, skill := range skills {
		state := states[skill]

		var status string

		switch {
		case state.completed:
			status = pathStatusCompleted
		case state.unavailable, !prevCompleted:
			status = pathStatusLocked
		default:
			status = pathStatusAvailable
		}

		out = append(out, courseStatus{Slug: skill, Status: status})
		prevCompleted = state.completed
	}

	return out
}

// completedCoursesCtx returns the set of course slugs (from slugs) that
// userID has completed. Completion is not path-scoped: a course finished
// in any path counts.
//
// The __complete__ sentinel in lesson_progress is the only source: it is
// written by course-service once every module of the course is done. Passed
// quiz modules used to count as well, which reported a course as finished
// on its first passed quiz — with the rest of its lessons and quizzes still
// untouched.
func (s *State) completedCoursesCtx(request *http.Request, userID string, slugs []string) map[string]struct{} {
	if len(slugs) == 0 {
		return nil
	}

	completed, err := s.Repos.LessonProgress.CompletedCourseSlugs(request.Context(), userID, slugs)
	if err != nil {
		zap.L().Error("failed to query completed courses", zap.String("userID", userID), zap.Error(err))

		return nil
	}

	result := make(map[string]struct{}, len(completed))
	for _, slug := range completed {
		result[slug] = struct{}{}
	}

	return result
}

// AdminListPathEnrollments lists all users enrolled in a learning path,
// including each user's per-course completion status when the path
// detail is reachable.
// @Summary   List users enrolled in a path (admin)
// @Tags      Admin - Paths
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Path slug"
// @Success   200   {object}  map[string]interface{}
// @Router    /api/admin/paths/{slug}/enrollments [get].
func (s *State) AdminListPathEnrollments(writer http.ResponseWriter, request *http.Request) {
	slug := param(request, "slug")

	detail, err := s.fetchPathDetail(request, slug)
	if err != nil {
		zap.L().Warn("failed to fetch path detail for enrollments", zap.String("slug", slug), zap.Error(err))
	}

	rows, err := s.Repos.Paths.ListBySlug(request.Context(), slug)
	if err != nil {
		zap.L().Error("failed to query path enrollments", zap.String("slug", slug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	users := s.buildEnrolledUsers(request.Context(), rows, detail)

	s.JSON(writer, http.StatusOK, map[string][]enrolledUser{adminJSONKeyUsers: users})
}

// buildEnrolledUsers renders every enrolled user's per-course completion
// within a path.
//
// Completion for the whole cohort is resolved in one query rather than one
// per user: listing a path with a thousand learners used to run a thousand
// queries to fill in the same handful of course slugs.
func (s *State) buildEnrolledUsers(
	ctx context.Context, rows []repository.PathEnrolledUserRow, detail *PathInfo,
) []enrolledUser {
	users := make([]enrolledUser, 0, len(rows))

	var completedByUser map[string]map[string]struct{}

	if detail != nil && len(detail.Courses) > 0 {
		userIDs := make([]string, 0, len(rows))
		for _, row := range rows {
			userIDs = append(userIDs, row.UserID)
		}

		completedByUser = s.completedCoursesByUser(ctx, userIDs, detail.Courses)
	}

	for _, row := range rows {
		member := enrolledUser{UserID: row.UserID, Email: row.Email, Role: row.Role, EnrolledAt: row.EnrolledAt}

		if detail != nil {
			member.TotalCourses = len(detail.Courses)
			member.Courses = buildCourseStatuses(detail.Courses, completedByUser[row.UserID])

			for _, status := range member.Courses {
				if status.Status == pathStatusCompleted {
					member.CompletedCourses++
				}
			}
		}

		users = append(users, member)
	}

	return users
}

// completedCoursesByUser returns, per user, the subset of slugs that user
// has completed.
func (s *State) completedCoursesByUser(
	ctx context.Context, userIDs, slugs []string,
) map[string]map[string]struct{} {
	byUser, err := s.Repos.LessonProgress.CompletedCourseSlugsByUsers(ctx, userIDs, slugs)
	if err != nil {
		zap.L().Error("failed to query completed courses for cohort",
			zap.Int("users", len(userIDs)), zap.Error(err))

		return nil
	}

	out := make(map[string]map[string]struct{}, len(byUser))

	for userID, completed := range byUser {
		set := make(map[string]struct{}, len(completed))
		for _, slug := range completed {
			set[slug] = struct{}{}
		}

		out[userID] = set
	}

	return out
}

// ManagerListPathEnrollments lists users enrolled in a learning path, filtered
// to those within the manager's group scope.
// @Summary   List users enrolled in a path (manager, scope-restricted)
// @Tags      Manager
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Path slug"
// @Success   200   {object}  map[string]interface{}
// @Router    /api/manager/paths/{slug}/enrollments [get].
func (s *State) ManagerListPathEnrollments(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	slug := param(request, "slug")
	claims := s.claims(request)

	groups, err := s.Repos.Groups.GetGroupsByUserID(ctx, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	groupIDs := managerScopeIDs(groups)

	detail, err := s.fetchPathDetail(request, slug)
	if err != nil {
		zap.L().Warn("failed to fetch path detail for manager enrollment list", zap.String("slug", slug), zap.Error(err))
	}

	rows, err := s.Repos.Paths.ListBySlug(ctx, slug)
	if err != nil {
		zap.L().Error("failed to query path enrollments", zap.String("slug", slug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	// One membership query for the whole cohort, not one per enrolled user.
	inScope, err := s.Repos.Groups.UsersInAnyGroup(ctx, pathEnrollmentUserIDs(rows), groupIDs)
	if err != nil {
		zap.L().Error("failed to resolve manager scope", zap.String("slug", slug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	scoped := make([]repository.PathEnrolledUserRow, 0, len(rows))

	for _, row := range rows {
		if inScope[row.UserID] {
			scoped = append(scoped, row)
		}
	}

	s.JSON(writer, http.StatusOK,
		map[string][]enrolledUser{adminJSONKeyUsers: s.buildEnrolledUsers(ctx, scoped, detail)})
}

// pathEnrollmentUserIDs extracts the user IDs of a path-enrollment listing.
func pathEnrollmentUserIDs(rows []repository.PathEnrolledUserRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}

	return ids
}

// AdminEnrollUserInPath enrolls a user in a learning path. Idempotent:
// enrolling an already-enrolled user is a no-op.
// The path enrollment and all course auto-enrollments are committed in a single
// transaction so a partial failure leaves no dangling state.
// @Summary   Enroll a user in a learning path (admin)
// @Tags      Admin - Paths
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     slug  path  string                   true  "Path slug"
// @Param     body  body  map[string]string  true  "userId"
// @Success   200   {object}  map[string]string
// @Failure   400   {object}  map[string]string
// @Router    /api/admin/paths/{slug}/enrollments [post].
func (s *State) AdminEnrollUserInPath(writer http.ResponseWriter, request *http.Request) {
	slug := param(request, "slug")

	var body struct {
		UserID string `json:"userId"`
	}

	err := decode(request, &body)
	if err != nil || body.UserID == "" {
		s.Error(writer, http.StatusBadRequest, "userId required")

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

	err = s.Repos.Paths.EnrollWithCourses(request.Context(), body.UserID, slug, courseSlugs)
	if err != nil {
		zap.L().Error("failed to enroll user in path", zap.String("slug", slug), zap.String("userID", body.UserID), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "Enrolled in path"})
}

// AdminUnenrollUserFromPath removes a user from a learning path.
// @Summary   Remove a user from a learning path (admin)
// @Tags      Admin - Paths
// @Security  BearerAuth
// @Produce   json
// @Param     slug     path  string  true  "Path slug"
// @Param     userId  path  string  true  "User UUID"
// @Success   200      {object}  map[string]string
// @Router    /api/admin/paths/{slug}/enrollments/{userId} [delete].
func (s *State) AdminUnenrollUserFromPath(writer http.ResponseWriter, request *http.Request) {
	slug := param(request, "slug")
	userID := param(request, "userId")

	err := s.Repos.Paths.Unenroll(request.Context(), userID, slug)
	if err != nil {
		zap.L().Error("failed to unenroll user from path", zap.String("slug", slug), zap.String("userID", userID), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "Unenrolled from path"})
}
