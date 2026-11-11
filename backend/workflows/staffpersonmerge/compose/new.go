package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge"
)

type runtime struct{ operatorID func(context.Context) int64 }

func (r runtime) OperatorID(ctx context.Context) int64 { return r.operatorID(ctx) }
func (runtime) RequireAdmin(ctx context.Context) error {
	if !tenant.IsAdminTx(ctx) {
		return staffpersonmerge.ErrUnauthorized
	}
	return tenant.RequireTransaction(ctx)
}

// Lock takes the school's merge gate exclusively, admitted in the unit of
// work's lock order first (tenant.LockStaffPersonMerge, #6702).
func (runtime) Lock(ctx context.Context, schoolID int64) error {
	if err := tenant.OrderLock(ctx, tenant.LockStaffPersonMerge, tenant.LockIDKey(schoolID)); err != nil {
		return err
	}
	return tenant.AcquireLock(ctx, peopledirectory.StaffPersonMergeLockKey(schoolID), false)
}
func (runtime) MarkRollback(ctx context.Context) { tenant.MarkRollback(ctx) }
func New(people peopledirectory.StaffPersonMerge, memberships schoolmembership.StaffPersonMerge, audit staffpersonmerge.Audit, operatorID func(context.Context) int64) (*staffpersonmerge.Workflow, error) {
	if operatorID == nil {
		return nil, fmt.Errorf("staff person merge: operator resolver required")
	}
	return staffpersonmerge.New(people, memberships, runtime{operatorID: operatorID}, audit)
}
