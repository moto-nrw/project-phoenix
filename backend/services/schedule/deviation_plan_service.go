// Package schedule — Vertretungsplan Phase-A planning (#1840).
//
// PlanDeviations owns the read-only "dry run" of the atomic Vertretungsplan
// save: day-locking, stale/concurrent-move detection, staff validation, the
// three planning passes (absences/presences/substitutions), the overstaffing
// projection, and the understaffed-ack reconciliation decision. It performs no
// writes — every 4xx aborts before a single row changes, exactly as the former
// in-handler Phase A did. The api/timetable handler executes the returned plan
// (Phase B) through InstanceService and maps DeviationPlanError to the wire
// contract.
package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// DeviationPlanError is a typed classification error returned by
// PlanDeviations / GuardDeviationDate. The handler maps it to the exact HTTP
// response via renderDeviationPlanError, reproducing the former in-handler
// common.Error* calls verbatim: HTTPStatus + Code + Message are the wire
// output; Cause (non-nil only on 5xx) is preserved for logging/Sentry.
type DeviationPlanError struct {
	HTTPStatus int
	Code       string
	Message    string
	Cause      error
}

func (e *DeviationPlanError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *DeviationPlanError) Unwrap() error { return e.Cause }

func devErr(status int, code, msg string) *DeviationPlanError {
	return &DeviationPlanError{HTTPStatus: status, Code: code, Message: msg}
}

// devWrap builds a 500 that keeps the client-facing message while preserving
// the cause for logging (mirrors common.ErrorInternalServerWrap).
func devWrap(clientMsg string, cause error) *DeviationPlanError {
	return &DeviationPlanError{HTTPStatus: http.StatusInternalServerError, Message: clientMsg, Cause: cause}
}

// DeviationAbsenceInput marks one staff member absent day-wide (optional reason).
type DeviationAbsenceInput struct {
	StaffID int64
	Reason  *string
}

// DeviationSubstitutionInput assigns SubstituteStaffID to cover AbsentStaffID
// day-wide.
type DeviationSubstitutionInput struct {
	AbsentStaffID     int64
	SubstituteStaffID int64
	Reason            *string
}

// DeviationInput is the parsed Vertretungsplan save request, mapped from the
// HTTP body by the handler.
type DeviationInput struct {
	Cancel           bool
	UnderstaffedAck  *bool
	UnderstaffedNote *string
	Absences         []DeviationAbsenceInput
	Substitutions    []DeviationSubstitutionInput
	Presences        []int64
}

// DeviationPresenceWrite clears a persisted absence on one row (#1840).
type DeviationPresenceWrite struct {
	Row      *scheduleModel.InstanceStaff
	Instance *scheduleModel.ActivityInstance
}

// DeviationAbsenceWrite marks one row absent day-wide.
type DeviationAbsenceWrite struct {
	Row      *scheduleModel.InstanceStaff
	Instance *scheduleModel.ActivityInstance
	Reason   *string
}

// DeviationSubstituteWrite carries one classified substitution to Phase B.
type DeviationSubstituteWrite struct {
	Op       SubstituteWriteOp
	SubID    int64
	Reason   *string
	Instance *scheduleModel.ActivityInstance
	Action   string
}

// DeviationPlan is the validated, write-ready result of Phase A. A Cancel plan
// carries only Instance; every other field is the executable roster/ack change
// set the handler applies through InstanceService.
type DeviationPlan struct {
	InstanceID  int64
	Cancel      bool
	Instance    *scheduleModel.ActivityInstance
	Presences   []DeviationPresenceWrite
	Absences    []DeviationAbsenceWrite
	Subs        []DeviationSubstituteWrite
	AckChanged  bool
	FinalAck    bool
	AckNote     *string
	ClearAckIDs []int64
	Warnings    []SubstituteTimeConflict
}

