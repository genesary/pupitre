package repository

import (
	"fmt"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// courseFromModel converts a persisted course row into its domain form,
// without its modules — callers that need those load them separately so
// catalog listings never pay for module rows they will not render.
func courseFromModel(row *models.Course) *content.Course {
	course := &content.Course{
		Slug:        row.Slug,
		Title:       row.Title,
		Description: row.Description,
		Category:    row.Category,
		Difficulty:  row.Difficulty,
		IsPublic:    row.Public,
		Hidden:      row.Hidden,
		Scope:       row.Scope,
		InPerson:    row.InPerson,
		XPRequired:  row.XPRequired,
		Skills:      fromTextArray(row.Skills),
	}

	if row.BadgeName != "" || row.BadgeIcon != "" {
		course.Badge = &content.Badge{Name: row.BadgeName, Icon: row.BadgeIcon}
	}

	return course
}

// prerequisiteFromModel converts a persisted prerequisite row into its
// domain form.
func prerequisiteFromModel(row models.CoursePrerequisite) content.CoursePrerequisite {
	return content.CoursePrerequisite{
		Course:   row.RequiredCourse,
		MinScore: row.MinScore,
		Modules:  fromTextArray(row.Modules),
	}
}

// sessionFromModel converts a persisted session row into its domain form.
func sessionFromModel(row models.CourseSession) content.Session {
	return content.Session{
		ID:       row.SessionID,
		Title:    row.Title,
		Date:     row.Date,
		Location: row.Location,
		Capacity: row.Capacity,
	}
}

// moduleFromModel converts a persisted module row into its domain form,
// decoding the jsonb payloads that hold the module's opaque leaf data.
func moduleFromModel(row *models.CourseModule) (content.Module, error) {
	module := content.Module{
		Name:          row.Name,
		Type:          row.Type,
		Src:           row.Src,
		Ref:           row.Ref,
		Path:          row.Path,
		LabURL:        row.LabURL,
		InlineContent: row.InlineContent,
		Replication:   row.Replication,
		Hidden:        row.Hidden,
		Inline:        row.Inline,
		Prerequisites: fromTextArray(row.Prerequisites),
		PassingScore:  row.PassingScore,
		Cooldown: content.CooldownSpec{
			Strategy:    row.CooldownStrategy,
			BaseSeconds: row.CooldownBaseSeconds,
			Multiplier:  row.CooldownMultiplier,
			MaxSeconds:  row.CooldownMaxSeconds,
		},
		MaxAttemptsPerQuestion: row.MaxAttemptsPerQuestion,
		LockOnMaxAttempts:      row.LockOnMaxAttempts,
		CheckProvider:          row.CheckProvider,
		CheckType:              row.CheckType,
		Skills:                 fromTextArray(row.Skills),
	}

	err := models.UnmarshalJSONB(row.CheckParams, &module.CheckParams)
	if err != nil {
		return content.Module{}, fmt.Errorf("module %s check params: %w", row.Slug, err)
	}

	err = models.UnmarshalJSONB(row.Steps, &module.Steps)
	if err != nil {
		return content.Module{}, fmt.Errorf("module %s steps: %w", row.Slug, err)
	}

	err = models.UnmarshalJSONB(row.Questions, &module.Questions)
	if err != nil {
		return content.Module{}, fmt.Errorf("module %s questions: %w", row.Slug, err)
	}

	return module, nil
}

// courseToModel converts a domain course into the row set that persists
// it: the course itself plus its modules, prerequisites and sessions.
//
// Defaults that used to be applied while converting a custom resource are
// applied here instead, so every write path — the admin API included —
// stores a fully-defaulted course.
func courseToModel(course *content.Course) (*models.Course, courseChildren, error) {
	title := course.Title
	if title == "" {
		title = course.Slug
	}

	modules, err := modulesToModel(course.Slug, course.Modules)
	if err != nil {
		return nil, courseChildren{}, err
	}

	row := &models.Course{
		Slug:        course.Slug,
		Title:       title,
		Description: course.Description,
		Category:    course.Category,
		Difficulty:  course.Difficulty,
		Public:      course.IsPublic,
		Hidden:      course.Hidden,
		Scope:       course.Scope,
		XPRequired:  course.XPRequired,
		InPerson:    course.InPerson,
		Skills:      textArray(content.AggregateSkills(course.Modules)),
	}

	if course.Badge != nil {
		row.BadgeName = course.Badge.Name
		row.BadgeIcon = course.Badge.Icon
	}

	children := courseChildren{
		modules:       modules,
		prerequisites: prerequisitesToModel(course.Slug, course.Prerequisites),
		sessions:      sessionsToModel(course.Slug, course.Sessions),
	}

	return row, children, nil
}

// modulesToModel converts a course's modules into their persisted form,
// numbering them by display order and filling in the name/type defaults a
// module may omit.
func modulesToModel(courseSlug string, modules []content.Module) ([]models.CourseModule, error) {
	rows := make([]models.CourseModule, 0, len(modules))

	for position, module := range modules {
		name := module.Name
		if name == "" {
			name = fmt.Sprintf("module-%d", position+1)
		}

		moduleType := module.Type
		if moduleType == "" {
			moduleType = content.ModuleTypeText
		}

		row := models.CourseModule{
			CourseSlug:             courseSlug,
			Position:               position,
			Slug:                   content.SlugifyModuleName(name),
			Name:                   name,
			Type:                   moduleType,
			Src:                    module.Src,
			Ref:                    module.Ref,
			Path:                   module.Path,
			LabURL:                 module.LabURL,
			InlineContent:          module.InlineContent,
			Replication:            module.Replication,
			Hidden:                 module.Hidden,
			Inline:                 module.Inline,
			Prerequisites:          textArray(module.Prerequisites),
			PassingScore:           module.PassingScore,
			MaxAttemptsPerQuestion: module.MaxAttemptsPerQuestion,
			LockOnMaxAttempts:      module.LockOnMaxAttempts,
			CheckProvider:          module.CheckProvider,
			CheckType:              module.CheckType,
			Skills:                 textArray(module.Skills),
		}

		applyCooldownDefaults(&row, module.Cooldown)

		err := encodeModuleDocuments(&row, module)
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// cooldownDefaultBaseSeconds is the cooldown base duration applied to a
// quiz module that configures a cooldown without one.
const cooldownDefaultBaseSeconds = 30

// cooldownDefaultMultiplier is the exponential cooldown multiplier applied
// to a quiz module that configures a cooldown without one.
const cooldownDefaultMultiplier = 1.0

// applyCooldownDefaults copies spec onto row, filling in the strategy,
// base duration and multiplier when the caller left them unset. A module
// that configures no cooldown at all keeps zeroed columns.
func applyCooldownDefaults(row *models.CourseModule, spec content.CooldownSpec) {
	if spec == (content.CooldownSpec{}) {
		return
	}

	row.CooldownStrategy = spec.Strategy
	if row.CooldownStrategy == "" {
		row.CooldownStrategy = content.CooldownStrategyExponential
	}

	row.CooldownBaseSeconds = spec.BaseSeconds
	if row.CooldownBaseSeconds == 0 {
		row.CooldownBaseSeconds = cooldownDefaultBaseSeconds
	}

	row.CooldownMultiplier = spec.Multiplier
	if row.CooldownMultiplier == 0 {
		row.CooldownMultiplier = cooldownDefaultMultiplier
	}

	row.CooldownMaxSeconds = spec.MaxSeconds
}

// encodeModuleDocuments serializes a module's jsonb payloads onto row,
// applying the per-question ID/difficulty/points defaults on the way in so
// they are stored once rather than recomputed on every read.
func encodeModuleDocuments(row *models.CourseModule, module content.Module) error {
	checkParams, err := models.MarshalJSONB(module.CheckParams)
	if err != nil {
		return fmt.Errorf("module %s: %w", row.Slug, err)
	}

	steps, err := models.MarshalJSONB(module.Steps)
	if err != nil {
		return fmt.Errorf("module %s: %w", row.Slug, err)
	}

	questions, err := models.MarshalJSONB(defaultedQuestions(module.Questions))
	if err != nil {
		return fmt.Errorf("module %s: %w", row.Slug, err)
	}

	row.CheckParams = checkParams
	row.Steps = steps
	row.Questions = questions

	return nil
}

// defaultedQuestions fills in the ID, difficulty and points a question may
// omit, so scoring never has to guess them at read time.
func defaultedQuestions(questions []content.Question) []content.Question {
	if len(questions) == 0 {
		return nil
	}

	out := make([]content.Question, len(questions))
	copy(out, questions)

	for index := range out {
		if out[index].ID == "" {
			out[index].ID = fmt.Sprintf("q-%d", index+1)
		}

		if out[index].Difficulty == "" {
			out[index].Difficulty = content.DifficultyMedium
		}

		if out[index].Points == 0 {
			out[index].Points = 1
		}
	}

	return out
}

// prerequisitesToModel converts a course's prerequisites into their
// persisted form, dropping entries that name no course.
func prerequisitesToModel(courseSlug string, prerequisites []content.CoursePrerequisite) []models.CoursePrerequisite {
	rows := make([]models.CoursePrerequisite, 0, len(prerequisites))

	for _, prereq := range prerequisites {
		if prereq.Course == "" {
			continue
		}

		rows = append(rows, models.CoursePrerequisite{
			CourseSlug:     courseSlug,
			RequiredCourse: prereq.Course,
			MinScore:       prereq.MinScore,
			Modules:        textArray(prereq.Modules),
		})
	}

	return rows
}

// sessionsToModel converts a course's sessions into their persisted form,
// dropping entries without an ID.
func sessionsToModel(courseSlug string, sessions []content.Session) []models.CourseSession {
	rows := make([]models.CourseSession, 0, len(sessions))

	for _, session := range sessions {
		if session.ID == "" {
			continue
		}

		rows = append(rows, models.CourseSession{
			CourseSlug: courseSlug,
			SessionID:  session.ID,
			Title:      session.Title,
			Date:       session.Date,
			Location:   session.Location,
			Capacity:   session.Capacity,
		})
	}

	return rows
}
