package fake

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// PathEnrollmentRepository is an in-memory repository.PathEnrollmentRepository
// for tests.
//
// ListBySlug's user identity/email/role fields come from Users, seeded
// separately — see the note on fake.UserRepository. Set Users before seeding
// enrollments if a test exercises the joined listing.
type PathEnrollmentRepository struct {
	mu          sync.Mutex
	enrollments []models.PathEnrollment
	// Users backs ListBySlug's join against user identity. Tests that don't
	// call ListBySlug can leave this nil.
	Users *UserRepository

	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
}

// NewPathEnrollmentRepository builds a fake PathEnrollmentRepository seeded
// with enrollments.
func NewPathEnrollmentRepository(seed ...models.PathEnrollment) *PathEnrollmentRepository {
	return &PathEnrollmentRepository{enrollments: append([]models.PathEnrollment{}, seed...)}
}

// MyEnrollments returns userID's path enrollments, most recent first.
func (f *PathEnrollmentRepository) MyEnrollments(
	_ context.Context, userID string, limit *int, offset int,
) ([]repository.PathEnrollmentRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	rows := make([]repository.PathEnrollmentRow, 0)

	for _, e := range f.enrollments {
		if e.UserID.String() == userID {
			rows = append(rows, repository.PathEnrollmentRow{Slug: e.PathSlug, EnrolledAt: e.EnrolledAt})
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].EnrolledAt.After(rows[j].EnrolledAt) })

	if offset >= len(rows) {
		return []repository.PathEnrollmentRow{}, nil
	}

	rows = rows[offset:]

	if limit != nil && *limit < len(rows) {
		rows = rows[:*limit]
	}

	return rows, nil
}

// ListBySlug returns every enrollment in pathSlug, joined with user identity.
func (f *PathEnrollmentRepository) ListBySlug(_ context.Context, pathSlug string) ([]repository.PathEnrolledUserRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	rows := make([]repository.PathEnrolledUserRow, 0)

	for _, enrollment := range f.enrollments {
		if enrollment.PathSlug != pathSlug {
			continue
		}

		row := repository.PathEnrolledUserRow{UserID: enrollment.UserID.String(), EnrolledAt: enrollment.EnrolledAt}

		if f.Users != nil {
			if i := f.Users.findIndexByID(enrollment.UserID); i >= 0 {
				row.Email = f.Users.users[i].Email
				row.Role = f.Users.users[i].Role
			}
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].EnrolledAt.After(rows[j].EnrolledAt) })

	return rows, nil
}

// Enroll enrolls userID in pathSlug, doing nothing if already enrolled.
func (f *PathEnrollmentRepository) Enroll(_ context.Context, userID, pathSlug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}

	for _, e := range f.enrollments {
		if e.UserID == parsedID && e.PathSlug == pathSlug {
			return nil
		}
	}

	f.enrollments = append(
		f.enrollments, models.PathEnrollment{UserID: parsedID, PathSlug: pathSlug, EnrolledAt: time.Now()},
	)

	return nil
}

// EnrolledInAny returns true if userID is enrolled in any of the given
// path slugs.
func (f *PathEnrollmentRepository) EnrolledInAny(_ context.Context, userID string, pathSlugs []string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("parse user id: %w", err)
	}

	set := make(map[string]struct{}, len(pathSlugs))
	for _, s := range pathSlugs {
		set[s] = struct{}{}
	}

	for _, e := range f.enrollments {
		if e.UserID == parsedID {
			if _, ok := set[e.PathSlug]; ok {
				return true, nil
			}
		}
	}

	return false, nil
}

// EnrollWithCourses enrolls userID in pathSlug and records each course slug,
// doing nothing on conflict. In the fake, course enrollments are not tracked
// separately — this is a no-op beyond the path enrollment.
func (f *PathEnrollmentRepository) EnrollWithCourses(_ context.Context, userID, pathSlug string, _ []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}

	for _, e := range f.enrollments {
		if e.UserID == parsedID && e.PathSlug == pathSlug {
			return nil
		}
	}

	f.enrollments = append(
		f.enrollments, models.PathEnrollment{UserID: parsedID, PathSlug: pathSlug, EnrolledAt: time.Now()},
	)

	return nil
}

// Unenroll removes userID's enrollment in pathSlug.
func (f *PathEnrollmentRepository) Unenroll(_ context.Context, userID, pathSlug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}

	for i, e := range f.enrollments {
		if e.UserID == parsedID && e.PathSlug == pathSlug {
			f.enrollments = append(f.enrollments[:i], f.enrollments[i+1:]...)

			return nil
		}
	}

	return nil
}
