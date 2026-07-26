// Package schedule — atomic Vertretungsplan save orchestration (#1840/#1886).
//
// ApplyDeviations owns the WHOLE deviations save in one service call:
// day-lock, validate + classify (Phase A, no writes), then the absence /
// presence / substitution writes plus acknowledgement reconciliation (Phase B).
// It was lifted out of the api/timetable handler so the handler only parses,
// dispatches, and shapes the response — the plan-then-write atomicity and every
// business rule live here, next to the write methods they drive.
//
// Atomicity note (same as the former handler): TenantTxMiddleware rolls the
// request tx back only on 5xx. A 409 rendered mid-save would commit prior
// writes, so Phase A validates + classifies everything before Phase B touches a
// single row. A DeviationError carries the exact HTTP status / code / message
// the handler renders, keeping the wire contract byte-identical.
package schedule

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// DeviationAbsenceInput marks one staff member absent day-wide with no
// substitute (a planned person left open, or a removed substitute).
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

// ApplyDeviationsInput is the parsed deviations request. All mutation fields are
// optional; a Cancel request is exclusive. UnderstaffedAck is a pointer so an
// omitted field ("no change") is distinguishable from an explicit false
// ("clear").
type ApplyDeviationsInput struct {
	Cancel           bool
	CancelReason     *string
	UnderstaffedAck  *bool
	UnderstaffedNote *string
	Absences         []DeviationAbsenceInput
	Substitutions    []DeviationSubstitutionInput
	Presences        []int64
	ActorAccountID   *int64
}

// DeviationAffected is one classified target the save touched — the neutral
// shape the handler maps onto its wire DTO.
type DeviationAffected struct {
	InstanceID int64
	Title      string
	StartTime  time.Time
	Action     string
}

// ApplyDeviationsResult is what ApplyDeviations returns on success. ActiveTouched
// plus the counts drive the handler's SSE broadcast and log line.
type ApplyDeviationsResult struct {
	InstanceID        int64
	Cancelled         bool
	UnderstaffedAck   bool
	Affected          []DeviationAffected
	Warnings          []SubstituteTimeConflict
	ActiveTouched     map[int64]*scheduleModel.ActivityInstance
	AppliedWrites     int
	AckChanged        bool
	ClearedAcks       int
	AbsenceCount      int
	PresenceCount     int
	SubstitutionCount int
	Message           string
}

// DeviationError carries the exact HTTP mapping the handler renders, so the
// deviations wire contract (status, code, message) stays byte-identical after
// the extraction. Cause is set only for 500 responses that wrap an internal
// error for logs while showing ClientMsg to the client.
type DeviationError struct {
	Status    int
	Code      string
	ClientMsg string
	Cause     error
}

func (e *DeviationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.ClientMsg, e.Cause)
	}
	return e.ClientMsg
}

func (e *DeviationError) Unwrap() error { return e.Cause }

func devErrBadRequest(msg string) *DeviationError {
	return &DeviationError{Status: http.StatusBadRequest, ClientMsg: msg}
}

func devErrNotFound(msg string) *DeviationError {
	return &DeviationError{Status: http.StatusNotFound, ClientMsg: msg}
}

func devErrConflict(code, msg string) *DeviationError {
	return &DeviationError{Status: http.StatusConflict, Code: code, ClientMsg: msg}
}

func devErrInternal(msg string, cause error) *DeviationError {
	return &DeviationError{Status: http.StatusInternalServerError, ClientMsg: msg, Cause: cause}
}

func devErrInternalPlain(msg string) *DeviationError {
	return &DeviationError{Status: http.StatusInternalServerError, ClientMsg: msg}
}

// deviationAbsenceOp pairs a plannable instance row of an absent staff member
// with its instance and the trimmed absence reason, ready for the Phase-B write.
type deviationAbsenceOp struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
	reason   *string
}

// deviationPresenceOp pairs a currently-absent row to be cleared with its
// instance.
type deviationPresenceOp struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
}

// deviationSubOp tags a classified substitution write with the substitute id and
// trimmed reason.
type deviationSubOp struct {
	write  SubstituteWriteOp
	subID  int64
	reason *string
}

