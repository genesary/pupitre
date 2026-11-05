package models

import (
	"time"

	"github.com/google/uuid"
)

// PathEnrollment records that a user has enrolled in a learning path.
type PathEnrollment struct {
	UserID     uuid.UUID `gorm:"column:userid;type:uuid;primaryKey"`
	PathSlug   string    `gorm:"column:path_slug;primaryKey;index:idx_path_enrollments_slug"`
	EnrolledAt time.Time `gorm:"column:enrolledat;not null;default:now()"`
}

// TableName pins the table name GORM maps this model to.
func (PathEnrollment) TableName() string { return "path_enrollments" }
