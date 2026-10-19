package api

import (
	"context"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// staffTimeTrackingNotifier returns the post-commit notification that
// invalidates the time-account views after a write changed a staff member's
// target working time. The root hands it to the work-time-model HTTP
// adapter, which must not reach the realtime platform itself.
func staffTimeTrackingNotifier(broadcaster realtime.Broadcaster) func(context.Context) {
	return func(ctx context.Context) {
		deliveryCompose.QueueStaffTimeTrackingChanged(ctx, broadcaster, nil)
	}
}
