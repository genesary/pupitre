package handlers

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
)

// courseProgress is one learner's state in one course, as course-service
// needs it while rendering: which lessons are viewed, what each quiz module
// scored, and the aggregates used to evaluate prerequisites.
//
// It is fetched once per request from a single user-service endpoint. The
// handlers used to assemble the same information from four separate calls
// (enrollment check, viewed lessons, module progress, course summary), some
// of them issued more than once while rendering one page.
type courseProgress struct {
	Enrolled      bool
	Viewed        map[string]bool
	Modules       map[int]moduleProgressData
	TotalScore    int
	PassedModules map[string]bool
	ViewedCount   int
}

// emptyCourseProgress is what every failed lookup returns, so callers can
// treat an unreachable user-service as "no progress recorded" without
// special-casing nil maps.
func emptyCourseProgress() *courseProgress {
	return &courseProgress{
		Viewed:        map[string]bool{},
		Modules:       map[int]moduleProgressData{},
		PassedModules: map[string]bool{},
	}
}

// UserProgress is what the courses domain needs from the users domain:
// a learner's enrollments and progress, read and written. In the monolith
// it is a set of function calls into the users domain; the HTTP
// implementation below serves the standalone course-service binary until
// it is removed (docs/specs/104-backend-in-process-calls.md).
type UserProgress interface {
	// CourseOverview returns a learner's full progress in one course.
	CourseOverview(ctx context.Context, userID, courseSlug string) (ProgressOverview, error)
	// CourseSummaries returns the prerequisite aggregates of several
	// courses, keyed by course slug.
	CourseSummaries(ctx context.Context, userID string, courseSlugs []string) (map[string]CourseSummary, error)
	// EnrolledInAnyPath reports whether the learner is enrolled in one of
	// the paths.
	EnrolledInAnyPath(ctx context.Context, userID string, pathSlugs []string) (bool, error)
	// IsEnrolled reports whether the learner is enrolled in the course.
	IsEnrolled(ctx context.Context, userID, courseSlug string) (bool, error)
	// AutoEnroll enrolls the learner in a public course.
	AutoEnroll(ctx context.Context, userID, courseSlug string) error
	// MarkLessonComplete records a viewed lesson and its XP.
	MarkLessonComplete(ctx context.Context, userID, courseSlug, lessonSlug string) error
	// MarkCourseComplete records a completed course: badge, XP and skills.
	MarkCourseComplete(ctx context.Context, completion CourseCompletion) error
	// RecordModule records a quiz or lab result and its XP.
	RecordModule(ctx context.Context, result ModuleResult) error
	// LabIdentity returns the login the identity provider gave the user,
	// or "" (docs/specs/190-backend-defects.md B11).
	LabIdentity(ctx context.Context, userID string) (string, error)
}

// ProgressOverview is one learner's progress in one course, as
// GET /internal/progress/overview reports it.
type ProgressOverview struct {
	Enrolled      bool             `json:"enrolled"`
	Viewed        []string         `json:"viewed"`
	Modules       []ModuleProgress `json:"modules"`
	TotalScore    int              `json:"totalScore"`
	PassedModules []string         `json:"passedModules"`
	ViewedCount   int              `json:"viewedCount"`
}

// ModuleProgress is the recorded result of one module.
type ModuleProgress struct {
	ModuleIndex int  `json:"moduleIndex"`
	BestScore   int  `json:"bestScore"`
	MaxScore    int  `json:"maxScore"`
	Passed      bool `json:"passed"`
	Attempts    int  `json:"attempts"`
}

// CourseSummary is the prerequisite aggregate of one course for a learner.
type CourseSummary struct {
	TotalScore    int      `json:"totalScore"`
	PassedModules []string `json:"passedModules"`
	ViewedCount   int      `json:"viewedCount"`
}

// users returns the users domain wired in by the monolith; a State built
// without one behaves as if the users domain were unreachable.
func (s *State) users() UserProgress { //nolint:ireturn // the wired users domain, or none
	if s.Users != nil {
		return s.Users
	}

	return noUsers{}
}

// errNoUsers is returned by a State that was not wired to the users domain.
var errNoUsers = errors.New("users domain not wired")

// noUsers answers every call with errNoUsers.
type noUsers struct{}

// CourseOverview implements UserProgress.
func (noUsers) CourseOverview(context.Context, string, string) (ProgressOverview, error) {
	return ProgressOverview{}, errNoUsers
}

// CourseSummaries implements UserProgress.
func (noUsers) CourseSummaries(context.Context, string, []string) (map[string]CourseSummary, error) {
	return nil, errNoUsers
}

// EnrolledInAnyPath implements UserProgress.
func (noUsers) EnrolledInAnyPath(context.Context, string, []string) (bool, error) {
	return false, errNoUsers
}

// IsEnrolled implements UserProgress.
func (noUsers) IsEnrolled(context.Context, string, string) (bool, error) { return false, errNoUsers }

// AutoEnroll implements UserProgress.
func (noUsers) AutoEnroll(context.Context, string, string) error { return errNoUsers }

// MarkLessonComplete implements UserProgress.
func (noUsers) MarkLessonComplete(context.Context, string, string, string) error { return errNoUsers }

