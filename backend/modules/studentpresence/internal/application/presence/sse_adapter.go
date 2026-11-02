package presence

import (
	"context"
	"strconv"
)

// broadcastSupervisionRefresh sends the tenant-wide refresh used by attendance
// and activity changes. The dashboard event carries the union of the former
// dashboard-count and active-supervision invalidation scopes.
//
// Carries no child identity (#2085). The scoped student_checkin /
// student_checkout emitted alongside it still carry the id.
func (s *service) broadcastSupervisionRefresh(ctx context.Context, activeGroupID, reason string, eduGroupIDs []string) {
	if s.Broadcaster == nil {
		return
	}
	s.Broadcaster.SupervisionRefresh(ctx, activeGroupID, reason, eduGroupIDs)
}

// queueSupervisionRefresh announces a change to a session, its supervisors or
// its visits once the surrounding transaction commits (#5017): „Aktuelle
// Aufsicht", the own supervision and the room views show them, and without
// the event they would wait for the five-minute reconcile. No child identity
// travels.
func (s *service) queueSupervisionRefresh(ctx context.Context, activeGroupID int64, reason string) {
	if s.Broadcaster == nil || activeGroupID <= 0 {
		return
	}
	s.Broadcaster.SupervisionChanged(ctx, activeGroupID, reason)
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
	if s.Broadcaster == nil {
		return
	}
	s.Broadcaster.DashboardCountsChanged(ctx, eduGroupIDs)
}

// eduGroupIDsOf formats the known routing group for tenant invalidations.
// Nil keeps the field absent so clients fall back to a broad refresh.
func eduGroupIDsOf(groupID *int64) []string {
	if groupID == nil {
		return nil
	}
	return []string{strconv.FormatInt(*groupID, 10)}
}
