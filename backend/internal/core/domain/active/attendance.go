package active

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// Attendance represents attendance tracking for RFID check-ins/outs
type Attendance struct {
	base.Model   `bun:"schema:active,table:attendance"`
	StudentID    int64      `bun:"student_id,notnull" json:"student_id"`
	Date         time.Time  `bun:"date,notnull" json:"date"`
	CheckInTime  time.Time  `bun:"check_in_time,notnull" json:"check_in_time"`
	CheckOutTime *time.Time `bun:"check_out_time" json:"check_out_time,omitempty"`
	CheckedInBy  int64      `bun:"checked_in_by,notnull" json:"checked_in_by"`
	CheckedOutBy *int64     `bun:"checked_out_by" json:"checked_out_by,omitempty"`
	DeviceID     int64      `bun:"device_id,notnull" json:"device_id"`
}

// BeforeAppendModel is commented out to let the repository control the table expression
// func (a *Attendance) BeforeAppendModel(query any) error {
// 	if q, ok := query.(*bun.SelectQuery); ok {
// 		q.ModelTableExpr("active.attendance")
// 	}
// 	if q, ok := query.(*bun.InsertQuery); ok {
// 		q.ModelTableExpr("active.attendance")
// 	}
// 	if q, ok := query.(*bun.UpdateQuery); ok {
// 		q.ModelTableExpr("active.attendance")
// 	}
// 	if q, ok := query.(*bun.DeleteQuery); ok {
// 		q.ModelTableExpr("active.attendance")
// 	}
// 	return nil
// }

// GetID returns the entity's ID
func (a *Attendance) GetID() interface{} {
	return a.ID
}

// GetCreatedAt returns the creation timestamp
func (a *Attendance) GetCreatedAt() time.Time {
	return a.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (a *Attendance) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}

// TableName returns the database table name
func (a *Attendance) TableName() string {
	return "active.attendance"
}

// IsCheckedIn returns true if the student is currently checked in
func (a *Attendance) IsCheckedIn() bool {
	return a.CheckOutTime == nil
}

// StaleAttendanceRecord represents an attendance record that needs cleanup
// (from before today with no check-out time)
type StaleAttendanceRecord struct {
	ID          int64
	StudentID   int64
	Date        time.Time
	CheckInTime time.Time
}