// MarkCourseComplete implements UserProgress.
func (noUsers) MarkCourseComplete(context.Context, CourseCompletion) error { return errNoUsers }

// RecordModule implements UserProgress.
func (noUsers) RecordModule(context.Context, ModuleResult) error { return errNoUsers }

// LabIdentity implements UserProgress.
func (noUsers) LabIdentity(context.Context, string) (string, error) { return "", errNoUsers }

// postInternalTimeout bounds a detached write to user-service, so a slow or
// unreachable dependency cannot leak goroutines indefinitely.
const postInternalTimeout = 10 * time.Second

// courseProgress fetches userID's full progress in courseSlug in one call.
//
// A failure is reported as empty progress rather than an error: every
// caller renders the course either way, and treating a user-service blip as
// "nothing completed yet" is what the per-call helpers did before.
func (s *State) courseProgress(ctx context.Context, courseSlug, userID string) *courseProgress {
	body, err := s.users().CourseOverview(ctx, userID, courseSlug)
	if err != nil {
		zap.L().Warn("failed to load progress overview",
			zap.String("courseSlug", courseSlug), zap.Error(err))

		return emptyCourseProgress()
	}

	progress := &courseProgress{
		Enrolled:      body.Enrolled,
		Viewed:        make(map[string]bool, len(body.Viewed)),
		Modules:       make(map[int]moduleProgressData, len(body.Modules)),
		TotalScore:    body.TotalScore,
		PassedModules: make(map[string]bool, len(body.PassedModules)),
		ViewedCount:   body.ViewedCount,
	}

	for _, slug := range body.Viewed {
		progress.Viewed[slug] = true
	}

	for _, row := range body.Modules {
		progress.Modules[row.ModuleIndex] = moduleProgressData{
			BestScore: row.BestScore,
			MaxScore:  row.MaxScore,
			Passed:    row.Passed,
			Attempts:  row.Attempts,
		}
	}

	for _, slug := range body.PassedModules {
		progress.PassedModules[slug] = true
	}

	return progress
}

// courseSummaries fetches the prerequisite-relevant aggregates for every
// course in slugs in one call, keyed by slug. A course the response does
// not mention maps to an empty summary.
func (s *State) courseSummaries(ctx context.Context, userID string, slugs []string) map[string]coursePrereqSummary {
	out := make(map[string]coursePrereqSummary, len(slugs))
	if len(slugs) == 0 {
		return out
	}

	summaries, err := s.users().CourseSummaries(ctx, userID, slugs)
	if err != nil {
		zap.L().Warn("failed to load course summaries", zap.Error(err))

		for _, slug := range slugs {
			out[slug] = emptyCoursePrereqSummary()
		}

		return out
	}

	for _, slug := range slugs {
		row, found := summaries[slug]
		if !found {
			out[slug] = emptyCoursePrereqSummary()

			continue
		}

		summary := coursePrereqSummary{
			TotalScore:    row.TotalScore,
			ViewedCount:   row.ViewedCount,
			PassedModules: make(map[string]bool, len(row.PassedModules)),
		}
		for _, moduleSlug := range row.PassedModules {
			summary.PassedModules[moduleSlug] = true
		}

		out[slug] = summary
	}

	return out
}

// pathEnrollmentCheck reports whether userID is enrolled in any of
// pathSlugs.
func (s *State) pathEnrollmentCheck(ctx context.Context, userID string, pathSlugs []string) bool {
	if len(pathSlugs) == 0 {
		return false
	}

	enrolled, err := s.users().EnrolledInAnyPath(ctx, userID, pathSlugs)
	if err != nil {
		zap.L().Warn("failed to check path enrollment", zap.Error(err))

		return false
	}

	return enrolled
}

// enrollmentCheck reports whether userID is enrolled in courseSlug. Prefer
// courseProgress when the caller also needs progress: it answers both.
func (s *State) enrollmentCheck(ctx context.Context, courseSlug, userID string) bool {
	enrolled, err := s.users().IsEnrolled(ctx, userID, courseSlug)
	if err != nil {
		zap.L().Warn("failed to check enrollment", zap.String("courseSlug", courseSlug), zap.Error(err))

		return false
	}

	return enrolled
}

// detached runs a write to the users domain from a goroutine detached from
// the request lifecycle, on its own bounded context. Used for writes whose
// result cannot change the response that has already been (or is about to
// be) sent: a failure is only logged.
func (s *State) detached(call string, write func(ctx context.Context, users UserProgress) error, fields ...zap.Field) {
	users := s.users()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), postInternalTimeout)
		defer cancel()

		err := write(ctx, users)
		if err != nil {
			zap.L().Warn("detached internal call failed",
				append(fields, zap.String("call", call), zap.Error(err))...)
		}
	}()
}

// skillTotalsFor counts how many courses teach each of a course's skills,
// returning nil when the course teaches none.
func (s *State) skillTotalsFor(ctx context.Context, course *content.Course) map[string]int {
	if len(course.Skills) == 0 {
		return nil
	}

	totals, err := s.Repos.Courses.SkillTotals(ctx, course.Skills)
	if err != nil {
		zap.L().Error("count skill totals failed", zap.String("courseSlug", course.Slug), zap.Error(err))

		return nil
	}

	return totals
}
