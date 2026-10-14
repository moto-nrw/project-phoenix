package compose

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StaffEmploymentQuery is the Workforce employment read: the template binding
// and rotation anchor are Workforce's own facts (#2753), so the schedule
// resolution reads them from their owner in one statement.
type StaffEmploymentQuery interface {
	StaffEmployments(context.Context, []int64) (map[int64]workforce.StaffEmployment, error)
}

type staffScheduleAssignments struct{ source StaffEmploymentQuery }

// StaffScheduleAssignments serves the time-tracking schedule query from the
// Workforce employment profiles.
func StaffScheduleAssignments(source StaffEmploymentQuery) StaffScheduleQuery {
	return staffScheduleAssignments{source: source}
}

func (q staffScheduleAssignments) ScheduleAssignment(ctx context.Context, staffID int64) (*StaffScheduleAssignment, error) {
	profiles, err := q.source.StaffEmployments(ctx, []int64{staffID})
	if err != nil {
		return nil, err
	}
	profile, ok := profiles[staffID]
	if !ok {
		return nil, fmt.Errorf("staff %d has no employment profile: %w", staffID, sql.ErrNoRows)
	}
	var anchor *timezone.Date
	if profile.RotationAnchorDate != "" {
		date := timezone.Date(profile.RotationAnchorDate)
		anchor = &date
	}
	return &StaffScheduleAssignment{WorkTimeModelID: profile.WorkTimeModelID, RotationAnchorDate: anchor}, nil
}
