package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// MarkdownPattern is a reusable markdown rendering rule (global or
// course-scoped) with an HTML/CSS/JS implementation.
type MarkdownPattern struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	Parameter   string  `json:"parameter"`
	HTML        string  `json:"html"`
	CSS         string  `json:"css"`
	JS          string  `json:"js"`
	Scope       string  `json:"scope"`
	FromConfig  bool    `json:"fromConfig"`
	CreatedBy   *string `json:"createdBy,omitempty"`
	// CreatedAt is when the pattern was first created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the pattern was last modified.
	UpdatedAt time.Time `json:"updatedAt"`
}

// patternDTO converts a repository-owned model into the wire-format DTO.
func patternDTO(pattern models.MarkdownPattern) MarkdownPattern {
	var createdBy *string

	if pattern.CreatedBy != nil {
		s := pattern.CreatedBy.String()
		createdBy = &s
	}

	return MarkdownPattern{
		ID: pattern.ID.String(), Name: pattern.Name, Label: pattern.Label,
		Description: pattern.Description, Parameter: pattern.Parameter,
		HTML: pattern.HTML, CSS: pattern.CSS, JS: pattern.JS, Scope: pattern.Scope,
		FromConfig: pattern.FromConfig, CreatedBy: createdBy,
		CreatedAt: pattern.CreatedAt, UpdatedAt: pattern.UpdatedAt,
	}
}

// patternDTOs converts every repository-owned model into its wire-format DTO.
func patternDTOs(patterns []models.MarkdownPattern) []MarkdownPattern {
	out := make([]MarkdownPattern, len(patterns))
	for i, pattern := range patterns {
		out[i] = patternDTO(pattern)
	}

	return out
}

// patternConfigFile is the on-disk YAML shape used to seed markdown patterns
// at startup.
type patternConfigFile struct {
	Patterns []struct {
		Name        string `yaml:"name"`
		Label       string `yaml:"label"`
		Description string `yaml:"description"`
		Parameter   string `yaml:"parameter"`
		HTML        string `yaml:"html"`
		CSS         string `yaml:"css"`
		JS          string `yaml:"js"`
		Scope       string `yaml:"scope"`
	} `yaml:"patterns"`
}

// patternsGlobalScope is the scope value shared by patterns visible to
// every course, as opposed to a single course-specific scope.
const patternsGlobalScope = "global"

// patternsRespKeyPatterns is the JSON response key holding a pattern list.
const patternsRespKeyPatterns = "patterns"

// LoadPatternsFromConfig reads patterns from the YAML file at path (a
// trusted, operator-supplied startup configuration path, not user input)
// and upserts them into the database.
func LoadPatternsFromConfig(ctx context.Context, patterns repository.PatternRepository, path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // path is operator-supplied startup config, not user input
	if err != nil {
		return fmt.Errorf("reading pattern config file %q: %w", path, err)
	}

	var cfg patternConfigFile

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return fmt.Errorf("parsing pattern config file %q: %w", path, err)
	}

	for _, entry := range cfg.Patterns {
		scope := entry.Scope
		if scope == "" {
			scope = patternsGlobalScope
		}

		err := patterns.UpsertFromConfig(ctx, entry.Name, entry.Label, entry.Description, entry.Parameter, entry.HTML, entry.CSS, entry.JS, scope)
		if err != nil {
			return fmt.Errorf("upserting pattern %q: %w", entry.Name, err)
		}
	}

	zap.L().Info("markdown patterns loaded from config", zap.String("path", path), zap.Int("count", len(cfg.Patterns)))

	return nil
}

// ListPatterns godoc
// @Summary   List global markdown patterns
// @Tags      Patterns
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  map[string]interface{}
// @Router    /api/patterns [get].
func (s *State) ListPatterns(writer http.ResponseWriter, req *http.Request) {
	patterns, err := s.Repos.Patterns.ListGlobal(req.Context())
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{patternsRespKeyPatterns: patternDTOs(patterns)})
}

// GetPattern godoc
// @Summary   Get a single markdown pattern
// @Tags      Patterns
// @Security  BearerAuth
// @Produce   json
// @Param     id  path  string  true  "Pattern ID"
// @Success   200  {object}  MarkdownPattern
// @Router    /api/patterns/{id} [get].
func (s *State) GetPattern(writer http.ResponseWriter, req *http.Request) {
	patternID := param(req, "id")

	patternUUID, err := uuid.Parse(patternID)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid pattern ID")

		return
	}

	pattern, err := s.Repos.Patterns.Get(req.Context(), patternUUID)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "Pattern not found")

		return
	}

	s.JSON(writer, http.StatusOK, patternDTO(*pattern))
}