// deviationPlan is the fully-classified Phase-A result: nothing here has written
// a row yet.
type deviationPlan struct {
	instance     *scheduleModel.ActivityInstance
	date         timezone.Date
	absencePlan  []deviationAbsenceOp
	presencePlan []deviationPresenceOp
	subPlan      []deviationSubOp
	subs         []DeviationSubstitutionInput
	finalAck     bool
	finalAckNote *string
	ackChanged   bool
}

// ApplyDeviations applies a whole Vertretungsplan slide-over save atomically.
// Runs inside the caller's tenant tx (TenantTxMiddleware).
func (s *instanceService) ApplyDeviations(ctx context.Context, instanceID int64, in ApplyDeviationsInput) (*ApplyDeviationsResult, error) {
	instance, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	// Past blocks are historical record; no deviation — including a cancellation
	// — may rewrite them. Guard before the exclusive cancel branch (#1840).
	if instance.Date.Before(timezone.TodayDate()) {
		return nil, devErrBadRequest("block date is in the past")
	}

	if in.Cancel {
		return s.cancelDeviation(ctx, instanceID, instance, in)
	}

	if !isPlannableInstance(instance) {
		return nil, devErrConflict("invalid_transition", fmt.Sprintf("cannot edit instance in status %q", instance.Status))
	}

	if in.UnderstaffedNote != nil && utf8.RuneCountInString(*in.UnderstaffedNote) > scheduleModel.ActivityExceptionReasonMaxLength {
		return nil, devErrBadRequest("note is too long")
	}

	// Reject non-positive staff ids on the RAW request, before dedupePositive
	// silently drops them (#1840).
	for _, a := range in.Absences {
		if a.StaffID <= 0 {
			return nil, devErrBadRequest("absent staff must be a positive id")
		}
	}
	for _, pid := range in.Presences {
		if pid <= 0 {
			return nil, devErrBadRequest("present staff must be a positive id")
		}
	}

	plan, err := s.planDeviations(ctx, instanceID, instance, in)
	if err != nil {
		return nil, err
	}
	return s.executeDeviationPlan(ctx, instanceID, in, plan)
}

// loadDeviationInstance loads the target instance, mapping the absent/other-
// tenant case to 404 and any other failure to a wrapped 500.
func (s *instanceService) loadDeviationInstance(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.deps.InstanceRepo.FindByID(ctx, instanceID)
	if err != nil {
		// FindByID wraps sql.ErrNoRows in a DatabaseError (never (nil, nil)), so a
		// stale link or deleted/other-tenant instance maps to 404 here.
		if modelBase.IsNoRows(err) {
			return nil, devErrNotFound("instance not found")
		}
		return nil, devErrInternal("load instance failed", err)
	}
	if instance == nil {
		return nil, devErrNotFound("instance not found")
	}
	return instance, nil
}

// cancelDeviation applies the exclusive cancel branch: it serializes against
// concurrent same-day saves, re-reads under the lock to catch a concurrent move,
// then delegates to the shared Cancel service.
func (s *instanceService) cancelDeviation(ctx context.Context, instanceID int64, instance *scheduleModel.ActivityInstance, in ApplyDeviationsInput) (*ApplyDeviationsResult, error) {
	if err := s.acquireSubstituteDayLock(ctx, instance.Date); err != nil {
		return nil, devErrInternal("lock day failed", err)
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
		return nil, devErrConflict("instance_moved", "block was changed concurrently; reopen it and try again")
	}
	// A move to a past day would rewrite history; the initial guard ran against a
	// possibly-stale read, so re-check under the lock.
	if locked.Date.Before(timezone.TodayDate()) {
		return nil, devErrBadRequest("block date is in the past")
	}
	cancelled, err := s.Cancel(ctx, instanceID, trimDeviationReason(in.CancelReason), in.ActorAccountID)
	if err != nil {
		return nil, err
	}
	return &ApplyDeviationsResult{
		InstanceID:      cancelled.ID,
		Cancelled:       true,
		UnderstaffedAck: cancelled.UnderstaffedAck,
		Affected:        []DeviationAffected{},
		Warnings:        []SubstituteTimeConflict{},
		Message:         "Block cancelled",
	}, nil
}

