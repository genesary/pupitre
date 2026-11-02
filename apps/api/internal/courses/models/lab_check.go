// Package models contains the GORM-mapped structs for course-service's
// persisted tables.
package models

import (
	"time"

	"github.com/lib/pq"
)

// LabCheck is a single recorded outcome of a lab module check, either
// server-verified (CheckModule) or client-reported (RecordLocalCheck).
type LabCheck struct {
	ID          int64          `gorm:"column:id;primaryKey"`
	Username    string         `gorm:"column:username;not null;index:lab_checks_username_idx"`
	CourseSlug  string         `gorm:"column:courseslug;not null;index:lab_checks_course_slug_idx"`
	ModuleIndex int            `gorm:"column:moduleindex;not null"`
	ModuleName  string         `gorm:"column:modulename;not null"`
	Allow       bool           `gorm:"column:allow;not null"`
	Violations  pq.StringArray `gorm:"column:violations;type:text[];not null;default:'{}'"`
	CheckedAt   time.Time      `gorm:"column:checkedat;not null;default:now()"`
	Verified    bool           `gorm:"column:verified;not null;default:false"`
}

// TableName pins the table name so GORM doesn't infer one from the type
// name (which would otherwise pluralize to "lab_checks" anyway, but this
// keeps the mapping explicit alongside the column tags above).
func (LabCheck) TableName() string {
	return "lab_checks"
}
