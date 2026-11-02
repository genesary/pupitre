package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// ErrUserNotFound is returned by lookups that find no matching user.
var ErrUserNotFound = errors.New("user not found")

// searchUsersLimit caps the number of rows returned by Search.
const searchUsersLimit = 10

// Column-map keys shared by the profile-update Updates() calls below.
const (
	colAvatarURL = "avatarurl"
	colBio       = "bio"
)

// AuthProviderCount is a single row of the admin auth-provider breakdown.
type AuthProviderCount struct {
	Provider string
	Count    int64
}

// AdminUserRow is a single row of the admin user listing, including the
// enrollment count computed via a LEFT JOIN against enrollments.
type AdminUserRow struct {
	ID              string  `json:"id"`
	Username        string  `json:"username"`
	Email           string  `json:"email"`
	Role            string  `json:"role"`
	IsActive        bool    `json:"isActive"`
	AvatarURL       *string `json:"avatarUrl,omitempty"`
	Bio             *string `json:"bio,omitempty"`
	AuthProvider    string  `json:"authProvider"`
	CreatedAt       string  `json:"createdAt"`
	EnrolledCourses int64   `json:"enrolledCourses"`
}

// AdminUserDetailRow is the admin single-user detail projection, including
// enrollment and viewed-lesson counts.
type AdminUserDetailRow struct {
	ID              string
	Username        string
	Email           string
	Role            string
	IsActive        bool
	AvatarURL       *string
	Bio             *string
	AuthProvider    string
	CreatedAt       string
	EnrolledCourses int64
	ViewedLessons   int64
}

// UserSearchRow is a single row returned by Search.
type UserSearchRow struct {
	ID       string
	Username string
	Email    string
}

// LeaderboardRow is a single row of the admin leaderboard, ranked by total
// quiz score.
type LeaderboardRow struct {
	ID              string
	Username        string
	Email           string
	AvatarURL       *string
	TotalScore      int64
	PassedModules   int64
	EnrolledCourses int64
}

// UserRepository is the persistence boundary for the users table, covering
// registration/login, profile self-service, SSO provisioning, and admin
// user management.
type UserRepository interface {
	// Registration / login
	ExistsByEmailOrUsername(ctx context.Context, email, username string) (bool, error)
	Create(ctx context.Context, u *models.User) error
	FindByEmailActive(ctx context.Context, email string) (*models.User, error)
	TouchLastLogin(ctx context.Context, id uuid.UUID) error

	// Profile self-service
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	ExistsUsernameExcluding(ctx context.Context, username string, excludeID uuid.UUID) (bool, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, username, bio, avatarURL *string) (*models.User, error)
	GetPasswordHash(ctx context.Context, id uuid.UUID) (*string, error)
	UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error

	// SSO provisioning (oauth.go)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	FindByProviderIdentity(ctx context.Context, provider, providerUserID string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	RefreshSSOProfile(ctx context.Context, id uuid.UUID, avatarURL, bio *string) (*models.User, error)
	LinkProviderIdentity(ctx context.Context, id uuid.UUID, provider, providerUserID string, avatarURL, bio *string) (*models.User, error)
	UpdateSSORole(ctx context.Context, id uuid.UUID, role string) (*models.User, error)

	// Sessions (docs/specs/190-backend-defects.md B7)
	SessionState(ctx context.Context, id uuid.UUID) (*SessionState, error)
	RevokeSessions(ctx context.Context, id uuid.UUID, validAfter time.Time) error

	// Identity given by the identity provider (docs/specs/190-backend-defects.md B11)
	SetIdentityName(ctx context.Context, provider, providerUserID, name string) error
	IdentityName(ctx context.Context, id uuid.UUID) (string, error)

	// Admin
	CountByRole(ctx context.Context, role string) (int64, error)
	ListAuthProviders(ctx context.Context) ([]AuthProviderCount, error)
	ListForAdmin(ctx context.Context, provider string) ([]AdminUserRow, error)
	GetForAdmin(ctx context.Context, id uuid.UUID) (*AdminUserDetailRow, error)
	UpdateAdminFields(ctx context.Context, id uuid.UUID, username, bio, avatarURL *string, isActive *bool, role *string) (*models.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Search(ctx context.Context, query string) ([]UserSearchRow, error)
	Leaderboard(ctx context.Context) ([]LeaderboardRow, error)
	ListByGroupIDs(ctx context.Context, groupIDs []string) ([]AdminUserRow, error)

	// Seeding
	CreateAdminIfAbsent(ctx context.Context, username, email, hash string) error
	UpsertAdminPassword(ctx context.Context, username, email, hash string) error
	UpsertMockStudent(ctx context.Context, username, email, hash string) (uuid.UUID, error)
}

// gormUserRepository is the GORM-backed UserRepository implementation.
type gormUserRepository struct {
	db *gorm.DB
}

// NewGormUserRepository builds a UserRepository backed by db.
//
//nolint:ireturn // repository constructors return the interface type by design
func NewGormUserRepository(db *gorm.DB) UserRepository {
	return &gormUserRepository{db: db}
}

// ExistsByEmailOrUsername reports whether a user with email or username
// already exists.
func (r *gormUserRepository) ExistsByEmailOrUsername(ctx context.Context, email, username string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("email = ? OR username = ?", email, username).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check existing user: %w", err)
	}

	return count > 0, nil
}