// planDeviations runs the whole Phase-A dry-run: it takes the day lock, re-reads
// under it, validates every staff reference, classifies the absence / presence /
// substitution writes, and reconciles the selected block's acknowledgement — all
// without writing a row.
func (s *instanceService) planDeviations(ctx context.Context, instanceID int64, instance *scheduleModel.ActivityInstance, in ApplyDeviationsInput) (*deviationPlan, error) {
	date := instance.Date

	// Serialize concurrent saves for the whole (tenant, date) BEFORE any
	// classification read. A substitution/absence is day-wide: it touches every
	// same-day block of the affected staff, not just this one (#1840).
	if err := s.acquireSubstituteDayLock(ctx, date); err != nil {
		return nil, devErrInternal("lock day failed", err)
	}
	// Re-read now that the day is locked: PUT /instances/{id} may have MOVED the
	// block to another day between the initial read and this lock. Detect a move
	// (or a concurrent cancel/complete) and abort (#1840).
	locked, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if locked.Date != date || !isPlannableInstance(locked) {
		return nil, devErrConflict("instance_moved", "block was changed concurrently; reopen it and try again")
	}
	instance = locked

	absenceIDs := make([]int64, 0, len(in.Absences))
	for _, a := range in.Absences {
		absenceIDs = append(absenceIDs, a.StaffID)
	}
	absenceOnly := dedupePositive(absenceIDs)
	absenceReason := make(map[int64]*string, len(in.Absences))
	for _, a := range in.Absences {
		absenceReason[a.StaffID] = trimDeviationReason(a.Reason)
	}
	absenceOnlySet := toSet(absenceOnly)

	// fullAbsent = absence-only staff plus every substituted person.
	fullAbsent := toSet(absenceOnly)
	for _, sub := range in.Substitutions {
		fullAbsent[sub.AbsentStaffID] = true
	}

	presences := dedupePositive(in.Presences)
	presenceSet := toSet(presences)

	if err := s.validateDeviationStaff(ctx, in, absenceOnlySet, fullAbsent, presenceSet, date); err != nil {
		return nil, err
	}

	absencePlan, err := s.planAbsences(ctx, absenceOnly, absenceReason, date)
	if err != nil {
		return nil, err
	}
	presencePlan, err := s.planPresences(ctx, presences, date)
	if err != nil {
		return nil, err
	}
	subPlan, newSubByInstance, err := s.planSubstitutions(ctx, in.Substitutions, absenceOnlySet, date)
	if err != nil {
		return nil, err
	}

	// Restoring a persisted absence must not orphan an already-assigned
	// substitute (over-staffing). Reject before any write (#1840).
	if err := s.rejectOverstaffingPresences(ctx, presencePlan, fullAbsent, presenceSet, newSubByInstance); err != nil {
		return nil, err
	}

	finalAck, finalAckNote, ackChanged, err := s.reconcileSelectedAck(ctx, instanceID, instance, in, fullAbsent, presenceSet, newSubByInstance)
	if err != nil {
		return nil, err
	}

	return &deviationPlan{
		instance:     instance,
		date:         date,
		absencePlan:  absencePlan,
		presencePlan: presencePlan,
		subPlan:      subPlan,
		subs:         in.Substitutions,
		finalAck:     finalAck,
		finalAckNote: finalAckNote,
		ackChanged:   ackChanged,
	}, nil
}

