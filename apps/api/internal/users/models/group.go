package models

import (
	"time"

	"github.com/google/uuid"
)

// GroupMaxDepth is the maximum nesting level allowed for subgroups (root = 0).
const GroupMaxDepth = 10

// Group is a row in the groups table — either a locally-created admin group
// or one mirrored from an IdP during SSO login.
type Group struct {
	ID          uuid.UUID  `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	Name        string     `gorm:"column:name;size:255;not null;uniqueIndex"`
	Description *string    `gorm:"column:description"`
	Source      string     `gorm:"column:source;size:32;not null;default:local"`
	ParentID    *uuid.UUID `gorm:"column:parent_id;type:uuid;index:idx_groups_parent_id"`
	Depth       int        `gorm:"column:depth;not null;default:0"`
	Path        string     `gorm:"column:path;not null;default:'';index:idx_groups_path"`
	CreatedAt   time.Time  `gorm:"column:createdat;not null;default:now()"`
	UpdatedAt   time.Time  `gorm:"column:updatedat;not null;default:now()"`
}

// TableName pins Group to the groups table.
func (Group) TableName() string { return "groups" }
