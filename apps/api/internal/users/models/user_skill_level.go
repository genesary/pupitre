package models

import "time"

// UserSkillLevel records the learner's expertise level for a given skill,
// derived from the courses they have completed.
type UserSkillLevel struct {
	UserID           string    `gorm:"column:userid;type:uuid;not null;uniqueIndex:idx_user_skill_unique,priority:1"`
	Skill            string    `gorm:"column:skill;not null;uniqueIndex:idx_user_skill_unique,priority:2"`
	Level            string    `gorm:"column:level;not null"`      // "beginner"|"intermediate"|"advanced"|"maître"
	LevelOrder       int       `gorm:"column:levelorder;not null"` // 1=beginner…3=advanced, 4=maître
	CompletedCourses int       `gorm:"column:completed_courses;not null;default:0"`
	TotalCourses     int       `gorm:"column:total_courses;not null;default:0"`
	UpdatedAt        time.Time `gorm:"column:updatedat;not null;default:now()"`
}

// TableName pins the table name so GORM doesn't infer one from the type name.
func (UserSkillLevel) TableName() string {
	return "user_skill_levels"
}
