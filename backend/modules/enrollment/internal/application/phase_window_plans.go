package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// followPendingActivations moves the open activation plans of the phase and
// the enrollment window of their still pending students with a service-window
// edit, in the edit's transaction (#6505). The store already moved the phase's
// pending weekly plans and class switches.
//
// Only what the phase still decides moves:
//   - An activation whose day has come counts as executed, as a class switch
//     does (replanApprovedChildClass): the hourly activation tick may already
//     have read the child as due. Only a start after today moves, which the
//     tick cannot have selected, so it never activates a moved start.
//   - School Membership moves a student only while it is pending, deciding
//     under the student's row lock; an activated child keeps its window and
//     the plan of an activated child stays as executed.
//   - A bound that no longer equals the phase's old bound came from another
//     enrollment or a manual edit and stays. A student with approvals in
//     several phases starts with the earliest and ends with the latest plan
//     inside its old window.
//
// The edit holds the recurrence gate and the phase row; the command takes
// only the student's row lock, no class gate, so the edit never waits for a
// gate after its own (ADR 0162). Students are moved in ascending id order.
func (s *Phases) followPendingActivations(ctx context.Context, before, after *enrollment.Phase) error {
	if s.deps.Students == nil || !calendar.Date(before.ServiceStartDate).After(s.todayDate()) {
		return nil
	}
	plans, err := s.pendingActivationPlans(ctx, after.ID, before.ServiceStartDate)
	if err != nil || len(plans) == 0 {
		return err
	}
	studentIDs := make([]int64, 0, len(plans))
	for studentID := range plans {
		studentIDs = append(studentIDs, studentID)
	}
	slices.Sort(studentIDs)
	periods, err := s.deps.Records.StudentCarePeriodsByStudentIDs(ctx, studentIDs)
	if err != nil {
		return fmt.Errorf("phase update: list approvals of pending students: %w", err)
	}
	others := map[int64][]*enrollment.StudentCarePeriod{}
	for _, period := range periods {
		if period != nil && period.PhaseID != after.ID {
			others[period.StudentID] = append(others[period.StudentID], period)
		}
	}
	for _, studentID := range studentIDs {
		if err := s.followStudentActivation(ctx, studentID, plans[studentID], others[studentID], before, after); err != nil {
			return err
		}
	}
	return nil
}

// pendingActivationPlans maps each student to the approved request children
// of the phase that plan its activation on the phase's old start.
func (s *Phases) pendingActivationPlans(ctx context.Context, phaseID int64, oldStart enrollment.Date) (map[int64][]int64, error) {
	children, err := s.deps.Records.PhaseResponseChildren(ctx, phaseID, []string{enrollment.ChildStatusApproved})
	if err != nil {
		return nil, fmt.Errorf("phase update: list approved children: %w", err)
	}
	plans := map[int64][]int64{}
	for _, row := range children {
		child := row.Child
		if !row.IsPhaseChild || child == nil || child.CreatedStudentID == nil ||
			child.ActivationMode != enrollment.ChildActivationScheduled || child.ActivateOn == nil ||
			enrollment.Date(*child.ActivateOn) != oldStart {
			continue
		}
		plans[*child.CreatedStudentID] = append(plans[*child.CreatedStudentID], child.ID)
	}
	return plans, nil
}

// followStudentActivation moves the student's activation plans of the phase
// and then asks School Membership to move the student's window. The request
// children are locked before the student (ADR 0162); a student that is no
// longer pending gets its plans back.
func (s *Phases) followStudentActivation(
	ctx context.Context, studentID int64, childIDs []int64, others []*enrollment.StudentCarePeriod, before, after *enrollment.Phase,
) error {
	startMoves := after.ServiceStartDate != before.ServiceStartDate
	if startMoves {
		if err := s.setActivationPlans(ctx, childIDs, after.ServiceStartDate); err != nil {
			return err
		}
	}
	move := followedWindow(others, before, after)
	move.StudentID = studentID
	pending, err := s.deps.Students.MovePendingWindow(ctx, move)
	if err != nil {
		if errors.Is(err, enrollment.ErrPhasePendingChildWindowConflict) && s.deps.Runtime.MarkRollback != nil {
			s.deps.Runtime.MarkRollback(ctx)
		}
		return fmt.Errorf("phase update: move enrollment window of student %d: %w", studentID, err)
	}
	if !pending && startMoves {
		return s.setActivationPlans(ctx, childIDs, before.ServiceStartDate)
	}
	return nil
}

