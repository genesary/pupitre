package fake

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// authProviderLocal is the AuthProvider value for locally-authenticated
// (non-SSO) seeded users.
const authProviderLocal = "local"

// UserRepository is an in-memory repository.UserRepository for tests.
//
// Cross-aggregate projections (ListForAdmin/GetForAdmin/Leaderboard) only
// have access to seeded user rows: enrollment/lesson/module-progress counts
// they'd normally join against always report as zero here.
type UserRepository struct {
	mu    sync.Mutex
	users []models.User

	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
	// CreateErr, when set, is returned by Create only, on top of Err — kept
	// separate so tests can simulate ExistsByEmailOrUsername succeeding right
	// before Create fails.
	CreateErr error
	// UpdatePasswordHashErr, when set, is returned by UpdatePasswordHash only,
	// on top of Err — kept separate so tests can simulate GetPasswordHash
	// succeeding right before UpdatePasswordHash fails.
	UpdatePasswordHashErr error
	// RefreshSSOProfileErr, when set, is returned by RefreshSSOProfile only,
	// on top of Err — kept separate so tests can simulate FindByProviderIdentity
	// succeeding right before RefreshSSOProfile fails.
	RefreshSSOProfileErr error
	// LinkProviderIdentityErr, when set, is returned by LinkProviderIdentity
	// only, on top of Err — kept separate so tests can simulate FindByEmail
	// succeeding right before LinkProviderIdentity fails.
	LinkProviderIdentityErr error
	// UpdateSSORoleErr, when set, is returned by UpdateSSORole only, on top of
	// Err — kept separate so tests can simulate the SSO upsert succeeding right
	// before the group-admin role synchronization fails.
	UpdateSSORoleErr error
}

// NewUserRepository builds a fake UserRepository seeded with users.
func NewUserRepository(seed ...models.User) *UserRepository {
	return &UserRepository{users: append([]models.User{}, seed...)}
}

// ExistsByEmailOrUsername reports whether any user has email or username.
func (f *UserRepository) ExistsByEmailOrUsername(_ context.Context, email, username string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	for _, u := range f.users {
		if u.Email == email || u.Username == username {
			return true, nil
		}
	}

	return false, nil
}

// Create appends user, assigning it an ID and timestamps if unset.
func (f *UserRepository) Create(_ context.Context, user *models.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	if f.CreateErr != nil {
		return f.CreateErr
	}

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	now := time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}

	user.UpdatedAt = now

	f.users = append(f.users, *user)

	return nil
}

// FindByEmailActive returns the active user matching email, or ErrUserNotFound.
func (f *UserRepository) FindByEmailActive(_ context.Context, email string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	for _, u := range f.users {
		if u.Email == email && u.IsActive {
			cp := u

			return &cp, nil
		}
	}

	return nil, repository.ErrUserNotFound
}

// FindByID returns the user with the given id, or ErrUserNotFound.
func (f *UserRepository) FindByID(_ context.Context, userID uuid.UUID) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	i := f.findIndexByID(userID)
	if i < 0 {
		return nil, repository.ErrUserNotFound
	}

	cp := f.users[i]

	return &cp, nil
}

// ExistsUsernameExcluding reports whether username is taken by another user.
func (f *UserRepository) ExistsUsernameExcluding(_ context.Context, username string, excludeID uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	for _, u := range f.users {
		if u.Username == username && u.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}

// UpdateProfile applies non-nil profile fields to the user with the given id.
func (f *UserRepository) UpdateProfile(_ context.Context, userID uuid.UUID, username, bio, avatarURL *string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	idx := f.findIndexByID(userID)
	if idx < 0 {
		return nil, repository.ErrUserNotFound
	}

	if username != nil {
		f.users[idx].Username = *username
	}

	if bio != nil {
		f.users[idx].Bio = bio
	}

	if avatarURL != nil {
		f.users[idx].AvatarURL = avatarURL
	}

	f.users[idx].UpdatedAt = time.Now()

	cp := f.users[idx]

	return &cp, nil
}

// GetPasswordHash returns the password hash of the user with the given id.
func (f *UserRepository) GetPasswordHash(_ context.Context, userID uuid.UUID) (*string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	i := f.findIndexByID(userID)
	if i < 0 {
		return nil, repository.ErrUserNotFound
	}

	return f.users[i].PasswordHash, nil
}

// UpdatePasswordHash sets the password hash of the user with the given id.
func (f *UserRepository) UpdatePasswordHash(_ context.Context, userID uuid.UUID, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	if f.UpdatePasswordHashErr != nil {
		return f.UpdatePasswordHashErr
	}

	idx := f.findIndexByID(userID)
	if idx < 0 {
		return repository.ErrUserNotFound
	}

	f.users[idx].PasswordHash = &hash
	f.users[idx].UpdatedAt = time.Now()

	return nil
}

// ExistsByUsername reports whether any user has the given username.
func (f *UserRepository) ExistsByUsername(_ context.Context, username string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}

	for _, u := range f.users {
		if u.Username == username {
			return true, nil
		}
	}

	return false, nil
}

