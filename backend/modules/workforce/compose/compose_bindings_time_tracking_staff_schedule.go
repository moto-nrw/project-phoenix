package compose

import (
	"context"
	"database/sql"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
)

type retainedStaffScheduleAssignments struct {
	Source interface {
		StaffEmployments(context.Context, []int64) (map[int64]workforce.StaffEmployment, error)
	}
}

func (q retainedStaffScheduleAssignments) ScheduleAssignment(ctx context.Context, staffID int64) (*timetracking.StaffScheduleAssignment, error) {
	return timetracking.ReadScheduleAssignment(ctx, staffID, q.Source, sql.ErrNoRows)
}
