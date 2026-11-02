package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
)

// pathCheckResponse is the JSON body returned by user-service
// /internal/paths/check.
type pathCheckResponse struct {
	Enrolled bool `json:"enrolled"`
}

// prerequisiteResponse describes a single course prerequisite in API
// responses.
type prerequisiteResponse struct {
	Course   string   `json:"course"`
	MinScore int      `json:"minScore,omitempty"`
	Modules  []string `json:"modules,omitempty"`
}

// CourseResponse is the public API representation of a course.
type CourseResponse struct {
	Slug            string                 `json:"slug"`
	ID              string                 `json:"id"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	Category        string                 `json:"category"`
	Difficulty      string                 `json:"difficulty"`
	IsPublic        bool                   `json:"isPublic"`
	ModuleCount     int                    `json:"moduleCount"`
	LabCount        int                    `json:"labCount"`
	EnrollmentCount int                    `json:"enrollmentCount"`
	Scope           string                 `json:"scope,omitempty"`
	Prerequisites   []prerequisiteResponse `json:"prerequisites,omitempty"`
	Skills          []string               `json:"skills,omitempty"`
	Badge           *content.Badge         `json:"badge,omitempty"`
	InPerson        bool                   `json:"inPerson,omitempty"`
	Sessions        []content.Session      `json:"sessions,omitempty"`
	XPRequired      int                    `json:"xpRequired,omitempty"`
}

// toCourseResponse converts an internal content.Course into the public
// CourseResponse shape returned by the API.
func toCourseResponse(course *content.Course) CourseResponse {
	prereqs := make([]prerequisiteResponse, 0, len(course.Prerequisites))

	for _, p := range course.Prerequisites {
		prereqs = append(prereqs, prerequisiteResponse{
			Course:   p.Course,
			MinScore: p.MinScore,
			Modules:  p.Modules,
		})
	}

	return CourseResponse{
		Slug:            course.Slug,
		ID:              course.Slug,
		Title:           course.Title,
		Description:     course.Description,
		Category:        course.Category,
		Difficulty:      course.Difficulty,
		IsPublic:        course.IsPublic,
		ModuleCount:     course.ModuleCount,
		LabCount:        course.ModuleCount,
		EnrollmentCount: 0,
		Scope:           course.Scope,
		Prerequisites:   prereqs,
		Skills:          course.Skills,
		Badge:           course.Badge,
		InPerson:        course.InPerson,
		Sessions:        course.Sessions,
		XPRequired:      course.XPRequired,
	}
}

// ListCourses godoc
// @Summary  List published courses
// @Tags     Courses
// @Produce  json
// @Param    category   query  string  false  "Filter by category"
// @Param    difficulty query  string  false  "Filter by difficulty"
// @Param    search     query  string  false  "Search by title or description"
// @Param    skill      query  string  false  "Filter by skill (slug)"
// @Success  200  {object}  map[string]interface{}
// @Router   /api/courses [get].
func (s *State) ListCourses(writer http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()

	// Filtering happens in the query rather than over an in-memory copy of
	// the catalog, so a narrow filter reads only the rows it matches.
	filter := repository.CourseFilter{
		PublicOnly: true,
		Category:   strings.TrimSpace(query.Get("category")),
		Difficulty: strings.TrimSpace(query.Get("difficulty")),
		Search:     strings.TrimSpace(query.Get("search")),
		Skill:      strings.TrimSpace(query.Get("skill")),
	}

	s.respondCourseList(writer, req, filter)
}

// ListAdminCourses godoc
// @Summary   List all courses including private (admin)
// @Tags      Admin
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/admin/courses [get].
func (s *State) ListAdminCourses(writer http.ResponseWriter, req *http.Request) {
	s.respondCourseList(writer, req, repository.CourseFilter{})
}

// respondCourseList runs a catalog query and writes its result.
func (s *State) respondCourseList(writer http.ResponseWriter, req *http.Request, filter repository.CourseFilter) {
	courses, err := s.Repos.Courses.List(req.Context(), filter)
	if err != nil {
		zap.L().Error("list courses failed", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return
	}

	out := make([]CourseResponse, 0, len(courses))
	for _, course := range courses {
		out = append(out, toCourseResponse(course))
	}

	s.JSON(writer, http.StatusOK, map[string]any{coursesJSONKey: out, totalJSONKey: len(out)})
}

// labResponse is the public API representation of a lab (a lab-type
// module) within a course.
type labResponse struct {
	ID          string `json:"id"`
	CourseID    string `json:"courseId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	LabType     string `json:"labType"`
	ModuleType  string `json:"moduleType"`
	Points      int    `json:"points"`
	OrderIndex  int    `json:"orderIndex"`
	IsPublished bool   `json:"isPublished"`
	Hidden      bool   `json:"hidden"`
}

