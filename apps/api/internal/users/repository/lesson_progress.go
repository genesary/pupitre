package repository

import (
	"context"
	"fmt"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// lessonSlugComplete is the sentinel lessonslug value that marks a course as
// fully completed (as opposed to an individual lesson view).
const lessonSlugComplete = "__complete__"

// LessonProgressRepository is the persistence boundary for the
// lesson_progress table.
type LessonProgressRepository interface {
	MarkComplete(ctx context.Context, userID, courseSlug, lessonSlug string) error
	ViewedSlugs(ctx context.Context, userID, courseSlug string) ([]string, error)

	// ViewedSlugsByCourses returns the viewed lesson slugs of every course
	// in courseSlugs, keyed by course slug, in one query. It is what keeps
	// the batched internal progress endpoints at a fixed query count
	// however many courses they are asked about.
	ViewedSlugsByCourses(ctx context.Context, userID string, courseSlugs []string) (map[string][]string, error)

	CountViewed(ctx context.Context, userID, courseSlug string) (int64, error)

	// CompletedCourseSlugs returns the subset of slugs the user has marked
	// complete via the __complete__ sentinel, which course-service writes
	// once every module of a course is done. It is the sole source of truth
	// for course completion.
	CompletedCourseSlugs(ctx context.Context, userID string, slugs []string) ([]string, error)

	// CompletedCourseSlugsByUsers answers the same question for a whole
	// cohort in one query, keyed by user ID. Reporting a path's enrollments
	// needs it for every enrolled learner at once.
	CompletedCourseSlugsByUsers(ctx context.Context, userIDs, slugs []string) (map[string][]string, error)

	// ViewedKeys returns the set of "courseSlug/lessonSlug" composite keys for
	// all non-sentinel lesson entries the user has viewed, across all courses.
	ViewedKeys(ctx context.Context, userID string) (map[string]struct{}, error)
}

// gormLessonProgressRepository is the GORM-backed LessonProgressRepository.
type gormLessonProgressRepository struct {
	db *gorm.DB
}

// NewGormLessonProgressRepository builds a LessonProgressRepository backed
// by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormLessonProgressRepository(db *gorm.DB) LessonProgressRepository {
	return &gormLessonProgressRepository{db: db}
}

// MarkComplete records that userID viewed lessonSlug in courseSlug, doing
// nothing if already recorded.
func (r *gormLessonProgressRepository) MarkComplete(ctx context.Context, userID, courseSlug, lessonSlug string) error {
	progress := models.LessonProgress{UserID: userID, CourseSlug: courseSlug, LessonSlug: lessonSlug}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: colUserID}, {Name: colCourseSlug}, {Name: colLessonSlug}},
		DoNothing: true,
	}).Create(&progress).Error
	if err != nil {
		return fmt.Errorf("mark lesson complete: %w", err)
	}

	return nil
}

// ViewedSlugs returns the lesson slugs userID has viewed in courseSlug.
func (r *gormLessonProgressRepository) ViewedSlugs(ctx context.Context, userID, courseSlug string) ([]string, error) {
	var slugs []string

	err := r.db.WithContext(ctx).Model(&models.LessonProgress{}).
		Where("userid = ?::uuid AND courseslug = ?", userID, courseSlug).
		Pluck(colLessonSlug, &slugs).Error
	if err != nil {
		return nil, fmt.Errorf("list viewed lessons: %w", err)
	}

	return slugs, nil
}

// ViewedSlugsByCourses returns the lesson slugs userID has viewed in each
// of courseSlugs, keyed by course slug.
//
// One `courseslug = ANY(...)` query answers for the whole set: the caller
// that used to loop over courses issuing one ViewedSlugs per course now
// issues exactly one query whether it asks about one course or a hundred.
func (r *gormLessonProgressRepository) ViewedSlugsByCourses(
	ctx context.Context, userID string, courseSlugs []string,
) (map[string][]string, error) {
	byCourse := make(map[string][]string, len(courseSlugs))
	if len(courseSlugs) == 0 {
		return byCourse, nil
	}

	var rows []struct {
		CourseSlug string `gorm:"column:courseslug"`
		LessonSlug string `gorm:"column:lessonslug"`
	}

	err := r.db.WithContext(ctx).Model(&models.LessonProgress{}).
		Select("courseslug, lessonslug").
		Where("userid = ?::uuid AND courseslug = ANY(?)", userID, pq.StringArray(courseSlugs)).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list viewed lessons by course: %w", err)
	}

	for _, row := range rows {
		byCourse[row.CourseSlug] = append(byCourse[row.CourseSlug], row.LessonSlug)
	}

	return byCourse, nil
}

// CountViewed returns how many lessons userID has viewed in courseSlug.
func (r *gormLessonProgressRepository) CountViewed(ctx context.Context, userID, courseSlug string) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.LessonProgress{}).
		Where("userid = ?::uuid AND courseslug = ?", userID, courseSlug).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count viewed lessons: %w", err)
	}

	return count, nil
}

// ViewedKeys returns the set of "courseSlug/lessonSlug" composite keys for
// all non-sentinel lesson entries userID has viewed, across all courses.
func (r *gormLessonProgressRepository) ViewedKeys(ctx context.Context, userID string) (map[string]struct{}, error) {
	var rows []struct {
		CourseSlug string `gorm:"column:courseslug"`
		LessonSlug string `gorm:"column:lessonslug"`
	}

	err := r.db.WithContext(ctx).Model(&models.LessonProgress{}).
		Select("courseslug, lessonslug").
		Where("userid = ?::uuid AND lessonslug IS NOT NULL AND lessonslug != ?", userID, lessonSlugComplete).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list viewed lesson keys: %w", err)
	}

	out := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		out[row.CourseSlug+"/"+row.LessonSlug] = struct{}{}
	}

	return out, nil
}

// CompletedCourseSlugs returns the subset of slugs userID has marked
// complete via the __complete__ sentinel.
func (r *gormLessonProgressRepository) CompletedCourseSlugs(ctx context.Context, userID string, slugs []string) ([]string, error) {
	var completed []string

	err := r.db.WithContext(ctx).Model(&models.LessonProgress{}).
		Where("userid = ?::uuid AND lessonslug = ? AND courseslug = ANY(?)", userID, lessonSlugComplete, pq.StringArray(slugs)).
		Distinct().Pluck(colCourseSlug, &completed).Error
	if err != nil {
		return nil, fmt.Errorf("list completed course slugs: %w", err)
	}

	return completed, nil
}

// CompletedCourseSlugsByUsers returns, for each user in userIDs, the subset
// of slugs they have marked complete via the __complete__ sentinel.
func (r *gormLessonProgressRepository) CompletedCourseSlugsByUsers(
	ctx context.Context, userIDs, slugs []string,
) (map[string][]string, error) {
	byUser := make(map[string][]string, len(userIDs))
	if len(userIDs) == 0 || len(slugs) == 0 {
		return byUser, nil
	}

	var rows []struct {
		UserID     string `gorm:"column:userid"`
		CourseSlug string `gorm:"column:courseslug"`
	}

	err := r.db.WithContext(ctx).Model(&models.LessonProgress{}).
		Select("DISTINCT userid::text AS userid, courseslug").
		Where("userid::text = ANY(?) AND lessonslug = ? AND courseslug = ANY(?)",
			pq.StringArray(userIDs), lessonSlugComplete, pq.StringArray(slugs)).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list completed course slugs by user: %w", err)
	}

	for _, row := range rows {
		byUser[row.UserID] = append(byUser[row.UserID], row.CourseSlug)
	}

	return byUser, nil
}
