// Package timetable — shared substitute/deviation response + broadcast helpers.
//
// The former POST /api/timetable/substitute endpoint was consolidated into
// POST /instances/{id}/deviations (#1886), and the plan/classify/write logic
// moved into services/schedule (InstanceService.ApplyDeviations, #1840). What
// remains here is the wire response row (AffectedInstance), the shared reason
// normalizer, the plannable-status predicate (both still unit-tested here), and
// the post-save SSE broadcast helpers the handlers drive.
package timetable

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf8"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
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
// cancelled ones are historical record. Mirrors the service-side predicate used
// by ApplyDeviations.
func isPlannableInstance(instance *scheduleModel.ActivityInstance) bool {
	return instance.Status == scheduleModel.InstanceStatusPlanned ||
		instance.Status == scheduleModel.InstanceStatusActive
}

// broadcastDeviationSaveEvents fires the SSE signals after a /deviations save:
// the group-scoped activity_update per touched active group, plus — when the
// save actually changed staffing state (any applied write, an acknowledgement
// change, or a cleared stale ack) — the tenant-wide staffing_deviation_changed
// invalidation (#1844).
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
// lifecycle event, and the group-scoped activity_update only reaches clients
// subscribed to the active group — so without this signal an open staff page
// (Betreuungsplan card, planner) stays stale until reload (#1844). source names
// the emitting flow for log review.
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
