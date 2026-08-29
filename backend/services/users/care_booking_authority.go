package users

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// ErrBookingAuthorityBlocked is returned when enabling booking-led care would
// immediately leave currently cared-for children without any care day.
//
//nolint:staticcheck // ST1005: user-facing German message
var ErrBookingAuthorityBlocked = errors.New("Der Buchungsmodus kann nicht aktiviert werden. Für mindestens ein aktuell betreutes Kind ist kein Betreuungstag gebucht.")

type BookingAuthorityImpactChild struct {
	StudentID           string         `json:"student_id"`
	FirstName           string         `json:"first_name"`
	LastName            string         `json:"last_name"`
	SchoolClass         string         `json:"school_class"`
	FirstBookinglessDay *timezone.Date `json:"first_bookingless_day,omitempty"`
}

type BookingAuthorityImpact struct {
	ReferenceDate      timezone.Date                 `json:"reference_date"`
	BlockingChildren   []BookingAuthorityImpactChild `json:"blocking_children"`
	PlannedCompletions []BookingAuthorityImpactChild `json:"planned_completions"`
}

func (s *careLifecycleService) PreviewBookingAuthorityImpact(
	ctx context.Context, on timezone.Date,
) (*BookingAuthorityImpact, error) {
	evaluations, err := s.evaluateCareBookings(ctx, on)
	if err != nil {
		return nil, err
	}
	return buildBookingAuthorityImpact(evaluations, on), nil
}

// ParticipatingStudentIDs applies the same durable first-bookingless-day
// boundary to every operational reader. Actual presence always wins: safety
// information must remain visible even when the expected care has ended.
func (s *careLifecycleService) ParticipatingStudentIDs(
	ctx context.Context, studentIDs []int64, on timezone.Date, actuallyPresent map[int64]bool,
) (map[int64]bool, error) {
	if actuallyPresent == nil && on == timezone.TodayDate() {
		var err error
		actuallyPresent, err = s.cleanupRepo.FindOpenPresence(ctx, studentIDs)
		if err != nil {
			return nil, err
		}
	}
	boundaries, err := s.participationBoundaries(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	participating := participatingOn(studentIDs, boundaries, on, actuallyPresent)
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	removeAlumni(participating, students, actuallyPresent)
	return participating, nil
}

// ResolveListParticipation owns the candidate, live-presence and dated care
// decision for API lists and reports. The caller supplies a frozen today so a
// request crossing midnight cannot combine two calendar days.
func (s *careLifecycleService) ResolveListParticipation(
	ctx context.Context, studentIDs []int64, on, today timezone.Date, includePending bool,
) (*CareParticipationResolution, error) {
	candidates := studentIDs
	if len(candidates) == 0 {
		var err error
		candidates, err = s.studentRepo.ListIDs(ctx)
		if err != nil {
			return nil, err
		}
	}
	present := map[int64]bool{}
	if on == today {
		var err error
		present, err = s.cleanupRepo.FindOpenPresence(ctx, candidates)
		if err != nil {
			return nil, err
		}
	}
	participating, err := s.resolveListParticipation(ctx, candidates, on, present, includePending)
	if err != nil {
		return nil, err
	}
	return &CareParticipationResolution{
		CandidateIDs: candidates, ParticipatingIDs: participating, ActuallyPresentIDs: present,
	}, nil
}

func (s *careLifecycleService) resolveListParticipation(
	ctx context.Context, candidates []int64, on timezone.Date, present map[int64]bool, includePending bool,
) (map[int64]bool, error) {
	if includePending {
		return s.AdministrativelyVisibleStudentIDs(ctx, candidates, on, present)
	}
	return s.ParticipatingStudentIDs(ctx, candidates, on, present)
}

func (s *careLifecycleService) ParticipatingStudentIDsByDate(
	ctx context.Context, studentIDs []int64, from, to timezone.Date,
) (map[timezone.Date]map[int64]bool, error) {
	boundaries, err := s.participationBoundaries(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[timezone.Date]map[int64]bool)
	for day := from; !day.After(to); day = day.AddDays(1) {
		participating := participatingOn(studentIDs, boundaries, day, nil)
		removeAlumni(participating, students, nil)
		result[day] = participating
	}
	return result, nil
}

func (s *careLifecycleService) AdministrativelyVisibleStudentIDs(
	ctx context.Context, studentIDs []int64, on timezone.Date, actuallyPresent map[int64]bool,
) (map[int64]bool, error) {
	retained, err := s.withdrawalRepo.ListPendingStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	for id, present := range actuallyPresent {
		if present {
			retained[id] = true
		}
	}
	return s.ParticipatingStudentIDs(ctx, studentIDs, on, retained)
}

func (s *careLifecycleService) participationBoundaries(
	ctx context.Context, studentIDs []int64,
) (map[int64]timezone.Date, error) {
	if len(studentIDs) == 0 {
		return map[int64]timezone.Date{}, nil
	}
	if s.bookingsAuthoritative == nil {
		return nil, errors.New("care lifecycle: bookings-authoritative resolver is not configured")
	}
	authoritative, err := s.bookingsAuthoritative(ctx)
	if err != nil {
		return nil, err
	}
	return s.withdrawalRepo.ListParticipationBoundaries(ctx, studentIDs, authoritative)
}

func removeAlumni(
	participating map[int64]bool, students map[int64]*userModels.Student, actuallyPresent map[int64]bool,
) {
	for id, student := range students {
		if student.Status == userModels.StudentStatusAlumnus && !actuallyPresent[id] {
			delete(participating, id)
		}
	}
}

func participatingOn(
	studentIDs []int64,
	boundaries map[int64]timezone.Date,
	on timezone.Date,
	actuallyPresent map[int64]bool,
) map[int64]bool {
	participating := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		participating[id] = true
	}
	for id, boundary := range boundaries {
		if !actuallyPresent[id] && !on.Before(boundary) {
			delete(participating, id)
		}
	}
	return participating
}

