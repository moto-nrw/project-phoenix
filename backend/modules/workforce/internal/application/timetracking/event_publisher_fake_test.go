package timetracking

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// recordingTimeTrackingPublisher captures every time-account change the
// retained services announce, keyed by the tenant of the announcing context.
// The Delivery producer bound by the composition root defers the actual
// broadcast until the surrounding transaction commits.
type recordingTimeTrackingPublisher struct {
	tenants []int64
}

func (r *recordingTimeTrackingPublisher) QueueStaffTimeTrackingChanged(ctx context.Context, _ *slog.Logger) {
	r.tenants = append(r.tenants, tenant.FromContext(ctx))
}
