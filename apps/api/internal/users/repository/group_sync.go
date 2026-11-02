package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// AddToDefault creates the shared "everyone" group if absent and links
// userID to it.
func (r *gormGroupRepository) AddToDefault(ctx context.Context, userID string) error {
	description := "Default group — all users are members automatically"
	group := models.Group{Name: defaultGroupName, Source: groupSourceLocal, Description: &description}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: colName}},
		DoUpdates: clause.Assignments(map[string]any{colUpdatedAt: gorm.Expr("NOW()")}),
	}).Create(&group).Error
	if err != nil {
		return fmt.Errorf("upsert default group: %w", err)
	}

	if group.ID == uuid.Nil {
		return nil
	}

	userGroup := models.UserGroup{UserID: userID, GroupID: group.ID}

	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&userGroup).Error
	if err != nil {
		return fmt.Errorf("link user to default group: %w", err)
	}

	return nil
}

// SyncEnrollments enrolls userID in every course reachable through their
// group enrollments, doing nothing for enrollments that already exist.
func (r *gormGroupRepository) SyncEnrollments(ctx context.Context, userID string) error {
	var courseSlugs []string

	err := r.db.WithContext(ctx).Table("group_enrollments AS ge").
		Joins("JOIN user_groups ug ON ug.groupid = ge.groupid").
		Where("ug.userid = ?::uuid", userID).
		Pluck("ge.courseslug", &courseSlugs).Error
	if err != nil {
		return fmt.Errorf("list group course slugs: %w", err)
	}

	if len(courseSlugs) == 0 {
		return nil
	}

	enrollments := make([]models.Enrollment, len(courseSlugs))
	for i, slug := range courseSlugs {
		enrollments[i] = models.Enrollment{UserID: userID, CourseSlug: slug}
	}

	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&enrollments).Error
	if err != nil {
		return fmt.Errorf("sync group enrollments: %w", err)
	}

	return nil
}

// SyncGroupsAndDeriveRole upserts groups from an IdP into the groups table,
// replaces the user's group memberships, and returns the highest platform
// role found in group_role_mappings ('admin' beats 'student'). Defaults to
// the user's current role, falling back to 'student' if unknown. The whole
// operation runs in a transaction so a failure partway through leaves the
// user's prior groups/role untouched instead of landing in a broken
// partial-sync state.
func (r *gormGroupRepository) SyncGroupsAndDeriveRole(
	ctx context.Context, userID string, groupNames []string, source string,
) (string, error) {
	var dbRole string

	role := roleStudent

	err := r.db.WithContext(ctx).Transaction(func(groupTx *gorm.DB) error {
		selErr := groupTx.Model(&models.User{}).Select("role").Where("id = ?::uuid", userID).Scan(&dbRole).Error
		if selErr == nil && dbRole != "" {
			role = dbRole
		}

		// Only remove memberships in groups from the same SSO source so that
		// manually managed local group memberships are preserved across logins.
		delErr := groupTx.Delete(&models.UserGroup{}, "userid = ?::uuid AND groupid IN (SELECT id FROM groups WHERE source = ?)", userID, source).Error
		if delErr != nil {
			return fmt.Errorf("clear group memberships: %w", delErr)
		}

		updatedRole, roleMapped, syncErr := applyGroupMemberships(groupTx, userID, groupNames, source, role)
		if syncErr != nil {
			return syncErr
		}

		role = updatedRole

		if roleMapped {
			updErr := groupTx.Model(&models.User{}).Where("id = ?::uuid", userID).Update("role", role).Error
			if updErr != nil {
				return fmt.Errorf("persist derived role: %w", updErr)
			}
		}

		return nil
	})
	if err != nil {
		return role, fmt.Errorf("sync groups and derive role: %w", err)
	}

	return role, nil
}

// applyGroupMemberships syncs userID's membership in every non-empty name in
// groupNames, deriving the platform role along the way. It returns the
// (possibly updated) role, whether any group mapping was applied, and any
// error.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func applyGroupMemberships(groupTx *gorm.DB, userID string, groupNames []string, source, role string) (string, bool, error) {
	roleMapped := false

	for _, name := range groupNames {
		if name == "" {
			continue
		}

		updatedRole, mapped, err := syncGroupMembership(groupTx, userID, name, source, role)
		if err != nil {
			return role, false, err
		}

		role = updatedRole

		if mapped {
			roleMapped = true
		}
	}

	return role, roleMapped, nil
}

// syncGroupMembership upserts a single group, links userID to it, and
// derives the platform role implied by that group's role mapping (if any).
// It returns the (possibly updated) role, whether a mapping was applied,
// and any error.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func syncGroupMembership(groupTx *gorm.DB, userID, name, source, currentRole string) (string, bool, error) {
	group := models.Group{Name: name, Source: source}

	err := groupTx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: colName}},
		DoUpdates: clause.Assignments(map[string]any{"source": source, colUpdatedAt: gorm.Expr("NOW()")}),
	}).Create(&group).Error
	if err != nil {
		return currentRole, false, fmt.Errorf("upsert group %q: %w", name, err)
	}

	userGroup := models.UserGroup{UserID: userID, GroupID: group.ID}

	err = groupTx.Clauses(clause.OnConflict{DoNothing: true}).Create(&userGroup).Error
	if err != nil {
		return currentRole, false, fmt.Errorf("link user to group %q: %w", name, err)
	}

	var mappedRole string

	scanErr := groupTx.Model(&models.GroupRoleMapping{}).
		Select("platformrole").Where("groupname = ?", name).Scan(&mappedRole).Error
	if scanErr != nil {
		return currentRole, false, fmt.Errorf("lookup group mapping %q: %w", name, scanErr)
	}

	if mappedRole == "" {
		return currentRole, false, nil
	}

	updatedRole := currentRole
	if mappedRole == roleAdmin {
		updatedRole = roleAdmin
	} else if updatedRole != roleAdmin {
		updatedRole = mappedRole
	}

	return updatedRole, true, nil
}