// CreatePattern godoc
// @Summary   Create a markdown pattern (admin)
// @Tags      Patterns
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Success   201  {object}  MarkdownPattern
// @Router    /api/patterns [post].
func (s *State) CreatePattern(writer http.ResponseWriter, req *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Parameter   string `json:"parameter"`
		HTML        string `json:"html"`
		CSS         string `json:"css"`
		JS          string `json:"js"`
		Scope       string `json:"scope"`
	}

	err := decode(req, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	if body.Name == "" || body.Label == "" || body.HTML == "" {
		s.Error(writer, http.StatusBadRequest, "name, label and html are required")

		return
	}

	if body.Scope == "" {
		body.Scope = patternsGlobalScope
	}

	claims := s.claims(req)

	pattern, err := s.Repos.Patterns.Create(req.Context(), body.Name, body.Label, body.Description, body.Parameter,
		body.HTML, body.CSS, body.JS, body.Scope, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusConflict, "Pattern with this name already exists in this scope")

		return
	}

	s.JSON(writer, http.StatusCreated, patternDTO(*pattern))
}

// UpdatePattern godoc
// @Summary   Update a markdown pattern (admin)
// @Tags      Patterns
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     id  path  string  true  "Pattern ID"
// @Success   200  {object}  MarkdownPattern
// @Router    /api/patterns/{id} [put].
func (s *State) UpdatePattern(writer http.ResponseWriter, req *http.Request) {
	name := param(req, "name")
	if name == "" {
		s.Error(writer, http.StatusBadRequest, "Missing pattern name")

		return
	}

	var body struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Parameter   string `json:"parameter"`
		HTML        string `json:"html"`
		CSS         string `json:"css"`
		JS          string `json:"js"`
		Scope       string `json:"scope"`
	}

	err := decode(req, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	if body.HTML == "" {
		s.Error(writer, http.StatusBadRequest, "html is required")

		return
	}

	if body.Name == "" {
		body.Name = name
	}

	if body.Scope == "" {
		body.Scope = patternsGlobalScope
	}

	pattern, err := s.Repos.Patterns.UpdateByName(req.Context(), name, body.Name, body.Label, body.Description,
		body.Parameter, body.HTML, body.CSS, body.JS, body.Scope)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "Pattern not found")

		return
	}

	s.JSON(writer, http.StatusOK, patternDTO(*pattern))
}

// DeletePattern godoc
// @Summary   Delete a markdown pattern (admin)
// @Tags      Patterns
// @Security  BearerAuth
// @Param     id  path  string  true  "Pattern ID"
// @Success   204
// @Router    /api/patterns/{id} [delete].
func (s *State) DeletePattern(writer http.ResponseWriter, req *http.Request) {
	name := param(req, "name")
	if name == "" {
		s.Error(writer, http.StatusBadRequest, "Missing pattern name")

		return
	}

	found, err := s.Repos.Patterns.DeleteByName(req.Context(), name)
	if err != nil || !found {
		s.Error(writer, http.StatusNotFound, "Pattern not found")

		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

// ListCoursePatterns godoc
// @Summary   List patterns for a course (global + course-specific)
// @Tags      Patterns
// @Security  BearerAuth
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Success   200  {object}  map[string]interface{}
// @Router    /api/courses/{slug}/patterns [get].
func (s *State) ListCoursePatterns(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")

	patterns, err := s.Repos.Patterns.ListForCourse(req.Context(), slug)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Database error")

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{patternsRespKeyPatterns: patternDTOs(patterns)})
}

// CreateCoursePattern godoc
// @Summary   Create a course-specific markdown pattern (admin)
// @Tags      Patterns
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     slug  path  string  true  "Course slug"
// @Success   201  {object}  MarkdownPattern
// @Router    /api/courses/{slug}/patterns [post].
func (s *State) CreateCoursePattern(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")

	var body struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Parameter   string `json:"parameter"`
		HTML        string `json:"html"`
		CSS         string `json:"css"`
		JS          string `json:"js"`
	}

	err := decode(req, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	if body.Name == "" || body.Label == "" || body.HTML == "" {
		s.Error(writer, http.StatusBadRequest, "name, label and html required")

		return
	}

	claims := s.claims(req)

	pattern, err := s.Repos.Patterns.Create(req.Context(), body.Name, body.Label, body.Description, body.Parameter,
		body.HTML, body.CSS, body.JS, slug, claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusConflict, "Pattern name already exists in scope")

		return
	}

	s.JSON(writer, http.StatusCreated, patternDTO(*pattern))
}

// DeleteCoursePattern godoc
// @Summary   Delete a course-specific markdown pattern (admin)
// @Tags      Patterns
// @Security  BearerAuth
// @Param     slug  path  string  true  "Course slug"
// @Param     id    path  string  true  "Pattern ID"
// @Success   204
// @Router    /api/courses/{slug}/patterns/{id} [delete].
func (s *State) DeleteCoursePattern(writer http.ResponseWriter, req *http.Request) {
	slug := param(req, "slug")

	patternID := param(req, "id")

	patternUUID, err := uuid.Parse(patternID)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid pattern ID")

		return
	}

	found, err := s.Repos.Patterns.DeleteByIDAndScope(req.Context(), patternUUID, slug)
	if err != nil || !found {
		s.Error(writer, http.StatusNotFound, "Pattern not found")

		return
	}

	writer.WriteHeader(http.StatusNoContent)
}
