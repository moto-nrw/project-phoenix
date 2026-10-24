package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func LockStaffQualificationsGate(ctx context.Context, staffID int64, lock func(context.Context, string) error) error {
	if staffID <= 0 {
		return errors.New("workforce compose: staff id is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("workforce compose: tenant id is required")
	}
	if err := lock(ctx, fmt.Sprintf("staff-qualifications:%d:%d", tenantID, staffID)); err != nil {
		return fmt.Errorf("lock staff qualification writes: %w", err)
	}
	return nil
}