// deviationPlannedOp is the internal classification of one target instance.
type deviationPlannedOp struct {
	instance *scheduleModel.ActivityInstance
	origRow  *scheduleModel.InstanceStaff
	action   string
}

// deviationSubOp tags a classified substitution op with the substitute id +
// trimmed reason.
type deviationSubOp struct {
	op     deviationPlannedOp
	subID  int64
	reason *string
}

// GuardDeviationDate enforces the historical-record + concurrency guard shared
// by the standalone /acknowledge-understaffed endpoint: past blocks are
// read-only, and a concurrent PUT that MOVED the block must abort the write.
// Returns nil when the instance is missing (FindByID never returns (nil, nil)
// in production; the caller then delegates status-only validation to
// SetUnderstaffedAck) or when the guard passes. All other outcomes are a
// *DeviationPlanError.
func (s *TimetableDataService) GuardDeviationDate(ctx context.Context, id int64) error {
	instance, err := s.GetActivityInstance(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return devErr(http.StatusNotFound, "", "instance not found")
		}
		return devWrap("load instance failed", err)
	}
	if instance == nil {
		return nil
	}
	if instance.Date.Before(timezone.TodayDate()) {
		return devErr(http.StatusBadRequest, "", "block date is in the past")
	}
	if err := s.AcquireSubstituteDayLock(ctx, instance.Date); err != nil {
		return devWrap("lock day failed", err)
	}
	locked, err := s.GetActivityInstance(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return devErr(http.StatusNotFound, "", "instance not found")
		}
		return devWrap("reload instance failed", err)
	}
	if locked == nil {
		return devErr(http.StatusNotFound, "", "instance not found")
	}
	if locked.Date != instance.Date {
		return devErr(http.StatusConflict, "instance_moved", "block was changed concurrently; reopen it and try again")
	}
	return nil
}

