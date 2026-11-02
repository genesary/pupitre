package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// ListMembers returns every user belonging to the group identified by groupID.
func (r *gormGroupRepository) ListMembers(ctx context.Context, groupID string) ([]AdminUserRow, error) {
	var rows []AdminUserRow

	err := r.db.WithContext(ctx).Table("users AS u").
		Select(`u.id::text AS id, u.username, u.email, u.role, u.isactive AS is_active,
			u.avatarurl AS avatar_url, u.bio, u.authprovider AS auth_provider,
			u.createdat::text AS created_at,
			COUNT(e.courseslug) AS enrolled_courses`).
		Joins("JOIN user_groups ug ON ug.userid = u.id").
		Joins("LEFT JOIN enrollments e ON e.userid = u.id").
		Where("ug.groupid = ?::uuid", groupID).
		Group("u.id, u.username, u.email, u.role, u.isactive, u.avatarurl, u.bio, u.authprovider, u.createdat").
		Order("u.username").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}

	return rows, nil
}

// AddMember links userID to the group identified by groupID, doing nothing
// if the membership already exists.
func (r *gormGroupRepository) AddMember(ctx context.Context, groupID, userID string) error {
	groupUUID, err := uuid.Parse(groupID)
	if err != nil {
		return fmt.Errorf("parse group id: %w", err)
	}

	link := models.UserGroup{UserID: userID, GroupID: groupUUID}

	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error
	if err != nil {
		return fmt.Errorf("add group member: %w", err)
	}

	return nil
}

// RemoveMember unlinks userID from the group identified by groupID.
func (r *gormGroupRepository) RemoveMember(ctx context.Context, groupID, userID string) error {
	err := r.db.WithContext(ctx).
		Where("groupid = ?::uuid AND userid = ?::uuid", groupID, userID).
		Delete(&models.UserGroup{}).Error
	if err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}

	return nil
}

// GetGroupsByUserID returns all groups the given user belongs to.
func (r *gormGroupRepository) GetGroupsByUserID(ctx context.Context, userID string) ([]GroupRow, error) {
	var rows []GroupRow

	err := r.db.WithContext(ctx).Table("groups AS g").
		Select(`g.id::text AS id, g.name, g.source,
			COALESCE(g.parent_id::text, '') AS parent_id,
			g.depth,
			g.createdat::text AS created_at,
			COUNT(ug2.userid) AS member_count,
			COALESCE(grm.platformrole, '') AS mapped_role`).
		Joins("JOIN user_groups ug ON ug.groupid = g.id").
		Joins("LEFT JOIN user_groups ug2 ON ug2.groupid = g.id").
		Joins("LEFT JOIN group_role_mappings grm ON grm.groupname = g.name").
		Where("ug.userid = ?::uuid", userID).
		Group("g.id, g.name, g.source, g.parent_id, g.depth, g.createdat, grm.platformrole").
		Order("g.depth, g.name").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get groups by user id: %w", err)
	}

	return rows, nil
}

// ListMemberIDs returns the user IDs (as strings) of every member of groupID.
func (r *gormGroupRepository) ListMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	var ids []string

	err := r.db.WithContext(ctx).Table("user_groups").
		Select("userid::text").
		Where("groupid = ?::uuid", groupID).
		Pluck("userid::text", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("list member ids: %w", err)
	}

	return ids, nil
}

// UserInAnyGroup reports whether userID belongs to at least one of the
// groups listed in groupIDs.
func (r *gormGroupRepository) UserInAnyGroup(ctx context.Context, userID string, groupIDs []string) (bool, error) {
	if len(groupIDs) == 0 {
		return false, nil
	}

	var count int64

	err := r.db.WithContext(ctx).Model(&models.UserGroup{}).
		Where("userid = ?::uuid AND groupid::text = ANY(?)", userID, pq.Array(groupIDs)).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check group membership: %w", err)
	}

	return count > 0, nil
}

