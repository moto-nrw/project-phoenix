package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type workSessionStaffRecords interface {
	staffScheduleRecords
	staffNameRecords
	Update(context.Context, *users.Staff) error
}

type workSessionStaff struct{ source workSessionStaffRecords }

func WorkSessionStaff(source workSessionStaffRecords) active.WorkSessionStaff {
	return workSessionStaff{source: source}
}

func (q workSessionStaff) ScheduleAssignment(ctx context.Context, staffID int64) (*active.StaffScheduleAssignment, error) {
	return StaffScheduleAssignments(q.source).ScheduleAssignment(ctx, staffID)
}

func (q workSessionStaff) StaffNames(ctx context.Context, ids []int64) (map[int64]active.WorkSessionStaffName, error) {
	rows, err := q.source.FindWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]active.WorkSessionStaffName, len(rows))
	for id, row := range rows {
		if row != nil && row.Person != nil {
			names[id] = active.WorkSessionStaffName{FirstName: row.Person.FirstName, LastName: row.Person.LastName}
		}
	}
	return names, nil
}

// BindSchedule writes the schedule binding onto the staff row through the
// owner's update path: the row is re-read so the write carries the current
// master data and only the binding columns change.
func (q workSessionStaff) BindSchedule(ctx context.Context, binding active.StaffScheduleBinding) error {
	staff, err := q.source.FindByID(ctx, binding.ID)
	if err != nil {
		return err
	}
	if staff == nil {
		return fmt.Errorf("bind schedule: staff %d not found", binding.ID)
	}
	staff.WorkTimeModelID = binding.WorkTimeModelID
	staff.RotationAnchorDate = binding.RotationAnchorDate
	return q.source.Update(ctx, staff)
}
