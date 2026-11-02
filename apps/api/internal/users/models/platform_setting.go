package models

import "time"

// PlatformSetting is a single key/value row in the admin-configurable
// platform settings table (registration policy, SSO config, ...).
type PlatformSetting struct {
	Key         string    `gorm:"column:key;size:64;primaryKey"`
	Value       string    `gorm:"column:value;not null"`
	Description *string   `gorm:"column:description"`
	UpdatedAt   time.Time `gorm:"column:updatedat;not null;default:now()"`
}

// TableName pins the table name so GORM doesn't infer one from the type
// name, keeping the mapping explicit alongside the column tags above.
func (PlatformSetting) TableName() string {
	return "platform_settings"
}