// PlanDeviations runs the Phase-A dry run for POST /instances/{id}/deviations.
// It never writes. On success it returns the executable DeviationPlan; on any
// precondition failure it returns a *DeviationPlanError the handler renders.
func (s *TimetableDataService) PlanDeviations(ctx context.Context, id int64, in DeviationInput) (*DeviationPlan, error) {
	instance, derr := s.loadDeviationInstance(ctx, id, "load instance failed")
	if derr != nil {
		return nil, derr
	}

	// Past blocks are historical record; no deviation — including a
	// cancellation — may rewrite them. Guard before the exclusive cancel branch.
	if instance.Date.Before(timezone.TodayDate()) {
		return nil, devErr(http.StatusBadRequest, "", "block date is in the past")
	}

	if in.Cancel {
		return s.planDeviationCancel(ctx, id, instance)
	}

	// Only planned/active blocks are editable; completed/cancelled are history.
	if !sickCascadePlannable(instance) {
		return nil, devErr(http.StatusConflict, "invalid_transition",
			fmt.Sprintf("cannot edit instance in status %q", instance.Status))
	}
	date := instance.Date
	if in.UnderstaffedNote != nil && utf8.RuneCountInString(*in.UnderstaffedNote) > scheduleModel.ActivityExceptionReasonMaxLength {
		return nil, devErr(http.StatusBadRequest, "", "note is too long")
	}

	// Reject non-positive staff ids on the RAW request, before dedupePositive
	// silently drops them below.
	for _, a := range in.Absences {
		if a.StaffID <= 0 {
			return nil, devErr(http.StatusBadRequest, "", "absent staff must be a positive id")
		}
	}
	for _, pid := range in.Presences {
		if pid <= 0 {
			return nil, devErr(http.StatusBadRequest, "", "present staff must be a positive id")
		}
	}

	// Serialize concurrent saves for the whole (tenant, date) BEFORE any
	// classification read, then re-read the instance under the lock to detect a
	// concurrent move/cancel (#1840).
	if err := s.AcquireSubstituteDayLock(ctx, date); err != nil {
		return nil, devWrap("lock day failed", err)
	}
	locked, derr := s.loadDeviationInstance(ctx, id, "reload instance failed")
	if derr != nil {
		return nil, derr
	}
	if locked.Date != date || !sickCascadePlannable(locked) {
		return nil, devErr(http.StatusConflict, "instance_moved", "block was changed concurrently; reopen it and try again")
	}
	instance = locked

	absenceOnly := dedupePositive(func() []int64 {
		ids := make([]int64, 0, len(in.Absences))
		for _, a := range in.Absences {
			ids = append(ids, a.StaffID)
		}
		return ids
	}())
	absenceReason := make(map[int64]*string, len(in.Absences))
	for _, a := range in.Absences {
		absenceReason[a.StaffID] = trimDeviationReason(a.Reason)
	}
	absenceOnlySet := toSet(absenceOnly)

	fullAbsent := toSet(absenceOnly)
	for _, sub := range in.Substitutions {
		fullAbsent[sub.AbsentStaffID] = true
	}

	presences := dedupePositive(in.Presences)
	presenceSet := toSet(presences)

	if derr := s.validateDeviationStaff(ctx, in, absenceOnlySet, fullAbsent, presenceSet, date); derr != nil {
		return nil, derr
	}

	absencePlan, derr := s.planAbsences(ctx, absenceOnly, date)
	if derr != nil {
		return nil, derr
	}
	presencePlan, derr := s.planPresences(ctx, presences, date)
	if derr != nil {
		return nil, derr
	}
	subPlan, newSubByInstance, derr := s.planSubstitutions(ctx, in.Substitutions, absenceOnlySet, date)
	if derr != nil {
		return nil, derr
	}

	if derr := s.rejectOverstaffingPresences(ctx, presencePlan, fullAbsent, presenceSet, newSubByInstance); derr != nil {
		return nil, derr
	}

	finalAck, ackChanged, derr := s.decideUnderstaffedAck(ctx, id, instance, in, fullAbsent, presenceSet, newSubByInstance)
	if derr != nil {
		return nil, derr
	}
	var ackNote *string
	if finalAck {
		ackNote = trimDeviationReason(in.UnderstaffedNote)
	}

	warnings := s.collectDeviationWarnings(ctx, in.Substitutions, subPlan, date)

	return &DeviationPlan{
		InstanceID:  id,
		Instance:    instance,
		Presences:   buildPresenceWrites(presencePlan),
		Absences:    buildAbsenceWrites(absencePlan, absenceReason),
		Subs:        buildSubstituteWrites(subPlan),
		AckChanged:  ackChanged,
		FinalAck:    finalAck,
		AckNote:     ackNote,
		ClearAckIDs: buildClearAckIDs(id, subPlan, presencePlan),
		Warnings:    warnings,
	}, nil
}

// loadDeviationInstance loads an instance, mapping a missing/deleted/other-tenant
// row to 404 and any other error to a 500 with loadMsg.
func (s *TimetableDataService) loadDeviationInstance(ctx context.Context, id int64, loadMsg string) (*scheduleModel.ActivityInstance, *DeviationPlanError) {
	instance, err := s.GetActivityInstance(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, devErr(http.StatusNotFound, "", "instance not found")
		}
		return nil, devWrap(loadMsg, err)
	}
	if instance == nil {
		return nil, devErr(http.StatusNotFound, "", "instance not found")
	}
	return instance, nil
}

// planDeviationCancel takes the day lock, reloads under it, and rejects a
// concurrent move or a move to a past day before returning a cancel plan.
func (s *TimetableDataService) planDeviationCancel(ctx context.Context, id int64, instance *scheduleModel.ActivityInstance) (*DeviationPlan, error) {
	if err := s.AcquireSubstituteDayLock(ctx, instance.Date); err != nil {
		return nil, devWrap("lock day failed", err)
	}
	locked, derr := s.loadDeviationInstance(ctx, id, "reload instance failed")
	if derr != nil {
		return nil, derr
	}
	if locked.Date != instance.Date {
		return nil, devErr(http.StatusConflict, "instance_moved", "block was changed concurrently; reopen it and try again")
	}
	if locked.Date.Before(timezone.TodayDate()) {
		return nil, devErr(http.StatusBadRequest, "", "block date is in the past")
	}
	return &DeviationPlan{InstanceID: id, Cancel: true, Instance: locked}, nil
}

