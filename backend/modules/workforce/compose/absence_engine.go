package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// LockStaffAbsence serializes overlap-sensitive absence writes of one staff
// member. It uses the same key the legacy repository did, so every writer of
// the same rows keeps queuing on one lock.
func (t transaction) LockStaffAbsence(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return errors.New("workforce compose: staff id is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("workforce compose: tenant id is required")
	}
	if err := t.acquireXactLock(ctx, fmt.Sprintf("staff-absence:%d:%d", tenantID, staffID)); err != nil {
		return fmt.Errorf("lock staff absence writes: %w", err)
	}
	return nil
}

// --- absence types ---

// --- audit ---

// --- group substitutions ---

// --- mapping ---
