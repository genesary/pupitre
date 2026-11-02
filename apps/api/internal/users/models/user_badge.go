package models

import "time"

// UserBadge records a badge earned by a user upon completing a course.
type UserBadge struct {
	ID         string    `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     string    `gorm:"column:userid;type:uuid;not null;uniqueIndex:idx_user_badge_unique,priority:1"`
	CourseSlug string    `gorm:"column:courseslug;not null;uniqueIndex:idx_user_badge_unique,priority:2"`
	EarnedAt   time.Time `gorm:"column:earnedat;not null;default:now()"`
}

// TableName pins the table name so GORM doesn't infer one from the type name.
func (UserBadge) TableName() string {
	return "user_badges"
}
