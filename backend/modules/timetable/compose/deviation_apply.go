package compose

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Atomic Vertretungsplan save orchestration (#1840/#1886).
//
// ApplyDeviations owns the WHOLE deviations save in one call: day-lock,
// validate + classify (Phase A, no writes, deviation_plan.go), then the
// absence / presence / substitution writes plus acknowledgement
// reconciliation (Phase B). The handler only parses, dispatches, and shapes
// the response — the plan-then-write atomicity and every business rule live
// here, next to the write methods they drive.
//
// Atomicity note: TenantTxMiddleware rolls the request tx back only on 5xx. A
// 409 rendered mid-save would commit prior writes, so Phase A validates +
// classifies everything before Phase B touches a single row. A
// timetable.DeviationError separates the stable client response from the
// internal cause retained for logs.

// Client messages shared by several rules of the save.
const (
	msgInstanceNotFound    = "der Termin wurde nicht gefunden"
	msgInstanceInPast      = "dieser Termin liegt in der Vergangenheit"
	msgInstanceNotEditable = "dieser Termin kann nicht mehr geändert werden"
	msgInstanceMoved       = "der Termin wurde gleichzeitig geändert. Öffnen Sie ihn erneut"
	msgSickScopeLocked     = "diese Abwesenheit kommt aus einer Krankmeldung und kann hier nicht geändert werden"
)

// ApplyDeviations applies a whole Vertretungsplan slide-over save atomically.
// Runs inside the caller's tenant tx (TenantTxMiddleware).
func (s *staffDeviations) ApplyDeviations(ctx context.Context, instanceID int64, in timetable.ApplyDeviationsInput) (*timetable.ApplyDeviationsResult, error) {
	instance, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	// Past blocks are historical record; no deviation — including a cancellation
	// — may rewrite them. Guard before the exclusive cancel branch (#1840).
	if instance.Date.Before(timezone.TodayDate()) {
		return nil, timetable.DeviationBadRequest(msgInstanceInPast)
	}

	if in.Cancel {
		return s.cancelDeviation(ctx, instanceID, instance, in)
	}

	if !isPlannableInstance(instance) {
		return nil, timetable.DeviationConflict("invalid_transition", msgInstanceNotEditable)
	}

	if in.UnderstaffedNote != nil && utf8.RuneCountInString(*in.UnderstaffedNote) > scheduleModel.ActivityExceptionReasonMaxLength {
		return nil, timetable.DeviationBadRequest("der Hinweis ist zu lang")
	}

	if err := rejectNonPositiveStaffIDs(in); err != nil {
		return nil, err
	}

	plan, err := s.planDeviations(ctx, instanceID, instance, in)
	if err != nil {
		return nil, err
	}
	return s.executeDeviationPlan(ctx, instanceID, in, plan)
}

// rejectNonPositiveStaffIDs rejects non-positive staff ids on the raw request
// before any repository lookup or write planning.
func rejectNonPositiveStaffIDs(in timetable.ApplyDeviationsInput) error {
	for _, a := range in.Absences {
		if a.StaffID <= 0 {
			return timetable.DeviationBadRequest("die Auswahl der abwesenden Person ist ungültig")
		}
	}
	for _, presence := range in.Presences {
		if presence.StaffID <= 0 {
			return timetable.DeviationBadRequest("die Auswahl der anwesenden Person ist ungültig")
		}
	}
	for _, removal := range in.SubstitutionRemovals {
		if removal.StaffID <= 0 {
			return timetable.DeviationBadRequest("die Auswahl der Ersatzperson ist ungültig")
		}
	}
	return nil
}

// loadDeviationInstance loads the target instance, mapping the absent/other-
// tenant case to 404 and any other failure to a wrapped 500.
func (s *staffDeviations) loadDeviationInstance(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.deps.Instances.FindByID(ctx, instanceID)
	if err != nil {
		// FindByID wraps sql.ErrNoRows in a DatabaseError (never (nil, nil)), so a
		// stale link or deleted/other-tenant instance maps to 404 here.
		if modelBase.IsNoRows(err) {
			return nil, timetable.DeviationNotFound(msgInstanceNotFound)
		}
		return nil, timetable.DeviationInternal("load instance failed", err)
	}
	if instance == nil {
		return nil, timetable.DeviationNotFound(msgInstanceNotFound)
	}
	return instance, nil
}

// cancelDeviation applies the exclusive cancel branch: it serializes against
// concurrent same-day saves, re-reads under the lock to catch a concurrent move,
// then delegates to the lifecycle's cancellation.
func (s *staffDeviations) cancelDeviation(ctx context.Context, instanceID int64, instance *scheduleModel.ActivityInstance, in timetable.ApplyDeviationsInput) (*timetable.ApplyDeviationsResult, error) {
	if err := s.acquireSubstituteDayLock(ctx, timezone.Date(instance.Date)); err != nil {
		return nil, timetable.DeviationInternal("lock day failed", err)
	}
	locked, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// A concurrent PUT may have MOVED the block to another day between the initial
	// read and this locked reload; the lock we hold is keyed on the stale date, so
	// cancelling the moved block would break the day-lock ordering. Abort so the
	// client reopens it on its new day (#1840).
	if locked.Date != instance.Date {
		return nil, timetable.DeviationConflict("instance_moved", msgInstanceMoved)
	}
	// A move to a past day would rewrite history; the initial guard ran against a
	// possibly-stale read, so re-check under the lock.
	if locked.Date.Before(timezone.TodayDate()) {
		return nil, timetable.DeviationBadRequest(msgInstanceInPast)
	}
	cancelled, err := s.deps.Lifecycle.CancelBlock(ctx, DeviationCancellation{
		InstanceID:     instanceID,
		Reason:         trimDeviationReason(in.CancelReason),
		ActorAccountID: in.ActorAccountID,
		GuardianNotice: in.GuardianNotice,
	})
	if err != nil {
		return nil, err
	}
	message := "Termin wurde abgesagt"
	if cancelled.GuardianNotice != nil && cancelled.GuardianNotice.FamilyCount > 0 {
		message = "Termin wurde abgesagt, die Eltern sind informiert"
	}
	return &timetable.ApplyDeviationsResult{
		InstanceID:      cancelled.InstanceID,
		Cancelled:       true,
		UnderstaffedAck: cancelled.UnderstaffedAck,
		Affected:        []timetable.DeviationAffected{},
		Warnings:        []timetable.SubstituteTimeConflict{},
		GuardianNotice:  cancelled.GuardianNotice,
		Message:         message,
	}, nil
}

// executeDeviationPlan runs Phase B: it applies every classified write, sets the
// selected block's acknowledgement, reconciles stale acks on the other covered
// blocks, and gathers time-conflict advisories.
func (s *staffDeviations) executeDeviationPlan(ctx context.Context, instanceID int64, in timetable.ApplyDeviationsInput, plan *deviationPlan) (*timetable.ApplyDeviationsResult, error) {
	affected, touched, err := s.writeDeviationOps(ctx, in.ActorAccountID, plan)
	if err != nil {
		return nil, err
	}

	if plan.ackChanged {
		if err := s.deps.Lifecycle.SetUnderstaffedAck(ctx, instanceID, plan.finalAck, plan.finalAckNote, in.ActorAccountID); err != nil {
			// A concurrent cancel/full-staffing of THIS instance makes the
			// acknowledgement return a 4xx after the writes above already
			// succeeded. TenantTxMiddleware commits non-5xx unless we ask
			// otherwise, so force the whole tx to roll back (#1840).
			tenant.MarkRollback(ctx)
			return nil, err
		}
	}

	cleared, err := s.reconcileOtherAcks(ctx, instanceID, in.ActorAccountID, plan)
	if err != nil {
		return nil, err
	}

	warnings, err := s.collectDeviationWarnings(ctx, plan.subs, plan.subPlan, plan.date)
	if err != nil {
		return nil, timetable.DeviationInternal("deviation time-conflict detection failed", err)
	}

	return &timetable.ApplyDeviationsResult{
		InstanceID:               instanceID,
		Cancelled:                false,
		UnderstaffedAck:          plan.finalAck,
		Affected:                 affected,
		Warnings:                 warnings,
		ActiveTouched:            touched,
		AppliedWrites:            len(affected),
		AckChanged:               plan.ackChanged,
		ClearedAcks:              cleared,
		AbsenceCount:             len(plan.absencePlan),
		PresenceCount:            len(plan.presencePlan),
		SubstitutionCount:        len(plan.subPlan),
		SubstitutionRemovalCount: len(plan.removalPlan),
		Message:                  "Vertretungen wurden gespeichert",
	}, nil
}

// writeDeviationOps applies presence, absence, substitution removal, and new
// substitution writes in that order. Removals must precede additions so one
// atomic request can replace a substitute on the same appointment.
func (s *staffDeviations) writeDeviationOps(ctx context.Context, actor *int64, plan *deviationPlan) ([]timetable.DeviationAffected, timetable.TouchedActivities, error) {
	now := time.Now()
	affected := make([]timetable.DeviationAffected, 0, len(plan.absencePlan)+len(plan.subPlan)+len(plan.presencePlan)+len(plan.removalPlan))
	touched := make(timetable.TouchedActivities)

	for _, op := range plan.presencePlan {
		if err := s.applyPresence(ctx, op.row, op.instance, actor, touched); err != nil {
			return nil, nil, timetable.DeviationInternal("clear absence failed", err)
		}
		affected = append(affected, affectedOf(op.instance, timetable.SubstituteActionMarkedPresent))
	}
	for _, op := range plan.absencePlan {
		if err := s.applyAbsence(ctx, op.row, op.instance, op.reason, actor, touched); err != nil {
			return nil, nil, timetable.DeviationInternal("mark absent failed", err)
		}
		affected = append(affected, affectedOf(op.instance, timetable.SubstituteActionMarkedAbsent))
	}
	for _, op := range plan.removalPlan {
		if err := s.removeSubstitute(ctx, op.row, op.instance, actor, touched); err != nil {
			return nil, nil, timetable.DeviationInternal("remove substitute failed", err)
		}
		affected = append(affected, affectedOf(op.instance, timetable.SubstituteActionRemoved))
	}
	for _, op := range plan.subPlan {
		if err := s.applySubstitute(ctx, op.write, op.subID, op.reason, now, actor, touched); err != nil {
			return nil, nil, timetable.DeviationInternal("assign substitute failed", err)
		}
		affected = append(affected, affectedOf(op.write.Instance, op.write.Action))
	}
	return affected, touched, nil
}

// reconcileOtherAcks clears stale "deliberately unstaffed" acknowledgements on
// the OTHER instances this save covered (the selected block's ack is handled
// explicitly). Only ever clears (#1840).
func (s *staffDeviations) reconcileOtherAcks(ctx context.Context, instanceID int64, actor *int64, plan *deviationPlan) (int, error) {
	clearAck := make(map[int64]bool)
	for _, op := range plan.subPlan {
		if op.write.Instance.ID != instanceID && coversAcknowledgedBlock(op) {
			clearAck[op.write.Instance.ID] = true
		}
	}
	for _, op := range plan.presencePlan {
		if op.instance.ID != instanceID && op.instance.UnderstaffedAck {
			clearAck[op.instance.ID] = true
		}
	}
	for id := range clearAck {
		if err := s.deps.Lifecycle.ClearUnderstaffedAckIfStaffed(ctx, id, actor); err != nil {
			return 0, timetable.DeviationInternal("clear stale understaffed ack failed", err)
		}
	}
	return len(clearAck), nil
}

// coversAcknowledgedBlock reports whether a substitution adds coverage to a
// block acknowledged as deliberately unstaffed, which makes the ack stale.
func coversAcknowledgedBlock(op deviationSubOp) bool {
	return op.write.Instance.UnderstaffedAck &&
		(op.write.Action == timetable.SubstituteActionSubstituted || op.write.Action == timetable.SubstituteActionAlreadyOnInstance)
}

// collectDeviationWarnings merges per-substitute time-conflict advisories. A
// lookup failure propagates as an error: the probe runs inside the tenant tx,
// and a PostgreSQL error aborts that tx, so the eventual commit would fail
// after the client already saw a 200.
func (s *staffDeviations) collectDeviationWarnings(
	ctx context.Context,
	subs []timetable.DeviationSubstitutionInput,
	plan []deviationSubOp,
	date timezone.Date,
) ([]timetable.SubstituteTimeConflict, error) {
	probes := buildSubstitutionWarningProbes(subs, plan)
	if len(probes) == 0 {
		return nil, nil
	}
	return s.loadSubstitutionWarnings(ctx, probes, date)
}

func buildSubstitutionWarningProbes(subs []timetable.DeviationSubstitutionInput, plan []deviationSubOp) []substitutionWarningProbe {
	opsByStaff := make(map[int64][]deviationSubOp)
	for _, op := range plan {
		opsByStaff[op.subID] = append(opsByStaff[op.subID], op)
	}
	seen := make(map[int64]bool, len(subs))
	probes := make([]substitutionWarningProbe, 0, len(subs))
	for _, sub := range subs {
		if seen[sub.SubstituteStaffID] || len(opsByStaff[sub.SubstituteStaffID]) == 0 {
			continue
		}
		seen[sub.SubstituteStaffID] = true
		probes = append(probes, substitutionProbeOf(sub.SubstituteStaffID, opsByStaff[sub.SubstituteStaffID]))
	}
	return probes
}

// substitutionProbeOf places one substitute on every block the save covered
// with them; an already-substituted block is excluded from the targets.
func substitutionProbeOf(staffID int64, ops []deviationSubOp) substitutionWarningProbe {
	probe := substitutionWarningProbe{staffID: staffID, targetIDs: make(map[int64]bool)}
	for _, op := range ops {
		probe.targetIDs[op.write.Instance.ID] = true
		if op.write.Action != timetable.SubstituteActionAlreadySubstitute {
			probe.targets = append(probe.targets, toConflictInstance(op.write.Instance))
		}
	}
	return probe
}
