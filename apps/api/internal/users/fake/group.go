package fake

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

const (
	// groupsRoleAdmin is the platform role derived from a group mapped to "admin".
	groupsRoleAdmin = "admin"
	// groupsRoleStudent is the default platform role when no mapping applies.
	groupsRoleStudent = "student"
)

// GroupRepository is an in-memory repository.GroupRepository for tests.
//
// UserRoles seeds each user's current platform role, read/written by
// SyncGroupsAndDeriveRole the same way the real repository reads/writes
// users.role. AddToDefault and SyncEnrollments are no-ops beyond error
// injection: no test exercises their cross-aggregate business logic, since
// the real callers (auth.go/oauth.go) already treat them as fire-and-forget.
type GroupRepository struct {
	mu sync.Mutex

	Groups           []models.Group
	UserGroup        []models.UserGroup
	Mappings         []models.GroupRoleMapping
	GroupEnrollments []models.GroupEnrollment
	UserRoles        map[string]string

	// Err, when set, is returned by every method call (simulates a DB failure).
	// For SyncGroupsAndDeriveRole, it is checked before any state is mutated,
	// mirroring the real repository's transactional rollback: a failed sync
	// leaves the user's prior groups/role untouched.
	Err error
}

// NewGroupRepository builds a fake GroupRepository seeded with groups.
func NewGroupRepository(seed ...models.Group) *GroupRepository {
	return &GroupRepository{
		Groups:    append([]models.Group{}, seed...),
		UserRoles: make(map[string]string),
	}
}

// Create adds a new local group named name, returning its generated ID.
func (f *GroupRepository) Create(_ context.Context, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return "", f.Err
	}

	for _, g := range f.Groups {
		if g.Name == name {
			return "", nil
		}
	}

	id := uuid.New()
	f.Groups = append(f.Groups, models.Group{ID: id, Name: name, Source: authProviderLocal})

	return id.String(), nil
}

// CreateAndJoin adds a new local group named name and links ownerID to it
// atomically. Returns the new group's ID, or empty string if the name is taken.
func (f *GroupRepository) CreateAndJoin(_ context.Context, name, ownerID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return "", f.Err
	}

	for _, grp := range f.Groups {
		if grp.Name == name {
			return "", errors.New("create group and join: group name already taken")
		}
	}

	id := uuid.New()
	f.Groups = append(f.Groups, models.Group{ID: id, Name: name, Source: authProviderLocal})
	f.UserGroup = append(f.UserGroup, models.UserGroup{UserID: ownerID, GroupID: id})

	return id.String(), nil
}

// Delete removes the local group with groupID, a no-op for non-local groups.
func (f *GroupRepository) Delete(_ context.Context, groupID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	for i, g := range f.Groups {
		if g.ID.String() == groupID && g.Source == authProviderLocal {
			f.Groups = append(f.Groups[:i], f.Groups[i+1:]...)

			return true, nil
		}
	}

	return false, nil
}

// List returns every group along with its member count and mapped role.
func (f *GroupRepository) List(_ context.Context) ([]repository.GroupRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	rows := make([]repository.GroupRow, 0, len(f.Groups))

	for _, group := range f.Groups {
		memberCount := int64(0)

		for _, ug := range f.UserGroup {
			if ug.GroupID == group.ID {
				memberCount++
			}
		}

		mappedRole := ""

		for _, m := range f.Mappings {
			if m.GroupName == group.Name {
				mappedRole = m.PlatformRole

				break
			}
		}

		rows = append(rows, repository.GroupRow{
			ID: group.ID.String(), Name: group.Name, Source: group.Source,
			CreatedAt: group.CreatedAt.String(), MemberCount: memberCount, MappedRole: mappedRole,
		})
	}

	return rows, nil
}

// ListMappings returns every group-to-role mapping.
func (f *GroupRepository) ListMappings(_ context.Context) ([]repository.GroupMapping, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	rows := make([]repository.GroupMapping, 0, len(f.Mappings))
	for _, m := range f.Mappings {
		rows = append(rows, repository.GroupMapping{GroupName: m.GroupName, PlatformRole: m.PlatformRole})
	}

	return rows, nil
}

// UpsertMapping creates or updates the role mapping for groupName.
func (f *GroupRepository) UpsertMapping(_ context.Context, groupName, platformRole string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for i, m := range f.Mappings {
		if m.GroupName == groupName {
			f.Mappings[i].PlatformRole = platformRole

			return nil
		}
	}

	f.Mappings = append(f.Mappings, models.GroupRoleMapping{GroupName: groupName, PlatformRole: platformRole})

	return nil
}

