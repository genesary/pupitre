package handlers

import (
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// pathKindSkill identifies a learning path that groups modules by skill.
const pathKindSkill = "skill"

// ListPaths returns learning paths known to the service, optionally
// paginated via the limit/offset query params. A zero, negative, or
// omitted limit means unlimited — callers can pass e.g. limit=-1 to
// explicitly request everything.
//
// Pagination is applied by the query, so asking for one page does not read
// every path out of the database.
// @Summary  List all learning paths
// @Tags     Paths
// @Produce  json
// @Param    limit  query  int  false  "Max paths to return (0 means unlimited)"
// @Param    offset  query  int  false  "Number of paths to skip"
// @Success  200  {object}  map[string]interface{}
// @Router   /api/paths [get].
func (s *State) ListPaths(writer http.ResponseWriter, req *http.Request) {
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(req.URL.Query().Get("offset"))

	paths, err := s.Repos.Paths.List(req.Context(), limit, offset)
	if err != nil {
		zap.L().Error("list paths failed", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return
	}

	s.JSON(writer, http.StatusOK, map[string][]*content.Path{pathsJSONKey: paths})
}

// GetPath returns a single learning path by its slug.
// @Summary  Get a learning path by slug
// @Tags     Paths
// @Produce  json
// @Param    slug  path  string  true  "Path slug"
// @Success  200   {object}  content.Path
// @Failure  404   {object}  map[string]string
// @Router   /api/paths/{slug} [get].
func (s *State) GetPath(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")

	path, err := s.Repos.Paths.Get(req.Context(), slug)
	if err != nil {
		s.writeRepoError(writer, err, pathNotFoundMessage, "load path", zap.String("slug", slug))

		return
	}

	// For course-kind paths, aggregate the skills of the member courses.
	// One query over the denormalized course skills replaces walking the
	// whole in-memory catalog.
	if path.Kind != pathKindSkill && len(path.Courses) > 0 {
		skills, skillErr := s.Repos.Paths.SkillsOfCourses(req.Context(), path.Courses)
		if skillErr != nil {
			zap.L().Error("aggregate path skills failed", zap.String("slug", slug), zap.Error(skillErr))
		} else if len(skills) > 0 {
			path.Skills = skills
		}
	}

	s.JSON(writer, http.StatusOK, path)
}
