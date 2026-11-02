package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/definition"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/markdown"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// Import modes decide what a document does to the course it names.
const (
	// importModeCreate refuses to touch an existing course.
	importModeCreate = "create"
	// importModeReplace swaps the course's modules for the document's.
	importModeReplace = "replace"
	// importModeAppend adds the document's modules after the existing ones.
	importModeAppend = "append"
)

// courseAlreadyExistsMessage is returned when a create import names a slug
// that is already taken.
const courseAlreadyExistsMessage = "Course already exists"

// markdownContentType is the media type an exported course is served as.
const markdownContentType = "text/markdown; charset=utf-8"

// courseImportRequest is the body of both markdown import endpoints.
type courseImportRequest struct {
	// Markdown is the whole document, pasted or read from a file.
	Markdown string `json:"markdown"`
	// Slug overrides the slug the document's frontmatter declares.
	Slug string `json:"slug,omitempty"`
	// Split names the heading level that starts a new module: none, h1,
	// h2, h3, h4, h5 or h6. Empty falls back to the document's own
	// `split` key, then to none.
	Split string `json:"split,omitempty"`
	// Mode is create, replace or append. Empty means create.
	Mode string `json:"mode,omitempty"`
}

// importModuleSummary describes one module a document would produce.
// The body is reported as a length rather than echoed back, so previewing
// a large course does not send it over the wire twice.
type importModuleSummary struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Type   string `json:"type"`
	Bytes  int    `json:"bytes"`
	Hidden bool   `json:"hidden,omitempty"`
	Src    string `json:"src,omitempty"`
	Path   string `json:"path,omitempty"`
	// Existing marks a module that was already stored and is being kept,
	// so an append preview shows where the new ones land.
	Existing bool `json:"existing,omitempty"`
}

// importPreviewResponse is what the document would produce, without
// writing any of it.
type importPreviewResponse struct {
	Slug     string                `json:"slug"`
	Mode     string                `json:"mode"`
	Course   definition.Course     `json:"course"`
	Modules  []importModuleSummary `json:"modules"`
	Warnings []string              `json:"warnings,omitempty"`
}

// importResponse acknowledges a document that was written.
type importResponse struct {
	Slug        string   `json:"slug"`
	Mode        string   `json:"mode"`
	ModuleCount int      `json:"moduleCount"`
	Warnings    []string `json:"warnings,omitempty"`
}

// importPlan is the course a document resolves to once merged with
// whatever is already stored under its slug.
type importPlan struct {
	slug string
	mode string
	spec definition.Course
	// kept counts the leading modules that came from the stored course
	// rather than from the document.
	kept     int
	warnings []string
}

// requestError is an import failure the caller can fix, paired with the
// status to answer it with. Anything else is a 500 and gets logged.
type requestError struct {
	status  int
	message string
}

// Error returns the client-facing message.
func (e requestError) Error() string {
	return e.message
}

// rejectf builds a requestError carrying a formatted message.
func rejectf(status int, format string, args ...any) error {
	return requestError{status: status, message: fmt.Sprintf(format, args...)}
}

// PreviewCourseMarkdownImport godoc
// @Summary  Preview what a markdown document would import (admin)
// @Tags     Admin - Courses
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    payload  body  courseImportRequest  true  "Document and options"
// @Success  200   {object}  importPreviewResponse
// @Failure  400   {object}  map[string]string
// @Failure  404   {object}  map[string]string
// @Failure  409   {object}  map[string]string
// @Router   /api/admin/courses/import/preview [post].
func (s *State) PreviewCourseMarkdownImport(writer http.ResponseWriter, req *http.Request) {
	plan, err := s.planImport(writer, req)
	if err != nil {
		return
	}

	s.JSON(writer, http.StatusOK, importPreviewResponse{
		Slug:     plan.slug,
		Mode:     plan.mode,
		Course:   courseSummary(plan.spec),
		Modules:  moduleSummaries(plan.spec.Modules, plan.kept),
		Warnings: plan.warnings,
	})
}

