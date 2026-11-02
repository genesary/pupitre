// Package fake provides in-memory fakes of user-service's repository
// interfaces, so handler tests can control persisted state without a real
// database connection.
package fake

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// EnrollmentRepository is an in-memory repository.EnrollmentRepository for
// tests.
//
// MyEnrollments only reports the enrolled course slug: CompletedLabs,
// TotalScore, and LastActivity always report zero/nil here, since they are
// normally computed via a join against lesson_progress/module_progress,
// which this fake has no access to.
type EnrollmentRepository struct {
	mu          sync.Mutex
	enrollments []models.Enrollment
	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
}

// NewEnrollmentRepository builds a fake EnrollmentRepository seeded with
// enrollments.
func NewEnrollmentRepository(seed ...models.Enrollment) *EnrollmentRepository {
	return &EnrollmentRepository{enrollments: append([]models.Enrollment{}, seed...)}
}

// Create enrolls userID in courseSlug, or does nothing if already enrolled.
func (f *EnrollmentRepository) Create(_ context.Context, userID, courseSlug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for _, e := range f.enrollments {
		if e.UserID == userID && e.CourseSlug == courseSlug {
			return nil
		}
	}

	f.enrollments = append(f.enrollments, models.Enrollment{
		ID: uuid.NewString(), UserID: userID, CourseSlug: courseSlug, EnrolledAt: time.Now(),
	})

	return nil
}

// Delete removes userID's enrollment in courseSlug, if any.
func (f *EnrollmentRepository) Delete(_ context.Context, userID, courseSlug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for i, e := range f.enrollments {
		if e.UserID == userID && e.CourseSlug == courseSlug {
			f.enrollments = append(f.enrollments[:i], f.enrollments[i+1:]...)

			return nil
		}
	}

	return nil
}

// Exists reports whether userID is enrolled in courseSlug.
func (f *EnrollmentRepository) Exists(_ context.Context, userID, courseSlug string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	for _, e := range f.enrollments {
		if e.UserID == userID && e.CourseSlug == courseSlug {
			return true, nil
		}
	}

	return false, nil
}

// MyEnrollments lists userID's enrolled course slugs.
func (f *EnrollmentRepository) MyEnrollments(_ context.Context, userID string) ([]repository.EnrollmentRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	rows := make([]repository.EnrollmentRow, 0)

	for _, e := range f.enrollments {
		if e.UserID == userID {
			rows = append(rows, repository.EnrollmentRow{Slug: e.CourseSlug})
		}
	}

	return rows, nil
}

// CountAll returns the total number of enrollments.
func (f *EnrollmentRepository) CountAll(_ context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return 0, f.Err
	}

	return int64(len(f.enrollments)), nil
}

// CountDistinctCourses returns the number of distinct courses with at least
// one enrollment.
func (f *EnrollmentRepository) CountDistinctCourses(_ context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return 0, f.Err
	}

	seen := map[string]bool{}
	for _, e := range f.enrollments {
		seen[e.CourseSlug] = true
	}

	return int64(len(seen)), nil
}

// ListByCourse only reports the enrolled user's ID: Username and Email
// always report empty here, since they are normally computed via a join
// against users, which this fake has no access to.
func (f *EnrollmentRepository) ListByCourse(_ context.Context, courseSlug string) ([]repository.CourseEnrollmentRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	rows := make([]repository.CourseEnrollmentRow, 0)

	for _, e := range f.enrollments {
		if e.CourseSlug == courseSlug {
			rows = append(rows, repository.CourseEnrollmentRow{UserID: e.UserID, EnrolledAt: e.EnrolledAt.Format(time.RFC3339)})
		}
	}

	return rows, nil
}
