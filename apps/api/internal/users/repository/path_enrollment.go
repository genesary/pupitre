package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// PathEnrollmentRow is a single row of a user's path-enrollments listing.
type PathEnrollmentRow struct {
	Slug       string
	EnrolledAt time.Time
}

// PathEnrolledUserRow is a single row of the admin per-path enrollment
// listing, joined with the enrolled user's identity.
type PathEnrolledUserRow struct {
	UserID     string
	Email      string
	Role       string
	EnrolledAt time.Time
}

// PathEnrollmentRepository is the persistence boundary for the
// path_enrollments table.
type PathEnrollmentRepository interface {
	MyEnrollments(ctx context.Context, userID string, limit *int, offset int) ([]PathEnrollmentRow, error)
	ListBySlug(ctx context.Context, pathSlug string) ([]PathEnrolledUserRow, error)
	// EnrolledInAny returns whether userID is enrolled in any of the given
	// path slugs.
	EnrolledInAny(ctx context.Context, userID string, pathSlugs []string) (bool, error)
	Enroll(ctx context.Context, userID, pathSlug string) error
	// EnrollWithCourses atomically enrolls userID in pathSlug and in each of
	// courseSlugs, rolling back all inserts on any error.
	EnrollWithCourses(ctx context.Context, userID, pathSlug string, courseSlugs []string) error
	Unenroll(ctx context.Context, userID, pathSlug string) error
}

// gormPathEnrollmentRepository is the GORM-backed PathEnrollmentRepository.
type gormPathEnrollmentRepository struct {
	db *gorm.DB
}

// NewGormPathEnrollmentRepository builds a PathEnrollmentRepository backed by
// db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormPathEnrollmentRepository(db *gorm.DB) PathEnrollmentRepository {
	return &gormPathEnrollmentRepository{db: db}
}

// MyEnrollments returns userID's path enrollments, most recent first.
func (r *gormPathEnrollmentRepository) MyEnrollments(
	ctx context.Context, userID string, limit *int, offset int,
) ([]PathEnrollmentRow, error) {
	rows := make([]PathEnrollmentRow, 0)

	query := r.db.WithContext(ctx).Model(&models.PathEnrollment{}).
		Select("path_slug AS slug, enrolledat AS enrolled_at").
		Where("userid = ?::uuid", userID).
		Order("enrolledat DESC").
		Offset(offset)
	if limit != nil {
		query = query.Limit(*limit)
	}

	err := query.Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query path enrollments: %w", err)
	}

	return rows, nil
}

// ListBySlug returns every user enrolled in pathSlug, most recently
// enrolled first.
func (r *gormPathEnrollmentRepository) ListBySlug(ctx context.Context, pathSlug string) ([]PathEnrolledUserRow, error) {
	rows := make([]PathEnrolledUserRow, 0)

	err := r.db.WithContext(ctx).Table("path_enrollments AS pe").
		Select("u.id::text AS user_id, u.email, u.role, pe.enrolledat AS enrolled_at").
		Joins("JOIN users u ON u.id = pe.userid").
		Where("pe.path_slug = ?", pathSlug).
		Order("pe.enrolledat DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list path enrollments: %w", err)
	}

	return rows, nil
}

// EnrolledInAny returns whether userID is enrolled in any of pathSlugs.
func (r *gormPathEnrollmentRepository) EnrolledInAny(ctx context.Context, userID string, pathSlugs []string) (bool, error) {
	if len(pathSlugs) == 0 {
		return false, nil
	}

	var count int64

	err := r.db.WithContext(ctx).Model(&models.PathEnrollment{}).
		Where("userid = ?::uuid AND path_slug = ANY(?)", userID, pq.StringArray(pathSlugs)).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check path enrollments: %w", err)
	}

	return count > 0, nil
}

// Enroll enrolls userID in pathSlug, doing nothing if already enrolled.
func (r *gormPathEnrollmentRepository) Enroll(ctx context.Context, userID, pathSlug string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}

	enrollment := models.PathEnrollment{UserID: uid, PathSlug: pathSlug}

	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&enrollment).Error
	if err != nil {
		return fmt.Errorf("enroll in path: %w", err)
	}

	return nil
}

// EnrollWithCourses atomically enrolls userID in pathSlug and in each of
// courseSlugs within a single DB transaction, rolling back on any failure.
func (r *gormPathEnrollmentRepository) EnrollWithCourses(
	ctx context.Context, userID, pathSlug string, courseSlugs []string,
) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}

	txErr := r.db.WithContext(ctx).Transaction(func(dbTx *gorm.DB) error {
		pathEnrollment := models.PathEnrollment{UserID: uid, PathSlug: pathSlug}

		txErr := dbTx.Clauses(clause.OnConflict{DoNothing: true}).Create(&pathEnrollment).Error
		if txErr != nil {
			return fmt.Errorf("enroll in path: %w", txErr)
		}

		for _, courseSlug := range courseSlugs {
			courseEnrollment := models.Enrollment{UserID: userID, CourseSlug: courseSlug}

			txErr = dbTx.Clauses(clause.OnConflict{DoNothing: true}).Create(&courseEnrollment).Error
			if txErr != nil {
				return fmt.Errorf("enroll in course %s: %w", courseSlug, txErr)
			}
		}

		return nil
	})
	if txErr != nil {
		return fmt.Errorf("enroll with courses: %w", txErr)
	}

	return nil
}

// Unenroll removes userID's enrollment in pathSlug.
func (r *gormPathEnrollmentRepository) Unenroll(ctx context.Context, userID, pathSlug string) error {
	err := r.db.WithContext(ctx).
		Where("userid = ?::uuid AND path_slug = ?", userID, pathSlug).
		Delete(&models.PathEnrollment{}).Error
	if err != nil {
		return fmt.Errorf("unenroll from path: %w", err)
	}

	return nil
}