func (s *Phases) setActivationPlans(ctx context.Context, childIDs []int64, on enrollment.Date) error {
	for _, childID := range childIDs {
		if err := s.deps.Records.UpdateChildActivationPlan(ctx, childID, enrollment.ChildActivationScheduled, &on); err != nil {
			return fmt.Errorf("phase update: set activation plan of request child %d: %w", childID, err)
		}
	}
	return nil
}

// followedWindow proposes the window of a pending student whose bounds still
// equal the phase's old ones. Approvals in other phases that lie inside the
// old window keep their days: the start becomes the earliest, the end the
// latest of them and the new phase bound. Only other bounds outside the old
// phase go along for widening: a phase whose old bound is already outside
// the student's window has been superseded and must not restore that bound.
func followedWindow(others []*enrollment.StudentCarePeriod, before, after *enrollment.Phase) enrollment.PendingWindowMove {
	oldStart, oldEnd := calendar.Date(before.ServiceStartDate), calendar.Date(before.ServiceEndDate)
	from, until := calendar.Date(after.ServiceStartDate), calendar.Date(after.ServiceEndDate)
	move := enrollment.PendingWindowMove{FromBefore: before.ServiceStartDate, UntilBefore: before.ServiceEndDate}
	for _, other := range others {
		start, end := calendar.Date(other.ServiceStartDate), calendar.Date(other.ServiceEndDate)
		if !start.Before(oldStart) && start.Before(from) {
			from = start
		}
		if !end.After(oldEnd) && end.After(until) {
			until = end
		}
		if start.Before(oldStart) {
			move.OtherStarts = append(move.OtherStarts, other.ServiceStartDate)
		}
		if end.After(oldEnd) {
			move.OtherEnds = append(move.OtherEnds, other.ServiceEndDate)
		}
	}
	move.FromAfter, move.UntilAfter = enrollment.Date(from), enrollment.Date(until)
	return move
}

// refuseEffectiveBookings rejects deleting a running or future phase while a
// booking of an approved child of its offerings still applies today or later
// (#6505). Validity is half-open: a booking ending today no longer applies.
// Bookings of applications not yet approved are no care and do not block.
// The caller holds the recurrence gate.
func (s *Phases) refuseEffectiveBookings(ctx context.Context, phaseID int64) error {
	if s.deps.Offerings == nil {
		return nil
	}
	phase, err := s.deps.Records.Phase(ctx, phaseID)
	if err != nil {
		if s.notFound(err) {
			return enrollment.ErrPhaseNotFound
		}
		return fmt.Errorf("phase delete: find phase: %w", err)
	}
	today := s.todayDate()
	if calendar.Date(phase.ServiceEndDate).Before(today) {
		return nil
	}
	booked, err := s.effectivelyBookedChildren(ctx, phaseID, today)
	if err != nil || len(booked) == 0 {
		return err
	}
	approved, err := s.approvedChildren(ctx, phaseID)
	if err != nil {
		return err
	}
	count := 0
	for childID := range booked {
		if approved[childID] {
			count++
		}
	}
	if count > 0 {
		return fmt.Errorf("%w: %d children", enrollment.ErrPhaseHasEffectiveBookings, count)
	}
	return nil
}

// effectivelyBookedChildren lists the request children with a booking of the
// phase's offerings that applies on today or later.
func (s *Phases) effectivelyBookedChildren(ctx context.Context, phaseID int64, today calendar.Date) (map[int64]bool, error) {
	bookings, err := s.deps.Offerings.CareOfferingBookingsForPhase(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("phase delete: list care offering bookings: %w", err)
	}
	booked := map[int64]bool{}
	for _, booking := range bookings {
		if booking.ValidUntil == nil || calendar.Date(*booking.ValidUntil).After(today) {
			booked[booking.RequestChildID] = true
		}
	}
	return booked, nil
}

// approvedChildren lists the approved request children of the phase that are
// linked to a child record.
func (s *Phases) approvedChildren(ctx context.Context, phaseID int64) (map[int64]bool, error) {
	children, err := s.deps.Records.PhaseResponseChildren(ctx, phaseID, []string{enrollment.ChildStatusApproved})
	if err != nil {
		return nil, fmt.Errorf("phase delete: list approved children: %w", err)
	}
	approved := map[int64]bool{}
	for _, row := range children {
		if row.IsPhaseChild && row.Child != nil && row.Child.CreatedStudentID != nil {
			approved[row.Child.ID] = true
		}
	}
	return approved, nil
}
