package application

import (
	"context"
	"fmt"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// attachApprovalToExistingStudent is the shared approval tail for a child that
// resolves to an already-existing student rather than a fresh Person + Student:
// the annual rollover flow (source row's created_student_id) and the
// existing_students re-enrollment audience (matched_student_id) both land here.
// It renews the student's class + enrollment window, materializes the phase's
// care offerings, stamps the activation plan and back-links the request_child.
//
// syncTargetedFields separates the two callers: it is false for the annual
// rollover (no fresh parent submission exists, so there is nothing to apply
// beyond class + window + offerings) and true for an existing_students
// re-enrollment, which IS a full parent form and therefore also reconciles the
// submitted guardian — primary link, phone, portal invitation — exactly like
// the fresh-create path. Assuming the matched student already carries the
// submitted guardian is wrong for an anonymous submission, for an imported or
// manually created child with no guardian link at all, and for the other parent
// submitting this year's renewal; without the reconciliation the approval
// silently discards the submitted guardian and their portal access (#1663).
func (d *Decisions) attachApprovalToExistingStudent(
	ctx context.Context,
	request *enrollmentModels.Request,
	child *RequestChild,
	phase *enrollment.Phase,
	studentID int64,
	reviewedBy int64,
	syncTargetedFields bool,
) (*enrollment.PendingGuardianInvite, error) {
	if err := d.lockExistingStudentGates(ctx); err != nil {
		return nil, err
	}
	existing, previousSchoolClass, activationPlan, err := d.renewExistingStudent(ctx, child, phase, studentID, reviewedBy)
	if err != nil {
		return nil, err
	}
	// Reconcile the submitted primary guardian BEFORE the targeted-field
	// dispatch, so the resolved profile is the one the dispatch enriches with
	// the submitted phone number.
	var guardian *GuardianProfile
	if syncTargetedFields {
		if guardian, err = d.reconcileRenewalGuardian(ctx, request, studentID, reviewedBy); err != nil {
			return nil, err
		}
	}
	// Keep account-before-instance lock order shared with staff offboarding:
	// a roster insert takes an instance FK lock, so guardian attachment must
	// precede this resync just as it precedes fresh approval materialization.
	// The class-writes and recurrence gates remain held from before the
	// student write. Changed classes must update all sourced future rosters
	// in the same transaction (#2147, #2709).
	if existing.SchoolClass != previousSchoolClass {
		if err := d.resyncOfferingSourcedTemplates(ctx, d.todayDate()); err != nil {
			return nil, fmt.Errorf("decision: resync sourced templates after approval class change: %w", err)
		}
	}
	if syncTargetedFields {
		if err := d.syncRenewalTargetedFields(ctx, request, child, existing, guardian, reviewedBy); err != nil {
			return nil, err
		}
	}
	// Link this request_child to the student so the admin UI can navigate
	// from the submission to the (single) student row. Linked BEFORE the
	// enrollment materialization: the multi-source union resync resolves the
	// child's student through created_student_id.
	if err := d.deps.Children.LinkCreatedStudent(ctx, child.ID, studentID); err != nil {
		return nil, fmt.Errorf("decision: link existing student: %w", err)
	}
	if err := d.materializeApprovedOfferings(ctx, child.ID, studentID, phase); err != nil {
		return nil, err
	}
	if err := d.stampActivationPlan(ctx, child.ID, activationPlan); err != nil {
		return nil, err
	}
	// Invite the submitted guardian when they have no portal account yet — the
	// re-enrollment form is exactly the moment a school hands a family portal
	// access, and the matched student's existing links say nothing about
	// whether THIS submitter can reach it. nil on the rollover path (guardian
	// stays nil there) and whenever the account attach above already linked
	// them.
	return d.pendingGuardianInvite(guardian, reviewedBy, false), nil
}

// lockExistingStudentGates takes the tenant-wide gates BEFORE the first
// student row lock, in the project-wide class-change order (shared
// class-writes gate first, recurrence gate second, row locks last) — the same
// order the direct school_class PUT and the approved-child sync take (#2147
// review rounds 12/15). The existing-student tail rewrites school_class, and
// the class-change resync needs the recurrence gate; acquiring both up front
// keeps the approval deadlock-free against a concurrent direct PUT or grade
// transition on the same student.
func (d *Decisions) lockExistingStudentGates(ctx context.Context) error {
	if err := d.deps.StudentEnrollment.LockEnrollmentClassWrites(ctx); err != nil {
		return fmt.Errorf("decision: lock class writes for existing-student approval: %w", err)
	}
	return d.lockTemplateRecurrence(ctx)
}

// renewExistingStudent updates school_class and the enrollment window.
// Already-active children stay active even for a future phase, so current
// attendance workflows are not interrupted. Inactive/pending children follow
// the approval-time activation plan. The window itself follows the phase KIND
// (see renewedEnrollmentWindow): only a school-year renewal may replace the
// master enrollment window.
func (d *Decisions) renewExistingStudent(ctx context.Context, child *RequestChild, phase *enrollment.Phase, studentID, reviewedBy int64) (*Student, string, approvalActivationPlan, error) {
	existing, err := d.readEnrollmentStudent(ctx, studentID, "")
	if err != nil {
		return nil, "", approvalActivationPlan{}, fmt.Errorf("decision: load existing student %d: %w", studentID, err)
	}
	if existing == nil {
		return nil, "", approvalActivationPlan{}, fmt.Errorf("decision: existing student %d not found", studentID)
	}
	beforeStatus := existing.Status
	plan := d.approvalActivationPlan(ctx, phase)
	previousSchoolClass := existing.SchoolClass
	existing.SchoolClass = resolveRolloverSchoolClass(child, existing.SchoolClass)
	enrolledFrom, enrolledUntil := renewedEnrollmentWindow(phase, existing.EnrolledFrom, existing.EnrolledUntil)
	existing.EnrolledFrom = &enrolledFrom
	existing.EnrolledUntil = &enrolledUntil
	if existing.Status != studentStatusActive {
		existing.Status = plan.StudentStatus
	}
	if err := d.deps.StudentEnrollment.RenewEnrollmentStudent(ctx, existing.ID, enrollmentStudentInput(existing)); err != nil {
		return nil, "", plan, fmt.Errorf("decision: update existing student: %w", err)
	}
	if err := d.auditRenewedStatus(ctx, existing, beforeStatus, reviewedBy); err != nil {
		return nil, "", plan, err
	}
	return existing, previousSchoolClass, plan, nil
}

// auditRenewedStatus records a status change of the renewal: attributed to
// the reviewer when there is one, as a system change otherwise.
func (d *Decisions) auditRenewedStatus(ctx context.Context, existing *Student, beforeStatus string, reviewedBy int64) error {
	if beforeStatus == existing.Status || d.deps.StudentAudit == nil {
		return nil
	}
	var err error
	if reviewedBy > 0 {
		before := &Student{Status: beforeStatus}
		after := &Student{ID: existing.ID, Status: existing.Status}
		err = d.deps.StudentAudit.RecordChangesForActor(ctx, before, after, reviewedBy)
	} else {
		err = d.deps.StudentAudit.RecordSystemStatusChange(ctx, existing.ID, beforeStatus, existing.Status)
	}
	if err != nil {
		return fmt.Errorf("decision: audit rollover student status: %w", err)
	}
	return nil
}

// reconcileRenewalGuardian links the submitted primary guardian to the
// matched student and attaches their existing portal account.
//
// pruneStalePrimary=false: a guardian already holding the primary link on the
// matched student comes from a different source than this submission (last
// year's approval, an import, the other parent), so they keep their link —
// and with it their pickup authority and parent-portal access — and are only
// demoted from primary by the DB trigger (#1663).
func (d *Decisions) reconcileRenewalGuardian(ctx context.Context, request *enrollmentModels.Request, studentID, reviewedBy int64) (*GuardianProfile, error) {
	guardianRequest, err := d.guardianIdentityRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	guardian, err := d.reconcilePrimaryGuardianLink(ctx, guardianRequest, studentID, false, reviewedBy)
	if err != nil {
		return nil, err
	}
	// Same cross-tenant account check the fresh-create approval runs: a
	// parent who already has a portal account (this school or another) gets
	// the tenant + profile attached directly instead of an invitation that
	// would overwrite their password.
	if err := d.attachGuardianAccountIfPresent(ctx, guardianRequest, guardian, false); err != nil {
		return nil, err
	}
	return guardian, nil
}

// syncRenewalTargetedFields dispatches every targeted form field the parent
// submitted onto the existing record (health/extra info, departure + arrival
// schedules, contact lists, consent-flag propagation) and materializes the
// co-guardians of the full form (#1663).
//
// ReplaceSchedules: the matched student most likely already has arrival /
// pickup schedule rows from its original enrollment; they are replaced, not
// appended to. ReplaceConsent: this form re-asks every configured consent,
// so a box the guardian left unchecked WITHDRAWS the consent the matched
// student still carries from an earlier enrollment.
//
// Unlike the fresh-create path, a dispatch failure here is FATAL:
// ReplaceSchedules has already deleted the matched student's live arrival /
// pickup rows before the re-insert runs, so swallowing the error would commit
// the approval with the existing schedule deleted but never rebuilt.
// Returning the error rolls the whole approval back through the surrounding
// tenant tx. Losing a pickup-authorized emergency contact is a data-integrity
// failure as well, so the co-guardian link is fatal too.
func (d *Decisions) syncRenewalTargetedFields(ctx context.Context, request *enrollmentModels.Request, child *RequestChild, existing *Student, guardian *GuardianProfile, reviewedBy int64) error {
	if _, err := d.applyTargetedFields(ctx, request, child, existing, guardian, reviewedBy, targetedFieldSyncOptions{
		ReplaceSchedules: true,
		ReplaceConsent:   true,
	}); err != nil {
		return fmt.Errorf("decision: targeted-field dispatch on existing student: %w", err)
	}
	if err := d.linkAdditionalGuardians(ctx, request, existing.ID); err != nil {
		return fmt.Errorf("decision: link additional guardians on existing student: %w", err)
	}
	return nil
}

// reconcileExistingStudentCareRenewal closes a stale complete-withdrawal task
// when an approved renewal starts care on or before its first bookingless day.
// It runs after materialization in the same tenant transaction, so the booking
// and task cannot disagree if either write fails.
func (d *Decisions) reconcileExistingStudentCareRenewal(
	ctx context.Context,
	requestChildID, studentID int64,
	phase *enrollment.Phase,
) error {
	if d.deps.CareWithdrawal == nil {
		return nil
	}
	links, err := enrollment.OfferingSelectionRecordsAt(ctx, d.deps.Children, requestChildID, calendar.Date(phase.ServiceStartDate))
	if err != nil {
		return fmt.Errorf("decision: list renewed care offerings: %w", err)
	}
	offerings, err := d.deps.Offerings.ListByIDs(ctx, uniqueCareOfferingIDs(links))
	if err != nil {
		return fmt.Errorf("decision: load renewed care offerings: %w", err)
	}
	if !offeringLinksHaveCareDays(links, careOfferingsByID(offerings)) {
		return nil
	}
	if err := d.deps.CareWithdrawal.ReconcileAuthoritativeBookingChange(ctx, careplan.CareWithdrawalBookingChange{
		StudentID: studentID, FirstBookinglessDay: calendar.Date(phase.ServiceStartDate),
	}); err != nil {
		return fmt.Errorf("decision: reconcile renewed care withdrawal: %w", err)
	}
	return nil
}

// offeringLinksHaveCareDays reports whether any booking of a care-counting
// offering covers a day: its selected days, or the fixed days of the
// offering.
func offeringLinksHaveCareDays(
	links []*enrollment.RequestChildOfferingRecord,
	offerings map[int64]*enrollmentModels.CareOffering,
) bool {
	for _, link := range links {
		if link == nil {
			continue
		}
		offering := offerings[link.CareOfferingID]
		if offering == nil || !offering.CountsAsCare {
			continue
		}
		hasCareDays := len(link.SelectedDays) > 0
		if offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
			hasCareDays = len(offering.AvailableDays) > 0
		}
		if hasCareDays {
			return true
		}
	}
	return false
}

