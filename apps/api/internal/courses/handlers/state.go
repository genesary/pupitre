package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/go-chi/chi/v5"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/config"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
)

// maxRequestBodyBytes caps the size of accepted request bodies (1 MB).
const maxRequestBodyBytes = 1 << 20

// maxAdminRequestBodyBytes caps the size of bodies accepted on the admin
// endpoints (10 MB). A course definition carries every module's markdown
// inline, and a markdown import is one document holding the whole course,
// so both run well past what a learner request is allowed.
const maxAdminRequestBodyBytes = 10 << 20

// statusOKValue is the JSON value reported for successful status fields.
const statusOKValue = "ok"

// statusJSONKey is the JSON key used for status fields in responses.
const statusJSONKey = "status"

// errorJSONKey is the JSON key used for error fields in responses.
const errorJSONKey = "error"

// modulesTypeValue identifies a module index entry of type "modules".
const modulesTypeValue = "modules"

// reposClearedJSONKey is the JSON key used for the repos-cleared count.
const reposClearedJSONKey = "reposCleared"

// roleAdmin is the claims role value granting administrative access.
const roleAdmin = "admin"

// courseServiceName identifies this service in health check responses.
const courseServiceName = "course-service"

// moduleTypeLab identifies a module whose content is an interactive lab.
const moduleTypeLab = "lab"

// moduleTypeQuiz identifies a module whose content is a graded quiz.
const moduleTypeQuiz = "quiz"

// moduleTypeText identifies a module whose content is plain text/markdown.
const moduleTypeText = "text"

// moduleTypeImage identifies a module whose content is an image.
const moduleTypeImage = "image"

// checkProviderLocal identifies a lab module verified locally via Tauri.
const checkProviderLocal = "local"

// checkProviderGitLab identifies a lab module verified via GitLab step checks.
const checkProviderGitLab = "gitlab"

// labTypeForm is the legacy labType reported for text-content modules.
const labTypeForm = "form"

// labTypeInteractive is the legacy labType reported for interactive
// modules (video, image, or anything without a dedicated labType).
const labTypeInteractive = "interactive"

// coursesJSONKey is the JSON key used for course-list response bodies.
const coursesJSONKey = "courses"

// pathsJSONKey is the JSON key used for path-list response bodies.
const pathsJSONKey = "paths"

// skillsJSONKey is the JSON key used for skill-keyed response bodies.
const skillsJSONKey = "skills"

// progressJSONKey is the JSON key used for progress response fields.
const progressJSONKey = "progress"

// userIDJSONKey is the JSON key used for user-id response/request fields.
const userIDJSONKey = "userId"

// courseSlugJSONKey is the JSON key used for course-slug fields.
const courseSlugJSONKey = "courseSlug"

// slugJSONKey is the JSON key used for course-slug response fields.
const slugJSONKey = "slug"

// specJSONKey is the JSON key carrying a resource definition on the admin
// endpoints.
const specJSONKey = "spec"

// messageJSONKey is the JSON key used for human-readable message fields.
const messageJSONKey = "message"

// lessonsJSONKey is the JSON key used for lesson-list response bodies.
const lessonsJSONKey = "lessons"

// lessonJSONKey is the JSON key used for single-lesson response bodies.
const lessonJSONKey = "lesson"

// lessonCompleteMessage is returned when a lesson is marked complete.
const lessonCompleteMessage = "Lesson marked as complete"

// totalJSONKey is the JSON key used for total-count response fields.
const totalJSONKey = "total"

// moduleTypeVideo identifies a module whose content is a video.
const moduleTypeVideo = "video"

// courseNotFoundMessage is returned when a course slug matches no course.
const courseNotFoundMessage = "Course not found"

// moduleNotFoundMessage is returned when a module index is out of range.
const moduleNotFoundMessage = "Module not found"

// internalErrorMessage is returned when a storage read or write fails.
const internalErrorMessage = "Internal error"

// State holds the shared dependencies used by the course-service HTTP
// handlers: the persistence repositories, the git content cache, and
// application config.
//
// It holds no course or path data of its own: every request reads what it
// needs from the database, so a replica that has just started serves the
// same content as one that has been up for a week.
type State struct {
	Config   *config.Config
	Repos    *repository.Repositories
	GitCreds *content.GitCredentialStore
	GitCache *content.GitCache

	// Users and Checker are the other domains this one calls, wired in by
	// the monolith (docs/specs/104-backend-in-process-calls.md).
	Users   UserProgress
	Checker LabChecker

	// Sessions, when set, is checked by every guard on every request
	// (docs/specs/190-backend-defects.md B7): the monolith sets it to the
	// users domain's CheckSession.
	Sessions middleware.SessionCheck
}

