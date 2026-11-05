package models

import "time"

// ModuleProgress records a user's best-attempt score and pass state for a
// course module.
type ModuleProgress struct {
	ID          string     `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID      string     `gorm:"column:userid;type:uuid;not null;uniqueIndex:idx_module_progress_unique,priority:1;index:idx_module_progress_slug,priority:1,where:moduleslug IS NOT NULL"`
	CourseSlug  string     `gorm:"column:courseslug;not null;uniqueIndex:idx_module_progress_unique,priority:2;index:idx_module_progress_slug,priority:2,where:moduleslug IS NOT NULL"`
	ModuleIndex int        `gorm:"column:moduleindex;not null;uniqueIndex:idx_module_progress_unique,priority:3"`
	ModuleSlug  *string    `gorm:"column:moduleslug;index:idx_module_progress_slug,priority:3,where:moduleslug IS NOT NULL"`
	BestScore   int        `gorm:"column:bestscore;not null;default:0"`
	MaxScore    int        `gorm:"column:maxscore;not null;default:0"`
	Passed      bool       `gorm:"column:passed;not null;default:false"`
	Attempts    int        `gorm:"column:attempts;not null;default:0"`
	ModuleType  string     `gorm:"column:moduletype"`
	CompletedAt *time.Time `gorm:"column:completed_at"` // already snake_case in SQL
	UpdatedAt   time.Time  `gorm:"column:updatedat;not null;default:now()"`
}

// TableName pins the table name so GORM doesn't infer one from the type
// name.
func (ModuleProgress) TableName() string {
	return "module_progress"
}
