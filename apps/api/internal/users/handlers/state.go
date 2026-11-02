package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"

	"github.com/go-chi/chi/v5"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// maxRequestBodyBytes caps the size of accepted request bodies (1 MB).
const maxRequestBodyBytes = 1 << 20

// State holds the shared dependencies used by every HTTP handler in
// this package: the GORM-backed repositories, application config, and the
// batched read-through view of the course catalog.
type State struct {
	Repos  *repository.Repositories
	Config *config.Config

	// Catalog resolves course, path and skill metadata from the courses
	// domain. Nil until catalog is first called, which builds it from
	// Courses — so a State assembled as a bare struct literal (as the
	// tests do) still gets one.
	Catalog *Catalog

	// Courses is the courses domain this one reads from, wired in by the
	// monolith (docs/specs/104-backend-in-process-calls.md).
	Courses CourseSource

	// Sessions, when set, is checked by every guard on every request
	// (docs/specs/190-backend-defects.md B7). The monolith sets it to
	// CheckSession.
	Sessions middleware.SessionCheck

	catalogOnce sync.Once
}

// Health godoc
// @Summary  Liveness check
// @Tags     Health
// @Produce  json
// @Success  200  {object}  map[string]string
// @Router   /health [get].
func (s *State) Health(w http.ResponseWriter, r *http.Request) {
	s.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "user-service",
		"version": "1.0.0",
	})
}

// JSON writes v to w as a JSON response with the given HTTP status.
func (s *State) JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		zap.L().Error("failed to encode JSON response", zap.Error(err))
	}
}

// Error writes a JSON error response with the given HTTP status and msg.
func (s *State) Error(w http.ResponseWriter, status int, msg string) {
	s.JSON(w, status, map[string]string{"error": msg})
}

// claims returns the authenticated claims stored on the request context.
func (s *State) claims(r *http.Request) *middleware.Claims {
	return middleware.GetClaims(r)
}

// param returns the chi URL parameter identified by key.
func param(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// decode reads and decodes the JSON request body into v.
func decode(r *http.Request, v any) error {
	err := json.NewDecoder(r.Body).Decode(v)
	if err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}

	return nil
}

// nullStr returns nil for an empty string, otherwise a pointer to s.
func nullStr(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// derefStr returns "" for a nil pointer, otherwise the pointed-to value.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

// courseSource returns the courses domain wired in by the monolith; a State
// built without one cannot read the catalog.
func (s *State) courseSource() CourseSource { //nolint:ireturn // the wired courses domain, or none
	if s.Courses != nil {
		return s.Courses
	}

	return noCourseSource{}
}

// errNoCourseSource is returned by a State that was not wired to the courses
// domain.
var errNoCourseSource = errors.New("courses domain not wired")

// noCourseSource answers every read with errNoCourseSource.
type noCourseSource struct{}

// Courses implements CourseSource.
func (noCourseSource) Courses(context.Context, []string) ([]CourseInfo, error) {
	return nil, errNoCourseSource
}

// Paths implements CourseSource.
func (noCourseSource) Paths(context.Context, []string) ([]PathInfo, error) {
	return nil, errNoCourseSource
}

// SkillModules implements CourseSource.
func (noCourseSource) SkillModules(context.Context, []string) (map[string][]SkillModule, error) {
	return nil, errNoCourseSource
}

// CourseSessions implements CourseSource.
func (noCourseSource) CourseSessions(context.Context, string) ([]CourseSessionInfo, error) {
	return nil, errNoCourseSource
}

// PublicCourseTitles implements CourseSource.
func (noCourseSource) PublicCourseTitles(context.Context) (map[string]string, error) {
	return nil, errNoCourseSource
}

// PathTitles implements CourseSource.
func (noCourseSource) PathTitles(context.Context) (map[string]string, error) {
	return nil, errNoCourseSource
}

// catalog returns the shared Catalog, building it on first use.
func (s *State) catalog() *Catalog {
	s.catalogOnce.Do(func() {
		if s.Catalog == nil {
			s.Catalog = NewCatalogFrom(s.courseSource())
		}
	})

	return s.Catalog
}
