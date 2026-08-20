package active

import (
	"context"
	"log/slog"
	"strconv"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// broadcastSupervisionRefresh sends the tenant-wide refresh used by attendance
// and activity changes. It preserves both legacy event types during a rolling
// deploy: older clients use active_supervision_changed for supervision rosters,
// while current clients recognize dashboard_counts_changed with Reason.
//
// Carries no child identity (#2085). The scoped student_checkin /
// student_checkout emitted alongside it still carry the id.
func (s *service) broadcastSupervisionRefresh(ctx context.Context, activeGroupID, reason string, eduGroupIDs []string) {
	if s.Broadcaster == nil {
		return
	}

	data := realtime.EventData{
		Reason: &reason,
	}
	if len(eduGroupIDs) > 0 {
		data.GroupIDs = &eduGroupIDs
	}

	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventDashboardCountsChanged, activeGroupID, data)
	if err := s.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.getLogger().Warn("SSE combined supervision broadcast failed",
			slog.String("error", err.Error()),
			slog.String("active_group_id", activeGroupID),
			slog.String("reason", reason),
			slog.Int64("tenant_id", tenantID),
		)
	}

	legacyEvent := realtime.NewEvent(realtime.EventActiveSupervisionChanged, activeGroupID, data)
	if err := s.Broadcaster.BroadcastToTenant(tenantID, legacyEvent); err != nil {
		s.getLogger().Warn("SSE legacy supervision broadcast failed",
			slog.String("error", err.Error()),
			slog.String("active_group_id", activeGroupID),
			slog.String("reason", reason),
			slog.Int64("tenant_id", tenantID),
		)
	}
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

	data := realtime.EventData{}
	if len(eduGroupIDs) > 0 {
		data.GroupIDs = &eduGroupIDs
	}

	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventDashboardCountsChanged, "", data)
	if err := s.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.getLogger().Warn("SSE dashboard counts broadcast failed",
			slog.String("error", err.Error()),
			slog.Int64("tenant_id", tenantID),
		)
	}
}

// eduGroupIDsOf returns the educational group id of a student as a slice for
// tenant invalidations / EventData.GroupIDs, or nil when unknown
// (nil student — e.g. a repository error during routing-data lookup — or a student
// without an OGS group). nil keeps the field absent so clients fall back to a
// broad refresh instead of scoping to nothing.
func eduGroupIDsOf(student *userModels.Student) []string {
	if student == nil || student.GroupID == nil {
		return nil
	}
	return []string{strconv.FormatInt(*student.GroupID, 10)}
}
