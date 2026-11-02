package models

import "time"

// UserXPEvent records a single XP gain for a learner.
// The XP total is derived by summing all events for a given user.
//
// (userid, source, source_slug) is unique: XP is earned once per thing, and
// the write path relies on it. Award() upserts with ON CONFLICT DO NOTHING,
// which without this index had no constraint to conflict against — so every
// repeat call inserted another row and the learner's total grew each time
// they re-submitted a quiz they had already passed, or re-completed a lesson
// in a course already marked finished.
type UserXPEvent struct {
	ID         uint      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID     string    `gorm:"column:userid;type:uuid;not null;index;uniqueIndex:idx_user_xp_events_unique,priority:1"`
	Source     string    `gorm:"column:source;size:16;not null;uniqueIndex:idx_user_xp_events_unique,priority:2"` // "lesson"|"module"|"course"
	SourceSlug string    `gorm:"column:source_slug;size:255;not null;uniqueIndex:idx_user_xp_events_unique,priority:3"`
	Amount     int       `gorm:"column:amount;not null"`
	EarnedAt   time.Time `gorm:"column:earned_at;not null;default:now()"`
}

// TableName pins the table name so GORM doesn't infer one from the type name.
func (UserXPEvent) TableName() string {
	return "user_xp_events"
}