// NewState builds a State from the given config and repositories, wiring
// up the git cache and optional git credentials.
func NewState(cfg *config.Config, repos *repository.Repositories) *State {
	gitCache := content.NewGitCache("/tmp/pupitre-git-cache", time.Duration(cfg.GitCacheTTL)*time.Minute)

	state := &State{
		Config:   cfg,
		Repos:    repos,
		GitCache: gitCache,
	}

	switch {
	case cfg.GitCredentialsPath != "":
		creds, err := content.LoadCredentials(cfg.GitCredentialsPath)

		switch {
		case err == nil:
			state.GitCreds = creds

			zap.L().Info("git credentials loaded", zap.String("path", cfg.GitCredentialsPath))
		case os.IsNotExist(err):
			zap.L().Debug("git credentials file not found, skipped", zap.String("path", cfg.GitCredentialsPath))
		default:
			zap.L().Warn("failed to load git credentials", zap.String("path", cfg.GitCredentialsPath), zap.Error(err))
		}
	case cfg.GitToken != "":
		zap.L().Info("git global token configured (GIT_TOKEN)")
	default:
		zap.L().Warn("no git token configured — modules from private repositories will fail")
	}

	return state
}

// JSON writes v as JSON with the given status code.
func (s *State) JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		zap.L().Error("encode JSON response", zap.Error(err))
	}
}

// Error writes a JSON error response.
func (s *State) Error(w http.ResponseWriter, status int, msg string) {
	s.JSON(w, status, map[string]string{errorJSONKey: msg})
}

// param retrieves a chi URL parameter by name.
func param(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// decode unmarshals the request body into v.
func decode(r *http.Request, v any) error {
	err := json.NewDecoder(r.Body).Decode(v)
	if err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}

	return nil
}

// readAllBody reads the whole request body, which the admin write
// endpoints need in order to try more than one payload shape against it.
// The size is already bounded by the router's RequestSize middleware.
func readAllBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}

	return body, nil
}

// ClearCache godoc
// @Summary   Clear the entire git content cache (admin)
// @Tags      Admin
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]string
// @Router    /api/admin/cache/clear [post].
func (s *State) ClearCache(w http.ResponseWriter, r *http.Request) {
	s.GitCache.Clear()
	zap.L().Info("git cache cleared by admin")
	s.JSON(w, http.StatusOK, map[string]string{statusJSONKey: statusOKValue})
}

// ClearCourseCache godoc
// @Summary   Clear the git cache for all repos used by a course (admin)
// @Tags      Admin
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Success   200   {object}  map[string]interface{}
// @Failure   404   {object}  map[string]string
// @Router    /api/admin/courses/{slug}/cache/clear [post].
func (s *State) ClearCourseCache(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")

	modules, err := s.Repos.Courses.Modules(req.Context(), slug)
	if err != nil {
		s.writeRepoError(writer, err, courseNotFoundMessage, "load modules", zap.String("slug", slug))

		return
	}

	cleared := 0
	seen := make(map[string]bool)

	for _, mod := range modules {
		if mod.Src == "" || mod.Ref == "" {
			continue
		}

		key := mod.Src + ":" + mod.Ref
		if !seen[key] {
			seen[key] = true

			s.GitCache.ClearRepo(mod.Src, mod.Ref)

			cleared++
		}
	}

	zap.L().Info("course cache cleared", zap.String("slug", slug), zap.Int("repos", cleared))
	s.JSON(writer, http.StatusOK, map[string]any{statusJSONKey: statusOKValue, reposClearedJSONKey: cleared})
}

