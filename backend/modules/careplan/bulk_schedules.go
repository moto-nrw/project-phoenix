package careplan

import (
	"context"
	"errors"
)

var (
	ErrBulkStudentUnauthorized = errors.New("bulk student selection contains an unauthorized student")
	ErrBulkStudentNotFound     = errors.New("bulk student selection contains a missing student")
)

// ScheduleStudent identifies an existing student in the current tenant after
// the bulk command has locked and rechecked the student's eligibility.
type ScheduleStudent struct {
	ID       int64
	TenantID int64
}

func (s ScheduleStudent) IsAuthorizationStudent() bool { return s.ID > 0 }

type PickupBulkFilter struct {
	StudentIDs []int64
	Authorize  func(context.Context, ScheduleStudent) (bool, error)
}

type BulkUpsertResult struct {
	StudentsAffected    int                `json:"students_affected"`
	OverwrittenStudents []OverwriteWarning `json:"overwritten_students,omitempty"`
	AffectedStudentIDs  []int64            `json:"-"`
}

type OverwriteWarning struct {
	StudentID    int64  `json:"student_id"`
	StudentName  string `json:"student_name"`
	Weekday      int    `json:"weekday"`
	WeekdayName  string `json:"weekday_name"`
	PreviousTime string `json:"previous_time"`
	NewTime      string `json:"new_time"`
}
