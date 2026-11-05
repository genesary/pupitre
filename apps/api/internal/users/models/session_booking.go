package models

import "time"

// SessionBooking records a learner's seat reservation for an in-person
// course session.
type SessionBooking struct {
	ID         string    `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     string    `gorm:"column:userid;type:uuid;not null;uniqueIndex:idx_session_booking_unique,priority:1"`
	CourseSlug string    `gorm:"column:courseslug;not null;uniqueIndex:idx_session_booking_unique,priority:2"`
	SessionID  string    `gorm:"column:sessionid;not null;uniqueIndex:idx_session_booking_unique,priority:3"`
	BookedAt   time.Time `gorm:"column:bookedat;not null;default:now()"`
	Present    bool      `gorm:"column:present;not null;default:false"`
}

// TableName pins the table name so GORM doesn't infer one from the type name.
func (SessionBooking) TableName() string {
	return "session_bookings"
}