// Create inserts a new user row.
func (r *gormUserRepository) Create(ctx context.Context, u *models.User) error {
	err := r.db.WithContext(ctx).Create(u).Error
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

// FindByEmailActive returns the active user with the given email, or
// ErrUserNotFound.
func (r *gormUserRepository) FindByEmailActive(ctx context.Context, email string) (*models.User, error) {
	var user models.User

	err := r.db.WithContext(ctx).
		Where("email = ? AND isactive = TRUE", email).
		First(&user).Error
	if err != nil {
		return nil, wrapNotFound(err, ErrUserNotFound)
	}

	return &user, nil
}

// FindByID returns the user identified by id, or ErrUserNotFound.
func (r *gormUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User

	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if err != nil {
		return nil, wrapNotFound(err, ErrUserNotFound)
	}

	return &user, nil
}

// ExistsUsernameExcluding reports whether username is taken by a user other
// than excludeID.
func (r *gormUserRepository) ExistsUsernameExcluding(ctx context.Context, username string, excludeID uuid.UUID) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("username = ? AND id != ?", username, excludeID).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check username taken: %w", err)
	}

	return count > 0, nil
}

// UpdateProfile updates the non-nil fields of the user identified by id and
// returns the updated row.
func (r *gormUserRepository) UpdateProfile(
	ctx context.Context, userID uuid.UUID, username, bio, avatarURL *string,
) (*models.User, error) {
	updates := map[string]any{}
	if username != nil {
		updates["username"] = *username
	}

	if bio != nil {
		updates[colBio] = *bio
	}

	if avatarURL != nil {
		updates[colAvatarURL] = *avatarURL
	}

	var user models.User

	err := r.db.WithContext(ctx).Model(&user).Clauses(returningAll).
		Where("id = ?", userID).Updates(updates).Error
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	return &user, nil
}

// GetPasswordHash returns the password hash for the user identified by id.
func (r *gormUserRepository) GetPasswordHash(ctx context.Context, id uuid.UUID) (*string, error) {
	var user models.User

	err := r.db.WithContext(ctx).Select("password_hash").First(&user, "id = ?", id).Error
	if err != nil {
		return nil, wrapNotFound(err, ErrUserNotFound)
	}

	return user.PasswordHash, nil
}

// UpdatePasswordHash sets the password hash for the user identified by id.
func (r *gormUserRepository) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).Update("password_hash", hash).Error
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}

	return nil
}

// ExistsByUsername reports whether a user with username already exists.
func (r *gormUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("username = ?", username).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check username exists: %w", err)
	}

	return count > 0, nil
}

// FindByProviderIdentity returns the user linked to the given SSO provider
// and provider user ID, or ErrUserNotFound.
func (r *gormUserRepository) FindByProviderIdentity(ctx context.Context, provider, providerUserID string) (*models.User, error) {
	var user models.User

	err := r.db.WithContext(ctx).
		Where("authprovider = ? AND provider_user_id = ?", provider, providerUserID).
		First(&user).Error
	if err != nil {
		return nil, wrapNotFound(err, ErrUserNotFound)
	}

	return &user, nil
}

// FindByEmail returns the user with the given email, or ErrUserNotFound.
func (r *gormUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User

	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, wrapNotFound(err, ErrUserNotFound)
	}

	return &user, nil
}

// bioIfBlank is the raw SQL fragment mirroring the original handler's
// CASE-WHEN: only overwrite bio when it is currently NULL/empty.
const bioIfBlank = "CASE WHEN (bio IS NULL OR bio = '') THEN ? ELSE bio END"

