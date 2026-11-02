package models

import "time"

// QuizQuestionAttempt records how many times a learner has answered one
// quiz question wrongly and until when their retry cooldown runs.
//
// This used to be a process-local map, which meant every pod restart or
// rollout handed learners a clean slate and two replicas disagreed about
// the same learner. Persisting it makes the backoff survive restarts and
// hold across every replica.
type QuizQuestionAttempt struct {
	Username    string `gorm:"column:username;primaryKey;size:255"`
	CourseSlug  string `gorm:"column:course_slug;primaryKey;size:255"`
	ModuleIndex int    `gorm:"column:module_index;primaryKey"`
	QuestionID  string `gorm:"column:question_id;primaryKey;size:255"`

	Attempts int `gorm:"column:attempts;not null;default:0"`
	// CooldownUntil is the instant the learner may retry this question.
	// A zero/NULL value means no cooldown is pending.
	CooldownUntil *time.Time `gorm:"column:cooldown_until"`

	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:now()"`
}

// TableName pins the table name so it does not depend on GORM's pluralizer.
func (QuizQuestionAttempt) TableName() string {
	return "quiz_question_attempts"
}
