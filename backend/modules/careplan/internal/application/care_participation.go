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

func (s *CareLifecycle) inUnitParticipatingStudentIDs(ctx context.Context, studentIDs []int64, on calendar.Date, actuallyPresent map[int64]bool) (map[int64]bool, error) {
	today := s.today()
	if actuallyPresent == nil && on == today {
		var err error
		if actuallyPresent, err = s.findOpenPresence(ctx, studentIDs); err != nil {
			return nil, err
		}
	}
	return s.participatingOn(ctx, studentIDs, on, today, actuallyPresent)
}

// participatingOn decides one day against the caller's today, which ends the
// wait for a scheduled care start on past days (domain.AwaitsCareStartOn).
func (s *CareLifecycle) participatingOn(ctx context.Context, studentIDs []int64, on, today calendar.Date, actuallyPresent map[int64]bool) (map[int64]bool, error) {
	students, boundaries, err := s.participationFacts(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	return domain.ParticipatingOn(studentIDs, students, boundaries, on, today, actuallyPresent), nil
}

func (s *CareLifecycle) inUnitResolveListParticipation(ctx context.Context, studentIDs []int64, on, today calendar.Date, includePending bool) (*careplan.CareParticipationResolution, error) {
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
		participating, err = s.administrativelyVisibleStudentIDs(ctx, candidates, on, today, present)
	} else {
		participating, err = s.participatingOn(ctx, candidates, on, today, present)
	}
	if err != nil {
		return nil, err
	}
	return &careplan.CareParticipationResolution{
		CandidateIDs: candidates, ParticipatingIDs: participating, ActuallyPresentIDs: present,
	}, nil
}

func (s *CareLifecycle) inUnitParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[calendar.Date]map[int64]bool, error) {
	students, boundaries, err := s.participationFacts(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[calendar.Date]map[int64]bool)
	today := s.today()
	for day := from; !day.After(to); day = day.AddDays(1) {
		result[day] = domain.ParticipatingOn(studentIDs, students, boundaries, day, today, nil)
	}
	return result, nil
}

// administrativelyVisibleStudentIDs keeps a child with a pending withdrawal
// task in the administrative lists until the task is resolved, and a child
// scheduled for a later care start before that start (#6041).
func (s *CareLifecycle) administrativelyVisibleStudentIDs(ctx context.Context, studentIDs []int64, on, today calendar.Date, actuallyPresent map[int64]bool) (map[int64]bool, error) {
	retained, err := s.records.ListPendingWithdrawalStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: list pending care withdrawal student ids: %w", err)
	}
	for id, present := range actuallyPresent {
		if present {
			retained[id] = true
		}
	}
	students, boundaries, err := s.participationFacts(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	return domain.AdministrativelyVisibleOn(studentIDs, students, boundaries, on, today, retained), nil
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

func (s *CareLifecycle) inUnitPreviewBookingAuthorityImpact(ctx context.Context, on calendar.Date) (*careplan.BookingAuthorityImpact, error) {
	evaluations, err := s.evaluateCareBookings(ctx, on)
	if err != nil {
		return nil, err
	}
	return domain.BuildBookingAuthorityImpact(evaluations, on), nil
}

func (s *CareLifecycle) inUnitApplyBookingAuthoritySetting(ctx context.Context, on calendar.Date, enabled bool) (*careplan.BookingAuthorityImpact, error) {
	if err := s.lockCareBookingWrites(ctx); err != nil {
		return nil, err
	}
	if !enabled {
		changed, err := s.records.MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx, time.Now())
		if err == nil && changed > 0 {
			s.announcePlanningChange(ctx)
		}
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
	changed, err := s.reconcileBookingEvaluations(ctx, evaluations, on)
	if err != nil {
		return nil, err
	}
	// The planned completions decide who still counts as cared for, so the
	// planned child counts move with the switch (#5554).
	if changed {
		s.announcePlanningChange(ctx)
	}
	return impact, nil
}

func (s *CareLifecycle) inUnitHistoricalParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[calendar.Date]map[int64]bool, error) {
	students, boundaries, err := s.participationFacts(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	today := s.today()
	attended, err := s.presenceWhileAwaitingCare(ctx, students, from, to, today)
	if err != nil {
		return nil, err
	}
	result := make(map[calendar.Date]map[int64]bool)
	for day := from; !day.After(to); day = day.AddDays(1) {
		result[day] = domain.HistoricalParticipationOn(studentIDs, students, boundaries, day, today, attended[day])
	}
	return result, nil
}

// presenceWhileAwaitingCare reads the recorded presence of the children that
// await their care on a day of the period (domain.AwaitsCareStartOn): real
// care on such a day still counts (#6041 review 3). Without such a child
// there is nothing to read.
func (s *CareLifecycle) presenceWhileAwaitingCare(ctx context.Context, students map[int64]domain.CareStudent, from, to, today calendar.Date) (map[calendar.Date]map[int64]bool, error) {
	var awaiting []int64
	for id, student := range students {
		for day := from; !day.After(to); day = day.AddDays(1) {
			if student.AwaitsCareStartOn(day, today) {
				awaiting = append(awaiting, id)
				break
			}
		}
	}
	if len(awaiting) == 0 {
		return nil, nil
	}
	slices.Sort(awaiting)
	attended, err := s.owners.Presence.PresenceDaysBetween(ctx, awaiting, from, to)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: list presence days: %w", err)
	}
	return attended, nil
}
