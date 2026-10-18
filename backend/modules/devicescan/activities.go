package devicescan

import (
	"context"
	"time"
)

// Activity is one activity template of the timetable.
type Activity struct {
	ID              int64
	Name            string
	MaxParticipants int
	PlannedRoomID   *int64
	IsSystem        bool
	IsOpen          bool
}

// HasParticipantLimit reports a bounded activity.
func (a Activity) HasParticipantLimit() bool { return a.MaxParticipants > 0 }

// Category is one activity category.
type Category struct {
	ID   int64
	Name string
}

// NewCategory describes a system category to provision.
type NewCategory struct {
	Name        string
	Description string
	Color       string
	IsSystem    bool
}

// NewActivity describes a system activity to provision.
type NewActivity struct {
	Name            string
	MaxParticipants int
	IsOpen          bool
	CategoryID      int64
	PlannedRoomID   *int64
	IsSystem        bool
}

// ActivityDirectory reads the kiosk activity picker.
type ActivityDirectory interface {
	Activities(context.Context) ([]DirectoryActivity, error)
}

// Session is one running or ended room session.
type Session struct {
	ID         int64
	RoomID     int64
	StartTime  time.Time
	EndTime    *time.Time
	DeviceID   *int64
	TemplateID *int64
	// Activity is the loaded template when the source carried it.
	Activity *Activity
	// RoomName and ActivityName are set by lookups that join the names.
	RoomName     string
	ActivityName string
}

// Supervisor is one staff assignment of a session.
type Supervisor struct {
	StaffID int64
	Ended   bool
}

// NewSession describes the session a special room provisions on scan.
type NewSession struct {
	ActivityID int64
	RoomID     int64
}
