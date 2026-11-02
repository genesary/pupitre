// Package fake provides in-memory fakes of course-service's repository
// interfaces, so handler tests can control persisted state without a real
// database connection.
package fake

import (
	"context"
	"slices"
	"sync"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// LabCheckRepository is an in-memory fake of repository.LabCheckRepository.
type LabCheckRepository struct {
	mu     sync.Mutex
	checks []models.LabCheck
	nextID int64
	// Err, when set, is returned by every method (simulates a DB failure).
	Err error
}

// NewLabCheckRepository builds a fake seeded with the given checks.
func NewLabCheckRepository(seed ...models.LabCheck) *LabCheckRepository {
	return &LabCheckRepository{checks: append([]models.LabCheck{}, seed...), nextID: int64(len(seed)) + 1}
}

// List returns checks most-recent-first, optionally filtered by course slug.
func (f *LabCheckRepository) List(_ context.Context, courseSlug string) ([]models.LabCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	results := make([]models.LabCheck, 0, len(f.checks))

	for _, check := range slices.Backward(f.checks) {
		if courseSlug != "" && check.CourseSlug != courseSlug {
			continue
		}

		results = append(results, check)
	}

	return results, nil
}

// Create appends check to the fake store, assigning it an ID.
func (f *LabCheckRepository) Create(_ context.Context, check *models.LabCheck) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	check.ID = f.nextID
	f.nextID++
	f.checks = append(f.checks, *check)

	return nil
}

// ListExport returns filtered checks and their total count.
func (f *LabCheckRepository) ListExport(_ context.Context, filter repository.LabCheckFilter, limit int) ([]models.LabCheck, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, 0, f.Err
	}

	results := make([]models.LabCheck, 0, len(f.checks))

	for _, check := range slices.Backward(f.checks) {
		if !matchesLabFilter(check, filter) {
			continue
		}

		results = append(results, check)
	}

	total := int64(len(results))

	if limit > 0 && int(total) > limit {
		results = results[:limit]
	}

	return results, total, nil
}

// StreamExport calls rowFn for each filtered check in reverse-insertion order.
func (f *LabCheckRepository) StreamExport(_ context.Context, filter repository.LabCheckFilter, rowFn func(*models.LabCheck) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for _, check := range slices.Backward(f.checks) {
		if !matchesLabFilter(check, filter) {
			continue
		}

		err := rowFn(&check)
		if err != nil {
			return err
		}
	}

	return nil
}

// matchesLabFilter returns true when check satisfies all non-nil filter fields.
func matchesLabFilter(check models.LabCheck, filter repository.LabCheckFilter) bool {
	if filter.CourseSlug != "" && check.CourseSlug != filter.CourseSlug {
		return false
	}

	if filter.Allow != nil && check.Allow != *filter.Allow {
		return false
	}

	if filter.From != nil && check.CheckedAt.Before(*filter.From) {
		return false
	}

	if filter.To != nil && check.CheckedAt.After(*filter.To) {
		return false
	}

	return true
}
