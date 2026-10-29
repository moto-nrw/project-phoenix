package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Date exposes the canonical calendar-date value used by attendance storage ports.
type Date = calendar.Date

// Attendance is the owner's calendar-date-aware, persistence-independent record.
type Attendance struct {
	ID, TenantID         int64
	CreatedAt, UpdatedAt time.Time
	StudentID            int64
	Date                 Date
	CheckInTime          time.Time
	CheckOutTime         *time.Time
	CheckedInBy          int64
	CheckedOutBy         *int64
	DeviceID             int64
	CheckedOutDeviceID   *int64
	YardSince            *time.Time
	CheckOutNote         *string
}

type AttendanceDay struct {
	StudentID int64
	Date      Date
}

type AttendanceStore interface {
	ListAttendanceDays(context.Context, Date, Date) ([]AttendanceDay, Stats, error)
	ListAttendanceSpans(context.Context, Date, Date, []int64) ([]AttendanceSpan, Stats, error)
	RecordAttendance(context.Context, *Attendance) (Stats, error)
	ReviseAttendance(context.Context, *Attendance) (Stats, error)
	DeleteAttendance(context.Context, int64) (Stats, error)
	EnsureAttendanceBatch(context.Context, []*Attendance) ([]int64, Stats, error)
	CloseStaleAttendance(context.Context, int64, time.Time, time.Time) (Stats, error)
	LockStudentAttendance(context.Context, int64) (Stats, error)
	HasAttendance(context.Context, studentpresence.AttendanceFilter) (bool, Stats, error)
	CountAttendanceByStaff(context.Context, int64) (int, Stats, error)
	ListOpenAttendanceStudentIDs(context.Context, string) ([]int64, Stats, error)
	FindAttendance(context.Context, int64) (*Attendance, Stats, error)
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]*Attendance, Stats, error)
	EnsureAttendance(context.Context, *Attendance) (bool, Stats, error)
	CloseAttendance(context.Context, studentpresence.AttendanceCheckout) ([]*Attendance, Stats, error)
	// DeleteAttendanceBefore deletes the tenant's attendance dated before
	// cutoff (#3957).
	DeleteAttendanceBefore(context.Context, Date) (Stats, error)
}
