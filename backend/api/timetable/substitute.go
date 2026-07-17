// Package timetable — shared substitute/deviation response helpers.
//
// The former POST /api/timetable/substitute endpoint was consolidated into
// POST /instances/{id}/deviations (#1886): its only frontend caller (the
// Betreuungsplan gap-fill quick action) now sends the same day-wide
// substitution through the atomic deviations save. The classification and
// planning logic moved into services/schedule (TimetableDataService.PlanDeviations,
// #1886); what remains here is the shared response row (AffectedInstance), the
// reason normalizer, the plannable-status predicate, and the SSE broadcast
// helpers — all consumed by the /deviations and /acknowledge-understaffed
// handlers.
package timetable

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf8"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// substituteActionType is the stable per-instance action string used when
// building the affected_instances response rows. The action vocabulary lives
// in services/schedule (single source, #1886); these aliases keep the handler
// code and wire format unchanged.
const (
	// Absent-only mode (#1840): substitute_staff_id omitted. The absent staff
	// is marked absent and the position is left open.
	substituteActionMarkedAbsent  = scheduleSvc.SubstituteActionMarkedAbsent
	substituteActionAlreadyAbsent = scheduleSvc.SubstituteActionAlreadyAbsent
	// Present mode (#1840): a persisted day-wide absence was cleared.
	substituteActionMarkedPresent = scheduleSvc.SubstituteActionMarkedPresent
)

// AffectedInstance is one row in the affected_instances list of the response.
type AffectedInstance struct {
	InstanceID int64  `json:"instance_id"`
	Title      string `json:"title"`
	StartTime  string `json:"start_time"`
	Action     string `json:"action"`
}

// trimReason normalizes an optional deviation reason: nil/blank becomes nil,
// and an over-long value is truncated to the shared note ceiling so a single
// oversized field can never bloat a row. The ceiling counts runes, not bytes:
// slicing on a byte offset can split a multi-byte UTF-8 rune, producing an
// invalid string that PostgreSQL rejects. The frontend's maxLength is likewise
// a character count, so a rune ceiling keeps the two limits consistent.
func trimReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*reason)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > understaffedAckNoteMaxLength {
		trimmed = string([]rune(trimmed)[:understaffedAckNoteMaxLength])
	}
	return &trimmed
}

// isPlannableInstance reports whether a substitute/absence write may touch this
// instance. Only planned and active blocks are editable: completed and
// cancelled ones are historical record. Mirrors the /gaps candidate filter and
// DeleteUpcomingByStaffID's "keep same-day history" rule.
func isPlannableInstance(instance *scheduleModel.ActivityInstance) bool {
	return instance.Status == scheduleModel.InstanceStatusPlanned ||
		instance.Status == scheduleModel.InstanceStatusActive
}

// broadcastDeviationSaveEvents fires the SSE signals after a /deviations save:
// the group-scoped activity_update per touched active group, plus — when the
// save actually changed staffing state (any applied write, an acknowledgement
// change, or a cleared stale ack) — the tenant-wide staffing_deviation_changed
// invalidation (#1844). Kept out of applyDeviations so the handler's gocognit
// score stays within its ratchet allowance.
func (rs *Resource) broadcastDeviationSaveEvents(
	ctx context.Context,
	touched map[int64]*scheduleModel.ActivityInstance,
	appliedWrites int,
	ackChanged bool,
	clearedAcks int,
) {
	rs.InstanceService.QueueActivityUpdates(ctx, touched)
	if appliedWrites > 0 || ackChanged || clearedAcks > 0 {
		rs.broadcastStaffingDeviationChanged(ctx, "deviations")
	}
}

// broadcastStaffingDeviationChanged queues one tenant-wide
// staffing_deviation_changed event after the surrounding tenant transaction
// commits. Deviation writes on still-planned blocks emit no instance_*
// lifecycle event, and the group-scoped activity_update below only reaches
// clients subscribed to the active group — so without this signal an open
// staff page (Betreuungsplan card, planner) stays stale until reload (#1844).
// source names the emitting flow for log review.
func (rs *Resource) broadcastStaffingDeviationChanged(ctx context.Context, source string) {
	if rs.Broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventStaffingDeviationChanged, "", realtime.EventData{Source: &source})
	tenant.RegisterAfterCommit(ctx, func() {
		if err := rs.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			rs.getLogger().Warn("SSE staffing deviation broadcast failed",
				slog.String("source", source),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
	})
}
