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

// defaultGroupName is the name of the group every user is automatically
// made a member of via AddToDefault.
const defaultGroupName = "everyone"

// groupSourceLocal is the source value for groups created directly by an
// admin, as opposed to one mirrored from an IdP during SSO login.
const groupSourceLocal = "local"

// GroupMemberRow is a single member row returned by ListMembersRecursive,
// flagged with direct vs. inherited membership.
type GroupMemberRow struct {
	ID               string  `json:"id"`
	Username         string  `json:"username"`
	Email            string  `json:"email"`
	Role             string  `json:"role"`
	IsActive         bool    `json:"isActive"`
	AvatarURL        *string `json:"avatarUrl,omitempty"`
	Bio              *string `json:"bio,omitempty"`
	AuthProvider     string  `json:"authProvider"`
	CreatedAt        string  `json:"createdAt"`
	EnrolledCourses  int64   `json:"enrolledCourses"`
	DirectMembership bool    `json:"directMembership"`
}

// GroupRow is a single row of the admin group listing, joined with its
// member count and any mapped platform role.
type GroupRow struct {
	ID          string
	Name        string
	Source      string
	ParentID    string
	Depth       int
	CreatedAt   string
	MemberCount int64
	MappedRole  string
}

// GroupMapping is a single groupName -> platformRole mapping row.
type GroupMapping struct {
	GroupName    string
	PlatformRole string
}

// GroupEnrollmentRow is a single row of the admin group-enrollment listing
// for a course, joined with the group's member count.
type GroupEnrollmentRow struct {
	ID          string
	Name        string
	Source      string
	MemberCount int64
	EnrolledAt  string
}

// GroupRepository is the persistence boundary for groups, group membership,
// and IdP group/role sync.
type GroupRepository interface {
	// Admin CRUD (groups.go handlers)
	Create(ctx context.Context, name string) (string, error)
	// CreateAndJoin creates a local group named name and adds ownerID as a
	// member in a single transaction, preventing a group from existing
	// without an owner if the membership step fails.
	CreateAndJoin(ctx context.Context, name, ownerID string) (string, error)
	// CreateSubgroup creates a local group as a child of parentID, with
	// memberIDs as its first members, in one transaction.
	CreateSubgroup(ctx context.Context, name, parentID string, memberIDs ...string) (string, error)
	Delete(ctx context.Context, id string) (bool, error)
	List(ctx context.Context) ([]GroupRow, error)
	ListMappings(ctx context.Context) ([]GroupMapping, error)
	UpsertMapping(ctx context.Context, groupName, platformRole string) error
	DeleteMapping(ctx context.Context, groupName string) (bool, error)

	// Hierarchy queries (groups.go handlers)
	GetByID(ctx context.Context, id string) (*models.Group, error)
	GetSubgroups(ctx context.Context, parentID string) ([]GroupRow, error)
	GetAncestors(ctx context.Context, id string) ([]GroupRow, error)
	GetDescendants(ctx context.Context, id string) ([]GroupRow, error)
	MoveGroup(ctx context.Context, id string, newParentID *string) error
	HasChildren(ctx context.Context, id string) (bool, error)
	// HasMembers reports whether the group identified by id has members.
	HasMembers(ctx context.Context, id string) (bool, error)
	// UpdateGroup renames the local group identified by id.
	UpdateGroup(ctx context.Context, id, name string) error
	// ListMembersRecursive returns members of groupID and all subgroups,
	// flagging direct vs. inherited membership.
	ListMembersRecursive(ctx context.Context, groupID string) ([]GroupMemberRow, error)

	// Login-time sync (auth.go/oauth.go)
	AddToDefault(ctx context.Context, userID string) error
	SyncEnrollments(ctx context.Context, userID string) error
	SyncGroupsAndDeriveRole(ctx context.Context, userID string, groupNames []string, source string) (string, error)

	// Admin course-enrollment cascade (admin.go)
	EnrollCourse(ctx context.Context, groupID, courseSlug string) (int64, error)
	ListCourseEnrollments(ctx context.Context, courseSlug string) ([]GroupEnrollmentRow, error)
	UnenrollCourse(ctx context.Context, groupID, courseSlug string) error

	// Admin member management (admin.go)
	ListMembers(ctx context.Context, groupID string) ([]AdminUserRow, error)
	AddMember(ctx context.Context, groupID, userID string) error
	RemoveMember(ctx context.Context, groupID, userID string) error

	// Manager scope queries (manager.go)
	GetGroupsByUserID(ctx context.Context, userID string) ([]GroupRow, error)
	UserInAnyGroup(ctx context.Context, userID string, groupIDs []string) (bool, error)
	// UsersInAnyGroup answers UserInAnyGroup for a whole cohort in one
	// query, keyed by user ID. Filtering a listing to a manager's scope
	// needs the answer for every row, and asking per row made that
	// listing cost a query per user.
	UsersInAnyGroup(ctx context.Context, userIDs, groupIDs []string) (map[string]bool, error)
	ListMemberIDs(ctx context.Context, groupID string) ([]string, error)
	// ListMemberIDsByGroups returns the member IDs of every group in
	// groupIDs, keyed by group ID, in one query.
	ListMemberIDsByGroups(ctx context.Context, groupIDs []string) (map[string][]string, error)

	// User-facing group queries (my_groups.go)
	GetGroupCourses(ctx context.Context, groupID string) ([]string, error)
	// GetGroupCoursesByGroups returns the enrolled course slugs of every
	// group in groupIDs, keyed by group ID, in one query.
	GetGroupCoursesByGroups(ctx context.Context, groupIDs []string) (map[string][]string, error)
}

