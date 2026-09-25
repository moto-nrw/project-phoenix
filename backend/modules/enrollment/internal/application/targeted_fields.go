package application

import (
	"context"
	"fmt"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// targetedFieldSyncOptions tune one targeted-field dispatch.
type targetedFieldSyncOptions struct {
	// Replace applies the submission as the complete state: a field without
	// a meaningful value clears its target.
	Replace bool
	// ReplaceSchedules deletes the student's existing arrival / pickup schedule
	// rows before re-inserting the resubmitted weekdays, WITHOUT the full-form
	// clearing semantics of Replace. Used by the existing_students re-enrollment
	// approval, where the matched student already has schedule rows that would
	// otherwise collide with the unique (tenant_id, student_id, weekday) key.
	// Implied by Replace.
	ReplaceSchedules bool
	// ReplaceConsent applies the submitted consent flags as the complete
	// consent state instead of an additive OR: a flag the parent left
	// unchecked CLEARS the matching timestamp on the student row. Used by the
	// existing_students re-enrollment approval, where the submission is a full
	// renewal of a student that already carries consent from a previous year —
	// without it a guardian who withdraws photo or email-contact consent on the
	// renewal form stays recorded as consenting (#1663). Implied by Replace.
	ReplaceConsent         bool
	PreviousSnapshot       map[string]any
	KeepGuardianProfileIDs map[int64]bool
}

// targetedFieldSync is one run of applyTargetedFields.
type targetedFieldSync struct {
	d          *Decisions
	ctx        context.Context
	request    *enrollmentModels.Request
	child      *RequestChild
	student    *Student
	guardian   *GuardianProfile
	reviewedBy int64
	options    targetedFieldSyncOptions

	errs         []string
	studentDirty bool
	// A unified departure target wins by replacing both legacy maps after the
	// loop; the legacy Buskind/Abholregelung targets mutate their own map
	// directly, so a form carrying both still combines correctly (#1610).
	explicitDeparture        *departure.DepartureDays
	explicitAllowedDeparture *departure.AllowedDepartureModes
	pickup                   pickupScheduleSync
	arrivalDeleted           bool
}

// pickupScheduleSync tracks the weekly pickup rewrite of one dispatch.
type pickupScheduleSync struct {
	deleted        bool
	lockTaken      bool
	changed        bool
	snapshotTaken  bool
	snapshotFailed bool
	before         map[int]string
}

// applyTargetedFields walks the request's pinned schema and dispatches every
// field carrying a non-empty Target onto the appropriate downstream record.
// The student row is mutated in place for scalar targets and persisted at the
// end.
//
// Best-effort overall: per-field errors are collected and returned in one
// combined error string. The fresh-student caller logs those ordinary field
// errors because the student and per-child records have already been written.
// Consent-audit failures are marked separately and must abort the approval so
// the surrounding tenant transaction rolls back the consent write. The
// companion refusals stay reachable via errors.Is so the enrollment handlers
// can answer with the actionable 4xx the student PUT gives instead of a blind
// 500.
//
// The returned bool reports whether the student write actually TRIMMED a
// "läuft mit" link — read from the write itself, not from the fact that the
// payload carried a departure plan. It is the caller's signal for the
// student_companions_changed broadcast, and a false positive there costs
// somebody an in-progress companion edit.
func (d *Decisions) applyTargetedFields(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *RequestChild,
	student *Student,
	guardian *GuardianProfile,
	reviewedBy int64,
	options targetedFieldSyncOptions,
) (bool, error) {
	if d.deps.Schemas == nil || request.SchemaID == nil {
		return false, nil
	}
	schema, err := d.deps.Schemas.Schema(ctx, *request.SchemaID)
	if err != nil {
		return false, fmt.Errorf("load pinned schema for targeted fields: %w", err)
	}
	if schema == nil {
		return false, fmt.Errorf("%w: %d", enrollment.ErrFormSchemaNotFound, *request.SchemaID)
	}
	consentBefore := *student
	run := &targetedFieldSync{
		d: d, ctx: ctx, request: request, child: child, student: student,
		guardian: guardian, reviewedBy: reviewedBy, options: options,
	}
	run.dispatchFields(schema.Fields)
	run.finishPickupSchedule()
	run.applyConsentFlags()
	run.applyGuardianPhone()
	run.applyExplicitDeparture()
	run.applyCompanionNote()
	return run.persist(&consentBefore)
}

// dispatchFields dispatches every targeted field. Under Replace, a field
// without a meaningful value leaves its target alone when another field of
// the same target carries one.
func (r *targetedFieldSync) dispatchFields(fields []enrollment.FormField) {
	fieldRaws := make([]any, len(fields))
	targetHasMeaningfulValue := make(map[string]bool, len(fields))
	for i := range fields {
		field := fields[i]
		if field.Target == "" {
			continue
		}
		raw := readFieldValue(r.request, r.child, &field)
		fieldRaws[i] = raw
		if targetedFieldHasMeaningfulValue(field.Target, raw) {
			targetHasMeaningfulValue[field.Target] = true
		}
	}
	for i := range fields {
		field := fields[i]
		if field.Target == "" {
			continue
		}
		raw := fieldRaws[i]
		if raw == nil && !r.options.Replace {
			continue
		}
		if r.options.Replace && !targetedFieldHasMeaningfulValue(field.Target, raw) && targetHasMeaningfulValue[field.Target] {
			continue
		}
		r.dispatchField(field, raw)
	}
}

func (r *targetedFieldSync) dispatchField(field enrollment.FormField, raw any) {
	switch field.Target {
	case enrollment.TargetStudentHealthInfo:
		r.applyTextField(&r.student.HealthInfo, raw)
	case enrollment.TargetStudentExtraInfo:
		r.applyTextField(&r.student.ExtraInfo, raw)
	case enrollment.TargetStudentDeparture:
		r.applyDepartureField(field.Target, raw)
	case enrollment.TargetStudentAllowedDepartureModes:
		r.applyAllowedDepartureField(field.Target, raw)
	case enrollment.TargetStudentBusDays, enrollment.TargetStudentBus:
		r.applyBusField(field.Target, raw)
	case enrollment.TargetStudentPickupStatus:
		r.applyPickupStatusField(field.Target, raw)
	case enrollment.TargetSchedulePickup:
		r.applyPickupScheduleField(field.Target, raw)
	case enrollment.TargetScheduleArrival:
		r.applyArrivalScheduleField(field.Target, raw)
	case enrollment.TargetStudentContacts:
		r.applyContactsField(field, raw)
	}
}

func (r *targetedFieldSync) fieldError(target string, err error) {
	r.errs = append(r.errs, fmt.Sprintf("%s: %v", target, err))
}

func (r *targetedFieldSync) applyTextField(target **string, raw any) {
	if str := stringValue(raw); str != "" {
		*target = &str
		r.studentDirty = true
	} else if r.options.Replace {
		*target = nil
		r.studentDirty = true
	}
}

// clearDeparturePlan empties the whole departure plan.
func (r *targetedFieldSync) clearDeparturePlan() {
	r.student.AllowedDepartureModes = departure.AllowedDepartureModes{}
	r.student.DepartureDays = departure.DepartureDays{}
	r.student.BusDays = departure.BusDays{}
	r.student.PickupDays = departure.PickupDays{}
	r.student.DepartureCompanionNote = nil
	r.studentDirty = true
}

// applyDepartureField unions with any earlier same-target field rather than
// overwriting: last-field-wins would let a later bus/pickup field silently
// drop an accompanied day an earlier field carried, which validation and
// sanitization (which use any-field semantics) already accepted (#1694).
func (r *targetedFieldSync) applyDepartureField(target string, raw any) {
	if raw == nil {
		r.clearDeparturePlan()
		return
	}
	days, err := decodeDepartureDays(raw)
	if err != nil {
		r.fieldError(target, err)
		return
	}
	if r.explicitDeparture != nil {
		days = r.explicitDeparture.Merge(days)
	}
	r.explicitDeparture = &days
	r.studentDirty = true
}

func (r *targetedFieldSync) applyAllowedDepartureField(target string, raw any) {
	if raw == nil {
		r.clearDeparturePlan()
		return
	}
	modes, err := decodeAllowedDepartureModes(raw)
	if err != nil {
		r.fieldError(target, err)
		return
	}
	if r.explicitAllowedDeparture != nil {
		modes = r.explicitAllowedDeparture.Merge(modes)
	}
	r.explicitAllowedDeparture = &modes
	r.studentDirty = true
}

func (r *targetedFieldSync) applyBusField(target string, raw any) {
	if raw == nil {
		r.student.BusDays = departure.BusDays{}
		r.studentDirty = true
		return
	}
	days, err := decodeBusDays(raw)
	if err != nil {
		r.fieldError(target, err)
		return
	}
	r.student.BusDays = days
	r.studentDirty = true
}

func (r *targetedFieldSync) applyPickupStatusField(target string, raw any) {
	if raw == nil {
		r.student.PickupDays = departure.PickupDays{}
		r.studentDirty = true
		return
	}
	days, err := decodePickupDays(raw)
	if err != nil {
		r.fieldError(target, err)
		return
	}
	r.student.PickupDays = days
	r.studentDirty = true
}

// applyPickupScheduleField rewrites the weekly pickup times. The student
// lock comes BEFORE the weekly rewrite — the shared first lock of every
// care-day writer — so the auto-excusal resync after the loop keeps the
// student → care-day lock order (#2360 review).
func (r *targetedFieldSync) applyPickupScheduleField(target string, raw any) {
	if r.pickup.snapshotFailed || !r.preparePickupSchedule(target) {
		return
	}
	schedules := r.d.deps.PickupSchedules
	if r.replaceSchedules() && schedules != nil && !r.pickup.deleted {
		r.pickup.deleted = true
		if err := schedules.DeletePickupSchedules(r.ctx, r.student.ID); err != nil {
			r.errs = append(r.errs, fmt.Sprintf("%s: delete existing: %v", target, err))
			return
		}
		r.pickup.changed = true
	}
	if raw == nil {
		return
	}
	if err := r.d.dispatchWeekdaySchedule(r.ctx, raw, r.student.ID, r.reviewedBy, true); err != nil {
		r.fieldError(target, err)
		return
	}
	r.pickup.changed = true
}

// preparePickupSchedule takes the student lock and the before-snapshot once
// per dispatch; it reports whether the rewrite may proceed.
func (r *targetedFieldSync) preparePickupSchedule(target string) bool {
	if !r.pickup.lockTaken {
		if err := r.d.lockPickupStudents(r.ctx, []int64{r.student.ID}); err != nil {
			r.fieldError(target, err)
			return false
		}
		r.pickup.lockTaken = true
	}
	if r.pickup.snapshotTaken {
		return true
	}
	r.pickup.snapshotTaken = true
	before, err := r.d.snapshotPickupWeekdayChanges(r.ctx, r.student.ID, r.d.todayDate())
	if err != nil {
		r.errs = append(r.errs, fmt.Sprintf("%s: snapshot existing: %v", target, err))
		r.pickup.snapshotFailed = true
		return false
	}
	r.pickup.before = before
	return true
}

func (r *targetedFieldSync) applyArrivalScheduleField(target string, raw any) {
	schedules := r.d.deps.ArrivalSchedules
	if r.replaceSchedules() && schedules != nil && !r.arrivalDeleted {
		r.arrivalDeleted = true
		if err := schedules.DeleteArrivalSchedules(r.ctx, r.student.ID); err != nil {
			r.errs = append(r.errs, fmt.Sprintf("%s: delete existing: %v", target, err))
			return
		}
	}
	if raw == nil {
		return
	}
	if err := r.d.dispatchWeekdaySchedule(r.ctx, raw, r.student.ID, r.reviewedBy, false); err != nil {
		r.fieldError(target, err)
	}
}

// applyContactsField links the submitted contacts; under Replace the contacts
// the previous submission named and this one no longer does are unlinked.
func (r *targetedFieldSync) applyContactsField(field enrollment.FormField, raw any) {
	oldContactIDs := map[int64]bool{}
	if r.options.Replace {
		var err error
		oldContactIDs, err = r.d.contactProfileIDsFromPreviousSnapshot(r.ctx, r.options.PreviousSnapshot, r.child, r.student.ID, field.Key)
		if err != nil {
			r.errs = append(r.errs, fmt.Sprintf("%s: resolve previous contacts: %v", field.Target, err))
			return
		}
	}
	newContactIDs := map[int64]bool{}
	if raw != nil {
		ids, err := r.d.dispatchContactList(r.ctx, raw, r.student.ID)
		if err != nil {
			r.fieldError(field.Target, err)
			return
		}
		newContactIDs = ids
	}
	if !r.options.Replace {
		return
	}
	keep := mergeGuardianProfileKeepSets(newContactIDs, r.options.KeepGuardianProfileIDs)
	if err := r.d.deleteRemovedStudentGuardianLinks(r.ctx, r.student.ID, oldContactIDs, keep, r.reviewedBy); err != nil {
		r.errs = append(r.errs, fmt.Sprintf("%s: remove stale links: %v", field.Target, err))
	}
}

// replaceSchedules: Replace implies schedule replacement; ReplaceSchedules
// asks for it alone (existing_students re-enrollment).
func (r *targetedFieldSync) replaceSchedules() bool {
	return r.options.Replace || r.options.ReplaceSchedules
}

// finishPickupSchedule re-derives the auto excusals after a replaced or
// re-dispatched weekly pickup plan. It moves the same baseline a staff weekly
// edit does, so the auto excusals of the student's future day exceptions must
// re-derive in the same transaction (#2360 review) — a pulled-forward day
// exception would otherwise keep an obsolete excusal state after
// re-enrollment until someone edits it.
func (r *targetedFieldSync) finishPickupSchedule() {
	if !r.pickup.changed {
		return
	}
	if err := r.d.resyncPickupAutoExcusals(r.ctx, []int64{r.student.ID}); err != nil {
		r.errs = append(r.errs, err.Error())
	}
	if err := r.d.recordPickupWeekdayChanges(r.ctx, r.student.ID, r.d.todayDate(), r.pickup.before); err != nil {
		r.errs = append(r.errs, err.Error())
	}
}

// applyGuardianPhone lands the submitted guardian phone on the resolved
// primary guardian. guardian is nil on the annual rollover path: that
// request_child carries no fresh parent submission, so there is no newly
// submitted phone number and no profile to enrich.
func (r *targetedFieldSync) applyGuardianPhone() {
	if r.request.GuardianPhone == nil || r.guardian == nil {
		return
	}
	if err := r.d.createGuardianPhoneNumber(r.ctx, r.guardian.ID, *r.request.GuardianPhone); err != nil {
		r.errs = append(r.errs, fmt.Sprintf("auto guardian_phone: %v", err))
	}
}

// applyExplicitDeparture applies the unified departure targets. The allowed
// modes win; otherwise a departure-days field replaces both legacy maps, and
// without either the legacy bus/pickup targets already set their map (each
// preserving the other). The owner folds bus_days + pickup_days into
// departure_days, the single source of truth, on write (#1610).
func (r *targetedFieldSync) applyExplicitDeparture() {
	student := r.student
	if r.explicitAllowedDeparture != nil {
		student.AllowedDepartureModes = r.explicitAllowedDeparture.Normalize()
		student.DepartureDays = student.AllowedDepartureModes.DepartureDays()
		student.BusDays = student.AllowedDepartureModes.BusDays()
		student.PickupDays = student.AllowedDepartureModes.PickupDays()
		return
	}
	if r.explicitDeparture != nil {
		student.AllowedDepartureModes = departure.AllowedDepartureModesFromDeparture(*r.explicitDeparture)
		student.BusDays = r.explicitDeparture.BusDays()
		student.PickupDays = r.explicitDeparture.PickupDays()
	}
}

// applyCompanionNote takes the coupled "mit wem" note (#1694): the enrollment
// form carries it on a reserved per-child custom-data key alongside
// allowed_departure_modes. It applies ONLY when the child's final departure
// plan actually allows the accompanied mode, so a note from a client that
// toggled accompanied off — or a crafted submit — never lands on a child with
// no "Mit anderem Kind" day. Capped server-side (the column is unbounded
// TEXT).
func (r *targetedFieldSync) applyCompanionNote() {
	if r.child == nil || r.child.CustomData == nil ||
		!r.student.AllowedDepartureModes.HasMode(departure.DepartureAccompanied) {
		return
	}
	note := strings.TrimSpace(stringValue(r.child.CustomData[enrollment.TargetStudentDepartureCompanionNote]))
	if note == "" {
		return
	}
	note = truncateRunes(note, departure.MaxDepartureCompanionNoteLen)
	r.student.DepartureCompanionNote = &note
	r.studentDirty = true
}

// truncateRunes cuts s to at most n runes.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func targetedFieldHasMeaningfulValue(target string, raw any) bool {
	if raw == nil {
		return false
	}
	switch target {
	case enrollment.TargetStudentHealthInfo, enrollment.TargetStudentExtraInfo:
		return strings.TrimSpace(stringValue(raw)) != ""
	default:
		return structuredTargetValueHasEntries(raw)
	}
}

func structuredTargetValueHasEntries(raw any) bool {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value) != ""
	case []any:
		return len(value) > 0
	case []map[string]any:
		return len(value) > 0
	case []string:
		return len(value) > 0
	case map[string]any:
		return len(value) > 0
	case map[string]string:
		return len(value) > 0
	case map[string][]string:
		return len(value) > 0
	case map[string][]any:
		return len(value) > 0
	default:
		return true
	}
}

// readFieldValue pulls the submission value for a field. Guardian-level
// fields live on the request's custom data; per-child fields on the child's.
func readFieldValue(request *enrollmentModels.Request, child *RequestChild, field *enrollment.FormField) any {
	if field.AppliesToCh {
		if child == nil || child.CustomData == nil {
			return nil
		}
		return child.CustomData[field.Key]
	}
	if request.CustomData == nil {
		return nil
	}
	return request.CustomData[field.Key]
}
