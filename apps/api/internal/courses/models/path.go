package models

import "time"

// Path is a sequential learning path composed of course slugs (kind
// "course") or skill tags (kind "skill"). Its ordered members live in
// PathCourse and PathSkill so ordering survives independently of how the
// rows come back.
type Path struct {
	Slug        string `gorm:"column:slug;primaryKey;size:255"`
	Title       string `gorm:"column:title;not null;default:'';index:paths_title_idx"`
	Description string `gorm:"column:description;not null;default:''"`
	Kind        string `gorm:"column:kind;not null;default:'course';index:paths_kind_idx"`
	Level       string `gorm:"column:level;not null;default:''"`

	CreatedAt time.Time `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:now()"`

	Courses []PathCourse `gorm:"foreignKey:PathSlug;references:Slug;constraint:OnDelete:CASCADE"`
	Skills  []PathSkill  `gorm:"foreignKey:PathSlug;references:Slug;constraint:OnDelete:CASCADE"`
}

// TableName pins the table name so it does not depend on GORM's pluralizer.
func (Path) TableName() string {
	return "paths"
}

// PathCourse links a course into a path at a fixed position. The index on
// CourseSlug is what replaces the in-memory reverse index that used to
// answer "which paths contain this course?".
type PathCourse struct {
	PathSlug   string `gorm:"column:path_slug;primaryKey;size:255"`
	Position   int    `gorm:"column:position;primaryKey"`
	CourseSlug string `gorm:"column:course_slug;not null;size:255;index:path_courses_course_idx"`
}

// TableName pins the table name so it does not depend on GORM's pluralizer.
func (PathCourse) TableName() string {
	return "path_courses"
}

// PathSkill links a skill tag into a skill-kind path at a fixed position.
type PathSkill struct {
	PathSlug string `gorm:"column:path_slug;primaryKey;size:255"`
	Position int    `gorm:"column:position;primaryKey"`
	Skill    string `gorm:"column:skill;not null;size:255;index:path_skills_skill_idx"`
}

// TableName pins the table name so it does not depend on GORM's pluralizer.
func (PathSkill) TableName() string {
	return "path_skills"
}