// validateDeviationStaff runs every 4xx precondition on the referenced staff.
func (s *TimetableDataService) validateDeviationStaff(
	ctx context.Context,
	in DeviationInput,
	absenceOnlySet, fullAbsent, presenceSet map[int64]bool,
	date timezone.Date,
) *DeviationPlanError {
	seen := make(map[int64]bool)
	ensure := func(staffID int64, label string) *DeviationPlanError {
		if staffID <= 0 {
			return devErr(http.StatusBadRequest, "", fmt.Sprintf("%s must be a positive id", label))
		}
		if seen[staffID] {
			return nil
		}
		seen[staffID] = true
		staff, err := s.deps.StaffRepo.FindByID(ctx, staffID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return devErr(http.StatusNotFound, "", fmt.Sprintf("%s not found", label))
			}
			return devWrap("load staff failed", err)
		}
		if staff == nil || staff.ID == 0 {
			return devErr(http.StatusNotFound, "", fmt.Sprintf("%s not found", label))
		}
		return nil
	}

	for id := range absenceOnlySet {
		if derr := ensure(id, "absent staff"); derr != nil {
			return derr
		}
	}
	for id := range presenceSet {
		if fullAbsent[id] {
			return devErr(http.StatusBadRequest, "", "staff cannot be marked present and absent in the same request")
		}
		if derr := ensure(id, "present staff"); derr != nil {
			return derr
		}
	}
	seenAbsentSub := make(map[int64]bool)
	for _, sub := range in.Substitutions {
		if seenAbsentSub[sub.AbsentStaffID] {
			return devErr(http.StatusBadRequest, "", "each absent staff member may have at most one substitute per request")
		}
		seenAbsentSub[sub.AbsentStaffID] = true
		if derr := ensure(sub.AbsentStaffID, "absent staff"); derr != nil {
			return derr
		}
		if derr := ensure(sub.SubstituteStaffID, "substitute staff"); derr != nil {
			return derr
		}
		if sub.AbsentStaffID == sub.SubstituteStaffID {
			return devErr(http.StatusBadRequest, "", "absent and substitute staff must differ")
		}
		if fullAbsent[sub.SubstituteStaffID] {
			return devErr(http.StatusBadRequest, "", "substitute is marked absent in this request")
		}
		subRows, err := s.GetInstanceStaffByStaffAndDate(ctx, sub.SubstituteStaffID, date)
		if err != nil {
			return devWrap("load substitute assignments failed", err)
		}
		for _, row := range subRows {
			if row.IsAbsent {
				return devErr(http.StatusBadRequest, "", "substitute is marked absent on this date")
			}
		}
	}
	return nil
}

// planPresences loads every to-be-restored staff member's plannable same-day
// rows that are currently marked absent (day-wide clear, #1840).
func (s *TimetableDataService) planPresences(ctx context.Context, presentStaffIDs []int64, date timezone.Date) ([]DeviationPresenceWrite, *DeviationPlanError) {
	plan := make([]DeviationPresenceWrite, 0)
	for _, staffID := range presentStaffIDs {
		rows, err := s.GetInstanceStaffByStaffAndDate(ctx, staffID, date)
		if err != nil {
			return nil, devWrap("load present assignments failed", err)
		}
		for _, row := range rows {
			if !row.IsAbsent {
				continue
			}
			instance, derr := s.loadPlannableInstance(ctx, row)
			if derr != nil {
				return nil, derr
			}
			if instance == nil {
				continue
			}
			plan = append(plan, DeviationPresenceWrite{Row: row, Instance: instance})
		}
	}
	return plan, nil
}

