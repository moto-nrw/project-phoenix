package services

import (
	"context"
	"fmt"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

type workSessionStaffRecords interface {
	staffNameRecords
	FindByID(context.Context, any) (*users.Staff, error)
	Update(context.Context, *users.Staff) error
}

type workSessionStaff struct {
	source    workSessionStaffRecords
	schedules staffScheduleRecords
}

// WorkSessionStaff reads names and writes the schedule binding through the
// staff records, and reads the binding from Workforce's employment query.
func WorkSessionStaff(source workSessionStaffRecords, schedules staffScheduleRecords) workforceCompose.WorkSessionStaff {
	return workSessionStaff{source: source, schedules: schedules}
}

func (q workSessionStaff) ScheduleAssignment(ctx context.Context, staffID int64) (*workforceCompose.StaffScheduleAssignment, error) {
	return StaffScheduleAssignments(q.schedules).ScheduleAssignment(ctx, staffID)
}

func (q workSessionStaff) StaffNames(ctx context.Context, ids []int64) (map[int64]workforceCompose.WorkSessionStaffName, error) {
	rows, err := q.source.FindWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]workforceCompose.WorkSessionStaffName, len(rows))
	for id, row := range rows {
		if row != nil && row.Person != nil {
			names[id] = workforceCompose.WorkSessionStaffName{FirstName: row.Person.FirstName, LastName: row.Person.LastName}
		}
	}
	return names, nil
}

// BindSchedule writes the schedule binding onto the staff row through the
// owner's update path: the row is re-read so the write carries the current
// master data and only the binding columns change.
func (q workSessionStaff) BindSchedule(ctx context.Context, binding workforceCompose.StaffScheduleBinding) error {
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
