package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func LockStaffBalanceGate(ctx context.Context, staffID int64, lock func(context.Context, string) error) error {
	if staffID <= 0 {
		return errors.New("workforce compose: staff id is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("workforce compose: tenant id is required")
	}
	if err := lock(ctx, fmt.Sprintf("staff-balance:%d:%d", tenantID, staffID)); err != nil {
		return fmt.Errorf("lock staff balance writes: %w", err)
	}
	return nil
}
