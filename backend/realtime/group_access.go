package realtime

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// QueueGroupAccessChanged emits the tenant-wide group_access_changed
// invalidation after the surrounding tenant transaction commits.
//
// Education writes call this through services/education, while staff
// offboarding calls it after its direct assignment cleanup. Handlers never
// emit. An education cascade then announces itself without each caller having
// to remember: deleting a group drops its teacher links AND its substitutions
// through the same service methods that emit.
//
// source names the emitting flow ("substitution_create", "group_teachers", …)
// and is the only field carried: the event reaches every staff client of the
// tenant, so it must not name the staff member (see EventGroupAccessChanged).
//
// A zero tenant ID means the caller is outside a tenant-scoped request; there
// is nobody to broadcast to, so the call is a no-op instead of a broadcast to
// tenant 0.
//
// One cascade may queue several events (a group delete emits per removed
// teacher link and per removed substitution). That is deliberate: the broadcast
// is a non-blocking channel send and clients debounce a burst into a single
// refetch, so deduplicating here would add bookkeeping for no measurable gain.
func QueueGroupAccessChanged(ctx context.Context, broadcaster Broadcaster, logger *slog.Logger, source string) {
	if broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	tenant.RegisterAfterCommit(ctx, func() {
		event := NewEvent(EventGroupAccessChanged, "", EventData{Source: &source})
		if err := broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			logger.Warn("SSE group access broadcast failed",
				slog.String("source", source),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
	})
}