// RefreshSSOProfile updates avatar/bio from the SSO provider, only
// overwriting bio when it is currently blank, and returns the updated row.
func (r *gormUserRepository) RefreshSSOProfile(ctx context.Context, id uuid.UUID, avatarURL, bio *string) (*models.User, error) {
	var user models.User

	err := r.db.WithContext(ctx).Model(&user).Clauses(returningAll).
		Where("id = ?", id).
		Updates(map[string]any{
			colAvatarURL: gorm.Expr("COALESCE(?, avatarurl)", avatarURL),
			colBio:       gorm.Expr(bioIfBlank, bio),
		}).Error
	if err != nil {
		return nil, fmt.Errorf("refresh sso profile: %w", err)
	}

	return &user, nil
}

// LinkProviderIdentity attaches an SSO provider identity to the user
// identified by id and refreshes avatar/bio, returning the updated row.
func (r *gormUserRepository) LinkProviderIdentity(
	ctx context.Context, id uuid.UUID, provider, providerUserID string, avatarURL, bio *string,
) (*models.User, error) {
	var user models.User

	err := r.db.WithContext(ctx).Model(&user).Clauses(returningAll).
		Where("id = ?", id).
		Updates(map[string]any{
			"authprovider":     provider,
			"provider_user_id": providerUserID,
			colAvatarURL:       gorm.Expr("COALESCE(?, avatarurl)", avatarURL),
			colBio:             gorm.Expr(bioIfBlank, bio),
		}).Error
	if err != nil {
		return nil, fmt.Errorf("link provider identity: %w", err)
	}

	return &user, nil
}

// CountByRole returns the number of users with the given role.
func (r *gormUserRepository) CountByRole(ctx context.Context, role string) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).Model(&models.User{}).Where("role = ?", role).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count users by role: %w", err)
	}

	return count, nil
}

// ListAuthProviders returns the number of users per auth provider.
func (r *gormUserRepository) ListAuthProviders(ctx context.Context) ([]AuthProviderCount, error) {
	var list []AuthProviderCount

	err := r.db.WithContext(ctx).Model(&models.User{}).
		Select("authprovider AS provider, COUNT(*)::bigint AS count").
		Group("authprovider").
		Order("authprovider").
		Scan(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list auth providers: %w", err)
	}

	return list, nil
}

// ListForAdmin returns every user for the admin listing, optionally filtered
// by auth provider, most recently created first.
func (r *gormUserRepository) ListForAdmin(ctx context.Context, provider string) ([]AdminUserRow, error) {
	var list []AdminUserRow

	query := r.db.WithContext(ctx).Table("users AS u").
		Select(`u.id::text AS id, u.username, u.email, u.role, u.isactive AS is_active,
			u.avatarurl AS avatar_url, u.bio, u.authprovider AS auth_provider, u.createdat::text AS created_at,
			COUNT(DISTINCT e.courseslug)::bigint AS enrolled_courses`).
		Joins("LEFT JOIN enrollments e ON e.userid = u.id")

	if provider != "" {
		query = query.Where("u.authprovider = ?", provider)
	}

	err := query.Group("u.id").Order("u.createdat DESC").Scan(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list admin users: %w", err)
	}

	return list, nil
}

// ListByGroupIDs returns the admin projection for every user belonging to at
// least one of the given group IDs (as UUID strings), ordered by username.
func (r *gormUserRepository) ListByGroupIDs(ctx context.Context, groupIDs []string) ([]AdminUserRow, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}

	var list []AdminUserRow

	err := r.db.WithContext(ctx).Table("users AS u").
		Select(`u.id::text AS id, u.username, u.email, u.role, u.isactive AS is_active,
			u.avatarurl AS avatar_url, u.bio, u.authprovider AS auth_provider, u.createdat::text AS created_at,
			COUNT(DISTINCT e.courseslug)::bigint AS enrolled_courses`).
		Joins("JOIN user_groups ug ON ug.userid = u.id").
		Joins("LEFT JOIN enrollments e ON e.userid = u.id").
		Where("ug.groupid::text = ANY(?)", pq.Array(groupIDs)).
		Group("u.id").
		Order("u.username").
		Scan(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list users by group ids: %w", err)
	}

	return list, nil
}

