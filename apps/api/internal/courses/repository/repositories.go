package repository

import "gorm.io/gorm"

// Repositories bundles every data-access interface the HTTP handlers
// depend on, so wiring them up is one call in main and one struct literal
// in tests.
type Repositories struct {
	Courses      CourseRepository
	Paths        PathRepository
	QuizAttempts QuizAttemptRepository
	LabChecks    LabCheckRepository
}

// NewGormRepositories builds the production repository set backed by db.
func NewGormRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Courses:      NewGormCourseRepository(db),
		Paths:        NewGormPathRepository(db),
		QuizAttempts: NewGormQuizAttemptRepository(db),
		LabChecks:    NewGormLabCheckRepository(db),
	}
}
