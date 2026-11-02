package realtimeevents

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// QueueArrivalScheduleChanged emits the arrival-schedule event after the
// surrounding transaction commits, so Aufsicht and Meine Gruppe refetch. A
// class-wide change concerns every child of the class, so no student ID
// travels with it; the source is the school portal.
func QueueArrivalScheduleChanged(ctx context.Context, broadcaster TenantPublisher, logger *slog.Logger) {
	if broadcaster == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return
	}
	tenant.RegisterAfterCommit(ctx, func() {
		source := "school"
		event := realtime.NewEvent(realtime.EventArrivalScheduleChanged, "", realtime.EventData{Source: &source})
		if err := broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			logger.Warn("failed to broadcast arrival schedule change",
				"error", err.Error(),
			)
		}
	})
}
