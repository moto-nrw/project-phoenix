package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

type attendanceStaffRecords interface {
	FindByID(context.Context, any) (*users.Staff, error)
	FindByIDForUpdate(context.Context, int64) (*users.Staff, error)
	FindByIDs(context.Context, []int64) (map[int64]*users.Staff, error)
}

type attendanceStaffDirectory struct{ source attendanceStaffRecords }

func NewAttendanceStaffDirectory(source attendanceStaffRecords) presenceservice.AttendanceStaff {
	return attendanceStaffDirectory{source: source}
}

func (q attendanceStaffDirectory) LockStaffExists(ctx context.Context, id int64) (bool, error) {
	staff, err := q.source.FindByIDForUpdate(ctx, id)
	return staff != nil, err
}

func (q attendanceStaffDirectory) ExistingStaffIDs(ctx context.Context, ids []int64) (map[int64]struct{}, error) {
	rows, err := q.source.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	found := make(map[int64]struct{}, len(rows))
	for id := range rows {
		found[id] = struct{}{}
	}
	return found, nil
}

func (q attendanceStaffDirectory) StaffTenantID(ctx context.Context, id int64) (*int64, error) {
	staff, err := q.source.FindByID(ctx, id)
	if err != nil || staff == nil {
		return nil, err
	}
	tenantID := staff.TenantID
	return &tenantID, nil
}
