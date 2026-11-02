// Package models contains the GORM-mapped structs for user-service's
// persisted tables.
package models

import (
	"time"

	"github.com/google/uuid"
)

// User is a platform account, either local (password-based) or SSO-backed
// (OIDC/LDAP), synced with groups derived from an identity provider.
type User struct {
	ID             uuid.UUID  `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	Username       string     `gorm:"column:username;size:64;not null;uniqueIndex"`
	Email          string     `gorm:"column:email;size:255;not null;uniqueIndex"`
	PasswordHash   *string    `gorm:"column:password_hash"` // nil for SSO-only accounts
	Role           string     `gorm:"column:role;size:16;not null;default:student"`
	AvatarURL      *string    `gorm:"column:avatarurl"`
	Bio            *string    `gorm:"column:bio"`
	IsActive       bool       `gorm:"column:isactive;not null;default:true"`
	AuthProvider   string     `gorm:"column:authprovider;size:32;not null;default:local;uniqueIndex:idx_users_provider_uid,priority:1"`
	ProviderUserID *string    `gorm:"column:provider_user_id;size:255;uniqueIndex:idx_users_provider_uid,priority:2"`
	CreatedAt      time.Time  `gorm:"column:createdat;not null;default:now()"`
	UpdatedAt      time.Time  `gorm:"column:updatedat;not null;default:now()"`
	LastLoginAt    *time.Time `gorm:"column:lastloginat"`
	// SessionsValidAfter revokes every token issued before it: set on a
	// password change, a role change, a deactivation and a sign-out
	// (docs/specs/190-backend-defects.md B7). Nil revokes nothing.
	SessionsValidAfter *time.Time `gorm:"column:sessionsvalidafter"`
	// IdentityName is the login the identity provider gave at the last
	// sign-in (OIDC preferred_username). Unlike Username, the user cannot
	// change it: lab checks find the learner's GitLab project with it
	// (docs/specs/190-backend-defects.md B11).
	IdentityName *string `gorm:"column:identityname;size:255"`
}

// TableName pins the table name so GORM doesn't infer one from the type
// name (which would otherwise pluralize to "users" anyway, but this keeps
// the mapping explicit alongside the column tags above).
func (User) TableName() string {
	return "users"
}