// ApplyBookingAuthoritySetting validates and reconciles a mode switch while
// the caller's tenant transaction is still open. The recurrence lock is the
// same lock held by booking mutations, so a preview made before this call
// cannot bypass the binding recheck.
func (s *careLifecycleService) ApplyBookingAuthoritySetting(
	ctx context.Context, on timezone.Date, enabled bool,
) (*BookingAuthorityImpact, error) {
	if s.lockCareBookingWrites == nil {
		return nil, errors.New("care lifecycle: booking write lock is not configured")
	}
	if err := s.lockCareBookingWrites(ctx); err != nil {
		return nil, err
	}
	if !enabled {
		_, err := s.withdrawalRepo.MarkPendingObsoleteForWeeklyPlans(ctx, time.Now())
		return &BookingAuthorityImpact{ReferenceDate: on, BlockingChildren: []BookingAuthorityImpactChild{}, PlannedCompletions: []BookingAuthorityImpactChild{}}, err
	}

	evaluations, err := s.evaluateCareBookings(ctx, on)
	if err != nil {
		return nil, err
	}
	impact := buildBookingAuthorityImpact(evaluations, on)
	if len(impact.BlockingChildren) > 0 {
		return impact, ErrBookingAuthorityBlocked
	}
	if err := s.reconcileBookingEvaluations(ctx, evaluations, on); err != nil {
		return nil, err
	}
	return impact, nil
}

func (s *careLifecycleService) evaluateCareBookings(
	ctx context.Context, on timezone.Date,
) ([]userModels.CareBookingEvaluation, error) {
	facts, err := s.cleanupRepo.ListCareBookingFacts(ctx, on, nil)
	if err != nil {
		return nil, err
	}
	return EvaluateCareBookingStates(facts, on), nil
}

func buildBookingAuthorityImpact(
	evaluations []userModels.CareBookingEvaluation, on timezone.Date,
) *BookingAuthorityImpact {
	impact := &BookingAuthorityImpact{
		ReferenceDate:      on,
		BlockingChildren:   make([]BookingAuthorityImpactChild, 0),
		PlannedCompletions: make([]BookingAuthorityImpactChild, 0),
	}
	for _, evaluation := range evaluations {
		child := BookingAuthorityImpactChild{
			StudentID:           strconv.FormatInt(evaluation.StudentID, 10),
			FirstName:           evaluation.FirstName,
			LastName:            evaluation.LastName,
			SchoolClass:         evaluation.SchoolClass,
			FirstBookinglessDay: evaluation.FirstBookinglessDay,
		}
		if !evaluation.HasCareDays {
			impact.BlockingChildren = append(impact.BlockingChildren, child)
			continue
		}
		if evaluation.FirstBookinglessDay != nil && evaluation.FirstBookinglessDay.After(on) {
			impact.PlannedCompletions = append(impact.PlannedCompletions, child)
		}
	}
	return impact
}

func (s *careLifecycleService) reconcileBookingEvaluations(
	ctx context.Context, evaluations []userModels.CareBookingEvaluation, on timezone.Date,
) error {
	studentIDs := make([]int64, len(evaluations))
	for i, evaluation := range evaluations {
		studentIDs[i] = evaluation.StudentID
	}
	pending, err := s.withdrawalRepo.ListPendingByStudentIDs(ctx, studentIDs)
	if err != nil {
		return err
	}
	for _, evaluation := range evaluations {
		change := userModels.CareWithdrawalBookingChange{StudentID: evaluation.StudentID, ConfirmedRole: "system"}
		if err := s.reconcileBookingEvaluation(ctx, evaluation, change, on, pending[evaluation.StudentID]); err != nil {
			return err
		}
	}
	return nil
}
