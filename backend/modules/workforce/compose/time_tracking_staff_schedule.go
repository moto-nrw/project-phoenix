package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// StaffEmploymentQuery is the Workforce employment read: the template binding
// and rotation anchor are Workforce's own facts (#2753), so the schedule
// resolution reads them from their owner in one statement.
type StaffEmploymentQuery interface {
	StaffEmployments(context.Context, []int64) (map[int64]workforce.StaffEmployment, error)
}

// StaffScheduleAssignments serves the time-tracking schedule query from the
// Workforce employment profiles.
func StaffScheduleAssignments(source StaffEmploymentQuery) StaffScheduleQuery {
	return retainedStaffScheduleAssignments{Source: source}
}
