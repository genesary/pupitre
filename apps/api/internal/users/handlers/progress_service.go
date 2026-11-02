package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// The methods below are what the courses domain asks of this one: a
// learner's enrollments and progress, read and written. The /internal/*
// handlers of the standalone user-service decode a request and call them;
// the monolith calls them directly (docs/specs/104-backend-in-process-calls.md).

// errMissingProgressKeys is returned when a progress read names no user or
// no course.
var errMissingProgressKeys = errors.New("userId and courseSlug are required")

// errMissingUserID is returned when a summaries read names no user.
var errMissingUserID = errors.New("userId is required")

// errTooManyCourseSlugs is returned when a summaries read names more
// courses than one request may.
var errTooManyCourseSlugs = fmt.Errorf("at most %d course slugs may be requested at once", maxSummaryCourseSlugs)

// ProgressOverview returns everything the courses domain needs to render
// one course for one learner.
func (s *State) ProgressOverview(ctx context.Context, userID, courseSlug string) (ProgressOverview, error) {
	if userID == "" || courseSlug == "" {
		return ProgressOverview{}, errMissingProgressKeys
	}

	slugs := []string{courseSlug}

	viewedByCourse, moduleRows, err := s.loadCourseProgress(ctx, userID, slugs)
	if err != nil {
		return ProgressOverview{}, err
	}

	enrolled, err := s.Repos.Enrollments.Exists(ctx, userID, courseSlug)
	if err != nil {
		return ProgressOverview{}, fmt.Errorf("check enrollment: %w", err)
	}

	viewed := viewedByCourse[courseSlug]
	if viewed == nil {
		viewed = []string{}
	}

	rows := moduleRows[courseSlug]
	summary := summarizeCourseProgress(rows, viewed)

	return ProgressOverview{
		Enrolled:      enrolled,
		Viewed:        viewed,
		Modules:       toModuleProgressRows(rows),
		TotalScore:    summary.TotalScore,
		PassedModules: summary.PassedModules,
		ViewedCount:   summary.ViewedCount,
	}, nil
}

// CourseSummaries returns the aggregate progress of every course in slugs,
// keyed by slug, with two queries whatever the number of courses.
func (s *State) CourseSummaries(ctx context.Context, userID string, slugs []string) (map[string]CourseProgressSummary, error) {
	if userID == "" {
		return nil, errMissingUserID
	}

	if len(slugs) > maxSummaryCourseSlugs {
		return nil, errTooManyCourseSlugs
	}

	summaries := make(map[string]CourseProgressSummary, len(slugs))
	if len(slugs) == 0 {
		return summaries, nil
	}

	viewedByCourse, moduleRows, err := s.loadCourseProgress(ctx, userID, slugs)
	if err != nil {
		return nil, err
	}

	for _, slug := range slugs {
		summaries[slug] = summarizeCourseProgress(moduleRows[slug], viewedByCourse[slug])
	}

	return summaries, nil
}

// IsEnrolled reports whether userID is enrolled in courseSlug.
func (s *State) IsEnrolled(ctx context.Context, userID, courseSlug string) (bool, error) {
	enrolled, err := s.Repos.Enrollments.Exists(ctx, userID, courseSlug)
	if err != nil {
		return false, fmt.Errorf("check enrollment: %w", err)
	}

	return enrolled, nil
}

// EnrolledInAnyPath reports whether userID is enrolled in one of the paths.
func (s *State) EnrolledInAnyPath(ctx context.Context, userID string, pathSlugs []string) (bool, error) {
	if userID == "" || len(pathSlugs) == 0 {
		return false, nil
	}

	enrolled, err := s.Repos.Paths.EnrolledInAny(ctx, userID, pathSlugs)
	if err != nil {
		return false, fmt.Errorf("check path enrollment: %w", err)
	}

	return enrolled, nil
}

// AutoEnroll enrolls userID in a public course.
func (s *State) AutoEnroll(ctx context.Context, userID, courseSlug string) error {
	err := s.Repos.Enrollments.Create(ctx, userID, courseSlug)
	if err != nil {
		return fmt.Errorf("auto-enroll: %w", err)
	}

	return nil
}

// MarkLessonComplete records that userID viewed a lesson, and grants its XP.
//
// Lesson XP is awarded here because this is the only path a learner
// completing a lesson actually reaches: the courses domain owns
// /api/courses/{slug}/lessons/{slug}/complete and reports the completion
// inwards. Award() is idempotent, so re-completing a lesson does not pay
// twice.
func (s *State) MarkLessonComplete(ctx context.Context, userID, courseSlug, lessonSlug string) error {
	err := s.Repos.LessonProgress.MarkComplete(ctx, userID, courseSlug, lessonSlug)
	if err != nil {
		return fmt.Errorf("mark lesson complete: %w", err)
	}

	_ = s.awardXP(ctx, userID, repository.XPSourceLesson, courseSlug+"/"+lessonSlug, repository.XPAmountLesson)

	return nil
}

// MarkCourseComplete records a completed course, then awards its badge, its
// XP and the skill levels it teaches. Only the completion itself can fail
// the call: the rewards are logged when they fail.
func (s *State) MarkCourseComplete(ctx context.Context, body CourseCompletion) error {
	err := s.Repos.LessonProgress.MarkComplete(ctx, body.UserID, body.CourseSlug, courseCompleteLessonSlug)
	if err != nil {
		return fmt.Errorf("mark course complete: %w", err)
	}

	awardErr := s.Repos.Badges.Award(ctx, body.UserID, body.CourseSlug)
	if awardErr != nil {
		zap.L().Warn("failed to award badge on course completion", zap.String("userID", body.UserID), zap.String("courseSlug", body.CourseSlug), zap.Error(awardErr))
	}

	_ = s.awardXP(ctx, body.UserID, repository.XPSourceCourse, body.CourseSlug, repository.XPAmountCourse)

	if body.Difficulty != "" && len(body.Skills) > 0 {
		err := s.Repos.SkillLevels.UpsertAll(ctx, body.UserID, body.Skills, body.Difficulty, body.SkillTotalCourses)
		if err != nil {
			zap.L().Error("failed to upsert skill levels",
				zap.String("userID", body.UserID), zap.Error(err))
		}
	}

	return nil
}

// LabIdentity returns the login the identity provider gave the user, which
// the courses domain uses to find the learner's GitLab project; "" when the
// user never signed in through the identity provider
// (docs/specs/190-backend-defects.md B11).
func (s *State) LabIdentity(ctx context.Context, userID string) (string, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return "", nil //nolint:nilerr // an unknown subject has no identity
	}

	name, err := s.Repos.Users.IdentityName(ctx, id)
	if errors.Is(err, repository.ErrUserNotFound) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("lab identity: %w", err)
	}

	return name, nil
}

// RecordModuleProgress records a quiz or lab result, and grants the
// module's XP when it is passed.
func (s *State) RecordModuleProgress(ctx context.Context, body ModuleResult) error {
	err := s.Repos.ModuleProgress.RecordProgress(ctx, body.UserID, body.CourseSlug,
		body.ModuleIndex, body.ModuleSlug, body.ModuleType, body.Score, body.MaxScore, body.Passed)
	if err != nil {
		return fmt.Errorf("record module progress: %w", err)
	}

	if body.Passed {
		_ = s.awardXP(ctx, body.UserID, repository.XPSourceModule, body.CourseSlug+"/"+body.ModuleSlug, repository.XPAmountModule)
	}

	return nil
}
