package application

import (
	"context"
	"fmt"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
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
	request *enrollmentNative.Request,
	child *enrollmentNative.RequestChild,
	phase *enrollmentNative.Phase,
	studentID int64,
	reviewedBy int64,
	syncTargetedFields bool,
) (*enrollmentNative.PendingGuardianInvite, error) {
	// Decide took the gates already; an existing-student approval rewrites
	// school_class and resyncs the class-filtered rosters under them.
	if err := d.LockApprovalGates(ctx); err != nil {
		return nil, err
	}
	renewal, err := d.renewExistingStudent(ctx, child, phase, studentID, reviewedBy)
	if err != nil {
		return nil, err
	}
	existing, previousSchoolClass := renewal.student, renewal.previousSchoolClass
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
		weeklyPlanOn := d.renewalWeeklyPlanDay(phase, renewal.careRunning)
		if err := d.syncRenewalTargetedFields(ctx, request, child, existing, guardian, reviewedBy, weeklyPlanOn); err != nil {
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
	if err := d.stampActivationPlan(ctx, child.ID, renewal.activationPlan); err != nil {
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

// renewExistingStudent updates school_class and the enrollment window.
// Already-active children stay active even for a future phase, so current
// attendance workflows are not interrupted. Inactive/pending children follow
// the approval-time activation plan. The window itself follows the phase KIND
// (see renewedEnrollmentWindow): only a school-year renewal may replace the
// master enrollment window.
//
// A child the school still cares for whose renewal starts in the future keeps
// its running class and enrollment start until the new year begins (#3917):
// the approval only extends the window's end and plans the class switch for
// the phase start, which the rollover tick applies (ApplyDueClassSwitches).
// Writing next year's values at once put the child in next year's class for
// the rest of the running year, and a future enrolled_from made every reader
// of the window treat it as not yet enrolled.
func (d *Decisions) renewExistingStudent(ctx context.Context, child *enrollmentNative.RequestChild, phase *enrollmentNative.Phase, studentID, reviewedBy int64) (*existingStudentRenewal, error) {
	existing, err := d.readEnrollmentStudent(ctx, studentID, "")
	if err != nil {
		return nil, fmt.Errorf("decision: load existing student %d: %w", studentID, err)
	}
	if existing == nil {
		return nil, fmt.Errorf("decision: existing student %d not found", studentID)
	}
	beforeStatus := existing.Status
	plan := d.approvalActivationPlan(ctx, phase)
	previousSchoolClass := existing.SchoolClass
	targetSchoolClass := existing.SchoolClass
	// Office matches and their rollover descendants renew enrollment only.
	// Their submitted class must never replace the school's master data.
	if !child.MatchedByOffice {
		targetSchoolClass = resolveRolloverSchoolClass(child, existing.SchoolClass)
	}
	switchOn := calendar.Date(phase.ServiceStartDate)
	deferred := phase.Kind == enrollmentNative.PhaseKindSchoolYear &&
		existing.Status == studentStatusActive && switchOn.After(d.todayDate())
	enrolledFrom, enrolledUntil := renewedEnrollmentWindow(phase, existing.EnrolledFrom, existing.EnrolledUntil)
	if !deferred {
		existing.SchoolClass = targetSchoolClass
		existing.EnrolledFrom = &enrolledFrom
	}
	existing.EnrolledUntil = &enrolledUntil
	if existing.Status != studentStatusActive {
		existing.Status = plan.StudentStatus
	}
	if err := d.deps.StudentEnrollment.RenewEnrollmentStudent(ctx, existing.ID, enrollmentStudentInput(existing)); err != nil {
		return nil, fmt.Errorf("decision: update existing student: %w", err)
	}
	var planned *enrollmentNative.ClassSwitch
	if deferred {
		planned = &enrollmentNative.ClassSwitch{RequestChildID: child.ID, From: previousSchoolClass, To: targetSchoolClass, On: enrollmentNative.Date(switchOn.String())}
	}
	if err := d.planClassSwitch(ctx, child.ID, planned); err != nil {
		return nil, err
	}
	if err := d.auditRenewedStatus(ctx, existing, beforeStatus, reviewedBy); err != nil {
		return nil, err
	}
	return &existingStudentRenewal{
		student: existing, previousSchoolClass: previousSchoolClass, activationPlan: plan,
		careRunning: beforeStatus == studentStatusActive,
	}, nil
}

// existingStudentRenewal is the outcome of renewExistingStudent.
type existingStudentRenewal struct {
	student             *Student
	previousSchoolClass string
	activationPlan      approvalActivationPlan
	// careRunning: the school cared for the child before the approval.
	careRunning bool
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
func (d *Decisions) reconcileRenewalGuardian(ctx context.Context, request *enrollmentNative.Request, studentID, reviewedBy int64) (*GuardianProfile, error) {
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
// weeklyPlanOn: a renewal approved before the care it renews starts keeps
// the running weekly plan — pickup times, care days, Heimweg — until that
// day; the approval only plans it, and the rollover tick applies the
// submission then (ApplyDueWeeklyPlans, #6043). nil applies it at once.
//
// ReplaceSchedules: the matched student most likely already has arrival /
// pickup schedule rows from its original enrollment; they are replaced, not
// appended to. ReplaceConsent: this form re-asks every configured consent,
// so a box the guardian left unchecked WITHDRAWS the consent the matched
// student still carries from an earlier enrollmentNative.
//
// Unlike the fresh-create path, a dispatch failure here is FATAL:
// ReplaceSchedules has already deleted the matched student's live arrival /
// pickup rows before the re-insert runs, so swallowing the error would commit
// the approval with the existing schedule deleted but never rebuilt.
// Returning the error rolls the whole approval back through the surrounding
// tenant tx. Losing a pickup-authorized emergency contact is a data-integrity
// failure as well, so the co-guardian link is fatal too.
func (d *Decisions) syncRenewalTargetedFields(ctx context.Context, request *enrollmentNative.Request, child *enrollmentNative.RequestChild, existing *Student, guardian *GuardianProfile, reviewedBy int64, weeklyPlanOn *enrollmentNative.Date) error {
	scope := allTargets
	if weeklyPlanOn != nil {
		scope = withoutWeeklyPlan
	}
	if _, err := d.applyTargetedFields(ctx, request, child, existing, guardian, reviewedBy, targetedFieldSyncOptions{
		ReplaceSchedules: true,
		ReplaceConsent:   true,
		Scope:            scope,
	}); err != nil {
		return fmt.Errorf("decision: targeted-field dispatch on existing student: %w", err)
	}
	if err := d.linkAdditionalGuardians(ctx, request, existing.ID); err != nil {
		return fmt.Errorf("decision: link additional guardians on existing student: %w", err)
	}
	return d.planWeeklyPlan(ctx, child.ID, weeklyPlanOn, reviewedBy)
}

// reconcileExistingStudentCareRenewal closes a stale complete-withdrawal task
// when an approved renewal starts care on or before its first bookingless day.
// It runs after materialization in the same tenant transaction, so the booking
// and task cannot disagree if either write fails.
func (d *Decisions) reconcileExistingStudentCareRenewal(
	ctx context.Context,
	requestChildID, studentID int64,
	phase *enrollmentNative.Phase,
) error {
	if d.deps.CareWithdrawal == nil {
		return nil
	}
	links, err := enrollmentNative.OfferingSelectionRecordsAt(ctx, d.deps.Children, requestChildID, calendar.Date(phase.ServiceStartDate))
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
	links []*enrollmentNative.RequestChildOfferingRecord,
	offerings map[int64]*offeringContract.CareOfferingRecord,
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
		if offering.DaysOfWeekMode == offeringContract.DaysOfWeekModeFixed {
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
	phase *enrollmentNative.Phase,
	currentFrom, currentUntil *calendar.Date,
) (calendar.Date, calendar.Date) {
	from := calendar.Date(phase.ServiceStartDate)
	until := calendar.Date(phase.ServiceEndDate)
	if phase.Kind == enrollmentNative.PhaseKindSchoolYear {
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