// renewedEnrollmentWindow decides the enrollment window an approval writes
// onto an ALREADY EXISTING student, from the phase kind (#1663):
//
//   - school_year: the phase IS the child's new master enrollment window
//     (annual rollover / re-enrollment), so it replaces the old one wholesale.
//   - holiday / custom: the phase describes a limited-time care period, NOT
//     the child's school membership. Overwriting the master window with it
//     would cut an annually enrolled child's enrollment short — a holiday
//     phase ending in October would satisfy FindActiveDueForDeactivation and
//     the scheduler would mark a perfectly active child inactive. So the
//     existing window is preserved and only WIDENED where the phase's service
//     period reaches beyond it (an inactive child re-enrolled for a holiday
//     block still needs a window that covers that block).
//
// A missing bound (nil) is treated as "not set" and takes the phase's date, so
// a legacy student without a window still ends up with one.
func renewedEnrollmentWindow(
	phase *enrollment.Phase,
	currentFrom, currentUntil *calendar.Date,
) (calendar.Date, calendar.Date) {
	from := calendar.Date(phase.ServiceStartDate)
	until := calendar.Date(phase.ServiceEndDate)
	if phase.Kind == enrollment.PhaseKindSchoolYear {
		return from, until
	}
	if currentFrom != nil && currentFrom.Before(from) {
		from = *currentFrom
	}
	if currentUntil != nil && currentUntil.After(until) {
		until = *currentUntil
	}
	return from, until
}