// HasMembers reports whether the group identified by id has any user members.
func (r *gormGroupRepository) HasMembers(ctx context.Context, id string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.UserGroup{}).
		Where("groupid = ?::uuid", id).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check members: %w", err)
	}

	return count > 0, nil
}

// ListMembersRecursive returns members of groupID and all its subgroups,
// flagging whether each user is a direct member of groupID.
func (r *gormGroupRepository) ListMembersRecursive(ctx context.Context, groupID string) ([]GroupMemberRow, error) {
	var group models.Group

	findErr := r.db.WithContext(ctx).Where("id = ?::uuid", groupID).First(&group).Error
	if findErr != nil {
		return nil, fmt.Errorf("get group for recursive members: %w", findErr)
	}

	var rows []GroupMemberRow

	err := r.db.WithContext(ctx).
		Table("users AS u").
		Select(`DISTINCT u.id::text AS id, u.username, u.email, u.role,
			u.isactive AS is_active, u.avatarurl AS avatar_url, u.bio,
			u.authprovider AS auth_provider, u.createdat::text AS created_at,
			0 AS enrolled_courses,
			EXISTS (
				SELECT 1 FROM user_groups ug2
				WHERE ug2.userid = u.id AND ug2.groupid = ?::uuid
			) AS direct_membership`, groupID).
		Joins("JOIN user_groups ug ON ug.userid = u.id").
		Joins("JOIN groups g ON g.id = ug.groupid").
		Where("g.id = ?::uuid OR g.path LIKE ?", groupID, group.Path+"/%").
		Order("u.username").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list recursive members: %w", err)
	}

	return rows, nil
}

// UsersInAnyGroup reports, for each user in userIDs, whether they belong
// to at least one of the groups listed in groupIDs. Users in none of them
// are absent from the result.
func (r *gormGroupRepository) UsersInAnyGroup(ctx context.Context, userIDs, groupIDs []string) (map[string]bool, error) {
	inScope := make(map[string]bool, len(userIDs))
	if len(userIDs) == 0 || len(groupIDs) == 0 {
		return inScope, nil
	}

	var ids []string

	err := r.db.WithContext(ctx).Table("user_groups").
		Distinct().
		Where("userid::text = ANY(?) AND groupid::text = ANY(?)", pq.Array(userIDs), pq.Array(groupIDs)).
		Pluck("userid::text", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("check group membership for cohort: %w", err)
	}

	for _, id := range ids {
		inScope[id] = true
	}

	return inScope, nil
}

// ListMemberIDsByGroups returns the member IDs of every group in groupIDs,
// keyed by group ID.
func (r *gormGroupRepository) ListMemberIDsByGroups(ctx context.Context, groupIDs []string) (map[string][]string, error) {
	return r.groupedByGroupID(ctx, groupIDs, "user_groups", "userid::text", "list member ids by group")
}

// groupedByGroupID reads one value column of a group-keyed table for every
// group in groupIDs, bucketed by group ID, in a single query.
//
// It backs both cohort-wide group reads — members and course enrollments —
// which differ only in the table and the column they collect.
func (r *gormGroupRepository) groupedByGroupID(
	ctx context.Context, groupIDs []string, table, valueColumn, what string,
) (map[string][]string, error) {
	byGroup := make(map[string][]string, len(groupIDs))
	if len(groupIDs) == 0 {
		return byGroup, nil
	}

	var rows []struct {
		GroupID string `gorm:"column:groupid"`
		Value   string `gorm:"column:value"`
	}

	err := r.db.WithContext(ctx).Table(table).
		Select("groupid::text AS groupid, "+valueColumn+" AS value").
		Where("groupid::text = ANY(?)", pq.Array(groupIDs)).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}

	for _, row := range rows {
		byGroup[row.GroupID] = append(byGroup[row.GroupID], row.Value)
	}

	return byGroup, nil
}
