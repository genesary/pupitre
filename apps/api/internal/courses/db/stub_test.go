package db

import (
	"context"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/repository"
)

// countingCourseRepository is a repository.CourseRepository that records
// how many writes it received, for asserting that seeding stays a no-op
// when it is not explicitly enabled.
type countingCourseRepository struct {
	creates int
	upserts int
}

// Create records a create call.
func (r *countingCourseRepository) Create(context.Context, *content.Course) error {
	r.creates++

	return nil
}

// Upsert records an upsert call.
func (r *countingCourseRepository) Upsert(context.Context, *content.Course) error {
	r.upserts++

	return nil
}

// List is unused by these tests.
func (r *countingCourseRepository) List(context.Context, repository.CourseFilter) ([]*content.Course, error) {
	return nil, nil
}

// Get always reports the course as missing.
func (r *countingCourseRepository) Get(context.Context, string) (*content.Course, error) {
	return nil, repository.ErrNotFound
}

// Modules is unused by these tests.
func (r *countingCourseRepository) Modules(context.Context, string) ([]content.Module, error) {
	return nil, nil
}

// Delete is unused by these tests.
func (r *countingCourseRepository) Delete(context.Context, string) error {
	return nil
}

// SkillTotals is unused by these tests.
func (r *countingCourseRepository) SkillTotals(context.Context, []string) (map[string]int, error) {
	return map[string]int{}, nil
}

// ModulesBySkills is unused by these tests.
func (r *countingCourseRepository) ModulesBySkills(context.Context, []string) (map[string][]repository.SkillModule, error) {
	return map[string][]repository.SkillModule{}, nil
}

// PutSession is unused by these tests.
func (r *countingCourseRepository) PutSession(context.Context, string, content.Session) error {
	return nil
}

// DeleteSession is unused by these tests.
func (r *countingCourseRepository) DeleteSession(context.Context, string, string) error {
	return nil
}

// SessionExists is unused by these tests.
func (r *countingCourseRepository) SessionExists(context.Context, string, string) (bool, error) {
	return false, nil
}
