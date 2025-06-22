package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Attendance represents attendance tracking for RFID check-ins/outs
type Attendance struct {
	base.Model     `bun:"schema:active,table:attendance"`
	StudentID      int64      `bun:"student_id,notnull" json:"student_id"`
	Date           time.Time  `bun:"date,notnull" json:"date"`
	CheckInTime    time.Time  `bun:"check_in_time,notnull" json:"check_in_time"`
	CheckOutTime   *time.Time `bun:"check_out_time" json:"check_out_time,omitempty"`
	CheckedInBy    int64      `bun:"checked_in_by,notnull" json:"checked_in_by"`
	CheckedOutBy   *int64     `bun:"checked_out_by" json:"checked_out_by,omitempty"`
	DeviceID       int64      `bun:"device_id,notnull" json:"device_id"`
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

// AttendanceRepository defines the interface for attendance data operations
type AttendanceRepository interface {
	// Create creates a new attendance record
	Create(ctx context.Context, attendance *Attendance) error

	// Update updates an existing attendance record
	Update(ctx context.Context, attendance *Attendance) error

	// FindByID finds an attendance record by ID
	FindByID(ctx context.Context, id int64) (*Attendance, error)

	// FindByStudentAndDate finds all attendance records for a student on a specific date
	FindByStudentAndDate(ctx context.Context, studentID int64, date time.Time) ([]*Attendance, error)

	// FindLatestByStudent finds the most recent attendance record for a student
	FindLatestByStudent(ctx context.Context, studentID int64) (*Attendance, error)

	// GetStudentCurrentStatus gets the current check-in status for a student
	GetStudentCurrentStatus(ctx context.Context, studentID int64) (*Attendance, error)

	// Delete deletes an attendance record
	Delete(ctx context.Context, id int64) error
}