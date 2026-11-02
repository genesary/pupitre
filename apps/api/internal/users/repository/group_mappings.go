package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// ListMappings returns every group-to-platform-role mapping, ordered by
// group name.
func (r *gormGroupRepository) ListMappings(ctx context.Context) ([]GroupMapping, error) {
	var rows []GroupMapping

	err := r.db.WithContext(ctx).Model(&models.GroupRoleMapping{}).
		Select("groupname AS group_name, platformrole AS platform_role").
		Order("groupname").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list group mappings: %w", err)
	}

	return rows, nil
}

// UpsertMapping creates or updates the platform role mapped to groupName.
func (r *gormGroupRepository) UpsertMapping(ctx context.Context, groupName, platformRole string) error {
	mapping := models.GroupRoleMapping{GroupName: groupName, PlatformRole: platformRole}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "groupname"}},
		DoUpdates: clause.AssignmentColumns([]string{"platformrole"}),
	}).Create(&mapping).Error
	if err != nil {
		return fmt.Errorf("upsert group mapping: %w", err)
	}

	return nil
}

// DeleteMapping removes the role mapping for groupName, reporting whether a
// row was deleted.
func (r *gormGroupRepository) DeleteMapping(ctx context.Context, groupName string) (bool, error) {
	result := r.db.WithContext(ctx).Where("groupname = ?", groupName).Delete(&models.GroupRoleMapping{})
	if result.Error != nil {
		return false, fmt.Errorf("delete group mapping: %w", result.Error)
	}

	return result.RowsAffected > 0, nil
}