// DeleteMapping removes the role mapping for groupName, if present.
func (f *GroupRepository) DeleteMapping(_ context.Context, groupName string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	for i, m := range f.Mappings {
		if m.GroupName == groupName {
			f.Mappings = append(f.Mappings[:i], f.Mappings[i+1:]...)

			return true, nil
		}
	}

	return false, nil
}

// AddToDefault is a no-op in the fake; it exists to satisfy the interface.
func (f *GroupRepository) AddToDefault(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.Err
}

// SyncEnrollments is a no-op in the fake; it exists to satisfy the interface.
func (f *GroupRepository) SyncEnrollments(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.Err
}

// SyncGroupsAndDeriveRole replaces userID's group memberships with groupNames
// and derives the platform role along the way.
func (f *GroupRepository) SyncGroupsAndDeriveRole(
	_ context.Context, userID string, groupNames []string, source string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	role := f.UserRoles[userID]
	if role == "" {
		role = groupsRoleStudent
	}

	if f.Err != nil {
		return role, f.Err
	}

	userGroups := make([]models.UserGroup, 0, len(f.UserGroup))

	for _, ug := range f.UserGroup {
		if ug.UserID != userID {
			userGroups = append(userGroups, ug)
		}
	}

	roleMapped := false

	for _, name := range groupNames {
		if name == "" {
			continue
		}

		var mapped bool

		userGroups, role, mapped = f.applyGroupMembership(userGroups, userID, name, source, role)
		if mapped {
			roleMapped = true
		}
	}

	f.UserGroup = userGroups
	if roleMapped {
		f.UserRoles[userID] = role
	}

	return role, nil
}

// EnrollCourse enrolls the group in courseSlug and returns the count of
// members backfilled into it.
func (f *GroupRepository) EnrollCourse(_ context.Context, groupID, courseSlug string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return 0, f.Err
	}

	gid, err := uuid.Parse(groupID)
	if err != nil {
		return 0, nil
	}

	found := false

	for _, ge := range f.GroupEnrollments {
		if ge.GroupID == gid && ge.CourseSlug == courseSlug {
			found = true

			break
		}
	}

	if !found {
		f.GroupEnrollments = append(f.GroupEnrollments, models.GroupEnrollment{GroupID: gid, CourseSlug: courseSlug})
	}

	var backfilled int64

	for _, ug := range f.UserGroup {
		if ug.GroupID != gid {
			continue
		}

		backfilled++
	}

	return backfilled, nil
}

// ListCourseEnrollments reports each enrolled group's member count from
// UserGroup, matching the real repository's LEFT JOIN against user_groups.
func (f *GroupRepository) ListCourseEnrollments(_ context.Context, courseSlug string) ([]repository.GroupEnrollmentRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	rows := make([]repository.GroupEnrollmentRow, 0)

	for _, enrollment := range f.GroupEnrollments {
		if enrollment.CourseSlug != courseSlug {
			continue
		}

		var group models.Group

		for _, g := range f.Groups {
			if g.ID == enrollment.GroupID {
				group = g

				break
			}
		}

		memberCount := int64(0)

		for _, ug := range f.UserGroup {
			if ug.GroupID == enrollment.GroupID {
				memberCount++
			}
		}

		rows = append(rows, repository.GroupEnrollmentRow{
			ID: group.ID.String(), Name: group.Name, Source: group.Source, MemberCount: memberCount,
		})
	}

	return rows, nil
}

// UnenrollCourse removes the group's enrollment in courseSlug, if present.
func (f *GroupRepository) UnenrollCourse(_ context.Context, groupID, courseSlug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for i, ge := range f.GroupEnrollments {
		if ge.GroupID.String() == groupID && ge.CourseSlug == courseSlug {
			f.GroupEnrollments = append(f.GroupEnrollments[:i], f.GroupEnrollments[i+1:]...)

			break
		}
	}

	return nil
}

// ListMembers returns every user belonging to the group identified by groupID.
// In the fake, user details are not available (cross-aggregate join); an empty
// list is returned to satisfy the interface.
func (f *GroupRepository) ListMembers(_ context.Context, groupID string) ([]repository.AdminUserRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	gid, err := uuid.Parse(groupID)
	if err != nil {
		return nil, nil
	}

	var rows []repository.AdminUserRow

	for _, ug := range f.UserGroup {
		if ug.GroupID == gid {
			rows = append(rows, repository.AdminUserRow{ID: ug.UserID})
		}
	}

	return rows, nil
}

// AddMember links userID to the group identified by groupID.
func (f *GroupRepository) AddMember(_ context.Context, groupID, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	gid, err := uuid.Parse(groupID)
	if err != nil {
		return fmt.Errorf("parse group uuid: %w", err)
	}

	for _, member := range f.UserGroup {
		if member.GroupID == gid && member.UserID == userID {
			return nil
		}
	}

	f.UserGroup = append(f.UserGroup, models.UserGroup{UserID: userID, GroupID: gid})

	return nil
}

