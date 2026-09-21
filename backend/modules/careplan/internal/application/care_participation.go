package application

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ParticipatingStudentIDs applies the same durable first-bookingless-day
// boundary to every operational reader. Actual presence always wins: safety
// information must remain visible even when the expected care has ended.
func (s *CareLifecycle) ParticipatingStudentIDs(ctx context.Context, studentIDs []int64, on calendar.Date, actuallyPresent map[int64]bool) (map[int64]bool, error) {
	if actuallyPresent == nil && on == calendar.TodayDate() {
		var err error
		if actuallyPresent, err = s.findOpenPresence(ctx, studentIDs); err != nil {
			return nil, err
		}
	}
	students, boundaries, err := s.participationFacts(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	return domain.ParticipatingOn(studentIDs, students, boundaries, on, actuallyPresent), nil
}

// ResolveListParticipation owns the candidate, live-presence and dated care
// decision for lists and reports. The caller supplies a frozen today so a
// request crossing midnight cannot combine two calendar days.
func (s *CareLifecycle) ResolveListParticipation(ctx context.Context, studentIDs []int64, on, today calendar.Date, includePending bool) (*careplan.CareParticipationResolution, error) {
	candidates := studentIDs
	if len(candidates) == 0 {
		var err error
		if candidates, err = s.owners.Students.ListStudentIDs(ctx); err != nil {
			return nil, err
		}
	}
	present := map[int64]bool{}
	if on == today {
		var err error
		if present, err = s.findOpenPresence(ctx, candidates); err != nil {
			return nil, err
		}
	}
	var participating map[int64]bool
	var err error
	if includePending {
		participating, err = s.administrativelyVisibleStudentIDs(ctx, candidates, on, present)
	} else {
		participating, err = s.ParticipatingStudentIDs(ctx, candidates, on, present)
	}
	if err != nil {
		return nil, err
	}
	return &careplan.CareParticipationResolution{
		CandidateIDs: candidates, ParticipatingIDs: participating, ActuallyPresentIDs: present,
	}, nil
}

func (s *CareLifecycle) ParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[calendar.Date]map[int64]bool, error) {
	students, boundaries, err := s.participationFacts(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[calendar.Date]map[int64]bool)
	for day := from; !day.After(to); day = day.AddDays(1) {
		result[day] = domain.ParticipatingOn(studentIDs, students, boundaries, day, nil)
	}
	return result, nil
}

// administrativelyVisibleStudentIDs keeps a child with a pending withdrawal
// task in the administrative lists until the task is resolved.
func (s *CareLifecycle) administrativelyVisibleStudentIDs(ctx context.Context, studentIDs []int64, on calendar.Date, actuallyPresent map[int64]bool) (map[int64]bool, error) {
	retained, err := s.records.ListPendingWithdrawalStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: list pending care withdrawal student ids: %w", err)
	}
	for id, present := range actuallyPresent {
		if present {
			retained[id] = true
		}
	}
	return s.ParticipatingStudentIDs(ctx, studentIDs, on, retained)
}

// participationFacts reads the children and their first day without
// participation: the day after the enrolment end, or the pending completion
// boundary when it comes first. Booking-led boundaries count only in
// booking-led mode.
func (s *CareLifecycle) participationFacts(ctx context.Context, studentIDs []int64) (map[int64]domain.CareStudent, map[int64]calendar.Date, error) {
	students, err := s.owners.Students.FindCareStudents(ctx, studentIDs, false)
	if err != nil {
		return nil, nil, err
	}
	if len(studentIDs) == 0 {
		return students, map[int64]calendar.Date{}, nil
	}
	authoritative, err := s.bookingsAuthoritative(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(students) == 0 {
		return students, map[int64]calendar.Date{}, nil
	}
	pending, err := s.records.ListPendingWithdrawalBoundaries(ctx, slices.Sorted(maps.Keys(students)), authoritative)
	if err != nil {
		return nil, nil, fmt.Errorf("care lifecycle: list care participation boundaries: %w", err)
	}
	pendingDays := make(map[int64]calendar.Date, len(pending))
	for id, day := range pending {
		pendingDays[id] = calendar.Date(day)
	}
	return students, domain.ParticipationBoundaries(students, pendingDays), nil
}

func (s *CareLifecycle) PreviewBookingAuthorityImpact(ctx context.Context, on calendar.Date) (*careplan.BookingAuthorityImpact, error) {
	evaluations, err := s.evaluateCareBookings(ctx, on)
	if err != nil {
		return nil, err
	}
	return domain.BuildBookingAuthorityImpact(evaluations, on), nil
}

// ApplyBookingAuthoritySetting validates and reconciles a mode switch while
// the caller's tenant transaction is still open. The booking-write lock is
// the one booking mutations hold, so a preview made before this call cannot
// bypass the binding recheck.
func (s *CareLifecycle) ApplyBookingAuthoritySetting(ctx context.Context, on calendar.Date, enabled bool) (*careplan.BookingAuthorityImpact, error) {
	if err := s.lockCareBookingWrites(ctx); err != nil {
		return nil, err
	}
	if !enabled {
		_, err := s.records.MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx, time.Now())
		return &careplan.BookingAuthorityImpact{
			ReferenceDate: on, BlockingChildren: []careplan.BookingAuthorityImpactChild{},
			PlannedCompletions: []careplan.BookingAuthorityImpactChild{},
		}, err
	}
	evaluations, err := s.evaluateCareBookings(ctx, on)
	if err != nil {
		return nil, err
	}
	impact := domain.BuildBookingAuthorityImpact(evaluations, on)
	if len(impact.BlockingChildren) > 0 {
		return impact, careplan.ErrBookingAuthorityBlocked
	}
	if err := s.reconcileBookingEvaluations(ctx, evaluations, on); err != nil {
		return nil, err
	}
	return impact, nil
}
