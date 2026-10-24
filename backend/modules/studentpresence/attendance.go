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
	// CheckOutNote is why the child went home earlier than planned (#3324).
	CheckOutNote *string
}

// IsOpen reports a school attendance that is checked in and not yet checked
// out.
func (a Attendance) IsOpen() bool { return a.CheckOutTime == nil }

// AttendanceCompletion reports whether the rows hold an open attendance and
// whether they hold a completed one; a day can hold both.
func AttendanceCompletion(rows []Attendance) (open, completed bool) {
	for _, row := range rows {
		if row.IsOpen() {
			open = true
		} else {
			completed = true
		}
	}
	return open, completed
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
	// Note is the optional reason for an early checkout (#3324). Empty
	// leaves the column NULL.
	Note string
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