// validateDeviationStaff runs every 4xx precondition on the referenced staff:
// existence (404), self-substitution (400), a substitute also being marked
// absent (400), and a substitute already absent in the DB that day (400).
func (s *instanceService) validateDeviationStaff(
	ctx context.Context,
	in ApplyDeviationsInput,
	absenceOnlySet, fullAbsent, presenceSet map[int64]bool,
	date timezone.Date,
) error {
	seen := make(map[int64]bool)
	ensure := func(staffID int64, label string) error {
		if staffID <= 0 {
			return devErrBadRequest(fmt.Sprintf("%s must be a positive id", label))
		}
		if seen[staffID] {
			return nil
		}
		seen[staffID] = true
		staff, err := s.deps.StaffRepo.FindByID(ctx, staffID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return devErrNotFound(fmt.Sprintf("%s not found", label))
			}
			return devErrInternal("load staff failed", err)
		}
		if staff == nil || staff.ID == 0 {
			return devErrNotFound(fmt.Sprintf("%s not found", label))
		}
		return nil
	}

	for id := range absenceOnlySet {
		if err := ensure(id, "absent staff"); err != nil {
			return err
		}
	}
	for id := range presenceSet {
		// A person cannot be marked present and absent (or substituted-away) in
		// the same save — contradictory day-wide states.
		if fullAbsent[id] {
			return devErrBadRequest("staff cannot be marked present and absent in the same request")
		}
		if err := ensure(id, "present staff"); err != nil {
			return err
		}
	}
	seenAbsentSub := make(map[int64]bool)
	for _, sub := range in.Substitutions {
		// Each absent person's substitution is day-wide, so an absent staff id must
		// appear at most once (#1840).
		if seenAbsentSub[sub.AbsentStaffID] {
			return devErrBadRequest("each absent staff member may have at most one substitute per request")
		}
		seenAbsentSub[sub.AbsentStaffID] = true
		if err := ensure(sub.AbsentStaffID, "absent staff"); err != nil {
			return err
		}
		if err := ensure(sub.SubstituteStaffID, "substitute staff"); err != nil {
			return err
		}
		if sub.AbsentStaffID == sub.SubstituteStaffID {
			return devErrBadRequest("absent and substitute staff must differ")
		}
		// A person being marked absent anywhere in this form cannot also cover a
		// shift — absence is day-wide.
		if fullAbsent[sub.SubstituteStaffID] {
			return devErrBadRequest("substitute is marked absent in this request")
		}
		// ...nor if they are already absent that day in the DB.
		subRows, err := s.deps.InstanceStaffRepo.FindByStaffAndDate(ctx, sub.SubstituteStaffID, date)
		if err != nil {
			return devErrInternal("load substitute assignments failed", err)
		}
		for _, row := range subRows {
			if row.IsAbsent {
				return devErrBadRequest("substitute is marked absent on this date")
			}
		}
	}
	return nil
}

// planAbsences loads every absent-only staff member's plannable same-day rows.
func (s *instanceService) planAbsences(ctx context.Context, absentStaffIDs []int64, absenceReason map[int64]*string, date timezone.Date) ([]deviationAbsenceOp, error) {
	plan := make([]deviationAbsenceOp, 0)
	for _, staffID := range absentStaffIDs {
		rows, err := s.deps.InstanceStaffRepo.FindByStaffAndDate(ctx, staffID, date)
		if err != nil {
			return nil, devErrInternal("load absent assignments failed", err)
		}
		for _, row := range rows {
			if row.IsAbsent {
				continue // idempotent: already absent, no write
			}
			instance, err := s.loadPlannableInstance(ctx, row)
			if err != nil {
				return nil, err
			}
			if instance == nil {
				continue // terminal instance, skip
			}
			plan = append(plan, deviationAbsenceOp{row: row, instance: instance, reason: absenceReason[row.StaffID]})
		}
	}
	return plan, nil
}

// planPresences loads every to-be-restored staff member's plannable same-day
// rows that are currently marked absent (day-wide clear, #1840). A non-absent
// row is a no-op.
func (s *instanceService) planPresences(ctx context.Context, presentStaffIDs []int64, date timezone.Date) ([]deviationPresenceOp, error) {
	plan := make([]deviationPresenceOp, 0)
	for _, staffID := range presentStaffIDs {
		rows, err := s.deps.InstanceStaffRepo.FindByStaffAndDate(ctx, staffID, date)
		if err != nil {
			return nil, devErrInternal("load present assignments failed", err)
		}
		for _, row := range rows {
			if !row.IsAbsent {
				continue // only a persisted absence can be cleared
			}
			instance, err := s.loadPlannableInstance(ctx, row)
			if err != nil {
				return nil, err
			}
			if instance == nil {
				continue // terminal instance, skip
			}
			plan = append(plan, deviationPresenceOp{row: row, instance: instance})
		}
	}
	return plan, nil
}

