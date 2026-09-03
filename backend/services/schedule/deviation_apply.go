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
// single row. A DeviationError separates the stable client response from the
// internal cause retained for logs.
package schedule

import (
	"context"
	"errors"
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

// DeviationAbsenceInput marks one staff member absent. Nil InstanceIDs means
// every plannable same-day appointment; a non-nil list targets exact ones.
type DeviationAbsenceInput struct {
	StaffID     int64
	Reason      *string
	InstanceIDs *[]int64
}

// DeviationSubstitutionInput assigns SubstituteStaffID to cover AbsentStaffID.
// Nil InstanceIDs means every plannable same-day appointment; a non-nil list
// targets exactly those appointments.
type DeviationSubstitutionInput struct {
	AbsentStaffID     int64
	SubstituteStaffID int64
	Reason            *string
	InstanceIDs       *[]int64
}

// DeviationPresenceInput clears an absence. Nil InstanceIDs means every
// plannable same-day appointment; a non-nil list targets exact ones.
type DeviationPresenceInput struct {
	StaffID     int64
	InstanceIDs *[]int64
}

// DeviationSubstitutionRemovalInput removes a substitute assignment without
// changing that person's other appointments. Nil InstanceIDs means every
// plannable same-day appointment; a non-nil list targets exact ones.
type DeviationSubstitutionRemovalInput struct {
	StaffID     int64
	InstanceIDs *[]int64
}

// ApplyDeviationsInput is the parsed deviations request. All mutation fields are
// optional; a Cancel request is exclusive. UnderstaffedAck is a pointer so an
// omitted field ("no change") is distinguishable from an explicit false
// ("clear").
type ApplyDeviationsInput struct {
	Cancel               bool
	CancelReason         *string
	UnderstaffedAck      *bool
	UnderstaffedNote     *string
	Absences             []DeviationAbsenceInput
	Substitutions        []DeviationSubstitutionInput
	SubstitutionRemovals []DeviationSubstitutionRemovalInput
	Presences            []DeviationPresenceInput
	ActorAccountID       *int64
	// GuardianNotice informs the families when Cancel is set (#2601). Ignored
	// on a non-cancel save.
	GuardianNotice *GuardianNoticeInput
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
	InstanceID      int64
	Cancelled       bool
	UnderstaffedAck bool
	Affected        []DeviationAffected
	Warnings        []SubstituteTimeConflict
	ActiveTouched   map[int64]*scheduleModel.ActivityInstance
	AppliedWrites   int
	// GuardianNotice is the notice outcome of a cancel save, nil otherwise.
	GuardianNotice           *GuardianNoticeResult
	AckChanged               bool
	ClearedAcks              int
	AbsenceCount             int
	PresenceCount            int
	SubstitutionCount        int
	SubstitutionRemovalCount int
	Message                  string
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

const deviationSaveErrorMessage = "Die Änderungen konnten nicht gespeichert werden. Versuchen Sie es erneut."

func devErrInternal(operation string, cause error) *DeviationError {
	return &DeviationError{
		Status:    http.StatusInternalServerError,
		ClientMsg: deviationSaveErrorMessage,
		Cause:     fmt.Errorf("%s: %w", operation, cause),
	}
}

func devErrInternalPlain(detail string) *DeviationError {
	return &DeviationError{
		Status:    http.StatusInternalServerError,
		ClientMsg: deviationSaveErrorMessage,
		Cause:     errors.New(detail),
	}
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

type deviationSubstitutionRemovalOp struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
}

// deviationStaffByInstance is the projected staff state keyed by concrete
// appointment. A person's state on one appointment must never leak into a
// different appointment on the same day.
type deviationStaffByInstance map[int64]map[int64]bool

// deviationPlan is the fully-classified Phase-A result: nothing here has written
// a row yet.
type deviationPlan struct {
	instance     *scheduleModel.ActivityInstance
	date         timezone.Date
	absencePlan  []deviationAbsenceOp
	presencePlan []deviationPresenceOp
	subPlan      []deviationSubOp
	removalPlan  []deviationSubstitutionRemovalOp
	subs         []DeviationSubstitutionInput
	finalAck     bool
	finalAckNote *string
	ackChanged   bool
}

type deviationReadSet struct {
	staffExists    map[int64]bool
	rowsByStaff    map[int64][]*scheduleModel.InstanceStaff
	instances      map[int64]*scheduleModel.ActivityInstance
	rowsByInstance map[int64][]*scheduleModel.InstanceStaff
}

func (s *instanceService) loadDeviationReadSet(
	ctx context.Context,
	instanceID int64,
	in ApplyDeviationsInput,
	date timezone.Date,
) (*deviationReadSet, error) {
	staffIDs := deviationInputStaffIDs(in)
	staff, err := s.deps.StaffRepo.FindByIDs(ctx, staffIDs)
	if err != nil {
		return nil, devErrInternal("load staff failed", err)
	}
	rows, err := s.deps.InstanceStaffRepo.FindByStaffIDsAndDate(ctx, staffIDs, date)
	if err != nil {
		return nil, devErrInternal("load staff assignments failed", err)
	}
	instanceIDs := deviationInputInstanceIDs(instanceID, in, rows)
	instances, err := s.deps.InstanceRepo.FindByIDs(ctx, instanceIDs)
	if err != nil {
		return nil, devErrInternal("load target instances failed", err)
	}
	allRows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, devErrInternal("load instance staff failed", err)
	}
	readSet := &deviationReadSet{
		staffExists:    make(map[int64]bool, len(staff)),
		rowsByStaff:    make(map[int64][]*scheduleModel.InstanceStaff, len(staffIDs)),
		instances:      make(map[int64]*scheduleModel.ActivityInstance, len(instances)),
		rowsByInstance: indexInstanceStaffRows(allRows),
	}
	for id := range staff {
		readSet.staffExists[id] = true
	}
	for _, row := range rows {
		readSet.rowsByStaff[row.StaffID] = append(readSet.rowsByStaff[row.StaffID], row)
	}
	for _, instance := range instances {
		readSet.instances[instance.ID] = instance
	}
	return readSet, nil
}

func deviationInputStaffIDs(in ApplyDeviationsInput) []int64 {
	seen := make(map[int64]bool)
	ids := make([]int64, 0)
	add := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, row := range in.Absences {
		add(row.StaffID)
	}
	for _, row := range in.Presences {
		add(row.StaffID)
	}
	for _, row := range in.Substitutions {
		add(row.AbsentStaffID)
		add(row.SubstituteStaffID)
	}
	for _, row := range in.SubstitutionRemovals {
		add(row.StaffID)
	}
	return ids
}

func deviationInputInstanceIDs(instanceID int64, in ApplyDeviationsInput, rows []*scheduleModel.InstanceStaff) []int64 {
	seen := make(map[int64]bool)
	ids := make([]int64, 0)
	if instanceID > 0 {
		seen[instanceID] = true
		ids = append(ids, instanceID)
	}
	addScope := func(scope *[]int64) {
		if scope == nil {
			return
		}
		for _, id := range *scope {
			if id > 0 && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	for _, row := range in.Absences {
		addScope(row.InstanceIDs)
	}
	for _, row := range in.Presences {
		addScope(row.InstanceIDs)
	}
	for _, row := range in.Substitutions {
		addScope(row.InstanceIDs)
	}
	for _, row := range in.SubstitutionRemovals {
		addScope(row.InstanceIDs)
	}
	for _, row := range rows {
		if !seen[row.InstanceID] {
			seen[row.InstanceID] = true
			ids = append(ids, row.InstanceID)
		}
	}
	return ids
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
		return nil, devErrBadRequest("dieser Termin liegt in der Vergangenheit")
	}

	if in.Cancel {
		return s.cancelDeviation(ctx, instanceID, instance, in)
	}

	if !isPlannableInstance(instance) {
		return nil, devErrConflict("invalid_transition", "dieser Termin kann nicht mehr geändert werden")
	}

	if in.UnderstaffedNote != nil && utf8.RuneCountInString(*in.UnderstaffedNote) > scheduleModel.ActivityExceptionReasonMaxLength {
		return nil, devErrBadRequest("der Hinweis ist zu lang")
	}

	// Reject non-positive staff ids on the raw request before any repository
	// lookup or write planning.
	for _, a := range in.Absences {
		if a.StaffID <= 0 {
			return nil, devErrBadRequest("die Auswahl der abwesenden Person ist ungültig")
		}
	}
	for _, presence := range in.Presences {
		if presence.StaffID <= 0 {
			return nil, devErrBadRequest("die Auswahl der anwesenden Person ist ungültig")
		}
	}
	for _, removal := range in.SubstitutionRemovals {
		if removal.StaffID <= 0 {
			return nil, devErrBadRequest("die Auswahl der Ersatzperson ist ungültig")
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
			return nil, devErrNotFound("der Termin wurde nicht gefunden")
		}
		return nil, devErrInternal("load instance failed", err)
	}
	if instance == nil {
		return nil, devErrNotFound("der Termin wurde nicht gefunden")
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
		return nil, devErrConflict("instance_moved", "der Termin wurde gleichzeitig geändert. Öffnen Sie ihn erneut")
	}
	// A move to a past day would rewrite history; the initial guard ran against a
	// possibly-stale read, so re-check under the lock.
	if locked.Date.Before(timezone.TodayDate()) {
		return nil, devErrBadRequest("dieser Termin liegt in der Vergangenheit")
	}
	cancelled, err := s.CancelWithNotice(ctx, CancelInstanceInput{
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
	return &ApplyDeviationsResult{
		InstanceID:      cancelled.Instance.ID,
		Cancelled:       true,
		UnderstaffedAck: cancelled.Instance.UnderstaffedAck,
		Affected:        []DeviationAffected{},
		Warnings:        []SubstituteTimeConflict{},
		GuardianNotice:  cancelled.GuardianNotice,
		Message:         message,
	}, nil
}

// planDeviations runs the whole Phase-A dry-run: it takes the day lock, re-reads
// under it, validates every staff reference, classifies the absence / presence /
// substitution writes, and reconciles the selected block's acknowledgement — all
// without writing a row.
func (s *instanceService) planDeviations(ctx context.Context, instanceID int64, instance *scheduleModel.ActivityInstance, in ApplyDeviationsInput) (*deviationPlan, error) {
	date := instance.Date

	// Serialize concurrent saves for the whole (tenant, date) BEFORE any
	// classification read. One request may target several appointments on the
	// day, while the Sammel-Vertretung still targets the whole day.
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
		return nil, devErrConflict("instance_moved", "der Termin wurde gleichzeitig geändert. Öffnen Sie ihn erneut")
	}
	instance = locked

	readSet, err := s.loadDeviationReadSet(ctx, instanceID, in, date)
	if err != nil {
		return nil, err
	}
	if err := validateDeviationStaff(in, readSet); err != nil {
		return nil, err
	}
	if err := rejectContradictoryDeviationScopes(in); err != nil {
		return nil, err
	}

	absencePlan, err := planAbsences(in.Absences, date, readSet)
	if err != nil {
		return nil, err
	}
	presencePlan, err := planPresences(in.Presences, date, readSet)
	if err != nil {
		return nil, err
	}
	removalPlan, err := planSubstitutionRemovals(in.SubstitutionRemovals, date, readSet)
	if err != nil {
		return nil, err
	}
	absenceOnlyByInstance := absentStaffFromAbsences(absencePlan)
	removedSubstitutes := removedSubstituteStaffFromPlans(removalPlan)
	// Removing an absent substitute row subsumes restoring that exact row. Keep
	// any other planned assignments in the presence plan so one atomic request
	// can remove the obsolete role here and restore the person elsewhere (#2577).
	presencePlan = withoutRemovedSubstitutePresences(presencePlan, removedSubstitutes)
	subPlan, newSubByInstance, err := planSubstitutions(in.Substitutions, absenceOnlyByInstance, removedSubstitutes, readSet)
	if err != nil {
		return nil, err
	}
	absencePlan = withoutSubstitutionTargets(absencePlan, subPlan)
	absentByInstance := absentStaffFromPlans(absencePlan, subPlan)
	presentByInstance := presentStaffFromPlans(presencePlan)

	// Restoring a persisted absence must not orphan an already-assigned
	// substitute (over-staffing). Reject before any write (#1840).
	if err := rejectOverstaffingPresences(presencePlan, absentByInstance, presentByInstance, newSubByInstance, removedSubstitutes, readSet); err != nil {
		return nil, err
	}

	finalAck, finalAckNote, ackChanged, err := reconcileSelectedAck(instanceID, instance, in, absentByInstance, presentByInstance, newSubByInstance, removedSubstitutes, readSet)
	if err != nil {
		return nil, err
	}

	return &deviationPlan{
		instance:     instance,
		date:         date,
		absencePlan:  absencePlan,
		presencePlan: presencePlan,
		subPlan:      subPlan,
		removalPlan:  removalPlan,
		subs:         in.Substitutions,
		finalAck:     finalAck,
		finalAckNote: finalAckNote,
		ackChanged:   ackChanged,
	}, nil
}

// withoutSubstitutionTargets prevents an all-day absence plus appointment-
// scoped coverage from staging the same original row twice. ApplySubstitute
// already marks its original row absent, so the standalone absence operation is
// needed only for the uncovered appointments.
func withoutSubstitutionTargets(absences []deviationAbsenceOp, substitutions []deviationSubOp) []deviationAbsenceOp {
	coveredRows := make(map[int64]bool, len(substitutions))
	for _, substitution := range substitutions {
		if substitution.write.OrigRow != nil {
			coveredRows[substitution.write.OrigRow.ID] = true
		}
	}
	filtered := make([]deviationAbsenceOp, 0, len(absences))
	for _, absence := range absences {
		if !coveredRows[absence.row.ID] {
			filtered = append(filtered, absence)
		}
	}
	return filtered
}

func absentStaffFromAbsences(absences []deviationAbsenceOp) deviationStaffByInstance {
	absent := make(deviationStaffByInstance)
	for _, op := range absences {
		absent.add(op.instance.ID, op.row.StaffID)
	}
	return absent
}

func absentStaffFromPlans(absences []deviationAbsenceOp, substitutions []deviationSubOp) deviationStaffByInstance {
	absent := absentStaffFromAbsences(absences)
	for _, op := range substitutions {
		if op.write.OrigRow != nil {
			absent.add(op.write.Instance.ID, op.write.OrigRow.StaffID)
		}
	}
	return absent
}

func presentStaffFromPlans(presences []deviationPresenceOp) deviationStaffByInstance {
	present := make(deviationStaffByInstance)
	for _, op := range presences {
		present.add(op.instance.ID, op.row.StaffID)
	}
	return present
}

func removedSubstituteStaffFromPlans(removals []deviationSubstitutionRemovalOp) deviationStaffByInstance {
	removed := make(deviationStaffByInstance)
	for _, op := range removals {
		removed.add(op.instance.ID, op.row.StaffID)
	}
	return removed
}

func withoutRemovedSubstitutePresences(
	presences []deviationPresenceOp,
	removed deviationStaffByInstance,
) []deviationPresenceOp {
	kept := make([]deviationPresenceOp, 0, len(presences))
	for _, presence := range presences {
		if presence.row.IsSubstitute && removed[presence.instance.ID][presence.row.StaffID] {
			continue
		}
		kept = append(kept, presence)
	}
	return kept
}

func (staff deviationStaffByInstance) add(instanceID, staffID int64) {
	if staff[instanceID] == nil {
		staff[instanceID] = make(map[int64]bool)
	}
	staff[instanceID][staffID] = true
}

// validateDeviationStaff runs every 4xx precondition on the referenced staff:
// existence (404), self-substitution (400), a substitute also being marked
// absent (400), and a substitute already absent in the DB that day (400).
func validateDeviationStaff(in ApplyDeviationsInput, readSet *deviationReadSet) error {
	seen := make(map[int64]bool)
	ensure := func(staffID int64, label string) error {
		if staffID <= 0 {
			return devErrBadRequest(fmt.Sprintf("die Auswahl für %s ist ungültig", label))
		}
		if seen[staffID] {
			return nil
		}
		seen[staffID] = true
		if !readSet.staffExists[staffID] {
			return devErrNotFound(fmt.Sprintf("%s wurde nicht gefunden", label))
		}
		return nil
	}

	for _, absence := range in.Absences {
		if err := ensure(absence.StaffID, "die abwesende Person"); err != nil {
			return err
		}
	}
	for _, presence := range in.Presences {
		if err := ensure(presence.StaffID, "die anwesende Person"); err != nil {
			return err
		}
	}
	seenAbsentSub := make(map[int64]bool)
	for _, sub := range in.Substitutions {
		// The editor chooses one scope and one replacement per absent person, so an
		// absent staff id must appear at most once in one save.
		if seenAbsentSub[sub.AbsentStaffID] {
			return devErrBadRequest("für eine abwesende Person darf nur eine Ersatzperson gewählt werden")
		}
		seenAbsentSub[sub.AbsentStaffID] = true
		if err := ensure(sub.AbsentStaffID, "die abwesende Person"); err != nil {
			return err
		}
		if err := ensure(sub.SubstituteStaffID, "die Ersatzperson"); err != nil {
			return err
		}
		if sub.AbsentStaffID == sub.SubstituteStaffID {
			return devErrBadRequest("die abwesende Person kann sich nicht selbst vertreten")
		}
		// A person cannot cover an appointment on which this same save marks them
		// absent. Appointment-scoped absences elsewhere on the day are independent.
		if inputMarksStaffAbsentInScope(in, sub.SubstituteStaffID, sub.InstanceIDs) {
			return devErrBadRequest("die Ersatzperson ist in einem ausgewählten Termin selbst abwesend")
		}
		// ...nor if they are already absent on an appointment this substitution
		// targets. A terminbezogene Abwesenheit elsewhere on the day does not make
		// the person unavailable for a different appointment.
		for _, row := range readSet.rowsByStaff[sub.SubstituteStaffID] {
			if row.IsAbsent && scopeContainsInstance(sub.InstanceIDs, row.InstanceID) {
				return devErrBadRequest("die Ersatzperson ist in einem ausgewählten Termin selbst abwesend")
			}
		}
	}
	for _, removal := range in.SubstitutionRemovals {
		if err := ensure(removal.StaffID, "die Ersatzperson"); err != nil {
			return err
		}
	}
	return nil
}

func rejectContradictoryDeviationScopes(in ApplyDeviationsInput) error {
	for _, presence := range in.Presences {
		for _, absence := range in.Absences {
			if presence.StaffID == absence.StaffID && deviationScopesOverlap(presence.InstanceIDs, absence.InstanceIDs) {
				return devErrBadRequest("eine Person kann im selben Termin nicht anwesend und abwesend sein")
			}
		}
		for _, substitution := range in.Substitutions {
			if presence.StaffID == substitution.AbsentStaffID && deviationScopesOverlap(presence.InstanceIDs, substitution.InstanceIDs) {
				return devErrBadRequest("eine Person kann im selben Termin nicht anwesend und abwesend sein")
			}
		}
	}
	return nil
}

func planSubstitutionRemovals(
	removals []DeviationSubstitutionRemovalInput,
	date timezone.Date,
	readSet *deviationReadSet,
) ([]deviationSubstitutionRemovalOp, error) {
	plan := make([]deviationSubstitutionRemovalOp, 0)
	seenRows := make(map[int64]bool)
	for _, removal := range removals {
		if err := validateExplicitScopeInstances(date, removal.InstanceIDs, readSet); err != nil {
			return nil, err
		}
		rows := readSet.rowsByStaff[removal.StaffID]
		selected := make(map[int64]bool)
		for _, row := range rows {
			if removal.InstanceIDs != nil && row.IsSubstitute && scopeContainsInstance(removal.InstanceIDs, row.InstanceID) {
				selected[row.InstanceID] = true
			}
			if seenRows[row.ID] || !row.IsSubstitute || !scopeContainsInstance(removal.InstanceIDs, row.InstanceID) {
				continue
			}
			instance, err := loadPlannableInstance(row, readSet)
			if err != nil {
				return nil, err
			}
			if instance == nil {
				if removal.InstanceIDs != nil {
					return nil, devErrConflict("instance_not_editable", "dieser Termin kann nicht mehr geändert werden")
				}
				continue
			}
			if row.SickAbsenceID != nil {
				return nil, devErrConflict(
					"sick_absence_scope_locked",
					"diese Abwesenheit kommt aus einer Krankmeldung und kann hier nicht geändert werden",
				)
			}
			seenRows[row.ID] = true
			plan = append(plan, deviationSubstitutionRemovalOp{row: row, instance: instance})
		}
		if removal.InstanceIDs != nil {
			for _, instanceID := range *removal.InstanceIDs {
				if !selected[instanceID] {
					return nil, devErrBadRequest("die Ersatzperson ist nicht für jeden ausgewählten Termin eingetragen")
				}
			}
		}
	}
	return plan, nil
}

func validateExplicitScopeInstances(date timezone.Date, instanceIDs *[]int64, readSet *deviationReadSet) error {
	if instanceIDs == nil {
		return nil
	}
	if len(*instanceIDs) == 0 {
		return devErrBadRequest("wählen Sie mindestens einen Termin aus")
	}
	seen := make(map[int64]bool, len(*instanceIDs))
	for _, instanceID := range *instanceIDs {
		if instanceID <= 0 || seen[instanceID] {
			return devErrBadRequest("die Terminauswahl ist ungültig")
		}
		seen[instanceID] = true
		instance := readSet.instances[instanceID]
		if instance == nil {
			return devErrNotFound("der Termin wurde nicht gefunden")
		}
		if instance.Date != date {
			return devErrBadRequest("alle ausgewählten Termine müssen am bearbeiteten Tag liegen")
		}
		if !isPlannableInstance(instance) {
			return devErrConflict("instance_not_editable", "dieser Termin kann nicht mehr geändert werden")
		}
	}
	return nil
}

func inputMarksStaffAbsentInScope(in ApplyDeviationsInput, staffID int64, targetScope *[]int64) bool {
	for _, absence := range in.Absences {
		if absence.StaffID == staffID && deviationScopesOverlap(absence.InstanceIDs, targetScope) {
			return true
		}
	}
	for _, substitution := range in.Substitutions {
		if substitution.AbsentStaffID == staffID && deviationScopesOverlap(substitution.InstanceIDs, targetScope) {
			return true
		}
	}
	return false
}

func deviationScopesOverlap(left, right *[]int64) bool {
	if left == nil || right == nil {
		return true
	}
	rightIDs := make(map[int64]bool, len(*right))
	for _, instanceID := range *right {
		rightIDs[instanceID] = true
	}
	for _, instanceID := range *left {
		if rightIDs[instanceID] {
			return true
		}
	}
	return false
}

func scopeContainsInstance(instanceIDs *[]int64, instanceID int64) bool {
	if instanceIDs == nil {
		return true
	}
	for _, selectedID := range *instanceIDs {
		if selectedID == instanceID {
			return true
		}
	}
	return false
}

// planAbsences resolves every absence scope and stages its currently-present
// plannable rows. Explicit scopes reject terminal appointments rather than
// silently widening or partially applying the request.
func planAbsences(absences []DeviationAbsenceInput, date timezone.Date, readSet *deviationReadSet) ([]deviationAbsenceOp, error) {
	plan := make([]deviationAbsenceOp, 0)
	seenRows := make(map[int64]bool)
	for _, absence := range absences {
		if err := validateExplicitScopeInstances(date, absence.InstanceIDs, readSet); err != nil {
			return nil, err
		}
		rows, err := scopedStaffRows(readSet.rowsByStaff[absence.StaffID], absence.InstanceIDs)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if seenRows[row.ID] || row.IsAbsent {
				continue // idempotent: already absent, no write
			}
			instance, err := loadPlannableInstance(row, readSet)
			if err != nil {
				return nil, err
			}
			if instance == nil {
				if absence.InstanceIDs != nil {
					return nil, devErrConflict("instance_not_editable", "dieser Termin kann nicht mehr geändert werden")
				}
				continue // terminal instance, skip
			}
			seenRows[row.ID] = true
			plan = append(plan, deviationAbsenceOp{row: row, instance: instance, reason: trimDeviationReason(absence.Reason)})
		}
	}
	return plan, nil
}

// planPresences loads every to-be-restored staff member's scoped, plannable
// rows that are currently marked absent. A non-absent row is a no-op.
func planPresences(presences []DeviationPresenceInput, date timezone.Date, readSet *deviationReadSet) ([]deviationPresenceOp, error) {
	plan := make([]deviationPresenceOp, 0)
	seenRows := make(map[int64]bool)
	for _, presence := range presences {
		if err := validateExplicitScopeInstances(date, presence.InstanceIDs, readSet); err != nil {
			return nil, err
		}
		rows, err := scopedStaffRows(readSet.rowsByStaff[presence.StaffID], presence.InstanceIDs)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			instance, err := loadPlannableInstance(row, readSet)
			if err != nil {
				return nil, err
			}
			if instance == nil {
				if presence.InstanceIDs != nil {
					return nil, devErrConflict("instance_not_editable", "dieser Termin kann nicht mehr geändert werden")
				}
				continue // terminal instance, skip
			}
			if seenRows[row.ID] || !row.IsAbsent {
				continue // only a persisted absence can be cleared
			}
			if row.SickAbsenceID != nil {
				return nil, devErrConflict(
					"sick_absence_scope_locked",
					"diese Abwesenheit kommt aus einer Krankmeldung und kann hier nicht geändert werden",
				)
			}
			seenRows[row.ID] = true
			plan = append(plan, deviationPresenceOp{row: row, instance: instance})
		}
	}
	return plan, nil
}

// planSubstitutions classifies every substitution against a projected view of
// each instance (absence-only staff read as absent). Returns the write plan and,
// per instance, how many NEW substitute rows will be added (for the ack check).
func planSubstitutions(
	subs []DeviationSubstitutionInput,
	absenceOnlyByInstance deviationStaffByInstance,
	removedSubstitutes deviationStaffByInstance,
	readSet *deviationReadSet,
) ([]deviationSubOp, map[int64]int, error) {
	plan := make([]deviationSubOp, 0)
	newSubByInstance := make(map[int64]int)
	// Track which NEW substitute rows are staged per instance in THIS request,
	// keyed by (instance, substitute). Only a REPEATED (instance, substitute)
	// pairing must collapse, or Phase B would insert the same row twice (#1840).
	stagedSubs := make(map[int64]map[int64]bool)

	for _, sub := range subs {
		origRows, err := scopedStaffRows(readSet.rowsByStaff[sub.AbsentStaffID], sub.InstanceIDs)
		if err != nil {
			return nil, nil, err
		}
		reason := trimDeviationReason(sub.Reason)
		for _, orig := range origRows {
			instance, err := loadPlannableInstance(orig, readSet)
			if err != nil {
				return nil, nil, err
			}
			if instance == nil {
				if sub.InstanceIDs != nil {
					return nil, nil, devErrConflict("instance_not_editable", "dieser Termin kann nicht mehr geändert werden")
				}
				continue
			}
			allRows := readSet.rowsByInstance[instance.ID]
			allRows = withoutRemovedSubstitutes(allRows, removedSubstitutes[instance.ID])
			projectedRows, origProjected := projectAbsent(allRows, absenceOnlyByInstance[instance.ID], orig)
			action, _, ok := classifySubstitute(projectedRows, origProjected, sub.SubstituteStaffID)
			if !ok {
				return nil, nil, devErrConflict("substitute_conflict",
					"dieser Termin hat bereits eine andere Ersatzperson. Entfernen Sie diese zuerst")
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

func withoutRemovedSubstitutes(rows []*scheduleModel.InstanceStaff, removed map[int64]bool) []*scheduleModel.InstanceStaff {
	if len(removed) == 0 {
		return rows
	}
	kept := make([]*scheduleModel.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		if row.IsSubstitute && removed[row.StaffID] {
			continue
		}
		kept = append(kept, row)
	}
	return kept
}

// scopedStaffRows resolves a day-wide or explicit appointment scope against
// the absent person's assignments. Explicit scopes fail closed: an empty list,
// duplicate/non-positive id, or an appointment that does not belong to this
// person on this day is rejected instead of broadening to the whole day.
func scopedStaffRows(rows []*scheduleModel.InstanceStaff, instanceIDs *[]int64) ([]*scheduleModel.InstanceStaff, error) {
	if instanceIDs == nil {
		return rows, nil
	}
	if len(*instanceIDs) == 0 {
		return nil, devErrBadRequest("wählen Sie mindestens einen Termin aus")
	}

	requested := make(map[int64]bool, len(*instanceIDs))
	for _, instanceID := range *instanceIDs {
		if instanceID <= 0 {
			return nil, devErrBadRequest("die Terminauswahl ist ungültig")
		}
		if requested[instanceID] {
			return nil, devErrBadRequest("die Terminauswahl enthält einen Termin mehrfach")
		}
		requested[instanceID] = true
	}

	selected := make([]*scheduleModel.InstanceStaff, 0, len(requested))
	for _, row := range rows {
		if requested[row.InstanceID] {
			selected = append(selected, row)
			delete(requested, row.InstanceID)
		}
	}
	if len(requested) > 0 {
		return nil, devErrBadRequest("mindestens ein ausgewählter Termin gehört nicht zu dieser Person")
	}
	return selected, nil
}

// rejectOverstaffingPresences returns a 409 when clearing a persisted absence
// would push any touched instance above its planned headcount, which only
// happens when a restore orphans an already-assigned substitute (#1840).
func rejectOverstaffingPresences(
	presencePlan []deviationPresenceOp,
	absentByInstance, presentByInstance deviationStaffByInstance,
	newSubByInstance map[int64]int,
	removedByInstance deviationStaffByInstance,
	readSet *deviationReadSet,
) error {
	checked := make(map[int64]bool)
	for _, op := range presencePlan {
		if checked[op.instance.ID] {
			continue
		}
		checked[op.instance.ID] = true
		rows := readSet.rowsByInstance[op.instance.ID]
		baseline := 0
		for _, row := range rows {
			if !row.IsSubstitute {
				baseline++
			}
		}
		if projectedNonAbsentCount(
			rows,
			absentByInstance[op.instance.ID],
			presentByInstance[op.instance.ID],
			newSubByInstance[op.instance.ID],
			removedByInstance[op.instance.ID],
		) > baseline {
			return devErrConflict("presence_would_overstaff",
				"der Termin ist bereits vollständig besetzt. Entfernen Sie zuerst die nicht mehr benötigte Vertretung")
		}
	}
	return nil
}

// loadPlannableInstance loads the instance behind a staff row, returning nil when
// it is terminal (completed/cancelled) so callers skip it.
func loadPlannableInstance(row *scheduleModel.InstanceStaff, readSet *deviationReadSet) (*scheduleModel.ActivityInstance, error) {
	instance := readSet.instances[row.InstanceID]
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
func reconcileSelectedAck(
	instanceID int64,
	instance *scheduleModel.ActivityInstance,
	in ApplyDeviationsInput,
	absentByInstance, presentByInstance deviationStaffByInstance,
	newSubByInstance map[int64]int,
	removedByInstance deviationStaffByInstance,
	readSet *deviationReadSet,
) (finalAck bool, note *string, ackChanged bool, err error) {
	thisRows := readSet.rowsByInstance[instanceID]
	projectedPresent := projectedNonAbsentCount(
		thisRows,
		absentByInstance[instanceID],
		presentByInstance[instanceID],
		newSubByInstance[instanceID],
		removedByInstance[instanceID],
	)
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
		InstanceID:               instanceID,
		Cancelled:                false,
		UnderstaffedAck:          plan.finalAck,
		Affected:                 affected,
		Warnings:                 warnings,
		ActiveTouched:            activeTouched,
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
func (s *instanceService) writeDeviationOps(ctx context.Context, actor *int64, plan *deviationPlan) ([]DeviationAffected, map[int64]*scheduleModel.ActivityInstance, error) {
	now := time.Now()
	affected := make([]DeviationAffected, 0, len(plan.absencePlan)+len(plan.subPlan)+len(plan.presencePlan)+len(plan.removalPlan))
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
	for _, op := range plan.removalPlan {
		if err := s.RemoveSubstitute(ctx, op.row, op.instance, actor, activeTouched); err != nil {
			return nil, nil, devErrInternal("remove substitute failed", err)
		}
		affected = append(affected, affectedOf(op.instance, SubstituteActionRemoved))
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
	probes := buildSubstitutionWarningProbes(subs, plan)
	if len(probes) == 0 {
		return nil, nil
	}
	return s.loadSubstitutionWarnings(ctx, probes, date)
}

func (s *instanceService) loadSubstitutionWarnings(
	ctx context.Context,
	probes []substitutionWarningProbe,
	date timezone.Date,
) ([]SubstituteTimeConflict, error) {
	staffIDs := make([]int64, 0, len(probes))
	for _, probe := range probes {
		staffIDs = append(staffIDs, probe.staffID)
	}
	rows, err := s.deps.InstanceStaffRepo.FindByStaffIDsAndDate(ctx, staffIDs, date)
	if err != nil {
		return nil, err
	}
	rowsByStaff := make(map[int64][]*scheduleModel.InstanceStaff, len(probes))
	for _, row := range rows {
		rowsByStaff[row.StaffID] = append(rowsByStaff[row.StaffID], row)
	}
	foreignIDs := warningForeignInstanceIDs(probes, rowsByStaff)
	foreigns, err := s.deps.InstanceRepo.FindByIDs(ctx, foreignIDs)
	if err != nil {
		return nil, err
	}
	foreignByID := make(map[int64]*scheduleModel.ActivityInstance, len(foreigns))
	for _, instance := range foreigns {
		foreignByID[instance.ID] = instance
	}
	for _, id := range foreignIDs {
		if foreignByID[id] == nil {
			return nil, fmt.Errorf("load substitute conflict instance %d: %w", id, modelBase.ErrNotFound)
		}
	}
	return mergeSubstitutionWarnings(probes, rowsByStaff, foreignByID), nil
}

// buildSubstituteTimeConflicts is the single-person adapter used by the staff
// move flow. Deviation saves call loadSubstitutionWarnings once for all people.
func (s *instanceService) buildSubstituteTimeConflicts(
	ctx context.Context,
	plan []SubstituteWriteOp,
	staffID int64,
	date timezone.Date,
) ([]SubstituteTimeConflict, error) {
	if len(plan) == 0 {
		return nil, nil
	}
	probe := substitutionWarningProbe{staffID: staffID, targetIDs: make(map[int64]bool)}
	for _, op := range plan {
		probe.targetIDs[op.Instance.ID] = true
		if op.Action != SubstituteActionAlreadySubstitute {
			probe.targets = append(probe.targets, toConflictInstance(op.Instance))
		}
	}
	return s.loadSubstitutionWarnings(ctx, []substitutionWarningProbe{probe}, date)
}

type substitutionWarningProbe struct {
	staffID   int64
	targets   []SubstituteConflictInstance
	targetIDs map[int64]bool
}

func buildSubstitutionWarningProbes(subs []DeviationSubstitutionInput, plan []deviationSubOp) []substitutionWarningProbe {
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
		probe := substitutionWarningProbe{staffID: sub.SubstituteStaffID, targetIDs: make(map[int64]bool)}
		for _, op := range opsByStaff[sub.SubstituteStaffID] {
			probe.targetIDs[op.write.Instance.ID] = true
			if op.write.Action != SubstituteActionAlreadySubstitute {
				probe.targets = append(probe.targets, toConflictInstance(op.write.Instance))
			}
		}
		probes = append(probes, probe)
	}
	return probes
}

func warningForeignInstanceIDs(probes []substitutionWarningProbe, rowsByStaff map[int64][]*scheduleModel.InstanceStaff) []int64 {
	seen := make(map[int64]bool)
	ids := make([]int64, 0)
	for _, probe := range probes {
		for _, row := range rowsByStaff[probe.staffID] {
			if row.IsAbsent || probe.targetIDs[row.InstanceID] || seen[row.InstanceID] {
				continue
			}
			seen[row.InstanceID] = true
			ids = append(ids, row.InstanceID)
		}
	}
	return ids
}

func mergeSubstitutionWarnings(
	probes []substitutionWarningProbe,
	rowsByStaff map[int64][]*scheduleModel.InstanceStaff,
	foreignByID map[int64]*scheduleModel.ActivityInstance,
) []SubstituteTimeConflict {
	warnings := make([]SubstituteTimeConflict, 0)
	seenConflict := make(map[[3]int64]bool)
	for _, probe := range probes {
		foreigns := make([]SubstituteConflictInstance, 0)
		for _, row := range rowsByStaff[probe.staffID] {
			if row.IsAbsent || probe.targetIDs[row.InstanceID] || foreignByID[row.InstanceID] == nil {
				continue
			}
			foreigns = append(foreigns, toConflictInstance(foreignByID[row.InstanceID]))
		}
		for _, conflict := range DetectSubstituteTimeConflicts(probe.targets, foreigns) {
			key := [3]int64{probe.staffID, conflict.InstanceID, conflict.OtherID}
			if seenConflict[key] {
				continue
			}
			seenConflict[key] = true
			warnings = append(warnings, conflict)
		}
	}
	return warnings
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
		return nil, devErrBadRequest("dieser Termin liegt in der Vergangenheit")
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
		return nil, devErrConflict("instance_moved", "der Termin wurde gleichzeitig geändert. Öffnen Sie ihn erneut")
	}
	return s.SetUnderstaffedAck(ctx, instanceID, ack, note, actorAccountID)
}

// acquireSubstituteDayLock takes the shared (tenant, date) advisory lock that
// serializes every same-day staffing mutation, within the caller's tx.
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
func projectedNonAbsentCount(rows []*scheduleModel.InstanceStaff, absent, presence map[int64]bool, newSubs int, removedSubs map[int64]bool) int {
	count := 0
	for _, row := range rows {
		if row.IsSubstitute && removedSubs[row.StaffID] {
			continue
		}
		if absent[row.StaffID] {
			continue
		}
		if row.IsAbsent && !presence[row.StaffID] {
			continue
		}
		count++
	}
	return count + newSubs
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
