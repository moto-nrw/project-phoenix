package compose

import (
	"context"

	compositionBinding "github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
)

// --- absence types ---

// --- audit ---

// --- group substitutions ---

// --- mapping ---

// LockStaffAbsence serializes overlap-sensitive absence writes of one staff
// member. It uses the same key the legacy repository did, so every writer of
// the same rows keeps queuing on one lock.
func (t transaction) LockStaffAbsence(ctx context.Context, staffID int64) error {
	return compositionBinding.LockStaffAbsenceGate(ctx, staffID, t.acquireXactLock)
}