// planAbsences loads every absent-only staff member's plannable same-day rows.
func (s *TimetableDataService) planAbsences(ctx context.Context, absentStaffIDs []int64, date timezone.Date) ([]DeviationPresenceWrite, *DeviationPlanError) {
	plan := make([]DeviationPresenceWrite, 0)
	for _, staffID := range absentStaffIDs {
		rows, err := s.GetInstanceStaffByStaffAndDate(ctx, staffID, date)
		if err != nil {
			return nil, devWrap("load absent assignments failed", err)
		}
		for _, row := range rows {
			if row.IsAbsent {
				continue
			}
			instance, derr := s.loadPlannableInstance(ctx, row)
			if derr != nil {
				return nil, derr
			}
			if instance == nil {
				continue
			}
			plan = append(plan, DeviationPresenceWrite{Row: row, Instance: instance})
		}
	}
	return plan, nil
}

// planSubstitutions classifies every substitution against a projected view of
// each instance (absence-only staff read as absent). Returns the write plan and,
// per instance, how many NEW substitute rows will be added (for the ack check).
func (s *TimetableDataService) planSubstitutions(
	ctx context.Context,
	subs []DeviationSubstitutionInput,
	absenceOnlySet map[int64]bool,
	date timezone.Date,
) ([]deviationSubOp, map[int64]int, *DeviationPlanError) {
	plan := make([]deviationSubOp, 0)
	newSubByInstance := make(map[int64]int)
	stagedSubs := make(map[int64]map[int64]bool)

	for _, sub := range subs {
		origRows, err := s.GetInstanceStaffByStaffAndDate(ctx, sub.AbsentStaffID, date)
		if err != nil {
			return nil, nil, devWrap("load absent assignments failed", err)
		}
		reason := trimDeviationReason(sub.Reason)
		for _, orig := range origRows {
			instance, derr := s.loadPlannableInstance(ctx, orig)
			if derr != nil {
				return nil, nil, derr
			}
			if instance == nil {
				continue
			}
			allRows, err := s.GetInstanceStaff(ctx, instance.ID)
			if err != nil {
				return nil, nil, devWrap("load instance staff failed", err)
			}
			projectedRows, origProjected := projectAbsent(allRows, absenceOnlySet, orig)
			action, conflictOther, ok := classifySubstitute(projectedRows, origProjected, sub.SubstituteStaffID)
			if !ok {
				return nil, nil, devErr(http.StatusConflict, "substitute_conflict",
					fmt.Sprintf("instance %d is already fully covered by another substitute (staff_id=%d); remove a replacement before adding one",
						instance.ID, conflictOther))
			}
			if action == SubstituteActionSubstituted {
				staged := stagedSubs[instance.ID]
				if staged == nil {
					staged = make(map[int64]bool)
					stagedSubs[instance.ID] = staged
				}
				if staged[sub.SubstituteStaffID] {
					action = SubstituteActionAlreadyOnInstance
				} else {
					staged[sub.SubstituteStaffID] = true
					newSubByInstance[instance.ID]++
				}
			}
			plan = append(plan, deviationSubOp{
				op:     deviationPlannedOp{instance: instance, origRow: orig, action: action},
				subID:  sub.SubstituteStaffID,
				reason: reason,
			})
		}
	}
	return plan, newSubByInstance, nil
}