// gormGroupRepository is the GORM-backed GroupRepository implementation.
type gormGroupRepository struct {
	db *gorm.DB
}

// NewGormGroupRepository builds a GroupRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormGroupRepository(db *gorm.DB) GroupRepository {
	return &gormGroupRepository{db: db}
}

// Create inserts a new root local group named name and sets its path to its
// own ID. Does nothing if a group with that name already exists.
func (r *gormGroupRepository) Create(ctx context.Context, name string) (string, error) {
	var newID string

	err := r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		group := models.Group{Name: name, Source: groupSourceLocal}

		createErr := txDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&group).Error
		if createErr != nil {
			return fmt.Errorf("create group: %w", createErr)
		}

		if group.ID == uuid.Nil {
			return nil
		}

		group.Path = group.ID.String()

		updateErr := txDB.Model(&group).Update("path", group.Path).Error
		if updateErr != nil {
			return fmt.Errorf("set group path: %w", updateErr)
		}

		newID = group.ID.String()

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("create group: %w", err)
	}

	return newID, nil
}

// CreateAndJoin creates name as a local group and links ownerID to it in a
// single transaction. If the membership insert fails the group creation is
// rolled back, preventing a group from existing without an owner.
func (r *gormGroupRepository) CreateAndJoin(ctx context.Context, name, ownerID string) (string, error) {
	var groupID string

	err := r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		group := models.Group{Name: name, Source: groupSourceLocal}

		createErr := txDB.Create(&group).Error
		if createErr != nil {
			return fmt.Errorf("create group: %w", createErr)
		}

		if group.ID == uuid.Nil {
			return errors.New("group name already taken")
		}

		link := models.UserGroup{UserID: ownerID, GroupID: group.ID}

		addErr := txDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error
		if addErr != nil {
			return fmt.Errorf("add owner to group: %w", addErr)
		}

		groupID = group.ID.String()

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("create group and join: %w", err)
	}

	return groupID, nil
}

// Delete removes the local group identified by id, reporting whether a row
// was deleted.
func (r *gormGroupRepository) Delete(ctx context.Context, id string) (bool, error) {
	result := r.db.WithContext(ctx).
		Where("id = ?::uuid AND source = ?", id, groupSourceLocal).
		Delete(&models.Group{})
	if result.Error != nil {
		return false, fmt.Errorf("delete group: %w", result.Error)
	}

	return result.RowsAffected > 0, nil
}

// List returns every group with its member count and mapped role, ordered
// by name.
func (r *gormGroupRepository) List(ctx context.Context) ([]GroupRow, error) {
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
		Group("g.id, g.name, g.source, g.parent_id, g.depth, g.createdat, grm.platformrole").
		Order("g.depth, g.name").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return rows, nil
}

// GetByID returns the group with the given ID.
func (r *gormGroupRepository) GetByID(ctx context.Context, id string) (*models.Group, error) {
	var group models.Group

	err := r.db.WithContext(ctx).Where("id = ?::uuid", id).First(&group).Error
	if err != nil {
		return nil, fmt.Errorf("get group by id: %w", err)
	}

	return &group, nil
}

// UpdateGroup renames the local group identified by id.
func (r *gormGroupRepository) UpdateGroup(ctx context.Context, id, name string) error {
	result := r.db.WithContext(ctx).Model(&models.Group{}).
		Where("id = ?::uuid AND source = ?", id, groupSourceLocal).
		Update("name", name)
	if result.Error != nil {
		return fmt.Errorf("update group: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("group not found or not a local group")
	}

	return nil
}
