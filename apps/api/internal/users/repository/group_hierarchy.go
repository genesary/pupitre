package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// colPath and colDepth are the column names used in hierarchy update maps.
const (
	colPath  = "path"
	colDepth = "depth"
)

// CreateSubgroup creates a local child group under parentID, enforcing the
// maximum nesting depth. Path and depth are derived from the parent.
func (r *gormGroupRepository) CreateSubgroup(ctx context.Context, name, parentID string, memberIDs ...string) (string, error) {
	var newID string

	err := r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		var parent models.Group

		findErr := txDB.Where("id = ?::uuid", parentID).First(&parent).Error
		if findErr != nil {
			return fmt.Errorf("parent group not found: %w", findErr)
		}

		if parent.Depth >= models.GroupMaxDepth {
			return fmt.Errorf("maximum subgroup depth (%d) exceeded", models.GroupMaxDepth)
		}

		group := models.Group{
			Name:     name,
			Source:   groupSourceLocal,
			ParentID: &parent.ID,
			Depth:    parent.Depth + 1,
			Path:     "",
		}

		createErr := txDB.Create(&group).Error
		if createErr != nil {
			return fmt.Errorf("create subgroup: %w", createErr)
		}

		group.Path = parent.Path + "/" + group.ID.String()

		updateErr := txDB.Model(&group).Update("path", group.Path).Error
		if updateErr != nil {
			return fmt.Errorf("update subgroup path: %w", updateErr)
		}

		for _, memberID := range memberIDs {
			memberErr := txDB.Create(&models.UserGroup{UserID: memberID, GroupID: group.ID}).Error
			if memberErr != nil {
				return fmt.Errorf("add subgroup member: %w", memberErr)
			}
		}

		newID = group.ID.String()

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("create subgroup: %w", err)
	}

	return newID, nil
}

// GetSubgroups returns the direct children of parentID with member counts.
func (r *gormGroupRepository) GetSubgroups(ctx context.Context, parentID string) ([]GroupRow, error) {
	var rows []GroupRow

	err := r.db.WithContext(ctx).Table("groups AS g").
		Select(`g.id::text AS id, g.name, g.source,
			COALESCE(g.parent_id::text, '') AS parent_id,
			g.depth,
			g.createdat::text AS created_at,
			COUNT(ug.userid) AS member_count,
			COALESCE(grm.platformrole, '') AS mapped_role`).
		Joins("LEFT JOIN user_groups ug ON ug.groupid = g.id").
		Joins("LEFT JOIN group_role_mappings grm ON grm.groupname = g.name").
		Where("g.parent_id = ?::uuid", parentID).
		Group("g.id, g.name, g.source, g.parent_id, g.depth, g.createdat, grm.platformrole").
		Order("g.name").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get subgroups: %w", err)
	}

	return rows, nil
}

// GetAncestors returns all ancestors of id ordered from root to parent.
// It uses a path-prefix DB query to avoid client-side string parsing.
func (r *gormGroupRepository) GetAncestors(ctx context.Context, groupID string) ([]GroupRow, error) {
	var group models.Group

	findErr := r.db.WithContext(ctx).Where("id = ?::uuid", groupID).First(&group).Error
	if findErr != nil {
		return nil, fmt.Errorf("get group for ancestors: %w", findErr)
	}

	var rows []GroupRow

	// Find every group whose path is a proper prefix of group.Path.
	// The expression `? LIKE g.path || '/%'` is true when group.Path starts
	// with g.path followed by '/', which identifies all ancestors exactly.
	err := r.db.WithContext(ctx).Table("groups AS g").
		Select(`g.id::text AS id, g.name, g.source, g.createdat::text AS created_at,
			COUNT(ug.userid) AS member_count,
			COALESCE(grm.platformrole, '') AS mapped_role`).
		Joins("LEFT JOIN user_groups ug ON ug.groupid = g.id").
		Joins("LEFT JOIN group_role_mappings grm ON grm.groupname = g.name").
		Where("? LIKE g.path || '/%'", group.Path).
		Group("g.id, g.name, g.source, g.createdat, grm.platformrole").
		Order("g.depth").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get ancestors: %w", err)
	}

	return rows, nil
}

