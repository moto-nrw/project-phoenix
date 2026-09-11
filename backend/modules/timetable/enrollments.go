package timetable

import (
	"context"
	"time"
)

const (
	AttendancePresent = "PRESENT"
	AttendanceAbsent  = "ABSENT"
	AttendanceExcused = "EXCUSED"
	AttendanceUnknown = "UNKNOWN"
)

// StudentEnrollment is the owner view of one activities.student_enrollments row.
// Calendar dates stay as YYYY-MM-DD strings at the module boundary.
type StudentEnrollment struct {
	ID                       int64     `json:"id"`
	TenantID                 int64     `json:"tenant_id"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
	StudentID                int64     `json:"student_id"`
	ActivityGroupID          int64     `json:"activity_group_id"`
	ValidFrom                string    `json:"valid_from"`
	ValidUntil               *string   `json:"valid_until,omitempty"`
	CalendarPeriodID         *int64    `json:"calendar_period_id,omitempty"`
	EnrollmentRequestChildID *int64    `json:"enrollment_request_child_id,omitempty"`
	SelectedWeekdays         []int     `json:"selected_weekdays,omitempty"`
	AttendanceStatus         *string   `json:"attendance_status,omitempty"`
	Weekday                  *int      `json:"weekday,omitempty"`
}

type StudentEnrollmentInput struct {
	StudentID                int64
	ActivityGroupID          int64
	ValidFrom                string
	ValidUntil               *string
	CalendarPeriodID         *int64
	EnrollmentRequestChildID *int64
	SelectedWeekdays         []int
	AttendanceStatus         *string
	Weekday                  *int
}

type StudentEnrollmentFilter struct {
	StudentIDs       []int64
	ActivityGroupIDs []int64
	ActiveOn         *string
	Limit            int
	Offset           int
	OrderByValidFrom bool
	OrderByGroupName bool
}

type StudentEnrollmentQuery interface {
	FindStudentEnrollment(context.Context, int64) (StudentEnrollment, error)
	ListStudentEnrollments(context.Context, StudentEnrollmentFilter) ([]StudentEnrollment, error)
}

type StudentEnrollmentCommand interface {
	CreateStudentEnrollment(context.Context, StudentEnrollmentInput) (StudentEnrollment, error)
	UpdateStudentEnrollment(context.Context, int64, StudentEnrollmentInput) (StudentEnrollment, error)
	DeleteStudentEnrollment(context.Context, int64) error
	BackfillStudentEnrollmentSource(context.Context, int64, int64, []int64) (int64, error)
	DeleteStudentEnrollmentsBySource(context.Context, int64, int64) (int64, error)
	CapActiveStudentEnrollments(context.Context, int64, string) (int64, error)
	SetStudentEnrollmentValidUntil(context.Context, int64, string) error
	CloseOpenStudentEnrollments(context.Context, int64, *int64, string) error
}

type StudentEnrollmentCapability interface {
	StudentEnrollmentQuery
	StudentEnrollmentCommand
}