// planSubstitutions classifies every substitution against a projected view of
// each instance (absence-only staff read as absent). Returns the write plan and,
// per instance, how many NEW substitute rows will be added (for the ack check).
func (s *instanceService) planSubstitutions(
	ctx context.Context,
	subs []DeviationSubstitutionInput,
	absenceOnlySet map[int64]bool,
	date timezone.Date,
) ([]deviationSubOp, map[int64]int, error) {
	plan := make([]deviationSubOp, 0)
	newSubByInstance := make(map[int64]int)
	// Track which NEW substitute rows are staged per instance in THIS request,
	// keyed by (instance, substitute). Only a REPEATED (instance, substitute)
	// pairing must collapse, or Phase B would insert the same row twice (#1840).
	stagedSubs := make(map[int64]map[int64]bool)

	for _, sub := range subs {
		origRows, err := s.deps.InstanceStaffRepo.FindByStaffAndDate(ctx, sub.AbsentStaffID, date)
		if err != nil {
			return nil, nil, devErrInternal("load absent assignments failed", err)
		}
		reason := trimDeviationReason(sub.Reason)
		for _, orig := range origRows {
			instance, err := s.loadPlannableInstance(ctx, orig)
			if err != nil {
				return nil, nil, err
			}
			if instance == nil {
				continue
			}
			allRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instance.ID)
			if err != nil {
				return nil, nil, devErrInternal("load instance staff failed", err)
			}
			projectedRows, origProjected := projectAbsent(allRows, absenceOnlySet, orig)
			action, conflictOther, ok := classifySubstitute(projectedRows, origProjected, sub.SubstituteStaffID)
			if !ok {
				return nil, nil, devErrConflict("substitute_conflict", fmt.Sprintf(
					"instance %d is already fully covered by another substitute (staff_id=%d); remove a replacement before adding one",
					instance.ID, conflictOther))
			}
			// The SAME substitute staged twice on one block would insert a duplicate
			// row in Phase B. Collapse the repeat into already_on_instance (#1840).
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
				write:  SubstituteWriteOp{Instance: instance, OrigRow: orig, Action: action},
				subID:  sub.SubstituteStaffID,
				reason: reason,
			})
		}
	}
	return plan, newSubByInstance, nil
}

// rejectOverstaffingPresences returns a 409 when clearing a persisted absence
// would push any touched instance above its planned headcount, which only
// happens when a restore orphans an already-assigned substitute (#1840).
func (s *instanceService) rejectOverstaffingPresences(
	ctx context.Context,
	presencePlan []deviationPresenceOp,
	fullAbsent, presenceSet map[int64]bool,
	newSubByInstance map[int64]int,
) error {
	checked := make(map[int64]bool)
	for _, op := range presencePlan {
		if checked[op.instance.ID] {
			continue
		}
		checked[op.instance.ID] = true
		rows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, op.instance.ID)
		if err != nil {
			return devErrInternal("load instance staff failed", err)
		}
		baseline := 0
		for _, row := range rows {
			if !row.IsSubstitute {
				baseline++
			}
		}
		if projectedNonAbsentCount(rows, fullAbsent, presenceSet, newSubByInstance[op.instance.ID]) > baseline {
			return devErrConflict("presence_would_overstaff",
				"das Wiederherstellen dieser Anwesenheit würde den Block überbesetzen; bitte zuerst die nicht mehr benötigte Vertretung entfernen")
		}
	}
	return nil
}