// FindByProviderIdentity returns the user matching provider and its ID.
func (f *UserRepository) FindByProviderIdentity(_ context.Context, provider, providerUserID string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	for _, u := range f.users {
		if u.AuthProvider == provider && u.ProviderUserID != nil && *u.ProviderUserID == providerUserID {
			cp := u

			return &cp, nil
		}
	}

	return nil, repository.ErrUserNotFound
}

// FindByEmail returns the user matching email, or ErrUserNotFound.
func (f *UserRepository) FindByEmail(_ context.Context, email string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	for _, u := range f.users {
		if u.Email == email {
			cp := u

			return &cp, nil
		}
	}

	return nil, repository.ErrUserNotFound
}

// RefreshSSOProfile updates avatarURL and, if unset, bio for the user.
func (f *UserRepository) RefreshSSOProfile(_ context.Context, userID uuid.UUID, avatarURL, bio *string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	if f.RefreshSSOProfileErr != nil {
		return nil, f.RefreshSSOProfileErr
	}

	idx := f.findIndexByID(userID)
	if idx < 0 {
		return nil, repository.ErrUserNotFound
	}

	if avatarURL != nil {
		f.users[idx].AvatarURL = avatarURL
	}

	if f.users[idx].Bio == nil || *f.users[idx].Bio == "" {
		if bio != nil {
			f.users[idx].Bio = bio
		}
	}

	f.users[idx].UpdatedAt = time.Now()

	cp := f.users[idx]

	return &cp, nil
}

// LinkProviderIdentity sets the auth provider and identity for the user.
func (f *UserRepository) LinkProviderIdentity(
	_ context.Context, userID uuid.UUID, provider, providerUserID string, avatarURL, bio *string,
) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	if f.LinkProviderIdentityErr != nil {
		return nil, f.LinkProviderIdentityErr
	}

	idx := f.findIndexByID(userID)
	if idx < 0 {
		return nil, repository.ErrUserNotFound
	}

	f.users[idx].AuthProvider = provider
	f.users[idx].ProviderUserID = &providerUserID

	if avatarURL != nil {
		f.users[idx].AvatarURL = avatarURL
	}

	if f.users[idx].Bio == nil || *f.users[idx].Bio == "" {
		if bio != nil {
			f.users[idx].Bio = bio
		}
	}

	f.users[idx].UpdatedAt = time.Now()

	cp := f.users[idx]

	return &cp, nil
}

// SessionState returns the session fields of the user, or ErrUserNotFound.
func (f *UserRepository) SessionState(_ context.Context, userID uuid.UUID) (*repository.SessionState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	idx := f.findIndexByID(userID)
	if idx < 0 {
		return nil, repository.ErrUserNotFound
	}

	user := f.users[idx]

	return &repository.SessionState{Role: user.Role, IsActive: user.IsActive, SessionsValidAfter: user.SessionsValidAfter}, nil
}

// RevokeSessions records the revocation date of the user.
func (f *UserRepository) RevokeSessions(_ context.Context, userID uuid.UUID, validAfter time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	idx := f.findIndexByID(userID)
	if idx < 0 {
		return repository.ErrUserNotFound
	}

	f.users[idx].SessionsValidAfter = &validAfter

	return nil
}