// RemoveMember unlinks userID from the group identified by groupID.
func (f *GroupRepository) RemoveMember(_ context.Context, groupID, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for i, ug := range f.UserGroup {
		if ug.GroupID.String() == groupID && ug.UserID == userID {
			f.UserGroup = append(f.UserGroup[:i], f.UserGroup[i+1:]...)

			return nil
		}
	}

	return nil
}

// GetGroupsByUserID returns all groups the given user belongs to.
func (f *GroupRepository) GetGroupsByUserID(_ context.Context, userID string) ([]repository.GroupRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	var rows []repository.GroupRow

	for _, membership := range f.UserGroup {
		if membership.UserID != userID {
			continue
		}

		for _, grp := range f.Groups {
			if grp.ID != membership.GroupID {
				continue
			}

			memberCount := int64(0)

			for _, m := range f.UserGroup {
				if m.GroupID == grp.ID {
					memberCount++
				}
			}

			mappedRole := ""

			for _, mapping := range f.Mappings {
				if mapping.GroupName == grp.Name {
					mappedRole = mapping.PlatformRole

					break
				}
			}

			rows = append(rows, repository.GroupRow{
				ID:          grp.ID.String(),
				Name:        grp.Name,
				Source:      grp.Source,
				MemberCount: memberCount,
				MappedRole:  mappedRole,
			})

			break
		}
	}

	return rows, nil
}

// UserInAnyGroup reports whether userID belongs to at least one group in
// groupIDs.
func (f *GroupRepository) UserInAnyGroup(_ context.Context, userID string, groupIDs []string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	idSet := make(map[string]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		idSet[id] = struct{}{}
	}

	for _, ug := range f.UserGroup {
		if ug.UserID == userID {
			if _, ok := idSet[ug.GroupID.String()]; ok {
				return true, nil
			}
		}
	}

	return false, nil
}

// GetGroupCourses returns the course slugs enrolled by groupID.
func (f *GroupRepository) GetGroupCourses(_ context.Context, groupID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	var slugs []string

	for _, enrollment := range f.GroupEnrollments {
		if enrollment.GroupID.String() == groupID {
			slugs = append(slugs, enrollment.CourseSlug)
		}
	}

	return slugs, nil
}

// ListMemberIDs returns the user IDs of every member of groupID.
func (f *GroupRepository) ListMemberIDs(_ context.Context, groupID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	var ids []string

	for _, membership := range f.UserGroup {
		if membership.GroupID.String() == groupID {
			ids = append(ids, membership.UserID)
		}
	}

	return ids, nil
}

// CreateSubgroup adds a child group under parentID in the fake store.
func (f *GroupRepository) CreateSubgroup(_ context.Context, name, parentID string, memberIDs ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return "", f.Err
	}

	parentUUID, err := uuid.Parse(parentID)
	if err != nil {
		return "", fmt.Errorf("parse parent id: %w", err)
	}

	groupID := uuid.New()
	f.Groups = append(f.Groups, models.Group{
		ID:       groupID,
		Name:     name,
		Source:   authProviderLocal,
		ParentID: &parentUUID,
	})

	for _, memberID := range memberIDs {
		f.UserGroup = append(f.UserGroup, models.UserGroup{UserID: memberID, GroupID: groupID})
	}

	return groupID.String(), nil
}

// GetByID returns the group with the given ID.
func (f *GroupRepository) GetByID(_ context.Context, groupID string) (*models.Group, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	for i, g := range f.Groups {
		if g.ID.String() == groupID {
			return &f.Groups[i], nil
		}
	}

	return nil, errors.New("group not found")
}

// GetSubgroups returns direct children of parentID.
func (f *GroupRepository) GetSubgroups(_ context.Context, parentID string) ([]repository.GroupRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	var rows []repository.GroupRow

	for _, g := range f.Groups {
		if g.ParentID != nil && g.ParentID.String() == parentID {
			rows = append(rows, repository.GroupRow{ID: g.ID.String(), Name: g.Name, Source: g.Source})
		}
	}

	return rows, nil
}

// GetAncestors returns an empty list in the fake (path traversal is a no-op).
func (f *GroupRepository) GetAncestors(_ context.Context, _ string) ([]repository.GroupRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return []repository.GroupRow{}, f.Err
}

// GetDescendants returns an empty list in the fake (path traversal is a no-op).
func (f *GroupRepository) GetDescendants(_ context.Context, _ string) ([]repository.GroupRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return []repository.GroupRow{}, f.Err
}

