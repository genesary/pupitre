package handlers

import (
	"context"
	"fmt"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// maxSummaryCourseSlugs caps how many courses one course-summaries request
// may ask about. A course's prerequisite list is a handful of entries, so
// this only ever rejects a malformed or hostile caller.
const maxSummaryCourseSlugs = 200

// CourseProgressSummary is one course's aggregate progress for a user.
type CourseProgressSummary struct {
	TotalScore    int64    `json:"totalScore"`
	PassedModules []string `json:"passedModules"`
	ViewedCount   int      `json:"viewedCount"`
}

// ProgressOverview is everything the courses domain needs to render one
// course for one learner, in a single response.
//
// It replaces four separate internal round-trips (enrollment check, viewed
// lessons, module progress, course summary) that every module and lesson
// request used to make in sequence — each with its own connection, JSON
// round-trip and set of database queries.
type ProgressOverview struct {
	Enrolled      bool                `json:"enrolled"`
	Viewed        []string            `json:"viewed"`
	Modules       []moduleProgressRow `json:"modules"`
	TotalScore    int64               `json:"totalScore"`
	PassedModules []string            `json:"passedModules"`
	ViewedCount   int                 `json:"viewedCount"`
}

// loadCourseProgress reads the lesson and module progress of every course
// in slugs with exactly two queries, whatever the length of slugs. It
// returns the viewed lesson slugs and the module progress rows, both keyed
// by course slug.
//
//nolint:gocritic // named results here would trip nonamedreturns instead
func (s *State) loadCourseProgress(
	ctx context.Context, userID string, slugs []string,
) (map[string][]string, map[string][]models.ModuleProgress, error) {
	viewedByCourse, err := s.Repos.LessonProgress.ViewedSlugsByCourses(ctx, userID, slugs)
	if err != nil {
		return nil, nil, fmt.Errorf("load viewed lessons: %w", err)
	}

	moduleRows, err := s.Repos.ModuleProgress.ListByUserCourses(ctx, userID, slugs)
	if err != nil {
		return nil, nil, fmt.Errorf("load module progress: %w", err)
	}

	return viewedByCourse, moduleRows, nil
}

// summarizeCourseProgress derives one course's aggregate progress from the
// rows already in memory, rather than asking the database to sum and filter
// them again.
func summarizeCourseProgress(rows []models.ModuleProgress, viewed []string) CourseProgressSummary {
	summary := CourseProgressSummary{
		PassedModules: make([]string, 0, len(rows)),
		ViewedCount:   len(viewed),
	}

	for _, row := range rows {
		summary.TotalScore += int64(row.BestScore)

		if row.Passed && row.ModuleSlug != nil {
			summary.PassedModules = append(summary.PassedModules, *row.ModuleSlug)
		}
	}

	return summary
}

// toModuleProgressRows converts persisted module progress into the wire
// shape shared by the internal progress endpoints.
func toModuleProgressRows(rows []models.ModuleProgress) []moduleProgressRow {
	out := make([]moduleProgressRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, moduleProgressRow{
			ModuleIndex: row.ModuleIndex,
			BestScore:   row.BestScore,
			MaxScore:    row.MaxScore,
			Passed:      row.Passed,
			Attempts:    row.Attempts,
		})
	}

	return out
}