// GetForAdmin returns the admin detail projection for the user identified
// by id.
func (r *gormUserRepository) GetForAdmin(ctx context.Context, userID uuid.UUID) (*AdminUserDetailRow, error) {
	var row AdminUserDetailRow

	err := r.db.WithContext(ctx).Table("users AS u").
		Select(`u.id::text AS id, u.username, u.email, u.role, u.isactive AS is_active,
			u.avatarurl AS avatar_url, u.bio, u.authprovider AS auth_provider, u.createdat::text AS created_at,
			COUNT(DISTINCT e.courseslug)::bigint AS enrolled_courses,
			COUNT(DISTINCT lp.lessonslug)::bigint AS viewed_lessons`).
		Joins("LEFT JOIN enrollments e ON e.userid = u.id").
		Joins("LEFT JOIN lesson_progress lp ON lp.userid = u.id").
		Where("u.id = ?", userID).
		Group("u.id").
		Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("get admin user: %w", err)
	}

	if row.ID == "" {
		return nil, ErrUserNotFound
	}

	return &row, nil
}

// UpdateSSORole synchronizes a role derived from an SSO identity provider.
func (r *gormUserRepository) UpdateSSORole(ctx context.Context, userID uuid.UUID, role string) (*models.User, error) {
	var user models.User

	err := r.db.WithContext(ctx).Model(&user).Clauses(returningAll).
		Where("id = ?", userID).Update("role", role).Error
	if err != nil {
		return nil, fmt.Errorf("update SSO role: %w", err)
	}

	if user.ID == uuid.Nil {
		return nil, ErrUserNotFound
	}

	return &user, nil
}

// SessionState is what the per-request session check reads of a user.
type SessionState struct {
	Role               string     `gorm:"column:role"`
	IsActive           bool       `gorm:"column:isactive"`
	SessionsValidAfter *time.Time `gorm:"column:sessionsvalidafter"`
}

// SessionState returns the role, the activity and the revocation date of
// the user identified by id, or ErrUserNotFound. One primary-key read.
func (r *gormUserRepository) SessionState(ctx context.Context, userID uuid.UUID) (*SessionState, error) {
	var rows []SessionState

	err := r.db.WithContext(ctx).Model(&models.User{}).
		Select("role", "isactive", "sessionsvalidafter").
		Where("id = ?", userID).Limit(1).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load session state: %w", err)
	}

	if len(rows) == 0 {
		return nil, ErrUserNotFound
	}

	return &rows[0], nil
}

// RevokeSessions invalidates every token of the user issued until validAfter.
func (r *gormUserRepository) RevokeSessions(ctx context.Context, userID uuid.UUID, validAfter time.Time) error {
	err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).Update("sessionsvalidafter", validAfter).Error
	if err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}

	return nil
}

// SetIdentityName records the login the identity provider gave the user
// signed in through provider as providerUserID.
func (r *gormUserRepository) SetIdentityName(ctx context.Context, provider, providerUserID, name string) error {
	err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("authprovider = ? AND provider_user_id = ?", provider, providerUserID).
		Update("identityname", name).Error
	if err != nil {
		return fmt.Errorf("set identity name: %w", err)
	}

	return nil
}

// IdentityName returns the login the identity provider gave the user, or
// "" when it gave none; ErrUserNotFound for an unknown user.
func (r *gormUserRepository) IdentityName(ctx context.Context, userID uuid.UUID) (string, error) {
	var rows []struct {
		IdentityName *string `gorm:"column:identityname"`
	}

	err := r.db.WithContext(ctx).Model(&models.User{}).
		Select("identityname").Where("id = ?", userID).Limit(1).Scan(&rows).Error
	if err != nil {
		return "", fmt.Errorf("load identity name: %w", err)
	}

	if len(rows) == 0 {
		return "", ErrUserNotFound
	}

	if rows[0].IdentityName == nil {
		return "", nil
	}

	return *rows[0].IdentityName, nil
}

// UpdateAdminFields updates the non-nil fields of the user identified by id
// and returns the updated row, or ErrUserNotFound if no row matched.
func (r *gormUserRepository) UpdateAdminFields(
	ctx context.Context, userID uuid.UUID, username, bio, avatarURL *string, isActive *bool, role *string,
) (*models.User, error) {
	updates := map[string]any{}
	if username != nil {
		updates["username"] = *username
	}

	if bio != nil {
		updates[colBio] = *bio
	}

	if avatarURL != nil {
		updates[colAvatarURL] = *avatarURL
	}

	if isActive != nil {
		updates["isactive"] = *isActive
	}

	if role != nil {
		updates["role"] = *role
	}

	var user models.User

	err := r.db.WithContext(ctx).Model(&user).Clauses(returningAll).
		Where("id = ?", userID).Updates(updates).Error
	if err != nil {
		return nil, fmt.Errorf("update admin fields: %w", err)
	}

	if user.ID == uuid.Nil {
		return nil, ErrUserNotFound
	}

	return &user, nil
}

