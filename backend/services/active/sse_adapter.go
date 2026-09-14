package active

import (
	"context"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
)

// broadcastSupervisionRefresh sends the tenant-wide refresh used by attendance
// and activity changes. The dashboard event carries the union of the former
// dashboard-count and active-supervision invalidation scopes.
//
// Carries no child identity (#2085). The scoped student_checkin /
// student_checkout emitted alongside it still carry the id.
func (s *service) broadcastSupervisionRefresh(ctx context.Context, activeGroupID, reason string, eduGroupIDs []string) {
	realtimeevents.PublishSupervisionRefresh(ctx, s.Broadcaster, s.getLogger(), activeGroupID, reason, eduGroupIDs)
}

// broadcastDashboardCountsChanged sends the tenant-wide dashboard refresh
// signal (#2057). eduGroupIDs are the affected educational (OGS) group ids —
// group ids only, never student identity — so clients can scope their
// ogs-students-{gid} revalidation instead of refetching every group list.
// Empty/nil ids omit the field entirely (clients then refresh broadly);
// callers must never pass a deliberately-empty-but-known scope.
//
// Tenant-scoped on purpose: the old BroadcastToAll fanned every school's
// check-in traffic out to every other school's clients, multiplying the
// refetch herd across tenants.
func (s *service) broadcastDashboardCountsChanged(ctx context.Context, eduGroupIDs []string) {
	realtimeevents.PublishDashboardCountsChanged(ctx, s.Broadcaster, s.getLogger(), eduGroupIDs)
}

// eduGroupIDsOf formats the known routing group for tenant invalidations.
// Nil keeps the field absent so clients fall back to a broad refresh.
func eduGroupIDsOf(groupID *int64) []string {
	if groupID == nil {
		return nil
	}
	return []string{strconv.FormatInt(*groupID, 10)}
}
