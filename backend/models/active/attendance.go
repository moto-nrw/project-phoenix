package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Attendance represents attendance tracking for RFID check-ins/outs
type Attendance struct {
	base.Model `bun:"schema:active,table:attendance"`
	base.TenantModel
	StudentID    int64      `bun:"student_id,notnull" json:"student_id"`
	Date         time.Time  `bun:"date,notnull" json:"date"`
	CheckInTime  time.Time  `bun:"check_in_time,notnull" json:"check_in_time"`
	CheckOutTime *time.Time `bun:"check_out_time" json:"check_out_time,omitempty"`
	CheckedInBy  int64      `bun:"checked_in_by,notnull" json:"checked_in_by"`
	CheckedOutBy *int64     `bun:"checked_out_by" json:"checked_out_by,omitempty"`
	DeviceID     int64      `bun:"device_id,notnull" json:"device_id"`
	// YardSince is set when the student transitions to "Schulhof" in binary
	// mode and cleared on a school checkout. Schema and read paths
	// (deriveAttendanceStatus, ResolveBinaryLocation, performCheckOut) are
	// in place, but the *write* originates from PyrePortal's "Schulhof"
	// kiosk endpoint, which is tracked separately. Until that ships,
	// `on_yard` is unreachable in production. See the cross-repo PyrePortal
	// ticket linked in the PR description.
	YardSince *time.Time `bun:"yard_since" json:"yard_since,omitempty"`
}

// IsOnYard returns true if the student is currently marked as being on the school yard
// (still on premises, outside the building). Only meaningful while IsCheckedIn is also true.
func (a *Attendance) IsOnYard() bool {
	return a.YardSince != nil && a.CheckOutTime == nil
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

	// CreateIfNoOpenForToday inserts the attendance row using ON CONFLICT
	// against the partial unique index on (student_id, date) WHERE
	// check_out_time IS NULL (migration 1.15.42). Returns inserted=true when
	// the row was written, false when the conflict path swallowed the insert
	// (a concurrent caller already opened today's attendance for the student).
	CreateIfNoOpenForToday(ctx context.Context, attendance *Attendance) (bool, error)

	// Update updates an existing attendance record
	Update(ctx context.Context, attendance *Attendance) error

	// FindByID finds an attendance record by ID
	FindByID(ctx context.Context, id int64) (*Attendance, error)

	// FindByStudentAndDate finds all attendance records for a student on a specific date
	FindByStudentAndDate(ctx context.Context, studentID int64, date time.Time) ([]*Attendance, error)

	// FindByStudentAndDateRange finds all attendance records for a student between two
	// dates (inclusive), ordered by date descending then check_in_time descending.
	FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*Attendance, error)

	// FindLatestByStudent finds the most recent attendance record for a student
	FindLatestByStudent(ctx context.Context, studentID int64) (*Attendance, error)

	// GetStudentCurrentStatus gets the current check-in status for a student
	GetStudentCurrentStatus(ctx context.Context, studentID int64) (*Attendance, error)

	// Delete deletes an attendance record
	Delete(ctx context.Context, id int64) error

	// GetTodayByStudentID gets today's attendance record for a student
	GetTodayByStudentID(ctx context.Context, studentID int64) (*Attendance, error)

	// GetTodayByStudentIDs gets today's attendance record for multiple students
	GetTodayByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*Attendance, error)

	// FindForDate finds all attendance records for a specific date
	FindForDate(ctx context.Context, date time.Time) ([]*Attendance, error)

	// CountByStaffID counts attendance records where the staff member checked in or checked out students
	CountByStaffID(ctx context.Context, staffID int64) (int, error)
}