// rejectOverstaffingPresences returns a 409 when clearing a persisted absence
// would push any touched instance above its planned headcount (#1840).
func (s *TimetableDataService) rejectOverstaffingPresences(
	ctx context.Context,
	presencePlan []DeviationPresenceWrite,
	fullAbsent, presenceSet map[int64]bool,
	newSubByInstance map[int64]int,
) *DeviationPlanError {
	checked := make(map[int64]bool)
	for _, op := range presencePlan {
		if checked[op.Instance.ID] {
			continue
		}
		checked[op.Instance.ID] = true
		rows, err := s.GetInstanceStaff(ctx, op.Instance.ID)
		if err != nil {
			return devWrap("load instance staff failed", err)
		}
		baseline := 0
		for _, row := range rows {
			if !row.IsSubstitute {
				baseline++
			}
		}
		if projectedNonAbsentCount(rows, fullAbsent, presenceSet, newSubByInstance[op.Instance.ID]) > baseline {
			return devErr(http.StatusConflict, "presence_would_overstaff",
				"das Wiederherstellen dieser Anwesenheit würde den Block überbesetzen; bitte zuerst die nicht mehr benötigte Vertretung entfernen")
		}
	}
	return nil
}

// decideUnderstaffedAck reconciles the "deliberately unstaffed" flag on the
// selected instance against its projected post-save coverage.
func (s *TimetableDataService) decideUnderstaffedAck(
	ctx context.Context,
	id int64,
	instance *scheduleModel.ActivityInstance,
	in DeviationInput,
	fullAbsent, presenceSet map[int64]bool,
	newSubByInstance map[int64]int,
) (finalAck, ackChanged bool, derr *DeviationPlanError) {
	thisRows, err := s.GetInstanceStaff(ctx, id)
	if err != nil {
		return false, false, devWrap("load instance staff failed", err)
	}
	projectedPresent := projectedNonAbsentCount(thisRows, fullAbsent, presenceSet, newSubByInstance[id])
	plannedBaseline := 0
	for _, row := range thisRows {
		if !row.IsSubstitute {
			plannedBaseline++
		}
	}
	projectedUnderstaffed := IsUnderstaffedCounts(projectedPresent, plannedBaseline)

	finalAck = instance.UnderstaffedAck
	if in.UnderstaffedAck != nil {
		if *in.UnderstaffedAck && !projectedUnderstaffed {
			return false, false, devErr(http.StatusConflict, "understaffed_still_staffed",
				"dieser Block kann nicht als bewusst unbesetzt markiert werden, solange er vollständig besetzt ist")
		}
		finalAck = *in.UnderstaffedAck
		ackChanged = finalAck != instance.UnderstaffedAck ||
			(finalAck && !sameNote(instance.UnderstaffedNote, in.UnderstaffedNote))
	} else if instance.UnderstaffedAck && !projectedUnderstaffed {
		finalAck = false
		ackChanged = true
	}
	return finalAck, ackChanged, nil
}

// loadPlannableInstance loads the instance behind a staff row, returning nil
// when it is terminal (completed/cancelled) so callers skip it.
func (s *TimetableDataService) loadPlannableInstance(ctx context.Context, row *scheduleModel.InstanceStaff) (*scheduleModel.ActivityInstance, *DeviationPlanError) {
	instance, err := s.GetActivityInstance(ctx, row.InstanceID)
	if err != nil {
		return nil, devWrap("load target instance failed", err)
	}
	if instance == nil {
		return nil, devErr(http.StatusInternalServerError, "",
			fmt.Sprintf("instance_staff %d references missing instance %d", row.ID, row.InstanceID))
	}
	if !sickCascadePlannable(instance) {
		return nil, nil
	}
	return instance, nil
}

