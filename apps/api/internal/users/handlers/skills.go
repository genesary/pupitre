package handlers

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

const (
	// skillModuleTypeQuiz identifies an assessable quiz module type.
	skillModuleTypeQuiz = "quiz"
	// skillModuleTypeLab identifies an assessable lab module type.
	skillModuleTypeLab = "lab"
)

// skillModuleStatus extends a skill module with the user's completion
// status.
type skillModuleStatus struct {
	SkillModule

	Status string `json:"status"` // "completed" | "available" | "locked"
}

// fetchSkillModules resolves every module tagged with skill through the
// catalog, which batches and caches the lookup.
func (s *State) fetchSkillModules(req *http.Request, skill string) ([]SkillModule, error) {
	if !slugRE.MatchString(skill) {
		return nil, fmt.Errorf("invalid skill slug: %q", skill)
	}

	modules, found := s.catalog().SkillModules(req.Context(), []string{skill})[skill]
	if !found {
		return nil, fmt.Errorf("fetch skill modules: %q unavailable", skill)
	}

	return modules, nil
}

// passedModulesCtx returns the set of "courseSlug/moduleSlug" for all
// modules the user has passed, across all courses.
func (s *State) passedModulesCtx(req *http.Request, userID string) map[string]struct{} {
	out, err := s.Repos.ModuleProgress.PassedKeys(req.Context(), userID)
	if err != nil {
		zap.L().Error("failed to query passed module keys", zap.String("userID", userID), zap.Error(err))

		return nil
	}

	return out
}

// viewedLessonsCtx returns the set of "courseSlug/lessonSlug" for all
// non-quiz/lab lessons the user has viewed.
func (s *State) viewedLessonsCtx(req *http.Request, userID string) map[string]struct{} {
	out, err := s.Repos.LessonProgress.ViewedKeys(req.Context(), userID)
	if err != nil {
		zap.L().Error("failed to query viewed lesson keys", zap.String("userID", userID), zap.Error(err))

		return nil
	}

	return out
}

// skillIsCompleted returns true when all assessable (quiz/lab) modules in the
// list have been completed by the user. Skills with no assessable modules are
// never considered completed. Quizzes: passed in module_progress; labs: viewed.
func skillIsCompleted(modules []SkillModule, passed, viewed map[string]struct{}) bool {
	hasAssessable := false

	for _, mod := range modules {
		if mod.Type != skillModuleTypeQuiz && mod.Type != skillModuleTypeLab {
			continue
		}

		hasAssessable = true

		key := mod.CourseSlug + "/" + mod.Slug

		if mod.Type == skillModuleTypeQuiz {
			if _, ok := passed[key]; !ok {
				return false
			}
		} else {
			if _, ok := viewed[key]; !ok {
				return false
			}
		}
	}

	return hasAssessable
}

// MySkillModules godoc
// @Summary  List modules teaching a skill with the user's completion status
// @Tags     Skills
// @Security BearerAuth
// @Produce  json
// @Param    slug  path  string  true  "Skill slug"
// @Success  200   {object}  map[string]interface{}
// @Router   /api/my/skills/{slug} [get].
func (s *State) MySkillModules(writer http.ResponseWriter, request *http.Request) {
	claims := s.claims(request)
	skill := param(request, "slug")

	if !slugRE.MatchString(skill) {
		s.Error(writer, http.StatusBadRequest, "Invalid skill slug")

		return
	}

	modules, err := s.fetchSkillModules(request, skill)
	if err != nil {
		zap.L().Warn("failed to fetch skill modules", zap.String("skill", skill), zap.Error(err))
		s.Error(writer, http.StatusBadGateway, "failed to fetch skill modules")

		return
	}

	passed := s.passedModulesCtx(request, claims.Subject)
	viewed := s.viewedLessonsCtx(request, claims.Subject)

	result := make([]skillModuleStatus, 0, len(modules))
	prevCompleted := true // first module has no prerequisite

	for _, mod := range modules {
		key := mod.CourseSlug + "/" + mod.Slug

		var isDone bool
		if mod.Type == skillModuleTypeQuiz {
			_, isDone = passed[key]
		} else {
			_, isDone = viewed[key] // labs and text/video: viewed in lesson_progress
		}

		var status string

		switch {
		case isDone:
			status = pathStatusCompleted
		case prevCompleted:
			status = pathStatusAvailable
		default:
			status = pathStatusLocked
		}

		prevCompleted = isDone

		result = append(result, skillModuleStatus{SkillModule: mod, Status: status})
	}

	s.JSON(writer, http.StatusOK, map[string]any{pathKindSkill: skill, "modules": result})
}

// MySkills godoc
// @Summary  List the learner's per-skill expertise levels
// @Tags     Skills
// @Security BearerAuth
// @Produce  json
// @Success  200  {object}  map[string]interface{}
// @Router   /api/my/skills [get].
func (s *State) MySkills(writer http.ResponseWriter, req *http.Request) { //nolint:dupl // same shape as MySessionBookings; different repository and response key
	claims := s.claims(req)

	levels, err := s.Repos.SkillLevels.ListByUser(req.Context(), claims.Subject)
	if err != nil {
		zap.L().Error("failed to list skill levels", zap.String("userID", claims.Subject), zap.Error(err))
		s.Error(writer, http.StatusInternalServerError, "Failed to fetch skill levels")

		return
	}

	if levels == nil {
		levels = []repository.SkillLevelRow{}
	}

	s.JSON(writer, http.StatusOK, map[string]any{"skills": levels})
}
