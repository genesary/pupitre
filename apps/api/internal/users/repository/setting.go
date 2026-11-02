package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// SettingRepository is the persistence boundary for the platform_settings
// table.
type SettingRepository interface {
	Get(ctx context.Context, key string) (string, bool, error)
	List(ctx context.Context) ([]models.PlatformSetting, error)
	Upsert(ctx context.Context, key, value string) error
}

// gormSettingRepository is the GORM-backed SettingRepository implementation.
type gormSettingRepository struct {
	db *gorm.DB
}

// NewGormSettingRepository builds a SettingRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormSettingRepository(db *gorm.DB) SettingRepository {
	return &gormSettingRepository{db: db}
}

// Get returns the value for key, whether it was found, and any error.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func (r *gormSettingRepository) Get(ctx context.Context, key string) (string, bool, error) {
	var setting models.PlatformSetting

	err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}

		return "", false, fmt.Errorf("get setting %s: %w", key, err)
	}

	return setting.Value, true, nil
}

// List returns every platform setting, ordered by key.
func (r *gormSettingRepository) List(ctx context.Context) ([]models.PlatformSetting, error) {
	settings := make([]models.PlatformSetting, 0)

	err := r.db.WithContext(ctx).Order("key").Find(&settings).Error
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}

	return settings, nil
}

// Upsert creates or updates the setting identified by key with value.
func (r *gormSettingRepository) Upsert(ctx context.Context, key, value string) error {
	setting := models.PlatformSetting{Key: key, Value: value}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", colUpdatedAt}),
	}).Create(&setting).Error
	if err != nil {
		return fmt.Errorf("upsert setting %s: %w", key, err)
	}

	return nil
}

// ReadSetting reads a platform setting value by key, returning fallback when
// the key is unset or the query fails.
func ReadSetting(ctx context.Context, repo SettingRepository, key, fallback string) string {
	value, ok, err := repo.Get(ctx, key)
	if err != nil || !ok {
		return fallback
	}

	return value
}
