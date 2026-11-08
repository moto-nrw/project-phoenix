package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The already-enrolled check of open and linked_parents phases (#6053).
// An existing_students phase pins the renewed student at submission and a
// new_students phase refuses an enrolled child; the other two audiences
// accept any child. Submission pins a child only through the signed-in
// parent's own authorized relationship to it. Name and birthday alone never
// pin: every other enrolled child with the submitted name and birthday is a
// candidate the office sees, and an approval without the office's choice is
// refused, so an enrolled child is never silently created a second time.
// The office's choice renews the enrollment only: it links none of the
// request's guardians and applies none of its form data, since the family
// holds no authorized relationship to that child.

// audienceChecksEnrolledChildren reports whether approvals of the phase ask
// the office about an already enrolled child. A phase stored before the
// audience column reads as open.
func audienceChecksEnrolledChildren(phase *enrollmentNative.Phase) bool {
	if phase == nil {
		return false
	}
	switch phase.Audience {
	case "", enrollmentNative.PhaseAudienceOpen, enrollmentNative.PhaseAudienceLinkedParents:
		return true
	default:
		return false
	}
}

// childAwaitsExistingChildChoice reports whether the child's approval still
// decides between an enrolled child and a new one: its phase asks, and no
// pin, rollover source or created student settles the student already.
func childAwaitsExistingChildChoice(phase *enrollmentNative.Phase, child *enrollmentNative.RequestChild) bool {
	return audienceChecksEnrolledChildren(phase) && child != nil &&
		child.MatchedStudentID == nil && child.RolloverSourceChildID == nil && child.CreatedStudentID == nil
}

// isParentPin identifies a non-office, non-rollover pin. Its account-bound
// authority is checked independently of the mutable phase audience.
func isParentPin(child *enrollmentNative.RequestChild) bool {
	return child != nil && child.MatchedStudentID != nil &&
		!child.MatchedByOffice && child.RolloverSourceChildID == nil
}

// parentPinLapsed reports whether a parent's pin lost its authority: the
// request's identity no longer holds parent_portal.enrollment.submit on its
// relationship to the pinned child, because the school removed or narrowed
// it after the submission. The pin was authorized at submission only, so
// every approval and every later edit asks again before it links guardians,
// grants portal access, invites or applies form data. Without the
// authorizer the pin counts as lapsed.
//
// Accountless pins keep the existing_students email/manual rule (#6173).
// Account-bound pins always require current authority, even if the audience
// changed since submission or the account was attached later.
func (d *Decisions) parentPinLapsed(ctx context.Context, request *enrollmentNative.Request, child *enrollmentNative.RequestChild) (bool, error) {
	if !isParentPin(child) {
		return false, nil
	}
	return d.parentStudentAuthorityLapsed(ctx, request, child.ID, *child.MatchedStudentID)
}

// parentStudentAuthorityLapsed checks both direct parent pins and the student
// inherited by an approved rollover descendant. Neither a past approval nor
// a rollover grants authority for new family data after revocation.
func (d *Decisions) parentStudentAuthorityLapsed(ctx context.Context, request *enrollmentNative.Request, childID, studentID int64) (bool, error) {
	if request == nil || request.GuardianAccountID == nil {
		return false, nil
	}
	submitter, err := reEnrollmentSubmitterForPersistedRequest(ctx, d.deps.LateInvites, request)
	if err != nil {
		return false, fmt.Errorf("decision: resolve re-enrollment identity of child %d: %w", childID, err)
	}
	granted, err := submitterMayRenewStudent(ctx, d.deps.GuardianAuthorizer, submitter, studentID, request.TenantID)
	if err != nil {
		return false, fmt.Errorf("decision: verify relationship of child %d: %w", childID, err)
	}
	return !granted, nil
}

// existingChildCandidates lists the enrolled children that may be the
// child. Without the directory port the check fails closed.
func (d *Decisions) existingChildCandidates(ctx context.Context, child *enrollmentNative.RequestChild) ([]enrollmentNative.ExistingChildCandidate, error) {
	students := d.deps.People.Students
	if students == nil {
		return nil, errors.New("decision: enrolled-child candidate lookup is not configured")
	}
	candidates, err := students.EnrolledMatchCandidates(ctx, child.FirstName, child.LastName, calendar.Date(child.DateOfBirth))
	if err != nil {
		return nil, fmt.Errorf("decision: list enrolled-child candidates for child %d: %w", child.ID, err)
	}
	return candidates, nil
}

// validateExistingChildChoice refuses a choice the decision cannot carry:
// both answers at once, or a choice for another status than approval.
func validateExistingChildChoice(input enrollmentNative.DecideInput) error {
	if input.ExistingStudentID == nil && !input.CreateNewStudent {
		return nil
	}
	if input.ExistingStudentID != nil && input.CreateNewStudent {
		return fmt.Errorf("%w: both an existing child and a new child chosen", enrollmentNative.ErrExistingChildChoiceInvalid)
	}
	if input.Status != enrollmentNative.DecisionApproved {
		return fmt.Errorf("%w: a choice belongs to an approval", enrollmentNative.ErrExistingChildChoiceInvalid)
	}
	if input.ExistingStudentID != nil && *input.ExistingStudentID <= 0 {
		return fmt.Errorf("%w: student id required", enrollmentNative.ErrExistingChildChoiceInvalid)
	}
	return nil
}

// resolveExistingChildChoice applies the office's answer before an approval
// writes anything. It runs on the locked decision target.
//
//   - a child whose student is settled takes no choice;
//   - "create new" approves a fresh child;
//   - "use existing" must name one of the current candidates, which is
//     then pinned exactly like an existing_students match, so the approval
//     renews it — under the same phase-wide uniqueness guard;
//   - no choice approves a fresh child only while no candidate exists.
//
// A parent's pin whose relationship authority lapsed since the submission
// (parentPinLapsed) settles nothing: the child asks the office like an
// unpinned one, and approval without a choice is refused even when no
// candidate is left, because the stale pin must never reach the renewal
// that links the family. "create new" drops the pin; "use existing" replaces
// it with the office's enrollment-only pin. An existing_students phase renews
// enrolled children only, so there "create new" is refused and the office
// renews the enrollment or refuses the request.
func (d *Decisions) resolveExistingChildChoice(ctx context.Context, target *decisionTarget, phase *enrollmentNative.Phase, input enrollmentNative.DecideInput) error {
	if input.Status != enrollmentNative.DecisionApproved {
		return nil
	}
	child := target.child
	lapsed, err := d.parentPinLapsed(ctx, target.request, child)
	if err != nil {
		return err
	}
	if !lapsed && !childAwaitsExistingChildChoice(phase, child) {
		if input.ExistingStudentID != nil || input.CreateNewStudent {
			return fmt.Errorf("%w: child %d needs no choice", enrollmentNative.ErrExistingChildChoiceInvalid, child.ID)
		}
		return nil
	}
	if lapsed {
		d.logger().Info("decision: parent pin lost its relationship authority, asking the office",
			slog.Int64("request_child_id", child.ID),
			slog.Int64("student_id", *child.MatchedStudentID),
		)
	}
	if input.CreateNewStudent {
		if !enrollmentNative.OfficeMayCreateNewChild(phase) {
			return fmt.Errorf("%w: phase %d renews enrolled children only", enrollmentNative.ErrExistingChildChoiceInvalid, phase.ID)
		}
		d.logger().Info("decision: office chose a new child despite possible enrolled match",
			slog.Int64("request_child_id", child.ID),
		)
		if lapsed {
			return d.dropLapsedPin(ctx, target)
		}
		return nil
	}
	return d.applyEnrolledChildChoice(ctx, target, phase, input.ExistingStudentID, lapsed)
}

// applyEnrolledChildChoice handles an approval that asks the office and did
// not choose a new child: without a choice it is refused while a candidate
// exists or the parent pin lapsed; a chosen candidate is pinned for an
// enrollment-only renewal.
func (d *Decisions) applyEnrolledChildChoice(ctx context.Context, target *decisionTarget, phase *enrollmentNative.Phase, chosen *int64, lapsed bool) error {
	child := target.child
	candidates, err := d.existingChildCandidates(ctx, child)
	if err != nil {
		return err
	}
	if chosen == nil {
		if lapsed || len(candidates) > 0 {
			return fmt.Errorf("%w: child %d has %d candidates", enrollmentNative.ErrChildPossiblyEnrolled, child.ID, len(candidates))
		}
		return nil
	}
	if !hasCandidate(candidates, *chosen) {
		return fmt.Errorf("%w: student %d is no candidate of child %d", enrollmentNative.ErrExistingChildChoiceInvalid, *chosen, child.ID)
	}
	return d.pinChosenStudent(ctx, target, phase, *chosen)
}

// pinChosenStudent pins the office's chosen student on the child, after the
// phase-wide guard proves no other active request renews it.
func (d *Decisions) pinChosenStudent(ctx context.Context, target *decisionTarget, phase *enrollmentNative.Phase, studentID int64) error {
	child := target.child
	if err := d.deps.Requests.AcquireExistingStudentMatchLock(ctx, phase.ID); err != nil {
		return fmt.Errorf("decision: acquire existing-student match lock: %w", err)
	}
	taken, err := d.deps.Requests.HasActiveRequestForMatchedStudent(ctx, phase.ID, studentID, child.ID)
	if err != nil {
		return fmt.Errorf("decision: matched-student duplicate check for child %d: %w", child.ID, err)
	}
	if taken {
		return fmt.Errorf("%w: child %d", enrollmentNative.ErrExistingStudentAlreadyRequested, child.ID)
	}
	if err := d.deps.Children.PinOfficeChosenStudent(ctx, child.ID, studentID); err != nil {
		return fmt.Errorf("decision: pin chosen student for child %d: %w", child.ID, err)
	}
	d.logger().Info("decision: office chose an enrolled child for renewal",
		slog.Int64("request_child_id", child.ID),
		slog.Int64("student_id", studentID),
	)
	child.MatchedStudentID, child.MatchedByOffice = &studentID, true
	return nil
}

// childAsksTheOffice reports whether the open child's approval needs the
// office's choice: no pin settles it yet, or its parent pin lapsed.
func (d *Decisions) childAsksTheOffice(ctx context.Context, request *enrollmentNative.Request, phase *enrollmentNative.Phase, child *enrollmentNative.RequestChild) (bool, error) {
	if (&enrollmentNative.RequestChild{Status: child.Status}).IsTerminal() {
		return false, nil
	}
	if childAwaitsExistingChildChoice(phase, child) {
		return true, nil
	}
	if child.CreatedStudentID != nil {
		return false, nil
	}
	return d.parentPinLapsed(ctx, request, child)
}

// dropLapsedPin clears a lapsed parent pin before the office's "create new"
// approval, so the child is created fresh and no later step reads the pin.
func (d *Decisions) dropLapsedPin(ctx context.Context, target *decisionTarget) error {
	child := target.child
	if err := d.deps.Children.UpdateMatchedStudent(ctx, child.ID, nil); err != nil {
		return fmt.Errorf("decision: drop lapsed pin of child %d: %w", child.ID, err)
	}
	child.MatchedStudentID, child.MatchedByOffice = nil, false
	return nil
}

func hasCandidate(candidates []enrollmentNative.ExistingChildCandidate, studentID int64) bool {
	for _, candidate := range candidates {
		if candidate.StudentID == studentID {
			return true
		}
	}
	return false
}

// attachExistingChildCandidates fills the detail read's candidates of every
// child whose approval still asks the office, including a child whose
// parent pin lapsed.
func (d *Decisions) attachExistingChildCandidates(ctx context.Context, request *enrollmentNative.Request, summary *enrollmentNative.DecisionSummary) error {
	if summary == nil || request == nil {
		return nil
	}
	for _, child := range summary.Children {
		asks, err := d.childAsksTheOffice(ctx, request, summary.Phase, child)
		if err != nil {
			return err
		}
		if !asks {
			continue
		}
		candidates, err := d.existingChildCandidates(ctx, child)
		if err != nil {
			return err
		}
		// An unpinned child needs no choice without candidates. A lapsed
		// pin still does, as childAsksTheOffice already established.
		if len(candidates) == 0 && childAwaitsExistingChildChoice(summary.Phase, child) {
			continue
		}
		if summary.ExistingChildCandidates == nil {
			summary.ExistingChildCandidates = map[int64][]enrollmentNative.ExistingChildCandidate{}
		}
		summary.ExistingChildCandidates[child.ID] = candidates
	}
	return nil
}
