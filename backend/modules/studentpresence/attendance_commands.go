package studentpresence

import (
	"context"
	"time"
)

type AttendanceHistoryCommand interface {
	RecordAttendance(context.Context, Attendance) (Attendance, error)
	ReviseAttendance(context.Context, Attendance) (Attendance, error)
	DeleteAttendance(context.Context, int64) error
	EnsureAttendanceBatch(context.Context, []Attendance) ([]int64, error)
	CloseStaleAttendance(context.Context, int64, time.Time, time.Time) (int64, error)
	LockStudentAttendance(context.Context, int64) error
}
type AttendanceDay struct {
	StudentID int64
	Date      string
}

type AttendanceSummaryQuery interface {
	ListAttendanceDays(context.Context, string, string) ([]AttendanceDay, error)
	HasAttendance(context.Context, AttendanceFilter) (bool, error)
	CountAttendanceByStaff(context.Context, int64) (int, error)
	ListOpenAttendanceStudentIDs(context.Context, string) ([]int64, error)
}
