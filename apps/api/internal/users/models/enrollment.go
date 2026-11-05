package models

import "time"

// Enrollment records a user's enrollment in a course, identified by slug.
type Enrollment struct {
	ID         string    `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     string    `gorm:"column:userid;type:uuid;not null;uniqueIndex:idx_enrollments_unique,priority:1"`
	CourseSlug string    `gorm:"column:courseslug;not null;uniqueIndex:idx_enrollments_unique,priority:2;index:idx_enrollments_slug"`
	EnrolledAt time.Time `gorm:"column:enrolledat;not null;default:now()"`
}

// TableName pins the table name so GORM doesn't infer one from the type
// name.
func (Enrollment) TableName() string {
	return "enrollments"
}
