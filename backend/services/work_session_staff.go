package services

import (
	"context"

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

func (q workSessionStaff) Update(ctx context.Context, staff *users.Staff) error {
	return q.source.Update(ctx, staff)
}
