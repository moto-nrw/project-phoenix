package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	peopleEnrollment "github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
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
//   - A student that is no longer pending was activated or changed by hand
//     and keeps its window.
//   - A bound that no longer equals the phase's old bound came from another
//     enrollment or a manual edit and stays. A student with approvals in
//     several phases starts with the earliest and ends with the latest plan
//     inside its current window.
//
// Each student is locked, in ascending id order, before its status is read,
// so a concurrent status change either commits first and is seen, or waits.
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
	student, err := s.deps.Students.ReadEnrollmentStudent(ctx, studentID, "update")
	if errors.Is(err, peopleEnrollment.ErrStudentNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("phase update: lock student %d: %w", studentID, err)
	}
	if student.Status != studentStatusPending {
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
	from, until := followedWindow(student, plans, before, after)
	if from == student.EnrolledFrom && until == student.EnrolledUntil {
		return nil
	}
	if until != "" && from > until {
		if s.deps.Runtime.MarkRollback != nil {
			s.deps.Runtime.MarkRollback(ctx)
		}
		return fmt.Errorf("%w: the new service window leaves a pending child of the phase without a care day; adjust the child's enrollment first", enrollment.ErrInvalidPhase)
	}
	if err := s.deps.Students.RenewEnrollmentStudent(ctx, studentID, peopleEnrollment.Input{
		PersonID: student.PersonID, SchoolClass: student.SchoolClass, Status: student.Status,
		EnrolledFrom: from, EnrolledUntil: until,
	}); err != nil {
		return fmt.Errorf("phase update: move enrollment window of student %d: %w", studentID, err)
	}
	return nil
}

// followedWindow derives the pending student's enrollment window after the
// edit. A bound follows only while it still equals the phase's old bound;
// approvals in other phases that lie inside the current window keep theirs.
func followedWindow(student peopleEnrollment.Record, plans []enrollment.ActivationPlan, before, after *enrollment.Phase) (string, string) {
	from, until := student.EnrolledFrom, student.EnrolledUntil
	oldStart, oldEnd := string(before.ServiceStartDate), string(before.ServiceEndDate)
	if oldStart != string(after.ServiceStartDate) && from == oldStart {
		from = string(after.ServiceStartDate)
		for _, plan := range plans {
			if plan.PhaseID != after.ID && string(plan.On) >= oldStart && string(plan.On) < from {
				from = string(plan.On)
			}
		}
	}
	if oldEnd != string(after.ServiceEndDate) && until == oldEnd {
		until = string(after.ServiceEndDate)
		for _, plan := range plans {
			if plan.PhaseID != after.ID && string(plan.PhaseEnd) <= oldEnd && string(plan.PhaseEnd) > until {
				until = string(plan.PhaseEnd)
			}
		}
	}
	return from, until
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