// moduleTypeToLabType maps an internal module type to the legacy labType
// value expected by the frontend.
func moduleTypeToLabType(moduleType string) string {
	switch moduleType {
	case moduleTypeText:
		return labTypeForm
	case moduleTypeVideo, moduleTypeImage:
		return labTypeInteractive
	default:
		return labTypeInteractive
	}
}

// GetLab godoc
// @Summary   Get a single lab
// @Tags      Labs
// @Security  BearerAuth
// @Produce   json
// @Param     slug    path  string  true  "Course slug"
// @Param     lab_id  path  string  true  "Lab ID"
// @Success   200  {object}  map[string]interface{}
// @Failure   404  {object}  map[string]string
// @Router    /api/courses/{slug}/labs/{lab_id} [get].
func (s *State) GetLab(writer http.ResponseWriter, req *http.Request) {
	courseSlug := param(req, "slug")
	labID := param(req, "lab_id")

	course, ok := s.course(writer, req, courseSlug)
	if !ok {
		return
	}

	for _, mod := range s.visibleModules(course, req) {
		if mod.Slug() == labID {
			s.JSON(writer, http.StatusOK, map[string]any{
				moduleTypeLab: labResponse{
					ID:          mod.Slug(),
					CourseID:    course.Slug,
					Title:       mod.Name,
					Description: "",
					LabType:     moduleTypeToLabType(mod.Type),
					ModuleType:  mod.Type,
					Points:      0,
					OrderIndex:  0,
					IsPublished: true,
					Hidden:      mod.Hidden,
				},
				progressJSONKey: nil,
			})

			return
		}
	}

	s.Error(writer, http.StatusNotFound, "Lab not found")
}

// ListLabs godoc
// @Summary   List labs for a course
// @Tags      Labs
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Success   200   {object}  map[string]interface{}
// @Failure   404   {object}  map[string]string
// @Router    /api/courses/{slug}/labs [get].
func (s *State) ListLabs(writer http.ResponseWriter, req *http.Request) {
	courseSlug := param(req, "slug")

	course, ok := s.course(writer, req, courseSlug)
	if !ok {
		return
	}

	modules := s.visibleModules(course, req)

	labs := make([]labResponse, 0, len(modules))
	for pos, mod := range modules {
		labs = append(labs, labResponse{
			ID:          mod.Slug(),
			CourseID:    course.Slug,
			Title:       mod.Name,
			Description: "",
			LabType:     moduleTypeToLabType(mod.Type),
			ModuleType:  mod.Type,
			Points:      0,
			OrderIndex:  pos + 1,
			IsPublished: true,
			Hidden:      mod.Hidden,
		})
	}

	s.JSON(writer, http.StatusOK, map[string]any{"labs": labs})
}