// loadPlannableInstance loads the instance behind a staff row, returning nil when
// it is terminal (completed/cancelled) so callers skip it.
func (s *instanceService) loadPlannableInstance(ctx context.Context, row *scheduleModel.InstanceStaff) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.deps.InstanceRepo.FindByID(ctx, row.InstanceID)
	if err != nil {
		return nil, devErrInternal("load target instance failed", err)
	}
	if instance == nil {
		return nil, devErrInternalPlain(fmt.Sprintf("instance_staff %d references missing instance %d", row.ID, row.InstanceID))
	}
	if !isPlannableInstance(instance) {
		return nil, nil
	}
	return instance, nil
}

// reconcileSelectedAck decides the selected block's final acknowledgement after
// the projected save. "Deliberately unstaffed" is valid whenever the block ends
// up understaffed; the deviation writes never change the planned baseline
// (#1840).
func (s *instanceService) reconcileSelectedAck(
	ctx context.Context,
	instanceID int64,
	instance *scheduleModel.ActivityInstance,
	in ApplyDeviationsInput,
	fullAbsent, presenceSet map[int64]bool,
	newSubByInstance map[int64]int,
) (finalAck bool, note *string, ackChanged bool, err error) {
	thisRows, loadErr := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instanceID)
	if loadErr != nil {
		return false, nil, false, devErrInternal("load instance staff failed", loadErr)
	}
	projectedPresent := projectedNonAbsentCount(thisRows, fullAbsent, presenceSet, newSubByInstance[instanceID])
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
			return false, nil, false, devErrConflict("understaffed_still_staffed",
				"dieser Block kann nicht als bewusst unbesetzt markiert werden, solange er vollständig besetzt ist")
		}
		finalAck = *in.UnderstaffedAck
		ackChanged = finalAck != instance.UnderstaffedAck ||
			(finalAck && !sameNote(instance.UnderstaffedNote, in.UnderstaffedNote))
		if finalAck {
			note = trimDeviationReason(in.UnderstaffedNote)
		}
	} else if instance.UnderstaffedAck && !projectedUnderstaffed {
		finalAck = false
		ackChanged = true
	}
	return finalAck, note, ackChanged, nil
}

// executeDeviationPlan runs Phase B: it applies every classified write, sets the
// selected block's acknowledgement, reconciles stale acks on the other covered
// blocks, and gathers time-conflict advisories.
func (s *instanceService) executeDeviationPlan(ctx context.Context, instanceID int64, in ApplyDeviationsInput, plan *deviationPlan) (*ApplyDeviationsResult, error) {
	affected, activeTouched, err := s.writeDeviationOps(ctx, in.ActorAccountID, plan)
	if err != nil {
		return nil, err
	}

	if plan.ackChanged {
		if _, err := s.SetUnderstaffedAck(ctx, instanceID, plan.finalAck, plan.finalAckNote, in.ActorAccountID); err != nil {
			// A concurrent cancel/full-staffing of THIS instance makes
			// SetUnderstaffedAck return a 4xx after the writes above already
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
		return nil, devErrInternal("deviation time-conflict detection failed", err)
	}

	return &ApplyDeviationsResult{
		InstanceID:        instanceID,
		Cancelled:         false,
		UnderstaffedAck:   plan.finalAck,
		Affected:          affected,
		Warnings:          warnings,
		ActiveTouched:     activeTouched,
		AppliedWrites:     len(affected),
		AckChanged:        plan.ackChanged,
		ClearedAcks:       cleared,
		AbsenceCount:      len(plan.absencePlan),
		PresenceCount:     len(plan.presencePlan),
		SubstitutionCount: len(plan.subPlan),
		Message:           "Deviations applied",
	}, nil
}

// writeDeviationOps applies the presence, absence, and substitution writes in
// that order and returns the affected-instance rows plus the active groups they
// touched.
func (s *instanceService) writeDeviationOps(ctx context.Context, actor *int64, plan *deviationPlan) ([]DeviationAffected, map[int64]*scheduleModel.ActivityInstance, error) {
	now := time.Now()
	affected := make([]DeviationAffected, 0, len(plan.absencePlan)+len(plan.subPlan)+len(plan.presencePlan))
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)

	for _, op := range plan.presencePlan {
		if err := s.ApplyPresence(ctx, op.row, op.instance, actor, activeTouched); err != nil {
			return nil, nil, devErrInternal("clear absence failed", err)
		}
		affected = append(affected, affectedOf(op.instance, SubstituteActionMarkedPresent))
	}
	for _, op := range plan.absencePlan {
		if err := s.ApplyAbsence(ctx, op.row, op.instance, op.reason, actor, activeTouched); err != nil {
			return nil, nil, devErrInternal("mark absent failed", err)
		}
		affected = append(affected, affectedOf(op.instance, SubstituteActionMarkedAbsent))
	}
	for _, op := range plan.subPlan {
		if err := s.ApplySubstitute(ctx, op.write, op.subID, op.reason, now, actor, activeTouched); err != nil {
			return nil, nil, devErrInternal("assign substitute failed", err)
		}
		affected = append(affected, affectedOf(op.write.Instance, op.write.Action))
	}
	return affected, activeTouched, nil
}

