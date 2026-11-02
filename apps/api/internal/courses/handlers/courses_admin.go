package handlers

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/definition"
)

// courseKind names the course resource in error messages, log lines and
// the filename an export falls back to.
const courseKind = "course"

// courseMessages is the client-facing wording for course definitions.
//
//nolint:gochecknoglobals // static response wording, read-only
var courseMessages = definitionMessages{
	Kind:         courseKind,
	InvalidSpec:  "Invalid course spec",
	Conflict:     "Course already exists",
	NotFound:     courseNotFoundMessage,
	CreateFailed: "Failed to create course",
	UpdateFailed: "Failed to update course",
}

// CreateCourse godoc
// @Summary  Create a course (admin)
// @Tags     Admin - Courses
// @Security BearerAuth
// @Router   /api/admin/courses [post].
func (s *State) CreateCourse(writer http.ResponseWriter, req *http.Request) {
	createDefinition(s, writer, req, courseMessages, definition.Course.ToCourse, s.Repos.Courses.Create)
}

// GetCourseDefinition godoc
// @Summary  Get a course's full definition (admin)
// @Tags     Admin - Courses
// @Security BearerAuth
// @Router   /api/admin/courses/{slug}/definition [get].
func (s *State) GetCourseDefinition(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")

	course, err := s.Repos.Courses.Get(req.Context(), slug)
	if err != nil {
		s.writeRepoError(writer, err, courseNotFoundMessage, "load course", zap.String("slug", slug))

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{
		slugJSONKey: slug,
		specJSONKey: definition.FromCourse(course),
	})
}

// UpdateCourse godoc
// @Summary  Replace a course definition (admin)
// @Tags     Admin - Courses
// @Security BearerAuth
// @Router   /api/admin/courses/{slug}/definition [put].
func (s *State) UpdateCourse(writer http.ResponseWriter, req *http.Request) {
	updateDefinition(s, writer, req, courseMessages, definition.Course.ToCourse, s.courseExists, s.Repos.Courses.Upsert)
}

// DeleteCourse godoc
// @Summary  Delete a course (admin)
// @Tags     Admin - Courses
// @Security BearerAuth
// @Router   /api/admin/courses/{slug}/definition [delete].
func (s *State) DeleteCourse(writer http.ResponseWriter, req *http.Request) {
	deleteDefinition(s, writer, req, courseMessages, "Course deleted", s.Repos.Courses.Delete)
}

// courseExists reports whether slug names a stored course, returning
// repository.ErrNotFound when it does not.
func (s *State) courseExists(ctx context.Context, slug string) error {
	_, err := s.Repos.Courses.Get(ctx, slug)
	if err != nil {
		return fmt.Errorf("look up course %s: %w", slug, err)
	}

	return nil
}
