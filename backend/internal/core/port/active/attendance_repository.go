package active

import (
	"context"
	"time"

	domain "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
)

type Attendance = domain.Attendance
type StaleAttendanceRecord = domain.StaleAttendanceRecord

// AttendanceRepository defines the interface for attendance data operations.
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

	// GetTodayByStudentID gets today's attendance record for a student
	GetTodayByStudentID(ctx context.Context, studentID int64) (*Attendance, error)

	// GetTodayByStudentIDs gets today's attendance record for multiple students
	GetTodayByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*Attendance, error)

	// FindForDate finds all attendance records for a specific date
	FindForDate(ctx context.Context, date time.Time) ([]*Attendance, error)

	// FindStaleRecords finds attendance records from before the given date that have no check-out time
	FindStaleRecords(ctx context.Context, beforeDate time.Time) ([]StaleAttendanceRecord, error)

	// CloseStaleRecord sets the check-out time for a stale attendance record
	CloseStaleRecord(ctx context.Context, id int64, checkOutTime time.Time) error
}