// reconcileOtherAcks clears stale "deliberately unstaffed" acknowledgements on
// the OTHER instances this save covered (the selected block's ack is handled
// explicitly). Only ever clears (#1840).
func (s *instanceService) reconcileOtherAcks(ctx context.Context, instanceID int64, actor *int64, plan *deviationPlan) (int, error) {
	clearAck := make(map[int64]bool)
	for _, op := range plan.subPlan {
		if op.write.Instance.ID == instanceID {
			continue
		}
		if op.write.Instance.UnderstaffedAck &&
			(op.write.Action == SubstituteActionSubstituted || op.write.Action == SubstituteActionAlreadyOnInstance) {
			clearAck[op.write.Instance.ID] = true
		}
	}
	for _, op := range plan.presencePlan {
		if op.instance.ID == instanceID {
			continue
		}
		if op.instance.UnderstaffedAck {
			clearAck[op.instance.ID] = true
		}
	}
	for id := range clearAck {
		if err := s.ClearUnderstaffedAckIfStaffed(ctx, id, actor); err != nil {
			return 0, devErrInternal("clear stale understaffed ack failed", err)
		}
	}
	return len(clearAck), nil
}

// collectDeviationWarnings merges per-substitute time-conflict advisories. A
// lookup failure propagates as an error: the probe runs inside the tenant tx,
// and a PostgreSQL error aborts that tx, so the eventual commit would fail
// after the client already saw a 200.
func (s *instanceService) collectDeviationWarnings(
	ctx context.Context,
	subs []DeviationSubstitutionInput,
	plan []deviationSubOp,
	date timezone.Date,
) ([]SubstituteTimeConflict, error) {
	warnings := make([]SubstituteTimeConflict, 0)
	// One substitute can cover several absent colleagues, duplicating conflicts
	// two ways: a repeated SubstituteStaffID in subs, and two ops staged on one
	// block. Skip repeated ids and drop duplicate (substitute, instance, other)
	// triples so each real overlap surfaces once (#1840).
	seenSub := make(map[int64]bool, len(subs))
	seenConflict := make(map[[3]int64]bool)
	for _, sub := range subs {
		if seenSub[sub.SubstituteStaffID] {
			continue
		}
		seenSub[sub.SubstituteStaffID] = true
		ops := make([]SubstituteWriteOp, 0, len(plan))
		for _, p := range plan {
			if p.subID == sub.SubstituteStaffID {
				ops = append(ops, p.write)
			}
		}
		w, err := s.buildSubstituteTimeConflicts(ctx, ops, sub.SubstituteStaffID, date)
		if err != nil {
			return nil, err
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
	return warnings, nil
}

// buildSubstituteTimeConflicts loads the substitute's OTHER (non-target)
// same-day non-absent assignments and returns overlap warnings. No writes.
func (s *instanceService) buildSubstituteTimeConflicts(
	ctx context.Context,
	plan []SubstituteWriteOp,
	subID int64,
	date timezone.Date,
) ([]SubstituteTimeConflict, error) {
	if len(plan) == 0 {
		return nil, nil
	}
	subRows, err := s.deps.InstanceStaffRepo.FindByStaffAndDate(ctx, subID, date)
	if err != nil {
		return nil, err
	}
	targetSet := make(map[int64]bool, len(plan))
	for _, op := range plan {
		targetSet[op.Instance.ID] = true
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
		inst, err := s.deps.InstanceRepo.FindByID(ctx, fid)
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
		if op.Action == SubstituteActionAlreadySubstitute {
			// Already substituted earlier — nothing written; skip as a target.
			continue
		}
		// already_on_instance is intentionally kept: the co-supervisor's existing
		// overlaps remain relevant information for the admin.
		targets = append(targets, toConflictInstance(op.Instance))
	}
	return DetectSubstituteTimeConflicts(targets, foreigns), nil
}

// AcknowledgeUnderstaffed applies the standalone "deliberately unstaffed"
// acknowledgement. It gates past blocks as historical record and serializes
// against same-day staffing saves before delegating to SetUnderstaffedAck; note
// trimming/validation is the caller's (the note arrives already normalized).
func (s *instanceService) AcknowledgeUnderstaffed(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// Past blocks are historical record and read-only. Status alone is
	// insufficient: a materialized past occurrence can still be planned/active,
	// so gate on the Berlin calendar date (#1840).
	if instance.Date.Before(timezone.TodayDate()) {
		return nil, devErrBadRequest("block date is in the past")
	}
	// Serialize with substitute/deviation saves on the same (tenant, date) before
	// validating and writing the acknowledgement (#1840).
	if err := s.acquireSubstituteDayLock(ctx, instance.Date); err != nil {
		return nil, devErrInternal("lock day failed", err)
	}
	// Re-read under the lock: a concurrent PUT may have MOVED the block to another
	// day; a move to a past day would bypass the historical-record guard, a move
	// to another day would leave this ack unsynchronized. Detect and reject.
	locked, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if locked.Date != instance.Date {
		return nil, devErrConflict("instance_moved", "block was changed concurrently; reopen it and try again")
	}
	return s.SetUnderstaffedAck(ctx, instanceID, ack, note, actorAccountID)
}

// acquireSubstituteDayLock takes the shared (tenant, date) advisory lock that
// serializes every day-wide staffing mutation, within the caller's tx.
func (s *instanceService) acquireSubstituteDayLock(ctx context.Context, date timezone.Date) error {
	return repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenant.FromContext(ctx), date))
}

