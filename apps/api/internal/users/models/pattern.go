package models

import (
	"time"

	"github.com/google/uuid"
)

// MarkdownPattern is a reusable markdown rendering rule (global or
// course-scoped) with an HTML/CSS/JS implementation.
type MarkdownPattern struct {
	ID          uuid.UUID  `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	Name        string     `gorm:"column:name;size:64;not null;uniqueIndex:idx_markdown_patterns_unique,priority:1"`
	Label       string     `gorm:"column:label;size:128;not null"`
	Description string     `gorm:"column:description;not null;default:''"`
	Parameter   string     `gorm:"column:parameter;not null;default:''"`
	HTML        string     `gorm:"column:html;not null"`
	CSS         string     `gorm:"column:css;not null;default:''"`
	JS          string     `gorm:"column:js;not null;default:''"`
	Scope       string     `gorm:"column:scope;not null;default:global;uniqueIndex:idx_markdown_patterns_unique,priority:2;index:idx_markdown_patterns_scope"`
	CreatedBy   *uuid.UUID `gorm:"column:createdby;type:uuid"`
	FromConfig  bool       `gorm:"column:from_config;not null;default:false"`
	CreatedAt   time.Time  `gorm:"column:createdat;not null;default:now()"`
	UpdatedAt   time.Time  `gorm:"column:updatedat;not null;default:now()"`
}

// TableName pins the table name GORM maps this model to.
func (MarkdownPattern) TableName() string { return "markdown_patterns" }