// collectDeviationWarnings merges per-substitute time-conflict advisories. Best
// effort: a lookup failure logs and yields an empty list (never blocks a save).
func (s *TimetableDataService) collectDeviationWarnings(
	ctx context.Context,
	subs []DeviationSubstitutionInput,
	plan []deviationSubOp,
	date timezone.Date,
) []SubstituteTimeConflict {
	warnings := make([]SubstituteTimeConflict, 0)
	seenSub := make(map[int64]bool, len(subs))
	seenConflict := make(map[[3]int64]bool)
	for _, sub := range subs {
		if seenSub[sub.SubstituteStaffID] {
			continue
		}
		seenSub[sub.SubstituteStaffID] = true
		ops := make([]deviationPlannedOp, 0, len(plan))
		for _, p := range plan {
			if p.subID == sub.SubstituteStaffID {
				ops = append(ops, p.op)
			}
		}
		w, err := s.buildSubstituteTimeConflicts(ctx, ops, sub.SubstituteStaffID, date)
		if err != nil {
			slog.Default().Warn("deviation time-conflict detection failed",
				"substitute_staff_id", sub.SubstituteStaffID,
				"error", err.Error(),
			)
			continue
		}
		for _, conflict := range w {
			key := [3]int64{sub.SubstituteStaffID, conflict.InstanceID, conflict.OtherID}
			if seenConflict[key] {
				continue
			}
			seenConflict[key] = true
			warnings = append(warnings, conflict)
		}
	}
	return warnings
}

// buildSubstituteTimeConflicts loads the substitute's OTHER (non-target)
// same-day non-absent assignments and returns overlap warnings. No writes.
func (s *TimetableDataService) buildSubstituteTimeConflicts(
	ctx context.Context,
	plan []deviationPlannedOp,
	subID int64,
	date timezone.Date,
) ([]SubstituteTimeConflict, error) {
	if len(plan) == 0 {
		return nil, nil
	}
	subRows, err := s.GetInstanceStaffByStaffAndDate(ctx, subID, date)
	if err != nil {
		return nil, err
	}
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
	foreigns := make([]SubstituteConflictInstance, 0, len(foreignIDs))
	for _, fid := range foreignIDs {
		inst, err := s.GetActivityInstance(ctx, fid)
		if err != nil {
			return nil, err
		}
		if inst == nil {
			continue
		}
		foreigns = append(foreigns, toConflictInstance(inst))
	}
	targets := make([]SubstituteConflictInstance, 0, len(plan))
	for _, op := range plan {
		if op.action == SubstituteActionAlreadySubstitute {
			continue
		}
		targets = append(targets, toConflictInstance(op.instance))
	}
	return DetectSubstituteTimeConflicts(targets, foreigns), nil
}

// toConflictInstance converts an ActivityInstance's TIME columns into the
// minutes-since-midnight form expected by the conflict helper.
func toConflictInstance(inst *scheduleModel.ActivityInstance) SubstituteConflictInstance {
	return SubstituteConflictInstance{
		ID:        inst.ID,
		StartMin:  MinutesOfTime(inst.StartTime.Hour(), inst.StartTime.Minute()),
		EndMin:    MinutesOfTime(inst.EndTime.Hour(), inst.EndTime.Minute()),
		StartHHMM: inst.StartTime.Format("15:04"),
	}
}

// buildClearAckIDs collects the OTHER instances whose stale "deliberately
// unstaffed" acknowledgement the save covers (#1840). The selected block's ack
// is handled explicitly, so it is skipped here.
func buildClearAckIDs(id int64, subPlan []deviationSubOp, presencePlan []DeviationPresenceWrite) []int64 {
	clearAck := make(map[int64]bool)
	for _, op := range subPlan {
		if op.op.instance.ID == id {
			continue
		}
		if op.op.instance.UnderstaffedAck &&
			(op.op.action == SubstituteActionSubstituted || op.op.action == SubstituteActionAlreadyOnInstance) {
			clearAck[op.op.instance.ID] = true
		}
	}
	for _, op := range presencePlan {
		if op.Instance.ID == id {
			continue
		}
		if op.Instance.UnderstaffedAck {
			clearAck[op.Instance.ID] = true
		}
	}
	ids := make([]int64, 0, len(clearAck))
	for instanceID := range clearAck {
		ids = append(ids, instanceID)
	}
	return ids
}

func buildPresenceWrites(plan []DeviationPresenceWrite) []DeviationPresenceWrite {
	return plan
}

