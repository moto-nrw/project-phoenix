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
func childAwaitsExistingChildChoice(phase *enrollmentNative.Phase, child *RequestChild) bool {
	return audienceChecksEnrolledChildren(phase) && child != nil &&
		child.MatchedStudentID == nil && child.RolloverSourceChildID == nil && child.CreatedStudentID == nil
}

// existingChildCandidates lists the enrolled children that may be the
// child. Without the directory port the check fails closed.
func (d *Decisions) existingChildCandidates(ctx context.Context, child *RequestChild) ([]enrollmentNative.ExistingChildCandidate, error) {
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
func (d *Decisions) resolveExistingChildChoice(ctx context.Context, target *decisionTarget, phase *enrollmentNative.Phase, input enrollmentNative.DecideInput) error {
	if input.Status != enrollmentNative.DecisionApproved {
		return nil
	}
	child := target.child
	if !childAwaitsExistingChildChoice(phase, child) {
		if input.ExistingStudentID != nil || input.CreateNewStudent {
			return fmt.Errorf("%w: child %d needs no choice", enrollmentNative.ErrExistingChildChoiceInvalid, child.ID)
		}
		return nil
	}
	if input.CreateNewStudent {
		d.logger().Info("decision: office chose a new child despite possible enrolled match",
			slog.Int64("request_child_id", child.ID),
		)
		return nil
	}
	candidates, err := d.existingChildCandidates(ctx, child)
	if err != nil {
		return err
	}
	if input.ExistingStudentID == nil {
		if len(candidates) > 0 {
			return fmt.Errorf("%w: child %d has %d candidates", enrollmentNative.ErrChildPossiblyEnrolled, child.ID, len(candidates))
		}
		return nil
	}
	studentID := *input.ExistingStudentID
	if !hasCandidate(candidates, studentID) {
		return fmt.Errorf("%w: student %d is no candidate of child %d", enrollmentNative.ErrExistingChildChoiceInvalid, studentID, child.ID)
	}
	return d.pinChosenStudent(ctx, target, phase, studentID)
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
	if target.raw != nil {
		target.raw.MatchedStudentID, target.raw.MatchedByOffice = &studentID, true
	}
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
// child whose approval still asks the office.
func (d *Decisions) attachExistingChildCandidates(ctx context.Context, summary *enrollmentNative.DecisionSummary) error {
	if summary == nil || !audienceChecksEnrolledChildren(summary.Phase) {
		return nil
	}
	children, err := childValues(summary.Children)
	if err != nil {
		return err
	}
	for _, child := range children {
		if !childAwaitsExistingChildChoice(summary.Phase, child) ||
			(&enrollmentNative.RequestChild{Status: child.Status}).IsTerminal() {
			continue
		}
		candidates, err := d.existingChildCandidates(ctx, child)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			continue
		}
		if summary.ExistingChildCandidates == nil {
			summary.ExistingChildCandidates = map[int64][]enrollmentNative.ExistingChildCandidate{}
		}
		summary.ExistingChildCandidates[child.ID] = candidates
	}
	return nil
}
