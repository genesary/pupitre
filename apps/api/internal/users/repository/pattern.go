package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// ErrPatternNotFound is returned by lookups that find no matching pattern.
var ErrPatternNotFound = errors.New("pattern not found")

// patternGlobalScope is the scope value shared by patterns visible to every
// course, as opposed to a single course-specific scope.
const patternGlobalScope = "global"

// markdown_patterns column names reused across Create/Update/Upsert calls
// below.
const (
	colLabel       = "label"
	colDescription = "description"
	colParameter   = "parameter"
	colHTML        = "html"
	colCSS         = "css"
	colScope       = "scope"
	colFromConfig  = "from_config"
)

// PatternRepository is the persistence boundary for the markdown_patterns
// table.
type PatternRepository interface {
	ListGlobal(ctx context.Context) ([]models.MarkdownPattern, error)
	ListForCourse(ctx context.Context, courseSlug string) ([]models.MarkdownPattern, error)
	Get(ctx context.Context, id uuid.UUID) (*models.MarkdownPattern, error)
	Create(ctx context.Context, name, label, description, parameter, html, css, js, scope, createdBy string) (*models.MarkdownPattern, error)
	UpdateByName(ctx context.Context, oldName, name, label, description, parameter, html, css, js, scope string) (*models.MarkdownPattern, error)
	DeleteByName(ctx context.Context, name string) (bool, error)
	DeleteByIDAndScope(ctx context.Context, id uuid.UUID, scope string) (bool, error)

	// UpsertFromConfig is an ON CONFLICT (name, scope) DO UPDATE upsert,
	// used by patterns.go's LoadPatternsFromConfig.
	UpsertFromConfig(ctx context.Context, name, label, description, parameter, html, css, js, scope string) error
}

// gormPatternRepository is the GORM-backed PatternRepository implementation.
type gormPatternRepository struct {
	db *gorm.DB
}

// NewGormPatternRepository builds a PatternRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormPatternRepository(db *gorm.DB) PatternRepository {
	return &gormPatternRepository{db: db}
}

// ListGlobal returns every pattern with global scope, ordered by name.
func (r *gormPatternRepository) ListGlobal(ctx context.Context) ([]models.MarkdownPattern, error) {
	patterns := make([]models.MarkdownPattern, 0)

	err := r.db.WithContext(ctx).Where("scope = ?", patternGlobalScope).Order(colName).Find(&patterns).Error
	if err != nil {
		return nil, fmt.Errorf("list global patterns: %w", err)
	}

	return patterns, nil
}

// ListForCourse returns every pattern visible to courseSlug: global-scope
// patterns plus patterns scoped to courseSlug itself.
func (r *gormPatternRepository) ListForCourse(ctx context.Context, courseSlug string) ([]models.MarkdownPattern, error) {
	patterns := make([]models.MarkdownPattern, 0)

	err := r.db.WithContext(ctx).
		Where("scope = ? OR scope = ?", patternGlobalScope, courseSlug).
		Order("scope, name").Find(&patterns).Error
	if err != nil {
		return nil, fmt.Errorf("list course patterns: %w", err)
	}

	return patterns, nil
}

// Get returns the pattern identified by id, or ErrPatternNotFound.
func (r *gormPatternRepository) Get(ctx context.Context, id uuid.UUID) (*models.MarkdownPattern, error) {
	var pattern models.MarkdownPattern

	err := r.db.WithContext(ctx).Where("id = ?", id).First(&pattern).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPatternNotFound
		}

		return nil, fmt.Errorf("get pattern: %w", err)
	}

	return &pattern, nil
}

// Create inserts a new pattern and returns the stored row.
func (r *gormPatternRepository) Create(
	ctx context.Context, name, label, description, parameter, html, css, jsCode, scope, createdBy string,
) (*models.MarkdownPattern, error) {
	createdByID, err := uuid.Parse(createdBy)
	if err != nil {
		return nil, fmt.Errorf("parse createdby: %w", err)
	}

	pattern := models.MarkdownPattern{
		Name:        name,
		Label:       label,
		Description: description,
		Parameter:   parameter,
		HTML:        html,
		CSS:         css,
		JS:          jsCode,
		Scope:       scope,
		CreatedBy:   &createdByID,
	}

	err = r.db.WithContext(ctx).Create(&pattern).Error
	if err != nil {
		return nil, fmt.Errorf("create pattern: %w", err)
	}

	return &pattern, nil
}

// UpdateByName replaces the pattern named oldName with the given fields and
// returns the updated row, or ErrPatternNotFound if no row matched.
func (r *gormPatternRepository) UpdateByName(
	ctx context.Context, oldName, name, label, description, parameter, html, css, jsCode, scope string,
) (*models.MarkdownPattern, error) {
	var pattern models.MarkdownPattern

	err := r.db.WithContext(ctx).Model(&pattern).Clauses(returningAll).
		Where("name = ?", oldName).
		Updates(map[string]any{
			colName:        name,
			colLabel:       label,
			colDescription: description,
			colParameter:   parameter,
			colHTML:        html,
			colCSS:         css,
			"js":           jsCode,
			colScope:       scope,
			colFromConfig:  false,
		}).Error
	if err != nil {
		return nil, fmt.Errorf("update pattern: %w", err)
	}

	if pattern.ID == uuid.Nil {
		return nil, ErrPatternNotFound
	}

	return &pattern, nil
}

// DeleteByName removes the pattern named name, reporting whether a row was
// deleted.
func (r *gormPatternRepository) DeleteByName(ctx context.Context, name string) (bool, error) {
	result := r.db.WithContext(ctx).Where("name = ?", name).Delete(&models.MarkdownPattern{})
	if result.Error != nil {
		return false, fmt.Errorf("delete pattern: %w", result.Error)
	}

	return result.RowsAffected > 0, nil
}

// DeleteByIDAndScope removes the pattern identified by id and scope,
// reporting whether a row was deleted.
func (r *gormPatternRepository) DeleteByIDAndScope(ctx context.Context, id uuid.UUID, scope string) (bool, error) {
	result := r.db.WithContext(ctx).Where("id = ? AND scope = ?", id, scope).Delete(&models.MarkdownPattern{})
	if result.Error != nil {
		return false, fmt.Errorf("delete course pattern: %w", result.Error)
	}

	return result.RowsAffected > 0, nil
}

// UpsertFromConfig inserts or updates a pattern loaded from static config,
// refreshing every field including parameter.
func (r *gormPatternRepository) UpsertFromConfig(
	ctx context.Context, name, label, description, parameter, html, css, jsCode, scope string,
) error {
	pattern := models.MarkdownPattern{
		Name:        name,
		Label:       label,
		Description: description,
		Parameter:   parameter,
		HTML:        html,
		CSS:         css,
		JS:          jsCode,
		Scope:       scope,
		FromConfig:  true,
	}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: colName}, {Name: colScope}},
		DoUpdates: clause.AssignmentColumns(
			[]string{colLabel, colDescription, colParameter, colHTML, colCSS, "js", colFromConfig, colUpdatedAt}),
	}).Create(&pattern).Error
	if err != nil {
		return fmt.Errorf("upsert pattern %q: %w", name, err)
	}

	return nil
}
