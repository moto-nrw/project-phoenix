package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func LockStaffShiftsGate(ctx context.Context, staffID int64, lock func(context.Context, string) error) error {
	if staffID <= 0 || tenant.FromContext(ctx) <= 0 {
		return errors.New("workforce: staff and tenant are required")
	}
	return lock(ctx, fmt.Sprintf("staff-shift:%d:%d", tenant.FromContext(ctx), staffID))
}
