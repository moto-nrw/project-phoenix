package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

type staffScheduleRecords interface {
	FindByID(context.Context, any) (*users.Staff, error)
}

type staffScheduleAssignments struct{ source staffScheduleRecords }

func StaffScheduleAssignments(source staffScheduleRecords) timetracking.StaffScheduleQuery {
	return staffScheduleAssignments{source: source}
}

func (q staffScheduleAssignments) ScheduleAssignment(ctx context.Context, staffID int64) (*timetracking.StaffScheduleAssignment, error) {
	staff, err := q.source.FindByID(ctx, staffID)
	if err != nil || staff == nil {
		return nil, err
	}
	return &timetracking.StaffScheduleAssignment{WorkTimeModelID: staff.WorkTimeModelID, RotationAnchorDate: staff.RotationAnchorDate}, nil
}