// ImportCourseMarkdown godoc
// @Summary  Create or update a course from a markdown document (admin)
// @Tags     Admin - Courses
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    payload  body  courseImportRequest  true  "Document and options"
// @Success  200   {object}  importResponse
// @Success  201   {object}  importResponse
// @Failure  400   {object}  map[string]string
// @Failure  404   {object}  map[string]string
// @Failure  409   {object}  map[string]string
// @Router   /api/admin/courses/import [post].
func (s *State) ImportCourseMarkdown(writer http.ResponseWriter, req *http.Request) {
	plan, err := s.planImport(writer, req)
	if err != nil {
		return
	}

	course := plan.spec.ToCourse(plan.slug)

	if plan.mode == importModeCreate {
		err = s.Repos.Courses.Create(req.Context(), course)
	} else {
		err = s.Repos.Courses.Upsert(req.Context(), course)
	}

	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			s.Error(writer, http.StatusConflict, courseAlreadyExistsMessage)

			return
		}

		zap.L().Error("markdown course import failed", zap.String("slug", plan.slug), zap.String("mode", plan.mode), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to import course")

		return
	}

	zap.L().Info("course imported from markdown",
		zap.String("slug", plan.slug), zap.String("mode", plan.mode), zap.Int("modules", len(plan.spec.Modules)))

	status := http.StatusOK
	if plan.mode == importModeCreate {
		status = http.StatusCreated
	}

	s.JSON(writer, status, importResponse{
		Slug:        plan.slug,
		Mode:        plan.mode,
		ModuleCount: len(plan.spec.Modules),
		Warnings:    plan.warnings,
	})
}

// ExportCourseMarkdown godoc
// @Summary  Export a course as a markdown document (admin)
// @Tags     Admin - Courses
// @Security BearerAuth
// @Produce  text/markdown
// @Param    slug   path  string  true   "Course slug"
// @Param    split  query  string  false  "Heading level (h1…h6)"
// @Success  200
// @Failure  404  {object}  map[string]string
// @Router   /api/admin/courses/{slug}/export/markdown [get].
func (s *State) ExportCourseMarkdown(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")

	course, err := s.Repos.Courses.Get(req.Context(), slug)
	if err != nil {
		s.writeRepoError(writer, err, courseNotFoundMessage, "load course", zap.String("slug", slug))

		return
	}

	document, err := markdown.Export(slug, definition.FromCourse(course), markdown.ExportOptions{
		Split: req.URL.Query().Get("split"),
	})
	if err != nil {
		s.Error(writer, http.StatusBadRequest, err.Error())

		return
	}

	filename := content.SlugifyModuleName(slug)
	if filename == "" {
		filename = courseKind
	}

	writer.Header().Set("Content-Type", markdownContentType)
	writer.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.md"`)
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(http.StatusOK)

	//nolint:gosec // G705: admin-authored course markdown, served as a nosniff attachment rather than HTML
	_, err = writer.Write(document)
	if err != nil {
		zap.L().Error("write markdown export", zap.String("slug", slug), zap.Error(err))
	}
}

// planImport decodes an import request and resolves it against what is
// already stored, writing the error response itself when it cannot.
func (s *State) planImport(writer http.ResponseWriter, req *http.Request) (importPlan, error) {
	var payload courseImportRequest

	err := decode(req, &payload)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid import request")

		return importPlan{}, err
	}

	plan, err := s.resolveImport(req.Context(), payload)
	if err != nil {
		rejection, ok := errors.AsType[requestError](err)
		if ok {
			s.Error(writer, rejection.status, rejection.message)

			return importPlan{}, err
		}

		zap.L().Error("resolve markdown import", zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return importPlan{}, err
	}

	return plan, nil
}

