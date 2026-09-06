package domain

import (
	"errors"
	"time"
)

var ErrStudentEnrollmentNotFound = errors.New("student enrollment not found")

const StudentEnrollmentActiveIndex = "idx_student_enrollments_active"

type StudentEnrollment struct {
	ID                       int64
	TenantID                 int64
	CreatedAt                time.Time
	UpdatedAt                time.Time
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

type StudentEnrollmentFields struct {
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