// SetIdentityName records the identity name of the user signed in through
// provider as providerUserID.
func (f *UserRepository) SetIdentityName(_ context.Context, provider, providerUserID, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	for idx := range f.users {
		user := &f.users[idx]
		if user.AuthProvider == provider && user.ProviderUserID != nil && *user.ProviderUserID == providerUserID {
			user.IdentityName = &name
		}
	}

	return nil
}

// IdentityName returns the identity name of the user, or "".
func (f *UserRepository) IdentityName(_ context.Context, userID uuid.UUID) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return "", f.Err
	}

	idx := f.findIndexByID(userID)
	if idx < 0 {
		return "", repository.ErrUserNotFound
	}

	if f.users[idx].IdentityName == nil {
		return "", nil
	}

	return *f.users[idx].IdentityName, nil
}

// UpdateSSORole synchronizes a role derived from an SSO identity provider.
func (f *UserRepository) UpdateSSORole(_ context.Context, userID uuid.UUID, role string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	if f.UpdateSSORoleErr != nil {
		return nil, f.UpdateSSORoleErr
	}

	idx := f.findIndexByID(userID)
	if idx < 0 {
		return nil, repository.ErrUserNotFound
	}

	f.users[idx].Role = role
	f.users[idx].UpdatedAt = time.Now()
	cp := f.users[idx]

	return &cp, nil
}

// CountByRole returns the number of users with the given role.
func (f *UserRepository) CountByRole(_ context.Context, role string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return 0, f.Err
	}

	var count int64

	for _, u := range f.users {
		if u.Role == role {
			count++
		}
	}

	return count, nil
}

// ListAuthProviders returns the count of users per auth provider.
func (f *UserRepository) ListAuthProviders(_ context.Context) ([]repository.AuthProviderCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	counts := map[string]int64{}
	for _, u := range f.users {
		counts[u.AuthProvider]++
	}

	list := make([]repository.AuthProviderCount, 0, len(counts))
	for provider, count := range counts {
		list = append(list, repository.AuthProviderCount{Provider: provider, Count: count})
	}

	return list, nil
}

// ListForAdmin returns admin rows for users, optionally filtered by provider.
func (f *UserRepository) ListForAdmin(_ context.Context, provider string) ([]repository.AdminUserRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	list := make([]repository.AdminUserRow, 0, len(f.users))

	for _, u := range f.users {
		if provider != "" && u.AuthProvider != provider {
			continue
		}

		list = append(list, adminUserRowFromModel(u))
	}

	return list, nil
}

// GetForAdmin returns the admin detail row for the user with the given id.
func (f *UserRepository) GetForAdmin(_ context.Context, userID uuid.UUID) (*repository.AdminUserDetailRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	i := f.findIndexByID(userID)
	if i < 0 {
		return nil, repository.ErrUserNotFound
	}

	row := adminUserRowFromModel(f.users[i])

	return &repository.AdminUserDetailRow{
		ID: row.ID, Username: row.Username, Email: row.Email, Role: row.Role, IsActive: row.IsActive,
		AvatarURL: row.AvatarURL, Bio: row.Bio, AuthProvider: row.AuthProvider, CreatedAt: row.CreatedAt,
		EnrolledCourses: row.EnrolledCourses,
	}, nil
}

