package repository

import "gorm.io/gorm"

// Repositories bundles every repository interface used by user-service's
// handlers, so State only needs to hold one field.
type Repositories struct {
	Users           UserRepository
	Settings        SettingRepository
	Enrollments     EnrollmentRepository
	LessonProgress  LessonProgressRepository
	ModuleProgress  ModuleProgressRepository
	Groups          GroupRepository
	Patterns        PatternRepository
	Paths           PathEnrollmentRepository
	Badges          BadgeRepository
	SkillLevels     SkillLevelRepository
	SessionBookings SessionBookingRepository
	XP              XPRepository
	Export          ExportRepository
}

// NewGormRepositories builds a Repositories backed by gdb.
func NewGormRepositories(gdb *gorm.DB) *Repositories {
	return &Repositories{
		Users:           NewGormUserRepository(gdb),
		Settings:        NewGormSettingRepository(gdb),
		Enrollments:     NewGormEnrollmentRepository(gdb),
		LessonProgress:  NewGormLessonProgressRepository(gdb),
		ModuleProgress:  NewGormModuleProgressRepository(gdb),
		Groups:          NewGormGroupRepository(gdb),
		Patterns:        NewGormPatternRepository(gdb),
		Paths:           NewGormPathEnrollmentRepository(gdb),
		Badges:          NewGormBadgeRepository(gdb),
		SkillLevels:     NewGormSkillLevelRepository(gdb),
		SessionBookings: NewGormSessionBookingRepository(gdb),
		XP:              NewGormXPRepository(gdb),
		Export:          NewGormExportRepository(gdb),
	}
}
