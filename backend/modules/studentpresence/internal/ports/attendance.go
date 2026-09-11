package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// Date exposes the canonical calendar-date value used by attendance storage ports.
type Date = timezone.Date

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
}

type AttendanceFilter struct {
	IDs          []int64
	StudentIDs   []int64
	FromDate     string
	UntilDate    string
	BeforeDate   string
	OpenOnly     bool
	NewestFirst  bool
	StudentOrder bool
	ForUpdate    bool
	Limit        int
}

type AttendanceCheckout struct {
	StudentIDs []int64
	Date       string
	At         time.Time
	StaffID    int64
	DeviceID   int64
}

type AttendanceStore interface {
	RecordAttendance(context.Context, *Attendance) (Stats, error)
	ReviseAttendance(context.Context, *Attendance) (Stats, error)
	DeleteAttendance(context.Context, int64) (Stats, error)
	EnsureAttendanceBatch(context.Context, []*Attendance) ([]int64, Stats, error)
	CloseStaleAttendance(context.Context, int64, time.Time, time.Time) (Stats, error)
	LockStudentAttendance(context.Context, int64) (Stats, error)
	HasAttendance(context.Context, AttendanceFilter) (bool, Stats, error)
	CountAttendanceByStaff(context.Context, int64) (int, Stats, error)
	ListOpenAttendanceStudentIDs(context.Context, string) ([]int64, Stats, error)
	FindAttendance(context.Context, int64) (*Attendance, Stats, error)
	ListAttendance(context.Context, AttendanceFilter) ([]*Attendance, Stats, error)
	EnsureAttendance(context.Context, *Attendance) (bool, Stats, error)
	CloseAttendance(context.Context, AttendanceCheckout) ([]*Attendance, Stats, error)
}
