package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/models"
)

// maxLabResults caps the number of rows returned by List, mirroring the
// LIMIT already enforced by the raw-SQL query it replaces.
const maxLabResults = 500

// LabCheckRepository persists and queries recorded lab check outcomes.
type LabCheckRepository interface {
	// List returns up to 500 lab checks, most recent first, optionally
	// filtered to a single course slug (empty string means no filter).
	List(ctx context.Context, courseSlug string) ([]models.LabCheck, error)
	// Create inserts a new lab check row.
	Create(ctx context.Context, check *models.LabCheck) error
	// ListExport returns filtered rows and their total count for export.
	// Pass limit=0 to fetch all rows (used for CSV download).
	ListExport(ctx context.Context, filter LabCheckFilter, limit int) ([]models.LabCheck, int64, error)
	// StreamExport calls rowFn for each filtered row in checkedat DESC
	// order. Rows are scanned one at a time; the full result set is never
	// held in memory.
	StreamExport(ctx context.Context, filter LabCheckFilter, rowFn func(*models.LabCheck) error) error
}

// LabCheckFilter holds optional filter criteria for ListExport.
type LabCheckFilter struct {
	CourseSlug string
	Allow      *bool
	From       *time.Time
	To         *time.Time
}

// gormLabCheckRepository is the GORM-backed LabCheckRepository.
type gormLabCheckRepository struct {
	db *gorm.DB
}

// NewGormLabCheckRepository builds a LabCheckRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormLabCheckRepository(db *gorm.DB) LabCheckRepository {
	return &gormLabCheckRepository{db: db}
}

// List returns up to 500 lab checks, most recent first, optionally filtered
// to a single course slug.
func (r *gormLabCheckRepository) List(ctx context.Context, courseSlug string) ([]models.LabCheck, error) {
	query := r.db.WithContext(ctx).Order("checkedat DESC").Limit(maxLabResults)
	if courseSlug != "" {
		query = query.Where("courseslug = ?", courseSlug)
	}

	results := make([]models.LabCheck, 0)

	err := query.Find(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}

// Create inserts a new lab check row.
func (r *gormLabCheckRepository) Create(ctx context.Context, check *models.LabCheck) error {
	return r.db.WithContext(ctx).Create(check).Error
}

// ListExport returns filtered lab check rows and their total count. When
// limit is 0, all matching rows are returned (CSV download).
func (r *gormLabCheckRepository) ListExport(ctx context.Context, filter LabCheckFilter, limit int) ([]models.LabCheck, int64, error) {
	query := r.applyLabFilter(r.db.WithContext(ctx).Model(&models.LabCheck{}), filter)

	var total int64

	countErr := query.Count(&total).Error
	if countErr != nil {
		return nil, 0, countErr
	}

	query = query.Order("checkedat DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	var rows []models.LabCheck

	err := query.Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

// StreamExport calls rowFn for each filtered lab check row in
// checkedat DESC order. Rows are scanned one at a time so the full
// result set is never held in memory.
func (r *gormLabCheckRepository) StreamExport(ctx context.Context, filter LabCheckFilter, rowFn func(*models.LabCheck) error) error {
	sqlRows, err := r.applyLabFilter(r.db.WithContext(ctx).Model(&models.LabCheck{}), filter).
		Order("checkedat DESC").
		Rows()
	if err != nil {
		return fmt.Errorf("open export cursor: %w", err)
	}

	defer func() { _ = sqlRows.Close() }()

	for sqlRows.Next() {
		var row models.LabCheck

		scanErr := r.db.ScanRows(sqlRows, &row)
		if scanErr != nil {
			return fmt.Errorf("scan export row: %w", scanErr)
		}

		fnErr := rowFn(&row)
		if fnErr != nil {
			return fnErr
		}
	}

	rowsErr := sqlRows.Err()
	if rowsErr != nil {
		return fmt.Errorf("export cursor error: %w", rowsErr)
	}

	return nil
}

// applyLabFilter adds WHERE clauses for each non-nil filter field.
func (r *gormLabCheckRepository) applyLabFilter(query *gorm.DB, filter LabCheckFilter) *gorm.DB {
	if filter.CourseSlug != "" {
		query = query.Where("courseslug = ?", filter.CourseSlug)
	}

	if filter.Allow != nil {
		query = query.Where("allow = ?", *filter.Allow)
	}

	if filter.From != nil {
		query = query.Where("checkedat >= ?", *filter.From)
	}

	if filter.To != nil {
		query = query.Where("checkedat <= ?", *filter.To)
	}

	return query
}
