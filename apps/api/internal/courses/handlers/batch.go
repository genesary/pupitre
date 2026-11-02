package handlers

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// maxBatchSlugs caps how many slugs one batch lookup may name. It bounds
// both the SQL IN list and the response size; callers with more than this
// many slugs page through them.
const maxBatchSlugs = 500

// slugsParam parses the repeated/comma-separated `slugs` query parameter
// into its deduplicated entries, preserving first-seen order.
func slugsParam(req *http.Request) []string {
	seen := make(map[string]struct{})

	var out []string

	for _, raw := range req.URL.Query()["slugs"] {
		for entry := range strings.SplitSeq(raw, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}

			if _, dup := seen[entry]; dup {
				continue
			}

			seen[entry] = struct{}{}

			out = append(out, entry)
		}
	}

	return out
}

// ListCoursesBatch godoc
// @Summary  Resolve several courses by slug in one request
// @Tags     Courses
// @Produce  json
// @Param    slugs  query  string  true  "Comma-separated course slugs"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  map[string]string
// @Router   /api/batch/courses [get].
//
// It exists so a caller holding a list of course slugs — a learner's
// enrollments, the courses of a learning path, the badges they have earned
// — resolves them all in one query instead of one HTTP request per slug.
func (s *State) ListCoursesBatch(writer http.ResponseWriter, req *http.Request) {
	slugs := slugsParam(req)

	if len(slugs) > maxBatchSlugs {
		s.Error(writer, http.StatusBadRequest, "too many slugs requested")

		return
	}

	if len(slugs) == 0 {
		s.JSON(writer, http.StatusOK, map[string]any{coursesJSONKey: []CourseResponse{}, totalJSONKey: 0})

		return
	}

	// The public route lists public courses only, like GET /api/courses
	// (docs/specs/190-backend-defects.md B10); the in-process BatchCourses
	// of the users domain still sees every course.
	out, err := s.batchCourses(req.Context(), slugs, true)
	if err != nil {
		zap.L().Error("batch course lookup failed", zap.Int("slugs", len(slugs)), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{coursesJSONKey: out, totalJSONKey: len(out)})
}

// BatchCourses returns the courses among slugs, private ones included, as
// GET /api/batch/courses reports them. The users domain reads its catalog
// through it (docs/specs/104-backend-in-process-calls.md).
func (s *State) BatchCourses(ctx context.Context, slugs []string) ([]CourseResponse, error) {
	return s.batchCourses(ctx, slugs, false)
}

// batchCourses resolves the courses among slugs, public ones only when
// publicOnly is set.
func (s *State) batchCourses(ctx context.Context, slugs []string, publicOnly bool) ([]CourseResponse, error) {
	courses, err := s.Repos.Courses.List(ctx, repository.CourseFilter{Slugs: slugs, PublicOnly: publicOnly})
	if err != nil {
		return nil, fmt.Errorf("list courses: %w", err)
	}

	out := make([]CourseResponse, 0, len(courses))
	for _, course := range courses {
		out = append(out, toCourseResponse(course))
	}

	return out, nil
}

// ListPathsBatch godoc
// @Summary  Resolve several learning paths by slug in one request
// @Tags     Paths
// @Produce  json
// @Param    slugs  query  string  true  "Comma-separated path slugs"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  map[string]string
// @Router   /api/batch/paths [get].
func (s *State) ListPathsBatch(writer http.ResponseWriter, req *http.Request) {
	slugs := slugsParam(req)

	if len(slugs) > maxBatchSlugs {
		s.Error(writer, http.StatusBadRequest, "too many slugs requested")

		return
	}

	if len(slugs) == 0 {
		s.JSON(writer, http.StatusOK, map[string][]*content.Path{pathsJSONKey: {}})

		return
	}

	paths, err := s.BatchPaths(req.Context(), slugs)
	if err != nil {
		zap.L().Error("batch path lookup failed", zap.Int("slugs", len(slugs)), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return
	}

	s.JSON(writer, http.StatusOK, map[string][]*content.Path{pathsJSONKey: paths})
}

// BatchPaths returns the paths among slugs, with the skills of course-kind
// paths aggregated, as GET /api/batch/paths reports them.
func (s *State) BatchPaths(ctx context.Context, slugs []string) ([]*content.Path, error) {
	paths, err := s.Repos.Paths.ListBySlugs(ctx, slugs)
	if err != nil {
		return nil, fmt.Errorf("list paths: %w", err)
	}

	s.aggregateCourseKindSkills(ctx, paths)

	if paths == nil {
		paths = []*content.Path{}
	}

	return paths, nil
}

// aggregateCourseKindSkills fills in the skills of every course-kind path
// in paths, matching what GET /api/paths/{slug} reports for one.
//
// The member courses of every path are unioned first, so the whole page
// costs one skills query rather than one per path.
func (s *State) aggregateCourseKindSkills(ctx context.Context, paths []*content.Path) {
	courseSlugs := courseKindMemberSlugs(paths)
	if len(courseSlugs) == 0 {
		return
	}

	skillsByCourse, err := s.Repos.Paths.SkillsByCourse(ctx, courseSlugs)
	if err != nil {
		zap.L().Error("aggregate batch path skills failed", zap.Error(err))

		return
	}

	for _, path := range paths {
		if path.Kind == pathKindSkill || len(path.Courses) == 0 {
			continue
		}

		if skills := unionSkills(path.Courses, skillsByCourse); len(skills) > 0 {
			path.Skills = skills
		}
	}
}

// courseKindMemberSlugs returns the deduplicated union of the courses named
// by every course-kind path in paths.
func courseKindMemberSlugs(paths []*content.Path) []string {
	seen := make(map[string]struct{})

	var courseSlugs []string

	for _, path := range paths {
		if path.Kind == pathKindSkill {
			continue
		}

		for _, slug := range path.Courses {
			if _, dup := seen[slug]; dup {
				continue
			}

			seen[slug] = struct{}{}

			courseSlugs = append(courseSlugs, slug)
		}
	}

	return courseSlugs
}

// unionSkills returns the deduplicated, sorted union of the skills taught
// by the given courses.
func unionSkills(courseSlugs []string, skillsByCourse map[string][]string) []string {
	seen := make(map[string]struct{})

	var out []string

	for _, courseSlug := range courseSlugs {
		for _, skill := range skillsByCourse[courseSlug] {
			if _, dup := seen[skill]; dup {
				continue
			}

			seen[skill] = struct{}{}

			out = append(out, skill)
		}
	}

	slices.Sort(out)

	return out
}
