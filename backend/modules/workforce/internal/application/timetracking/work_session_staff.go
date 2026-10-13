package timetracking

import (
	"context"
)

type WorkSessionStaffName struct {
	FirstName string
	LastName  string
}

type WorkSessionStaff interface {
	StaffScheduleQuery
	StaffNames(context.Context, []int64) (map[int64]WorkSessionStaffName, error)
	// BindSchedule persists the work time model and rotation anchor of the
	// staff member.
	BindSchedule(context.Context, StaffScheduleBinding) error
}
