package fake

import "github.com/jojo91550/pupitre2/apps/api/internal/users/repository"

// NewRepositories builds a *repository.Repositories backed entirely by
// empty fakes, letting tests override only the aggregates they need.
func NewRepositories() *repository.Repositories {
	return &repository.Repositories{
		Users:           NewUserRepository(),
		Settings:        NewSettingRepository(),
		Enrollments:     NewEnrollmentRepository(),
		LessonProgress:  NewLessonProgressRepository(),
		ModuleProgress:  NewModuleProgressRepository(),
		Groups:          NewGroupRepository(),
		Patterns:        NewPatternRepository(),
		Paths:           NewPathEnrollmentRepository(),
		Badges:          NewBadgeRepository(),
		SkillLevels:     NewSkillLevelRepository(),
		SessionBookings: NewSessionBookingRepository(),
		XP:              NewXPRepository(),
		Export:          NewExportRepository(),
	}
}
