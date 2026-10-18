package application

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// AnnounceOfferingPickupChange announces a changed offering pickup projection
// once the tenant transaction committed, so a rolled-back change stays
// silent: staff views refetch the school's pickup schedule, and every
// affected child's card refreshes for the guardians. Either audience may be
// absent; without both it returns nil and nothing is announced.
func AnnounceOfferingPickupChange(staff ports.PickupScheduleStaff, guardians ports.PickupScheduleGuardians, hooks ports.TransactionHooks, logger *slog.Logger) func(context.Context, []int64) {
	if staff == nil && guardians == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context, studentIDs []int64) {
		tenantID := hooks.TenantID(ctx)
		hooks.AfterCommit(ctx, func() {
			if tenantID <= 0 || len(studentIDs) == 0 {
				return
			}
			if staff != nil {
				if err := staff.PickupScheduleChanged(tenantID); err != nil {
					logger.Warn(
						"offering pickup projection: failed to broadcast schedule change",
						slog.Int64("tenant_id", tenantID),
						slog.String("error", err.Error()),
					)
				}
			}
			if guardians != nil {
				for _, studentID := range studentIDs {
					guardians.BroadcastChildUpdateToGuardians(tenantID, studentID)
				}
			}
		})
	}
}
