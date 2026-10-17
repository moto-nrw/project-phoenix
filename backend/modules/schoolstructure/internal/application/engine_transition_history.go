package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// The ledger commands run only inside the deletion workflow's tenant
// transaction. They take the tenant from context like ListGroups does and
// never open a transaction of their own.

func (e Engine) CountStudentTransitionHistory(ctx context.Context, studentID int64) (int, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	count, err := e.service.CountStudentTransitionHistory(ctx, tenantID.Int64(), studentID)
	return count, MapError(err)
}

func (e Engine) AnonymizeStudentTransitionHistory(ctx context.Context, studentID int64) (int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := e.service.AnonymizeStudentTransitionHistory(ctx, tenantID.Int64(), studentID)
	return rows, MapError(err)
}
