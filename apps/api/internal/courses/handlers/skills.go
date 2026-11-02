package handlers

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// SkillModuleEntry is a single module entry in a skill drill-down response.
type SkillModuleEntry struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Index       int    `json:"index"`
	Type        string `json:"type"`
	CourseSlug  string `json:"courseSlug"`
	CourseTitle string `json:"courseTitle"`
}

// toSkillModuleEntries converts repository rows into their wire form.
func toSkillModuleEntries(found []repository.SkillModule) []SkillModuleEntry {
	modules := make([]SkillModuleEntry, 0, len(found))
	for _, mod := range found {
		modules = append(modules, SkillModuleEntry{
			Name:        mod.Name,
			Slug:        mod.Slug,
			Index:       mod.Index,
			Type:        mod.Type,
			CourseSlug:  mod.CourseSlug,
			CourseTitle: mod.CourseTitle,
		})
	}

	return modules
}

// ListSkillModules godoc
// @Summary  List all modules tagged with a given skill
// @Tags     Skills
// @Produce  json
// @Param    slug  path  string  true  "Skill slug"
// @Success  200   {object}  map[string]interface{}
// @Router   /api/skills/{slug}/modules [get].
func (s *State) ListSkillModules(writer http.ResponseWriter, req *http.Request) {
	skill := param(req, "slug")

	// One indexed query over course_modules replaces scanning every module
	// of every course in the catalog.
	bySkill, err := s.Repos.Courses.ModulesBySkills(req.Context(), []string{skill})
	if err != nil {
		zap.L().Error("list skill modules failed", zap.String("skill", skill), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{
		pathKindSkill:    skill,
		modulesTypeValue: toSkillModuleEntries(bySkill[skill]),
	})
}

// ListSkillModulesBatch godoc
// @Summary  List the modules of several skills in one request
// @Tags     Skills
// @Produce  json
// @Param    slugs  query  string  true  "Comma-separated skill slugs"
// @Success  200    {object}  map[string]interface{}
// @Failure  400    {object}  map[string]string
// @Router   /api/batch/skills [get].
//
// A skill-kind learning path is a list of skills; rendering one used to
// mean one request per skill, in series. This answers the whole path in a
// single query.
func (s *State) ListSkillModulesBatch(writer http.ResponseWriter, req *http.Request) {
	skills := slugsParam(req)

	if len(skills) > maxBatchSlugs {
		s.Error(writer, http.StatusBadRequest, "too many slugs requested")

		return
	}

	out := make(map[string][]SkillModuleEntry, len(skills))

	if len(skills) == 0 {
		s.JSON(writer, http.StatusOK, map[string]any{skillsJSONKey: out})

		return
	}

	out, err := s.BatchSkillModules(req.Context(), skills)
	if err != nil {
		zap.L().Error("batch skill modules lookup failed", zap.Int("skills", len(skills)), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, internalErrorMessage)

		return
	}

	s.JSON(writer, http.StatusOK, map[string]any{skillsJSONKey: out})
}

// BatchSkillModules returns the modules teaching each skill, as
// GET /api/batch/skills reports them.
func (s *State) BatchSkillModules(ctx context.Context, skills []string) (map[string][]SkillModuleEntry, error) {
	out := make(map[string][]SkillModuleEntry, len(skills))
	if len(skills) == 0 {
		return out, nil
	}

	bySkill, err := s.Repos.Courses.ModulesBySkills(ctx, skills)
	if err != nil {
		return nil, fmt.Errorf("list modules by skill: %w", err)
	}

	for _, skill := range skills {
		out[skill] = toSkillModuleEntries(bySkill[skill])
	}

	return out, nil
}
