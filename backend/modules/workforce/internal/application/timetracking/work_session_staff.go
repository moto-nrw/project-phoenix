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

// ExternalCaregiverStaff tells whether a staff member is an external
// caregiver (#3823), with or without an account (#4573). Externals record no
// working time, so starting a supervision opens no work session for them.
// The root binds it to School Membership's rule.
type ExternalCaregiverStaff interface {
	IsExternalCaregiver(ctx context.Context, staffID int64) (bool, error)
}
