package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// ModuleProgressRepository is the persistence boundary for the
// module_progress table.
type ModuleProgressRepository interface {
	// RecordProgress upserts a module attempt: it keeps the best score, ORs
	// the passed flag across attempts, and increments the attempt count.
	RecordProgress(ctx context.Context, userID, courseSlug string, moduleIndex int, moduleSlug, moduleType string, score, maxScore int, passed bool) error
	TotalScore(ctx context.Context, userID, courseSlug string) (int64, error)
	PassedModuleSlugs(ctx context.Context, userID, courseSlug string) ([]string, error)
	ListByUserCourse(ctx context.Context, userID, courseSlug string) ([]models.ModuleProgress, error)

	// ListByUserCourses returns the module progress rows of every course in
	// courseSlugs, keyed by course slug, in one query. The batched internal
	// progress endpoints derive total score, passed slugs and per-module
	// state from these rows rather than issuing an aggregate query each.
	ListByUserCourses(ctx context.Context, userID string, courseSlugs []string) (map[string][]models.ModuleProgress, error)

	// PassedKeys returns the set of "courseSlug/moduleSlug" composite keys for
	// all quiz and lab modules the user has passed, across all courses.
	PassedKeys(ctx context.Context, userID string) (map[string]struct{}, error)
}

// gormModuleProgressRepository is the GORM-backed ModuleProgressRepository.
type gormModuleProgressRepository struct {
	db *gorm.DB
}

// NewGormModuleProgressRepository builds a ModuleProgressRepository backed
// by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormModuleProgressRepository(db *gorm.DB) ModuleProgressRepository {
	return &gormModuleProgressRepository{db: db}
}

// RecordProgress upserts userID's score for a module, keeping the best
// score/pass state seen across attempts rather than overwriting them.
func (r *gormModuleProgressRepository) RecordProgress(
	ctx context.Context, userID, courseSlug string, moduleIndex int, moduleSlug, moduleType string, score, maxScore int, passed bool,
) error {
	var moduleSlugPtr *string
	if moduleSlug != "" {
		moduleSlugPtr = &moduleSlug
	}

	var completedAt *time.Time

	if passed {
		now := time.Now()
		completedAt = &now
	}

	progress := models.ModuleProgress{
		UserID:      userID,
		CourseSlug:  courseSlug,
		ModuleIndex: moduleIndex,
		ModuleSlug:  moduleSlugPtr,
		ModuleType:  moduleType,
		BestScore:   score,
		MaxScore:    maxScore,
		Passed:      passed,
		Attempts:    1,
		CompletedAt: completedAt,
	}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: colUserID}, {Name: colCourseSlug}, {Name: "moduleindex"}},
		DoUpdates: clause.Assignments(map[string]any{
			"attempts":   gorm.Expr("module_progress.attempts + 1"),
			"bestscore":  gorm.Expr("GREATEST(module_progress.bestscore, ?)", score),
			colMaxScore:  maxScore,
			colPassed:    gorm.Expr("module_progress.passed OR ?", passed),
			"moduleslug": gorm.Expr("COALESCE(module_progress.moduleslug, ?)", moduleSlugPtr),
			"moduletype": moduleType,
			colCompletedAt: gorm.Expr(
				"CASE WHEN ? AND module_progress.completed_at IS NULL THEN NOW() ELSE module_progress.completed_at END", passed),
			colUpdatedAt: gorm.Expr("NOW()"),
		}),
	}).Create(&progress).Error
	if err != nil {
		return fmt.Errorf("record module progress: %w", err)
	}

	return nil
}

// TotalScore returns the sum of userID's best scores across every module in
// courseSlug.
func (r *gormModuleProgressRepository) TotalScore(ctx context.Context, userID, courseSlug string) (int64, error) {
	var total int64

	err := r.db.WithContext(ctx).Model(&models.ModuleProgress{}).
		Where("userid = ?::uuid AND courseslug = ?", userID, courseSlug).
		Select("COALESCE(SUM(bestscore), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("total module score: %w", err)
	}

	return total, nil
}

// PassedModuleSlugs returns the module slugs userID has passed in
// courseSlug.
func (r *gormModuleProgressRepository) PassedModuleSlugs(ctx context.Context, userID, courseSlug string) ([]string, error) {
	var slugs []string

	err := r.db.WithContext(ctx).Model(&models.ModuleProgress{}).
		Where("userid = ?::uuid AND courseslug = ? AND passed = TRUE AND moduleslug IS NOT NULL", userID, courseSlug).
		Pluck("moduleslug", &slugs).Error
	if err != nil {
		return nil, fmt.Errorf("list passed module slugs: %w", err)
	}

	return slugs, nil
}

// ListByUserCourse returns every module_progress row for userID in
// courseSlug.
func (r *gormModuleProgressRepository) ListByUserCourse(ctx context.Context, userID, courseSlug string) ([]models.ModuleProgress, error) {
	var list []models.ModuleProgress

	err := r.db.WithContext(ctx).
		Where("userid = ?::uuid AND courseslug = ?", userID, courseSlug).
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list module progress: %w", err)
	}

	return list, nil
}

// ListByUserCourses returns every module_progress row for userID in each
// of courseSlugs, keyed by course slug.
func (r *gormModuleProgressRepository) ListByUserCourses(
	ctx context.Context, userID string, courseSlugs []string,
) (map[string][]models.ModuleProgress, error) {
	byCourse := make(map[string][]models.ModuleProgress, len(courseSlugs))
	if len(courseSlugs) == 0 {
		return byCourse, nil
	}

	var list []models.ModuleProgress

	err := r.db.WithContext(ctx).
		Where("userid = ?::uuid AND courseslug = ANY(?)", userID, pq.StringArray(courseSlugs)).
		Order("courseslug, moduleindex").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list module progress by course: %w", err)
	}

	for _, row := range list {
		byCourse[row.CourseSlug] = append(byCourse[row.CourseSlug], row)
	}

	return byCourse, nil
}

// PassedKeys returns the set of "courseSlug/moduleSlug" composite keys for
// all modules userID has passed, across all courses.
func (r *gormModuleProgressRepository) PassedKeys(ctx context.Context, userID string) (map[string]struct{}, error) {
	var rows []struct {
		CourseSlug string `gorm:"column:courseslug"`
		ModuleSlug string `gorm:"column:moduleslug"`
	}

	err := r.db.WithContext(ctx).Model(&models.ModuleProgress{}).
		Select("courseslug, moduleslug").
		Where("userid = ?::uuid AND passed = true AND moduleslug IS NOT NULL", userID).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list passed module keys: %w", err)
	}

	out := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		out[row.CourseSlug+"/"+row.ModuleSlug] = struct{}{}
	}

	return out, nil
}
