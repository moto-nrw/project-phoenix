package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func NewStaffPersonMerge() schoolmembership.StaffPersonMerge {
	return admittedStaffPersonMerge{postgres.New(postgres.NewContextDatabase(tenant.TenantFromContext, tenant.TransactionFromContext, tenant.IsAdminTx))}
}

// admittedStaffPersonMerge admits the school's staff rows, ascending, in the
// unit of work's lock order before the merge locks them (tenant.LockStaffRows,
// #6702). The merge holds the school's merge gate exclusively, so no staff
// row is created between the unlocked read and the lock.
type admittedStaffPersonMerge struct {
	*postgres.Store
}

func (m admittedStaffPersonMerge) StaffMergeMemberships(ctx context.Context, schoolID int64, lock bool) ([]schoolmembership.Staff, error) {
	if lock {
		rows, err := m.Store.StaffMergeMemberships(ctx, schoolID, false)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if err := tenant.OrderLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(row.ID)); err != nil {
				return nil, err
			}
		}
	}
	return m.Store.StaffMergeMemberships(ctx, schoolID, lock)
}