// Delete removes the user identified by id.
func (r *gormUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.db.WithContext(ctx).Delete(&models.User{}, "id = ?", id).Error
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

// TouchLastLogin updates the lastloginat timestamp for the given user to NOW().
func (r *gormUserRepository) TouchLastLogin(ctx context.Context, id uuid.UUID) error {
	err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).
		Update("lastloginat", gorm.Expr("NOW()")).Error
	if err != nil {
		return fmt.Errorf("touch last login: %w", err)
	}

	return nil
}

// Search returns users whose username or email matches query, up to
// searchUsersLimit rows.
func (r *gormUserRepository) Search(ctx context.Context, query string) ([]UserSearchRow, error) {
	pattern := "%" + query + "%"

	var results []UserSearchRow

	err := r.db.WithContext(ctx).Model(&models.User{}).
		Select("id::text AS id, username, email").
		Where("LOWER(username) LIKE ? OR LOWER(email) LIKE ?", pattern, pattern).
		Order("username").
		Limit(searchUsersLimit).
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}

	return results, nil
}

// Leaderboard returns every user ranked by total quiz score, highest first.
func (r *gormUserRepository) Leaderboard(ctx context.Context) ([]LeaderboardRow, error) {
	var list []LeaderboardRow

	err := r.db.WithContext(ctx).Table("users AS u").
		Select(`u.id::text AS id, u.username, u.email, u.avatarurl AS avatar_url,
			COALESCE(SUM(mp.bestscore), 0)::bigint AS total_score,
			COUNT(DISTINCT CASE WHEN mp.passed THEN mp.courseslug || ':' || mp.moduleindex::text END)::bigint AS passed_modules,
			COUNT(DISTINCT e.courseslug)::bigint AS enrolled_courses`).
		Joins("LEFT JOIN module_progress mp ON mp.userid = u.id").
		Joins("LEFT JOIN enrollments e ON e.userid = u.id").
		Where("u.role = ? AND u.isactive = TRUE", roleStudent).
		Group("u.id, u.username, u.email, u.avatarurl").
		Order("total_score DESC, passed_modules DESC").
		Scan(&list).Error
	if err != nil {
		return nil, fmt.Errorf("leaderboard: %w", err)
	}

	return list, nil
}

// CreateAdminIfAbsent inserts the default admin account, doing nothing if a
// conflicting row already exists.
func (r *gormUserRepository) CreateAdminIfAbsent(ctx context.Context, username, email, hash string) error {
	user := models.User{Username: username, Email: email, PasswordHash: &hash, Role: roleAdmin}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&user).Error
	if err != nil {
		return fmt.Errorf("insert default admin: %w", err)
	}

	return nil
}

// UpsertAdminPassword creates the admin account with hash, or updates the
// password hash (and email) of the existing one matching username.
// Conflicts on username are used as the arbiter because the admin username
// is the stable identifier across email domain changes.
func (r *gormUserRepository) UpsertAdminPassword(ctx context.Context, username, email, hash string) error {
	user := models.User{Username: username, Email: email, PasswordHash: &hash, Role: roleAdmin}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: colUsername}},
		DoUpdates: clause.AssignmentColumns([]string{"email", "password_hash", colUpdatedAt}),
	}).Create(&user).Error
	if err != nil {
		return fmt.Errorf("upsert admin password: %w", err)
	}

	return nil
}

// UpsertMockStudent creates the mock student account with hash, or updates
// the username of the existing one matching email, returning its ID.
func (r *gormUserRepository) UpsertMockStudent(ctx context.Context, username, email, hash string) (uuid.UUID, error) {
	user := models.User{Username: username, Email: email, PasswordHash: &hash, Role: roleStudent}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: colEmail}},
		DoUpdates: clause.AssignmentColumns([]string{"username"}),
	}).Create(&user).Error
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert mock student: %w", err)
	}

	return user.ID, nil
}

// wrapNotFound maps gorm.ErrRecordNotFound to sentinel, leaving other errors
// wrapped with their original context.
func wrapNotFound(err, sentinel error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sentinel
	}

	return fmt.Errorf("query user: %w", err)
}