// ClearModuleCache godoc
// @Summary   Clear the git cache for a specific module (admin)
// @Tags      Admin
// @Security  BearerAuth
// @Produce   json
// @Param     slug   path  string  true  "Course slug"
// @Param     index  path  int     true  "Module index (0-based)"
// @Success   200    {object}  map[string]interface{}
// @Failure   404    {object}  map[string]string
// @Router    /api/admin/courses/{slug}/modules/{index}/cache/clear [post].
func (s *State) ClearModuleCache(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")

	course, ok := s.course(writer, req, slug)
	if !ok {
		return
	}

	modules := s.visibleModules(course, req)

	idx, err := strconv.Atoi(param(req, "index"))
	if err != nil || idx < 0 || idx >= len(modules) {
		s.Error(writer, http.StatusNotFound, moduleNotFoundMessage)

		return
	}

	mod := modules[idx]
	if mod.Src == "" || mod.Ref == "" {
		s.JSON(writer, http.StatusOK, map[string]any{statusJSONKey: statusOKValue, reposClearedJSONKey: 0})

		return
	}

	s.GitCache.ClearRepo(mod.Src, mod.Ref)
	zap.L().Info("module cache cleared", zap.String("slug", slug), zap.Int("index", idx), zap.String("module", mod.Name))
	s.JSON(writer, http.StatusOK, map[string]any{statusJSONKey: statusOKValue, reposClearedJSONKey: 1})
}

// ServeUpload serves a previously uploaded file by filename from the
// configured uploads directory.
// GET /uploads/{filename}.
func (s *State) ServeUpload(writer http.ResponseWriter, req *http.Request) {
	filename := chi.URLParam(req, "filename")
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		s.Error(writer, http.StatusBadRequest, "Invalid filename")

		return
	}

	path := filepath.Join(s.Config.UploadsDir, filename)
	http.ServeFile(writer, req, path) //nolint:gosec // filename validated above to reject path traversal ('..' or '/')
}

// course loads a course with its modules, writing the appropriate error
// response and returning ok=false when it is missing or unreadable.
//
// A missing course and an unreachable database are reported differently on
// purpose: the in-memory store could only ever say "not found" for both,
// which turned an outage into a catalog that silently looked empty.
func (s *State) course(writer http.ResponseWriter, req *http.Request, slug string) (*content.Course, bool) {
	course, err := s.Repos.Courses.Get(req.Context(), slug)
	if err != nil {
		s.writeRepoError(writer, err, courseNotFoundMessage, "load course", zap.String("slug", slug))

		return nil, false
	}

	return course, true
}

// writeRepoError maps a repository error onto an HTTP response: ErrNotFound
// becomes a 404 carrying notFoundMsg, anything else a logged 500.
func (s *State) writeRepoError(writer http.ResponseWriter, err error, notFoundMsg, operation string, fields ...zap.Field) {
	if errors.Is(err, repository.ErrNotFound) {
		s.Error(writer, http.StatusNotFound, notFoundMsg)

		return
	}

	zap.L().Error(operation+" failed", append(fields, zap.Error(err))...)
	s.Error(writer, http.StatusInternalServerError, internalErrorMessage)
}

// visibleModules expands type:modules index entries, then returns all
// modules for admins or only non-hidden modules for regular users.
func (s *State) visibleModules(c *content.Course, req *http.Request) []content.Module {
	expanded := s.expandModuleIndexes(req.Context(), c.Modules)

	claims := s.claims(req)
	if claims != nil && claims.Role == roleAdmin {
		return expanded
	}

	var out []content.Module

	for _, m := range expanded {
		if !m.Hidden {
			out = append(out, m)
		}
	}

	return out
}

// expandModuleIndexes replaces every type:modules entry with the modules
// its git-hosted index file lists, leaving other modules untouched. An
// index that cannot be fetched is skipped with a warning, matching how a
// module with unreachable content behaves elsewhere.
func (s *State) expandModuleIndexes(ctx context.Context, modules []content.Module) []content.Module {
	expanded := make([]content.Module, 0, len(modules))

	for _, mod := range modules {
		if mod.Type != modulesTypeValue {
			expanded = append(expanded, mod)

			continue
		}

		subs, err := content.FetchModuleIndex(ctx, s.GitCache, mod, s.tokenForRepo(mod.Src))
		if err != nil {
			zap.L().Warn("failed to expand module index, skipping", zap.String("module", mod.Name), zap.Error(err))

			continue
		}

		expanded = append(expanded, subs...)
	}

	return expanded
}

// tokenForRepo returns the git token to use for the given repo URL, falling
// back to the service-wide token when no repo-specific credential matches.
func (s *State) tokenForRepo(repoURL string) string {
	if s.GitCreds != nil {
		if t := s.GitCreds.Match(repoURL); t != "" {
			return t
		}
	}

	return s.Config.GitToken
}

// claims retrieves the authenticated user's claims from the request context.
func (s *State) claims(r *http.Request) *middleware.Claims {
	return middleware.GetClaims(r)
}