// adminUserRowFromModel converts u into its admin listing row representation.
func adminUserRowFromModel(u models.User) repository.AdminUserRow {
	return repository.AdminUserRow{
		ID: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, IsActive: u.IsActive,
		AvatarURL: u.AvatarURL, Bio: u.Bio, AuthProvider: u.AuthProvider, CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

// UpdateAdminFields applies non-nil admin-editable fields to the user.
func (f *UserRepository) UpdateAdminFields(
	_ context.Context, userID uuid.UUID, username, bio, avatarURL *string, isActive *bool, role *string,
) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	idx := f.findIndexByID(userID)
	if idx < 0 {
		return nil, repository.ErrUserNotFound
	}

	if username != nil {
		f.users[idx].Username = *username
	}

	if bio != nil {
		f.users[idx].Bio = bio
	}

	if avatarURL != nil {
		f.users[idx].AvatarURL = avatarURL
	}

	if isActive != nil {
		f.users[idx].IsActive = *isActive
	}

	if role != nil {
		f.users[idx].Role = *role
	}

	f.users[idx].UpdatedAt = time.Now()

	cp := f.users[idx]

	return &cp, nil
}

// Delete removes the user with the given id, if present.
func (f *UserRepository) Delete(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	i := f.findIndexByID(userID)
	if i < 0 {
		return nil
	}

	f.users = append(f.users[:i], f.users[i+1:]...)

	return nil
}

// Search returns users whose username or email contains query.
func (f *UserRepository) Search(_ context.Context, query string) ([]repository.UserSearchRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	q := strings.ToLower(query)

	results := make([]repository.UserSearchRow, 0)

	for _, u := range f.users {
		if strings.Contains(strings.ToLower(u.Username), q) || strings.Contains(strings.ToLower(u.Email), q) {
			results = append(results, repository.UserSearchRow{ID: u.ID.String(), Username: u.Username, Email: u.Email})
		}
	}

	return results, nil
}

// Leaderboard returns active students for the leaderboard listing.
func (f *UserRepository) Leaderboard(_ context.Context) ([]repository.LeaderboardRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	list := make([]repository.LeaderboardRow, 0)

	for _, student := range f.users {
		if student.Role != groupsRoleStudent || !student.IsActive {
			continue
		}

		list = append(list, repository.LeaderboardRow{
			ID: student.ID.String(), Username: student.Username, Email: student.Email, AvatarURL: student.AvatarURL,
		})
	}

	return list, nil
}

// ListByGroupIDs returns the admin projection for every user whose ID appears
// in any of the given group IDs. In the fake, groups are tracked through the
// GroupRepository; here we simply return all seeded users as a stub.
func (f *UserRepository) ListByGroupIDs(_ context.Context, _ []string) ([]repository.AdminUserRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	list := make([]repository.AdminUserRow, 0, len(f.users))

	for _, usr := range f.users {
		list = append(list, repository.AdminUserRow{
			ID:       usr.ID.String(),
			Username: usr.Username,
			Email:    usr.Email,
			Role:     usr.Role,
			IsActive: usr.IsActive,
		})
	}

	return list, nil
}

// CreateAdminIfAbsent seeds an admin user unless email already exists.
func (f *UserRepository) CreateAdminIfAbsent(_ context.Context, username, email, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, u := range f.users {
		if u.Email == email {
			return nil
		}
	}

	f.users = append(f.users, models.User{
		ID: uuid.New(), Username: username, Email: email, PasswordHash: &hash, Role: groupsRoleAdmin,
		IsActive: true, AuthProvider: authProviderLocal, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	return nil
}

// UpsertAdminPassword sets the admin's hash, creating the admin if absent.
func (f *UserRepository) UpsertAdminPassword(_ context.Context, username, email, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, u := range f.users {
		if u.Email == email {
			f.users[i].PasswordHash = &hash
			f.users[i].UpdatedAt = time.Now()

			return nil
		}
	}

	f.users = append(f.users, models.User{
		ID: uuid.New(), Username: username, Email: email, PasswordHash: &hash, Role: groupsRoleAdmin,
		IsActive: true, AuthProvider: authProviderLocal, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	return nil
}

// UpsertMockStudent upserts a student by email, returning its ID.
func (f *UserRepository) UpsertMockStudent(_ context.Context, username, email, hash string) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, u := range f.users {
		if u.Email == email {
			f.users[i].Username = username

			return f.users[i].ID, nil
		}
	}

	newID := uuid.New()
	f.users = append(f.users, models.User{
		ID: newID, Username: username, Email: email, PasswordHash: &hash, Role: groupsRoleStudent,
		IsActive: true, AuthProvider: authProviderLocal, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	return newID, nil
}

// TouchLastLogin updates the lastloginat field for the given user.
func (f *UserRepository) TouchLastLogin(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	now := time.Now()
	i := f.findIndexByID(userID)

	if i >= 0 {
		f.users[i].LastLoginAt = &now
	}

	return nil
}

// findIndexByID returns the index of the user with the given id, or -1.
func (f *UserRepository) findIndexByID(id uuid.UUID) int {
	for i := range f.users {
		if f.users[i].ID == id {
			return i
		}
	}

	return -1
}
