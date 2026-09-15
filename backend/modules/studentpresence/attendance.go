package studentpresence

import (
	"context"
	"errors"
	"time"
)

// Attendance is one recorded school stay. Date is a YYYY-MM-DD calendar day.
type Attendance struct {
	ID                 int64
	TenantID           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	StudentID          int64
	Date               string
	CheckInTime        time.Time
	CheckOutTime       *time.Time
	CheckedInBy        int64
	CheckedOutBy       *int64
	DeviceID           int64
	CheckedOutDeviceID *int64
	YardSince          *time.Time
}

// AttendanceFilter restricts an attendance history query. Nil ID lists mean
// unrestricted; non-nil empty lists mean no matches.
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

var ErrAttendanceNotFound = errors.New("attendance not found")

type AttendanceQuery interface {
	AttendanceSummaryQuery
	ListSchoolStatuses(context.Context, []int64, string) ([]SchoolStatus, error)
	FindAttendance(context.Context, int64) (*Attendance, error)
	ListAttendance(context.Context, AttendanceFilter) ([]Attendance, error)
}
type AttendanceCommand interface {
	AttendanceHistoryCommand
	EnsureAttendance(context.Context, Attendance) (Attendance, bool, error)
	CloseAttendance(context.Context, AttendanceCheckout) ([]Attendance, error)
}

func (m *Module) FindAttendance(ctx context.Context, id int64) (*Attendance, error) {
	return m.engine.FindAttendance(ctx, id)
}
func (m *Module) ListAttendance(ctx context.Context, filter AttendanceFilter) ([]Attendance, error) {
	return m.engine.ListAttendance(ctx, filter)
}
func (m *Module) EnsureAttendance(ctx context.Context, attendance Attendance) (Attendance, bool, error) {
	return m.engine.EnsureAttendance(ctx, attendance)
}
func (m *Module) CloseAttendance(ctx context.Context, checkout AttendanceCheckout) ([]Attendance, error) {
	return m.engine.CloseAttendance(ctx, checkout)
}
