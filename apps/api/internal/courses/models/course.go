package models

import (
	"time"

	"github.com/lib/pq"
)

// Course is the persisted definition of a course. Scalar fields that the
// catalog filters or sorts on are real columns so listing never has to
// touch the module rows; the nested content lives in CourseModule,
// CoursePrerequisite, and CourseSession.
type Course struct {
	Slug        string `gorm:"column:slug;primaryKey;size:255"`
	Title       string `gorm:"column:title;not null;default:'';index:courses_title_idx"`
	Description string `gorm:"column:description;not null;default:''"`
	Category    string `gorm:"column:category;not null;default:'';index:courses_category_idx"`
	Difficulty  string `gorm:"column:difficulty;not null;default:'';index:courses_difficulty_idx"`

	// Public marks the course as visible in the anonymous catalog; users
	// are auto-enrolled on first access.
	Public bool `gorm:"column:public;not null;default:false;index:courses_public_idx"`
	// Hidden keeps the course out of the catalog without unpublishing it.
	Hidden bool `gorm:"column:hidden;not null;default:false"`

	Scope      string `gorm:"column:scope;not null;default:''"`
	XPRequired int    `gorm:"column:xp_required;not null;default:0"`
	BadgeName  string `gorm:"column:badge_name;not null;default:''"`
	BadgeIcon  string `gorm:"column:badge_icon;not null;default:''"`
	InPerson   bool   `gorm:"column:in_person;not null;default:false"`

	// Skills is the union of every module's skill tags, denormalized onto
	// the course row so the catalog can filter by skill without joining
	// course_modules. It is recomputed on every course write.
	Skills pq.StringArray `gorm:"column:skills;type:text[];not null;default:'{}'"`

	CreatedAt time.Time `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:now()"`

	Modules       []CourseModule       `gorm:"foreignKey:CourseSlug;references:Slug;constraint:OnDelete:CASCADE"`
	Prerequisites []CoursePrerequisite `gorm:"foreignKey:CourseSlug;references:Slug;constraint:OnDelete:CASCADE"`
	Sessions      []CourseSession      `gorm:"foreignKey:CourseSlug;references:Slug;constraint:OnDelete:CASCADE"`
}

// TableName pins the table name so it does not depend on GORM's pluralizer.
func (Course) TableName() string {
	return "courses"
}

// CourseModule is one element of a course, in display order. Opaque leaf
// payloads (quiz questions, lab check steps and their provider-specific
// parameters) are stored as jsonb: they are always read and written whole
// and are never filtered on.
type CourseModule struct {
	ID int64 `gorm:"column:id;primaryKey"`

	// CourseSlug and Position together identify a module; the unique index
	// on the pair is what makes "module N of course X" a single indexed
	// row read.
	CourseSlug string `gorm:"column:course_slug;not null;size:255;uniqueIndex:course_modules_pos_idx,priority:1"`
	Position   int    `gorm:"column:position;not null;uniqueIndex:course_modules_pos_idx,priority:2"`

	// Slug is derived from Name and persisted so modules can be looked up
	// by slug without recomputing it over the whole course.
	Slug string `gorm:"column:slug;not null;default:'';index:course_modules_slug_idx"`
	Name string `gorm:"column:name;not null;default:''"`
	Type string `gorm:"column:type;not null;default:'text'"`

	Src  string `gorm:"column:src;not null;default:''"`
	Ref  string `gorm:"column:ref;not null;default:''"`
	Path string `gorm:"column:path;not null;default:''"`

	LabURL        string `gorm:"column:lab_url;not null;default:''"`
	InlineContent string `gorm:"column:inline_content;not null;default:''"`

	Replication bool `gorm:"column:replication;not null;default:false"`
	Hidden      bool `gorm:"column:hidden;not null;default:false"`
	Inline      bool `gorm:"column:inline;not null;default:false"`

	// Prerequisites holds the module slugs that must be completed first.
	Prerequisites pq.StringArray `gorm:"column:prerequisites;type:text[];not null;default:'{}'"`

	PassingScore        int     `gorm:"column:passing_score;not null;default:0"`
	CooldownStrategy    string  `gorm:"column:cooldown_strategy;not null;default:''"`
	CooldownBaseSeconds int     `gorm:"column:cooldown_base_seconds;not null;default:0"`
	CooldownMultiplier  float64 `gorm:"column:cooldown_multiplier;not null;default:0"`
	CooldownMaxSeconds  int     `gorm:"column:cooldown_max_seconds;not null;default:0"`

	MaxAttemptsPerQuestion *int `gorm:"column:max_attempts_per_question"`
	LockOnMaxAttempts      bool `gorm:"column:lock_on_max_attempts;not null;default:false"`

	CheckProvider string `gorm:"column:check_provider;not null;default:''"`
	CheckType     string `gorm:"column:check_type;not null;default:''"`

	CheckParams JSONB `gorm:"column:check_params;type:jsonb"`
	Steps       JSONB `gorm:"column:steps;type:jsonb"`
	Questions   JSONB `gorm:"column:questions;type:jsonb"`

	Skills pq.StringArray `gorm:"column:skills;type:text[];not null;default:'{}'"`
}

// TableName pins the table name so it does not depend on GORM's pluralizer.
func (CourseModule) TableName() string {
	return "course_modules"
}

// CoursePrerequisite is one condition a learner must satisfy before
// enrolling in the owning course.
type CoursePrerequisite struct {
	ID         int64  `gorm:"column:id;primaryKey"`
	CourseSlug string `gorm:"column:course_slug;not null;size:255;index:course_prereqs_course_idx"`

	// RequiredCourse is the slug of the course that must be completed.
	RequiredCourse string `gorm:"column:required_course;not null;size:255"`
	MinScore       int    `gorm:"column:min_score;not null;default:0"`
	// Modules narrows the requirement to specific module slugs within
	// RequiredCourse instead of the whole course.
	Modules pq.StringArray `gorm:"column:modules;type:text[];not null;default:'{}'"`
}

// TableName pins the table name so it does not depend on GORM's pluralizer.
func (CoursePrerequisite) TableName() string {
	return "course_prerequisites"
}

// CourseSession is one scheduled in-person session of a course.
type CourseSession struct {
	CourseSlug string `gorm:"column:course_slug;primaryKey;size:255"`
	SessionID  string `gorm:"column:session_id;primaryKey;size:255"`

	Title    string `gorm:"column:title;not null;default:''"`
	Date     string `gorm:"column:date;not null;default:''"`
	Location string `gorm:"column:location;not null;default:''"`
	Capacity int    `gorm:"column:capacity;not null;default:0"`
}

// TableName pins the table name so it does not depend on GORM's pluralizer.
func (CourseSession) TableName() string {
	return "course_sessions"
}
