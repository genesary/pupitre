package models

import "time"

// ExportLog records every CSV download for audit purposes.
type ExportLog struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	UserID       string `gorm:"not null"`
	UserEmail    string `gorm:"not null"`
	Category     string `gorm:"not null"`
	Fields       string `gorm:"not null"`
	RowCount     int
	DownloadedAt time.Time `gorm:"default:now()"`
}
