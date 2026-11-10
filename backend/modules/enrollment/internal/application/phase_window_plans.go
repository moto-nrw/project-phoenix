package application

import (
	"context"
	"errors"
	"fmt"

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
	plans, err := s.deps.Records.PhaseActivationPlans(ctx, after.ID, before.ServiceStartDate)
	if err != nil {
		return fmt.Errorf("phase update: list activation plans: %w", err)
	}
	for start := 0; start < len(plans); {
		end := start + 1
		for end < len(plans) && plans[end].StudentID == plans[start].StudentID {
			end++
		}
		if err := s.followStudentActivation(ctx, plans[start].StudentID, plans[start:end], before, after); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (s *Phases) followStudentActivation(ctx context.Context, studentID int64, plans []enrollment.ActivationPlan, before, after *enrollment.Phase) error {
	from, until := followedWindow(plans, before, after)
	pending, err := s.deps.Students.MovePendingWindow(ctx, PendingWindowMove{
		StudentID:  studentID,
		FromBefore: calendar.Date(before.ServiceStartDate), FromAfter: from,
		UntilBefore: calendar.Date(before.ServiceEndDate), UntilAfter: until,
	})
	if err != nil {
		if errors.Is(err, enrollment.ErrInvalidPhase) && s.deps.Runtime.MarkRollback != nil {
			s.deps.Runtime.MarkRollback(ctx)
		}
		return fmt.Errorf("phase update: move enrollment window of student %d: %w", studentID, err)
	}
	if !pending {
		return nil
	}
	newStart := after.ServiceStartDate
	for _, plan := range plans {
		if plan.PhaseID != after.ID || plan.On != before.ServiceStartDate || plan.On == newStart {
			continue
		}
		if err := s.deps.Records.UpdateChildActivationPlan(ctx, plan.RequestChildID, enrollment.ChildActivationScheduled, &newStart); err != nil {
			return fmt.Errorf("phase update: move activation plan of request child %d: %w", plan.RequestChildID, err)
		}
	}
	return nil
}

// followedWindow derives the window the edit proposes for a pending student
// whose bounds still equal the phase's old ones. Approvals in other phases
// that lie inside the old window keep their day: the start becomes the
// earliest, the end the latest of them and the new phase bound.
func followedWindow(plans []enrollment.ActivationPlan, before, after *enrollment.Phase) (calendar.Date, calendar.Date) {
	oldStart, oldEnd := before.ServiceStartDate, before.ServiceEndDate
	from, until := after.ServiceStartDate, after.ServiceEndDate
	for _, plan := range plans {
		if plan.PhaseID == after.ID {
			continue
		}
		if plan.On >= oldStart && plan.On < from {
			from = plan.On
		}
		if plan.PhaseEnd <= oldEnd && plan.PhaseEnd > until {
			until = plan.PhaseEnd
		}
	}
	return calendar.Date(from), calendar.Date(until)
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
	bookings, err := s.deps.Offerings.CareOfferingBookingsForPhase(ctx, phaseID)
	if err != nil {
		return fmt.Errorf("phase delete: list care offering bookings: %w", err)
	}
	booked := make([]int64, 0, len(bookings))
	for _, booking := range bookings {
		if booking.ValidUntil == nil || calendar.Date(*booking.ValidUntil).After(today) {
			booked = append(booked, booking.RequestChildID)
		}
	}
	if len(booked) == 0 {
		return nil
	}
	children, err := s.deps.Records.ChildrenByID(ctx, booked)
	if err != nil {
		return fmt.Errorf("phase delete: load booked children: %w", err)
	}
	approved := 0
	for _, child := range children {
		if child != nil && child.Status == enrollment.ChildStatusApproved && child.CreatedStudentID != nil {
			approved++
		}
	}
	if approved > 0 {
		return fmt.Errorf("%w: %d children", enrollment.ErrPhaseHasEffectiveBookings, approved)
	}
	return nil
}
