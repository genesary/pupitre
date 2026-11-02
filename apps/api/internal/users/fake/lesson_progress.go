package fake

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// LessonSlugComplete is the sentinel lesson slug value that marks a course as
// fully completed.
const LessonSlugComplete = "__complete__"

// LessonProgressRepository is an in-memory repository.LessonProgressRepository
// for tests.
type LessonProgressRepository struct {
	mu       sync.Mutex
	progress []models.LessonProgress
	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
}

// NewLessonProgressRepository builds a fake LessonProgressRepository seeded
// with progress rows.
func NewLessonProgressRepository(seed ...models.LessonProgress) *LessonProgressRepository {
	return &LessonProgressRepository{progress: append([]models.LessonProgress{}, seed...)}
}

// MarkComplete records that userID viewed lessonSlug in courseSlug, or does
// nothing if already recorded.
func (f *LessonProgressRepository) MarkComplete(_ context.Context, userID, courseSlug, lessonSlug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for _, p := range f.progress {
		if p.UserID == userID && p.CourseSlug == courseSlug && p.LessonSlug == lessonSlug {
			return nil
		}
	}

	f.progress = append(f.progress, models.LessonProgress{
		ID: uuid.NewString(), UserID: userID, CourseSlug: courseSlug, LessonSlug: lessonSlug, ViewedAt: time.Now(),
	})

	return nil
}

// ViewedSlugs lists the lesson slugs userID has viewed in courseSlug.
func (f *LessonProgressRepository) ViewedSlugs(_ context.Context, userID, courseSlug string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	slugs := make([]string, 0)

	for _, p := range f.progress {
		if p.UserID == userID && p.CourseSlug == courseSlug {
			slugs = append(slugs, p.LessonSlug)
		}
	}

	return slugs, nil
}

// ViewedSlugsByCourses lists the lesson slugs userID has viewed in each of
// courseSlugs, keyed by course slug.
func (f *LessonProgressRepository) ViewedSlugsByCourses(
	_ context.Context, userID string, courseSlugs []string,
) (map[string][]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	wanted := make(map[string]bool, len(courseSlugs))
	for _, slug := range courseSlugs {
		wanted[slug] = true
	}

	byCourse := make(map[string][]string, len(courseSlugs))

	for _, p := range f.progress {
		if p.UserID == userID && wanted[p.CourseSlug] {
			byCourse[p.CourseSlug] = append(byCourse[p.CourseSlug], p.LessonSlug)
		}
	}

	return byCourse, nil
}

// CountViewed returns the number of lessons userID has viewed in courseSlug.
func (f *LessonProgressRepository) CountViewed(_ context.Context, userID, courseSlug string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return 0, f.Err
	}

	var count int64

	for _, p := range f.progress {
		if p.UserID == userID && p.CourseSlug == courseSlug {
			count++
		}
	}

	return count, nil
}

// ViewedKeys returns the set of "courseSlug/lessonSlug" composite keys for
// all non-sentinel lessons the user has viewed.
func (f *LessonProgressRepository) ViewedKeys(_ context.Context, userID string) (map[string]struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	out := make(map[string]struct{})

	for _, p := range f.progress {
		if p.UserID == userID && p.LessonSlug != "" && p.LessonSlug != LessonSlugComplete {
			out[p.CourseSlug+"/"+p.LessonSlug] = struct{}{}
		}
	}

	return out, nil
}

// CompletedCourseSlugs filters slugs down to the courses userID has
// completed.
func (f *LessonProgressRepository) CompletedCourseSlugs(_ context.Context, userID string, slugs []string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	wanted := make(map[string]bool, len(slugs))
	for _, s := range slugs {
		wanted[s] = true
	}

	seen := make(map[string]bool)
	completed := make([]string, 0)

	for _, p := range f.progress {
		if p.UserID == userID && p.LessonSlug == LessonSlugComplete && wanted[p.CourseSlug] && !seen[p.CourseSlug] {
			seen[p.CourseSlug] = true

			completed = append(completed, p.CourseSlug)
		}
	}

	return completed, nil
}

// CompletedCourseSlugsByUsers filters slugs down to the courses each user
// in userIDs has completed.
func (f *LessonProgressRepository) CompletedCourseSlugsByUsers(
	_ context.Context, userIDs, slugs []string,
) (map[string][]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	wantedUser := make(map[string]bool, len(userIDs))
	for _, id := range userIDs {
		wantedUser[id] = true
	}

	wantedSlug := make(map[string]bool, len(slugs))
	for _, slug := range slugs {
		wantedSlug[slug] = true
	}

	byUser := make(map[string][]string, len(userIDs))
	seen := make(map[string]bool)

	for _, entry := range f.progress {
		key := entry.UserID + "/" + entry.CourseSlug
		if entry.LessonSlug != LessonSlugComplete || !wantedUser[entry.UserID] ||
			!wantedSlug[entry.CourseSlug] || seen[key] {
			continue
		}

		seen[key] = true

		byUser[entry.UserID] = append(byUser[entry.UserID], entry.CourseSlug)
	}

	return byUser, nil
}
