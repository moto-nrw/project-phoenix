package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/schoolmembership"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// StaffPersonMerge admits the school's staff rows, ascending, in the unit of
// work's lock order before the merge locks them (tenant.LockStaffRows,
// #6702). The merge holds the school's merge gate exclusively, so no staff
// row is created between the unlocked read and the lock.
type StaffPersonMerge struct {
	schoolmembership.StaffPersonMerge
}

func (m StaffPersonMerge) StaffMergeMemberships(ctx context.Context, schoolID int64, lock bool) ([]schoolmembership.Staff, error) {
	if lock {
		rows, err := m.StaffPersonMerge.StaffMergeMemberships(ctx, schoolID, false)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if err := tenant.OrderLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(row.ID)); err != nil {
				return nil, err
			}
		}
	}
	return m.StaffPersonMerge.StaffMergeMemberships(ctx, schoolID, lock)
}
