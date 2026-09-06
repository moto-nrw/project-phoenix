package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (e engine) LockOpenStudentAssignments(ctx context.Context, studentIDs []int64) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("timetable: assignment locks require a transaction")
	}
	return mapError(e.service.LockOpenStudentAssignments(ctx, studentIDs))
}

func (e engine) ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, studentIDs, pickupExceptionIDs []int64, removals []timetable.InstanceStudent) error {
	return mapError(e.service.ReconnectCareExitAssignmentPickupExceptions(ctx, studentIDs, pickupExceptionIDs, domainCareExitAssignments(removals)))
}
