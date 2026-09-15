package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// StaffTimeTrackingNotifier returns the post-commit notification that
// invalidates the time-account views after a write changed a staff member's
// target working time. The composition root hands it to the work-time-model
// HTTP adapter, which must not reach the realtime platform itself.
func StaffTimeTrackingNotifier(broadcaster realtime.Broadcaster) func(context.Context) {
	return func(ctx context.Context) {
		realtimeevents.QueueStaffTimeTrackingChanged(ctx, broadcaster, nil)
	}
}