func buildAbsenceWrites(plan []DeviationPresenceWrite, reasons map[int64]*string) []DeviationAbsenceWrite {
	out := make([]DeviationAbsenceWrite, 0, len(plan))
	for _, op := range plan {
		out = append(out, DeviationAbsenceWrite{Row: op.Row, Instance: op.Instance, Reason: reasons[op.Row.StaffID]})
	}
	return out
}

func buildSubstituteWrites(plan []deviationSubOp) []DeviationSubstituteWrite {
	out := make([]DeviationSubstituteWrite, 0, len(plan))
	for _, op := range plan {
		out = append(out, DeviationSubstituteWrite{
			Op:       SubstituteWriteOp{Instance: op.op.instance, OrigRow: op.op.origRow, Action: op.op.action},
			SubID:    op.subID,
			Reason:   op.reason,
			Instance: op.op.instance,
			Action:   op.op.action,
		})
	}
	return out
}

// classifySubstitute decides the action for a single target instance. Pure
// logic, no DB. Returns (action, conflictingOtherStaffID, ok=false on 409).
func classifySubstitute(
	allRows []*scheduleModel.InstanceStaff,
	origRow *scheduleModel.InstanceStaff,
	subID int64,
) (action string, conflictOtherStaff int64, ok bool) {
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

	if existingSubOfSub != nil {
		if origRow.IsAbsent {
			return SubstituteActionAlreadySubstitute, 0, true
		}
		return SubstituteActionAlreadyOnInstance, 0, true
	}

	if origRow.IsAbsent && activeSubsOfOther >= absentPlanned {
		return "", anyActiveSubOfOther.StaffID, false
	}

	if subAsNonAbsent != nil {
		return SubstituteActionAlreadyOnInstance, 0, true
	}
	return SubstituteActionSubstituted, 0, true
}

// projectAbsent returns a shallow-copied view of rows with the given staff ids
// forced absent, plus the copy of origRow. The originals are never mutated.
func projectAbsent(rows []*scheduleModel.InstanceStaff, absent map[int64]bool, origRow *scheduleModel.InstanceStaff) (projected []*scheduleModel.InstanceStaff, origProjected *scheduleModel.InstanceStaff) {
	projected = make([]*scheduleModel.InstanceStaff, len(rows))
	for i, row := range rows {
		clone := *row
		if absent[clone.StaffID] {
			clone.IsAbsent = true
		}
		projected[i] = &clone
		if row.ID == origRow.ID {
			origProjected = &clone
		}
	}
	if origProjected == nil {
		clone := *origRow
		origProjected = &clone
	}
	return projected, origProjected
}

// projectedNonAbsentCount counts staff that remain non-absent on an instance
// after the deviation writes.
func projectedNonAbsentCount(rows []*scheduleModel.InstanceStaff, fullAbsent, presence map[int64]bool, newSubs int) int {
	count := 0
	for _, row := range rows {
		if fullAbsent[row.StaffID] {
			continue
		}
		if row.IsAbsent && !presence[row.StaffID] {
			continue
		}
		count++
	}
	return count + newSubs
}

// dedupePositive returns the unique positive ids in order.
func dedupePositive(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func toSet(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// sameNote reports whether two optional notes carry the same trimmed text.
func sameNote(a, b *string) bool {
	av := ""
	if a != nil {
		av = *a
	}
	bv := ""
	if b != nil {
		bv = *b
	}
	return av == bv
}

// trimDeviationReason normalizes an optional deviation reason: nil/blank becomes
// nil, and an over-long value is truncated to the shared note ceiling on a rune
// boundary. Mirrors the api trimReason helper byte-for-byte.
func trimDeviationReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*reason)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > scheduleModel.ActivityExceptionReasonMaxLength {
		trimmed = string([]rune(trimmed)[:scheduleModel.ActivityExceptionReasonMaxLength])
	}
	return &trimmed
}