// affectedOf builds a neutral affected-instance row.
func affectedOf(inst *scheduleModel.ActivityInstance, action string) DeviationAffected {
	return DeviationAffected{
		InstanceID: inst.ID,
		Title:      inst.Title,
		StartTime:  inst.StartTime,
		Action:     action,
	}
}

// isPlannableInstance reports whether a substitute/absence write may touch this
// instance. Only planned and active blocks are editable; completed and cancelled
// ones are historical record.
func isPlannableInstance(instance *scheduleModel.ActivityInstance) bool {
	return instance.Status == scheduleModel.InstanceStatusPlanned ||
		instance.Status == scheduleModel.InstanceStatusActive
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

	// The substitute already holds a substitute row on this instance. A second
	// insert would violate UNIQUE(instance_id, staff_id) (#1840).
	if existingSubOfSub != nil {
		if origRow.IsAbsent {
			return SubstituteActionAlreadySubstitute, 0, true
		}
		return SubstituteActionAlreadyOnInstance, 0, true
	}

	// Count-based overstaffing guard (#1840): reject a new substitute only when
	// origRow is already flagged absent AND every absent position is already
	// covered — adding one more would overstaff.
	if origRow.IsAbsent && activeSubsOfOther >= absentPlanned {
		return "", anyActiveSubOfOther.StaffID, false
	}

	if subAsNonAbsent != nil {
		// Substitute is already a co-supervisor on this instance; mark the absent's
		// row and leave the co-supervisor row untouched (is_substitute=false).
		return SubstituteActionAlreadyOnInstance, 0, true
	}
	return SubstituteActionSubstituted, 0, true
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

// sameNote reports whether two optional notes carry the same text.
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
// nil, and an over-long value is truncated to the shared note ceiling (rune-
// based so a multi-byte rune is never split).
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