// resolveImport parses the document and merges it with the stored course
// the mode targets.
func (s *State) resolveImport(ctx context.Context, payload courseImportRequest) (importPlan, error) {
	if strings.TrimSpace(payload.Markdown) == "" {
		return importPlan{}, rejectf(http.StatusBadRequest, "markdown is required")
	}

	mode, err := importMode(payload.Mode)
	if err != nil {
		return importPlan{}, err
	}

	parsed, err := markdown.Import([]byte(payload.Markdown), markdown.Options{Split: payload.Split, Slug: payload.Slug})
	if err != nil {
		return importPlan{}, rejectf(http.StatusBadRequest, "%s", err.Error())
	}

	// The slug can come from the request, the frontmatter or the title, and
	// none of those are constrained: hold this endpoint to the same rule as
	// the definition endpoints so an import cannot store a course under a
	// handle no route could address.
	if !validSlug(parsed.Slug) {
		return importPlan{}, rejectf(http.StatusBadRequest, "%s", invalidSlugMessage)
	}

	plan := importPlan{slug: parsed.Slug, mode: mode, spec: parsed.Spec, warnings: parsed.Warnings}

	stored, err := s.Repos.Courses.Get(ctx, parsed.Slug)

	switch {
	case errors.Is(err, repository.ErrNotFound):
		if mode != importModeCreate {
			return importPlan{}, rejectf(http.StatusNotFound, courseNotFoundMessage)
		}
	case err != nil:
		return importPlan{}, fmt.Errorf("look up course %s: %w", parsed.Slug, err)
	case mode == importModeCreate:
		return importPlan{}, rejectf(http.StatusConflict, courseAlreadyExistsMessage)
	default:
		plan = mergeStored(plan, definition.FromCourse(stored), parsed.SpecFromDocument)
	}

	return plan, nil
}

// importMode validates the requested mode, defaulting a blank one to
// create.
func importMode(requested string) (string, error) {
	switch requested {
	case "":
		return importModeCreate, nil
	case importModeCreate, importModeReplace, importModeAppend:
		return requested, nil
	default:
		return "", rejectf(http.StatusBadRequest, "unknown mode %q: expected create, replace or append", requested)
	}
}

// mergeStored folds a parsed document into the course already stored under
// its slug.
//
// Append never touches the course's own fields — it only adds modules to
// the end. Replace hands the whole definition over to the document when it
// carries frontmatter (the round trip of an export), and otherwise swaps
// the modules while leaving title, category and the rest as they are, so
// pasting a bare markdown file cannot silently blank a course's metadata.
func mergeStored(plan importPlan, stored definition.Course, documentHasFrontmatter bool) importPlan {
	imported := plan.spec.Modules

	switch {
	case plan.mode == importModeAppend:
		if documentHasFrontmatter {
			plan.warnings = append(plan.warnings, "append mode: the document's frontmatter was ignored, only its modules were added")
		}

		plan.kept = len(stored.Modules)
		plan.spec = stored
		plan.spec.Modules = append(append([]content.Module{}, stored.Modules...), imported...)
	case documentHasFrontmatter:
		plan.warnings = append(plan.warnings, "replace mode: the stored course was replaced by the document's frontmatter")
	default:
		plan.spec = stored
		plan.spec.Modules = imported
	}

	return plan
}

// courseSummary strips the modules off a definition, so a preview reports
// the course's own fields without repeating the bodies below.
func courseSummary(spec definition.Course) definition.Course {
	spec.Modules = nil

	return spec
}

// moduleSummaries describes the modules a plan would store, marking the
// first kept of them as already existing.
func moduleSummaries(modules []content.Module, kept int) []importModuleSummary {
	out := make([]importModuleSummary, 0, len(modules))

	for index, module := range modules {
		out = append(out, importModuleSummary{
			Name:     module.Name,
			Slug:     module.Slug(),
			Type:     module.Type,
			Bytes:    len(module.InlineContent),
			Hidden:   module.Hidden,
			Src:      module.Src,
			Path:     module.Path,
			Existing: index < kept,
		})
	}

	return out
}