// GetCourseProgress godoc
// @Summary   Get course progress summary
// @Tags      Courses
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Success   200   {object}  map[string]interface{}
// @Router    /api/courses/{slug}/progress [get].
func (s *State) GetCourseProgress(writer http.ResponseWriter, req *http.Request) {
	courseSlug := param(req, "slug")

	// An unknown course reports an empty summary rather than 404, which is
	// what the frontend expects; a storage failure is still an error.
	total := 0

	course, err := s.Repos.Courses.Get(req.Context(), courseSlug)

	switch {
	case err == nil:
		total = len(s.visibleModules(course, req))
	case errors.Is(err, repository.ErrNotFound):
	default:
		zap.L().Error("load course failed", zap.String("slug", courseSlug), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{
		"courseId":              courseSlug,
		userIDJSONKey:           "",
		"total_labs":            total,
		"completedLabs":         0,
		"total_points_possible": 0,
		"total_points_earned":   0,
		"completion_percentage": 0,
		"lab_progress":          []any{},
	})
}

// isEnrolledViaPath returns true if userID is enrolled in any learning path
// that contains courseSlug.
func (s *State) isEnrolledViaPath(ctx context.Context, courseSlug, userID string) bool {
	pathSlugs, err := s.Repos.Paths.SlugsContainingCourse(ctx, courseSlug)
	if err != nil {
		zap.L().Error("list paths containing course failed",
			zap.String("courseSlug", courseSlug), zap.Error(err))

		return false
	}

	return s.pathEnrollmentCheck(ctx, userID, pathSlugs)
}

// canViewPrivateCourse reports whether the requester is authorized to view
// a course that is not public: an admin, a user enrolled in the course, or
// a user enrolled in any path that contains the course.
func (s *State) canViewPrivateCourse(req *http.Request, slug string) bool {
	claims := s.claims(req)
	if claims == nil {
		auth := req.Header.Get("Authorization")

		rest, hasBearer := strings.CutPrefix(auth, "Bearer ")
		if hasBearer {
			parsed, err := middleware.VerifyToken(rest, s.Config.JWTSecret)
			if err == nil {
				claims = parsed
			}
		}
	}

	if claims == nil {
		return false
	}

	if claims.Role == roleAdmin {
		return true
	}

	ctx := req.Context()

	return s.enrollmentCheck(ctx, slug, claims.Subject) || s.isEnrolledViaPath(ctx, slug, claims.Subject)
}

// GetCourse godoc
// @Summary  Get a course by slug
// @Tags     Courses
// @Produce  json
// @Param    slug  path  string  true  "Course slug"
// @Success  200   {object}  CourseResponse
// @Failure  404   {object}  map[string]string
// @Router   /api/courses/{slug} [get].
func (s *State) GetCourse(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")

	course, ok := s.course(writer, req, slug)
	if !ok {
		return
	}

	if !course.IsPublic && !s.canViewPrivateCourse(req, slug) {
		s.Error(writer, http.StatusNotFound, courseNotFoundMessage)

		return
	}

	s.JSON(writer, http.StatusOK, toCourseResponse(course))
}

// PublicCourseTitles returns the title of every public course, by slug, as
// the anonymous GET /api/courses lists them. The users domain labels its
// CSV exports with it.
func (s *State) PublicCourseTitles(ctx context.Context) (map[string]string, error) {
	courses, err := s.Repos.Courses.List(ctx, repository.CourseFilter{PublicOnly: true})
	if err != nil {
		return nil, fmt.Errorf("list public courses: %w", err)
	}

	titles := make(map[string]string, len(courses))
	for _, course := range courses {
		titles[course.Slug] = course.Title
	}

	return titles, nil
}

// PathTitles returns the title of every path, by slug, as GET /api/paths
// lists them.
func (s *State) PathTitles(ctx context.Context) (map[string]string, error) {
	paths, err := s.Repos.Paths.List(ctx, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("list paths: %w", err)
	}

	titles := make(map[string]string, len(paths))
	for _, path := range paths {
		titles[path.Slug] = path.Title
	}

	return titles, nil
}

// CourseDetail returns a course as GET /api/courses/{slug} shows it,
// private or not: repository.ErrNotFound for an unknown course. The users
// domain reads session capacities through it. It used to refuse a private
// course, like an anonymous read, so the capacity of a private course's
// sessions was never enforced (docs/specs/190-backend-defects.md B24).
func (s *State) CourseDetail(ctx context.Context, slug string) (CourseResponse, error) {
	course, err := s.Repos.Courses.Get(ctx, slug)
	if err != nil {
		return CourseResponse{}, fmt.Errorf("load course %s: %w", slug, err)
	}

	return toCourseResponse(course), nil
}
