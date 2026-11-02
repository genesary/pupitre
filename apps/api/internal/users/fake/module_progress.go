package fake

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// ModuleProgressRepository is an in-memory repository.ModuleProgressRepository
// for tests.
type ModuleProgressRepository struct {
	mu       sync.Mutex
	progress []models.ModuleProgress
	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
}

// NewModuleProgressRepository builds a fake ModuleProgressRepository seeded
// with progress rows.
func NewModuleProgressRepository(seed ...models.ModuleProgress) *ModuleProgressRepository {
	return &ModuleProgressRepository{progress: append([]models.ModuleProgress{}, seed...)}
}

// RecordProgress upserts a module attempt: it keeps the best score, ORs the
// passed flag across attempts, and increments the attempt count.
func (f *ModuleProgressRepository) RecordProgress(
	_ context.Context, userID, courseSlug string, moduleIndex int, moduleSlug, moduleType string, score, maxScore int, passed bool,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	now := time.Now()

	idx := f.findIndex(userID, courseSlug, moduleIndex)
	if idx < 0 {
		var completedAt *time.Time
		if passed {
			completedAt = &now
		}

		var slugPtr *string
		if moduleSlug != "" {
			slugPtr = &moduleSlug
		}

		f.progress = append(f.progress, models.ModuleProgress{
			ID: uuid.NewString(), UserID: userID, CourseSlug: courseSlug, ModuleIndex: moduleIndex,
			ModuleSlug: slugPtr, ModuleType: moduleType, BestScore: score, MaxScore: maxScore,
			Passed: passed, Attempts: 1, CompletedAt: completedAt, UpdatedAt: now,
		})

		return nil
	}

	entry := &f.progress[idx]
	entry.ModuleType = moduleType
	updateModuleProgress(entry, moduleSlug, score, maxScore, passed, now)

	return nil
}

// updateModuleProgress applies a new attempt onto an existing progress
// entry, keeping the best score and OR-ing the passed flag, as
// RecordProgress documents.
func updateModuleProgress(entry *models.ModuleProgress, moduleSlug string, score, maxScore int, passed bool, now time.Time) {
	entry.Attempts++

	if score > entry.BestScore {
		entry.BestScore = score
	}

	entry.MaxScore = maxScore
	entry.Passed = entry.Passed || passed

	if entry.ModuleSlug == nil && moduleSlug != "" {
		entry.ModuleSlug = &moduleSlug
	}

	if passed && entry.CompletedAt == nil {
		entry.CompletedAt = &now
	}

	entry.UpdatedAt = now
}

// TotalScore returns the sum of best scores for userID's modules in
// courseSlug.
func (f *ModuleProgressRepository) TotalScore(_ context.Context, userID, courseSlug string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return 0, f.Err
	}

	var total int64

	for _, p := range f.progress {
		if p.UserID == userID && p.CourseSlug == courseSlug {
			total += int64(p.BestScore)
		}
	}

	return total, nil
}

// PassedModuleSlugs returns the slugs of modules userID has passed in
// courseSlug.
func (f *ModuleProgressRepository) PassedModuleSlugs(_ context.Context, userID, courseSlug string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	slugs := make([]string, 0)

	for _, p := range f.progress {
		if p.UserID == userID && p.CourseSlug == courseSlug && p.Passed && p.ModuleSlug != nil {
			slugs = append(slugs, *p.ModuleSlug)
		}
	}

	return slugs, nil
}

// ListByUserCourse returns all module progress rows for userID in
// courseSlug.
func (f *ModuleProgressRepository) ListByUserCourse(_ context.Context, userID, courseSlug string) ([]models.ModuleProgress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	list := make([]models.ModuleProgress, 0)

	for _, p := range f.progress {
		if p.UserID == userID && p.CourseSlug == courseSlug {
			list = append(list, p)
		}
	}

	return list, nil
}

// ListByUserCourses returns all module progress rows for userID in each of
// courseSlugs, keyed by course slug.
func (f *ModuleProgressRepository) ListByUserCourses(
	_ context.Context, userID string, courseSlugs []string,
) (map[string][]models.ModuleProgress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	wanted := make(map[string]bool, len(courseSlugs))
	for _, slug := range courseSlugs {
		wanted[slug] = true
	}

	byCourse := make(map[string][]models.ModuleProgress, len(courseSlugs))

	for _, p := range f.progress {
		if p.UserID == userID && wanted[p.CourseSlug] {
			byCourse[p.CourseSlug] = append(byCourse[p.CourseSlug], p)
		}
	}

	return byCourse, nil
}

// PassedKeys returns the set of "courseSlug/moduleSlug" composite keys for
// all modules the user has passed, across all courses.
func (f *ModuleProgressRepository) PassedKeys(_ context.Context, userID string) (map[string]struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	out := make(map[string]struct{})

	for _, p := range f.progress {
		if p.UserID == userID && p.Passed && p.ModuleSlug != nil {
			out[p.CourseSlug+"/"+*p.ModuleSlug] = struct{}{}
		}
	}

	return out, nil
}

// findIndex returns the index of the matching progress row, or -1.
func (f *ModuleProgressRepository) findIndex(userID, courseSlug string, moduleIndex int) int {
	for i := range f.progress {
		p := f.progress[i]
		if p.UserID == userID && p.CourseSlug == courseSlug && p.ModuleIndex == moduleIndex {
			return i
		}
	}

	return -1
}