// GetDescendants returns all descendants of id using a path prefix query.
func (r *gormGroupRepository) GetDescendants(ctx context.Context, groupID string) ([]GroupRow, error) {
	var group models.Group

	findErr := r.db.WithContext(ctx).Where("id = ?::uuid", groupID).First(&group).Error
	if findErr != nil {
		return nil, fmt.Errorf("get group for descendants: %w", findErr)
	}

	var rows []GroupRow

	err := r.db.WithContext(ctx).Table("groups AS g").
		Select(`g.id::text AS id, g.name, g.source, g.createdat::text AS created_at,
			COUNT(ug.userid) AS member_count,
			COALESCE(grm.platformrole, '') AS mapped_role`).
		Joins("LEFT JOIN user_groups ug ON ug.groupid = g.id").
		Joins("LEFT JOIN group_role_mappings grm ON grm.groupname = g.name").
		Where("g.path LIKE ?", group.Path+"/%").
		Group("g.id, g.name, g.source, g.createdat, grm.platformrole").
		Order("g.depth, g.name").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}

	return rows, nil
}

// MoveGroup moves groupID under newParentID (nil = promote to root), updating
// path and depth for the group and all its descendants atomically.
//
//nolint:funlen,gocyclo,cyclop // path rewriting + cycle detection require multiple steps; extraction would hurt readability
func (r *gormGroupRepository) MoveGroup(ctx context.Context, groupID string, newParentID *string) error {
	err := r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		var group models.Group

		findErr := txDB.Where("id = ?::uuid", groupID).First(&group).Error
		if findErr != nil {
			return fmt.Errorf("group not found: %w", findErr)
		}

		var newDepth int

		var newPath string

		if newParentID != nil {
			var parent models.Group

			parentErr := txDB.Where("id = ?::uuid", *newParentID).First(&parent).Error
			if parentErr != nil {
				return fmt.Errorf("new parent not found: %w", parentErr)
			}

			// Prevent cycle: new parent must not be a descendant of the group being moved.
			if len(parent.Path) >= len(group.Path) && parent.Path[:len(group.Path)] == group.Path {
				return errors.New("cannot move group under one of its own descendants")
			}

			subtreeDepth := maxDescendantDepth(txDB, group.Path)
			if parent.Depth+1+subtreeDepth > models.GroupMaxDepth {
				return fmt.Errorf("move would exceed maximum depth (%d)", models.GroupMaxDepth)
			}

			newDepth = parent.Depth + 1
			newPath = parent.Path + "/" + groupID
		} else {
			newDepth = 0
			newPath = groupID
		}

		depthDelta := newDepth - group.Depth
		oldPrefix := group.Path

		updateErr := txDB.Model(&models.Group{}).
			Where("path LIKE ?", oldPrefix+"/%").
			Updates(map[string]any{
				// The FROM offset must be cast to int explicitly: with the pgx
				// driver an untyped placeholder in substring(... FROM $n) is
				// inferred as text and the bind fails ("unable to encode N
				// into text format").
				colPath:  gorm.Expr("? || substring(path FROM ?::int)::text", newPath, len(oldPrefix)+1),
				colDepth: gorm.Expr("depth + ?::int", depthDelta),
			}).Error
		if updateErr != nil {
			return fmt.Errorf("update descendants paths: %w", updateErr)
		}

		parentUUID := (*uuid.UUID)(nil)

		if newParentID != nil {
			parsed, parseErr := uuid.Parse(*newParentID)
			if parseErr != nil {
				return fmt.Errorf("parse new parent id: %w", parseErr)
			}

			parentUUID = &parsed
		}

		return txDB.Model(&group).Updates(map[string]any{
			"parent_id": parentUUID,
			colDepth:    newDepth,
			colPath:     newPath,
		}).Error
	})
	if err != nil {
		return fmt.Errorf("move group: %w", err)
	}

	return nil
}

// HasChildren reports whether the group identified by id has direct children.
func (r *gormGroupRepository) HasChildren(ctx context.Context, id string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.Group{}).
		Where("parent_id = ?::uuid", id).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check children: %w", err)
	}

	return count > 0, nil
}

// maxDescendantDepth returns the maximum relative depth within the subtree
// rooted at pathPrefix (0 if no descendants).
func maxDescendantDepth(tx *gorm.DB, pathPrefix string) int {
	var maxDepth int

	tx.Model(&models.Group{}).
		Where("path LIKE ?", pathPrefix+"/%").
		Select("COALESCE(MAX(depth), 0)").
		Scan(&maxDepth)

	return maxDepth
}
