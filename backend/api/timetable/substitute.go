// Package timetable — shared substitute/deviation classification helpers.
//
// The former POST /api/timetable/substitute endpoint was consolidated into
// POST /instances/{id}/deviations (#1886): its only frontend caller (the
// Betreuungsplan gap-fill quick action) now sends the same day-wide
// substitution through the atomic deviations save. What remains here is the
// pure classification logic (classifySubstitute), the shared response row
// (AffectedInstance), the time-conflict advisory builder, and the SSE
// broadcast helper — all consumed by deviations.go. The write path lives in
// services/schedule (ApplyAbsence/ApplySubstitute, #1886).
package timetable

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// substituteActionType is the stable per-instance action string returned in
// the response. Callers switch on it rather than parsing messages.
// The action vocabulary lives in services/schedule (single source, #1886);
// these aliases keep the handler code and wire format unchanged.
const (
	substituteActionSubstituted       = scheduleSvc.SubstituteActionSubstituted
	substituteActionAlreadySubstitute = scheduleSvc.SubstituteActionAlreadySubstitute
	substituteActionAlreadyOnInstance = scheduleSvc.SubstituteActionAlreadyOnInstance
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

// plannedOp holds one classified target instance and the decision of what
// Phase B should do with it. Both pointer fields are always populated —
// Phase A pushes an op only after successfully loading the instance and
// classifying the origRow.
type plannedOp struct {
	instance *scheduleModel.ActivityInstance
	origRow  *scheduleModel.InstanceStaff
	action   string
}

// writeOp maps the handler-side classification onto the service write input.
func (op plannedOp) writeOp() scheduleSvc.SubstituteWriteOp {
	return scheduleSvc.SubstituteWriteOp{
		Instance: op.instance,
		OrigRow:  op.origRow,
		Action:   op.action,
	}
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
// cancelled ones are historical record. GetInstanceStaffByStaffAndDate returns
// every same-day assignment regardless of status, so a staff member with an
// already-finished morning block and a planned afternoon block would otherwise
// have the completed block's staffing rewritten. Mirrors the /gaps candidate
// filter and DeleteUpcomingByStaffID's "keep same-day history" rule.
func isPlannableInstance(instance *scheduleModel.ActivityInstance) bool {
	return instance.Status == scheduleModel.InstanceStatusPlanned ||
		instance.Status == scheduleModel.InstanceStatusActive
}

// classifySubstitute decides the action for a single target instance. Pure
// logic, no DB. Returns (action, conflictingOtherStaffID, ok=false on 409).
//
// allRows: every instance_staff row of the target instance.
// origRow: the absent's row on the target instance.
// subID:   the substitute's staff id.
func classifySubstitute(
	allRows []*scheduleModel.InstanceStaff,
	origRow *scheduleModel.InstanceStaff,
	subID int64,
) (action string, conflictOtherStaff int64, ok bool) {
	// Scan once for the signals we need: whether subID already has a row here,
	// whether subID is a present co-supervisor, and the block's coverage balance
	// (absent planned positions vs OTHER active substitutes already covering them).
	var existingSubOfSub *scheduleModel.InstanceStaff
	var subAsNonAbsent *scheduleModel.InstanceStaff
	var anyActiveSubOfOther *scheduleModel.InstanceStaff
	absentPlanned := 0
	activeSubsOfOther := 0
	for _, row := range allRows {
		switch {
		case row.IsSubstitute && row.StaffID == subID:
			existingSubOfSub = row
		case row.IsSubstitute && !row.IsAbsent:
			activeSubsOfOther++
			anyActiveSubOfOther = row
		case !row.IsSubstitute && row.IsAbsent:
			absentPlanned++
		}
		if !row.IsSubstitute && row.StaffID == subID && !row.IsAbsent {
			subAsNonAbsent = row
		}
	}

	// The substitute already holds a substitute row on this instance. A second
	// insert would violate UNIQUE(instance_id, staff_id), so we never add another
	// row — the person already provides coverage. This holds regardless of whether
	// the absent's row is flagged yet: in a combined /deviations save the absent's
	// row is still non-absent at classification time (it is flipped in Phase B),
	// and the same substitute may already cover a co-absent colleague on this
	// block. When the absent row is already flagged there is nothing left to do
	// (idempotent retry, e.g. re-running /substitute); otherwise the row still
	// needs flagging, handled exactly like an already-present co-supervisor (mark
	// absent, insert nothing) so Phase B cannot hit the unique constraint (#1840).
	if existingSubOfSub != nil {
		if origRow.IsAbsent {
			return substituteActionAlreadySubstitute, 0, true
		}
		return substituteActionAlreadyOnInstance, 0, true
	}

	// Count-based overstaffing guard (#1840). A substitute covers exactly one
	// absent planned position, but the data model carries NO substitute→absent
	// link, so coverage is tracked by BALANCE, not identity: the block still has
	// room for another substitute only while active substitutes are FEWER than
	// absent planned positions. Reject a new substitute only when origRow is
	// already flagged absent AND every absent position is already covered
	// (activeSubsOfOther >= absentPlanned) — adding one more would overstaff, so
	// the admin must remove a redundant replacement first. When an absent slot is
	// still open the substitute fills a real gap and is accepted; this is what lets
	// a later save cover a still-open position without first tearing down another
	// position's valid replacement. When origRow is not yet absent (a fresh
	// substitution Phase B will flag) the guard does not apply — the block gains a
	// matching absence in the same save, so coverage stays balanced.
	// anyActiveSubOfOther is non-nil here: origRow.IsAbsent makes absentPlanned>=1,
	// so activeSubsOfOther>=absentPlanned implies activeSubsOfOther>=1.
	if origRow.IsAbsent && activeSubsOfOther >= absentPlanned {
		return "", anyActiveSubOfOther.StaffID, false
	}

	if subAsNonAbsent != nil {
		// Substitute is already a co-supervisor on this instance. We cannot
		// insert a second row for the same staff (UNIQUE(instance_id,
		// staff_id) would reject it); semantically the person is already
		// covering. Mark the absent's row and leave the co-supervisor row
		// untouched — it remains is_substitute=false so the reporting layer
		// can distinguish planned co-cover from a Vertretung.
		return substituteActionAlreadyOnInstance, 0, true
	}
	return substituteActionSubstituted, 0, true
}

// buildSubstituteTimeConflicts loads the substitute's OTHER (non-target)
// same-day non-absent assignments and returns overlap warnings. No writes.
func (rs *Resource) buildSubstituteTimeConflicts(
	ctx context.Context,
	plan []plannedOp,
	subID int64,
	date timezone.Date,
) ([]scheduleSvc.SubstituteTimeConflict, error) {
	if len(plan) == 0 {
		return nil, nil
	}
	subRows, err := rs.TimetableData.GetInstanceStaffByStaffAndDate(ctx, subID, date)
	if err != nil {
		return nil, err
	}
	// Target IDs to exclude from "foreign" set.
	targetSet := make(map[int64]bool, len(plan))
	for _, op := range plan {
		targetSet[op.instance.ID] = true
	}

	foreignIDs := make([]int64, 0, len(subRows))
	for _, row := range subRows {
		if row.IsAbsent {
			continue
		}
		if targetSet[row.InstanceID] {
			continue
		}
		foreignIDs = append(foreignIDs, row.InstanceID)
	}
	if len(foreignIDs) == 0 {
		return nil, nil
	}
	foreigns := make([]scheduleSvc.SubstituteConflictInstance, 0, len(foreignIDs))
	for _, fid := range foreignIDs {
		inst, err := rs.TimetableData.GetActivityInstance(ctx, fid)
		if err != nil {
			return nil, err
		}
		if inst == nil {
			continue
		}
		foreigns = append(foreigns, toConflictInstance(inst))
	}
	targets := make([]scheduleSvc.SubstituteConflictInstance, 0, len(plan))
	for _, op := range plan {
		if op.action == substituteActionAlreadySubstitute {
			// Already substituted earlier — we didn't write anything; still
			// safe to compare the substitute's existing foreign assignments.
			continue
		}
		// already_on_instance is intentionally included: no new substitute
		// row is created, but the co-supervisor's pre-existing overlaps with
		// other same-day assignments remain relevant information for the
		// admin. Pruning them would hide a truthful signal.
		targets = append(targets, toConflictInstance(op.instance))
	}
	return scheduleSvc.DetectSubstituteTimeConflicts(targets, foreigns), nil
}

// toConflictInstance converts an ActivityInstance's TIME columns (year-0000
// time.Time after bun decode) into the minutes-since-midnight form expected
// by the conflict helper.
func toConflictInstance(inst *scheduleModel.ActivityInstance) scheduleSvc.SubstituteConflictInstance {
	return scheduleSvc.SubstituteConflictInstance{
		ID:        inst.ID,
		StartMin:  scheduleSvc.MinutesOfTime(inst.StartTime.Hour(), inst.StartTime.Minute()),
		EndMin:    scheduleSvc.MinutesOfTime(inst.EndTime.Hour(), inst.EndTime.Minute()),
		StartHHMM: inst.StartTime.Format("15:04"),
	}
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
	rs.broadcastSubstituteEvents(ctx, touched)
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

// broadcastSubstituteEvents queues one activity_update per affected active
// group after the surrounding tenant transaction commits.
func (rs *Resource) broadcastSubstituteEvents(
	ctx context.Context,
	touched map[int64]*scheduleModel.ActivityInstance,
) {
	if rs.Broadcaster == nil || len(touched) == 0 {
		return
	}
	tenantID := tenant.FromContext(ctx)
	for activeGroupID, inst := range touched {
		activeGroupIDStr := fmt.Sprintf("%d", activeGroupID)
		instanceIDStr := fmt.Sprintf("%d", inst.ID)
		instanceDate := inst.Date.Format(dateLayout)
		instanceStart := inst.StartTime.Format("15:04:05")
		data := realtime.EventData{
			InstanceID:        &instanceIDStr,
			InstanceDate:      &instanceDate,
			InstanceStartTime: &instanceStart,
		}
		event := realtime.NewEvent(realtime.EventActivityUpdate, activeGroupIDStr, data)
		tenant.RegisterAfterCommit(ctx, func() {
			if err := rs.Broadcaster.BroadcastToGroup(tenantID, activeGroupIDStr, event); err != nil {
				rs.getLogger().Warn("SSE substitute broadcast failed",
					slog.String("active_group_id", activeGroupIDStr),
					slog.String("error", err.Error()),
				)
			}
		})
	}
}
