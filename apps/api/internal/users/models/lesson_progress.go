package models

import "time"

// LessonProgress records that a user has viewed/completed a lesson within a
// course.
type LessonProgress struct {
	ID         string    `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     string    `gorm:"column:userid;type:uuid;not null;index:idx_lesson_progress_user;uniqueIndex:idx_lesson_progress_unique,priority:1"`
	CourseSlug string    `gorm:"column:courseslug;not null;index:idx_lesson_progress_key,priority:1;uniqueIndex:idx_lesson_progress_unique,priority:2"`
	LessonSlug string    `gorm:"column:lessonslug;not null;index:idx_lesson_progress_key,priority:2;uniqueIndex:idx_lesson_progress_unique,priority:3"`
	ViewedAt   time.Time `gorm:"column:viewed_at;not null;default:now()"` // already snake_case in SQL
}

// TableName pins the table name so GORM doesn't infer one from the type
// name.
func (LessonProgress) TableName() string {
	return "lesson_progress"
}