// MoveGroup is a no-op in the fake.
func (f *GroupRepository) MoveGroup(_ context.Context, _ string, _ *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.Err
}

// HasMembers reports whether the group has any user members.
func (f *GroupRepository) HasMembers(_ context.Context, groupID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	gid, parseErr := uuid.Parse(groupID)
	if parseErr != nil {
		return false, nil
	}

	for _, ug := range f.UserGroup {
		if ug.GroupID == gid {
			return true, nil
		}
	}

	return false, nil
}

// UpdateGroup renames the local group identified by groupID.
func (f *GroupRepository) UpdateGroup(_ context.Context, groupID, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for i, g := range f.Groups {
		if g.ID.String() == groupID && g.Source == authProviderLocal {
			f.Groups[i].Name = name

			return nil
		}
	}

	return errors.New("group not found or not a local group")
}

// ListMembersRecursive returns an empty list in the fake (path
// traversal is a no-op without a real database).
func (f *GroupRepository) ListMembersRecursive(_ context.Context, _ string) ([]repository.GroupMemberRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	return []repository.GroupMemberRow{}, nil
}

// HasChildren reports whether the group has any direct children.
func (f *GroupRepository) HasChildren(_ context.Context, groupID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	for _, g := range f.Groups {
		if g.ParentID != nil && g.ParentID.String() == groupID {
			return true, nil
		}
	}

	return false, nil
}

// UsersInAnyGroup reports which of userIDs belong to at least one group in
// groupIDs.
func (f *GroupRepository) UsersInAnyGroup(_ context.Context, userIDs, groupIDs []string) (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	wantedUser := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		wantedUser[id] = struct{}{}
	}

	wantedGroup := make(map[string]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		wantedGroup[id] = struct{}{}
	}

	inScope := make(map[string]bool, len(userIDs))

	for _, ug := range f.UserGroup {
		_, wantUser := wantedUser[ug.UserID]
		_, wantGroup := wantedGroup[ug.GroupID.String()]

		if wantUser && wantGroup {
			inScope[ug.UserID] = true
		}
	}

	return inScope, nil
}

// ListMemberIDsByGroups returns the member IDs of every group in groupIDs,
// keyed by group ID.
func (f *GroupRepository) ListMemberIDsByGroups(_ context.Context, groupIDs []string) (map[string][]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	return groupBy(groupIDs, f.UserGroup,
		func(row models.UserGroup) (string, string) { return row.GroupID.String(), row.UserID }), nil
}

// GetGroupCoursesByGroups returns the course slugs each group in groupIDs
// is enrolled in, keyed by group ID.
func (f *GroupRepository) GetGroupCoursesByGroups(_ context.Context, groupIDs []string) (map[string][]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	return groupBy(groupIDs, f.GroupEnrollments,
		func(row models.GroupEnrollment) (string, string) { return row.GroupID.String(), row.CourseSlug }), nil
}

// groupBy buckets the values of rows under their group key, keeping only
// the keys named in wanted.
func groupBy[R any](wanted []string, rows []R, split func(R) (string, string)) map[string][]string {
	keep := make(map[string]struct{}, len(wanted))
	for _, key := range wanted {
		keep[key] = struct{}{}
	}

	out := make(map[string][]string, len(wanted))

	for _, row := range rows {
		key, value := split(row)
		if _, ok := keep[key]; ok {
			out[key] = append(out[key], value)
		}
	}

	return out
}

// applyGroupMembership adds userID to the group named name (creating it if
// absent) and returns the updated userGroups slice, the (possibly updated)
// role, and whether a group-role mapping applied.
func (f *GroupRepository) applyGroupMembership(
	userGroups []models.UserGroup, userID, name, source, role string,
) ([]models.UserGroup, string, bool) {
	groupID := f.findOrCreateGroup(name, source)
	userGroups = append(userGroups, models.UserGroup{UserID: userID, GroupID: groupID})

	for _, mapping := range f.Mappings {
		if mapping.GroupName != name {
			continue
		}

		if mapping.PlatformRole == groupsRoleAdmin {
			role = groupsRoleAdmin
		} else if role != groupsRoleAdmin {
			role = mapping.PlatformRole
		}

		return userGroups, role, true
	}

	return userGroups, role, false
}

// findOrCreateGroup returns the ID of the group named name, upserting it
// (and its source) if it doesn't already exist.
func (f *GroupRepository) findOrCreateGroup(name, source string) uuid.UUID {
	for i, g := range f.Groups {
		if g.Name == name {
			f.Groups[i].Source = source

			return g.ID
		}
	}

	id := uuid.New()
	f.Groups = append(f.Groups, models.Group{ID: id, Name: name, Source: source})

	return id
}
