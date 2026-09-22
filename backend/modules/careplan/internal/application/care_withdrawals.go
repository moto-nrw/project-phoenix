package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *CareLifecycle) ListPendingWithdrawals(ctx context.Context, filter careplan.CareWithdrawalFilter) ([]careplan.WithdrawalCompletion, int, error) {
	if filter.StudentID < 0 {
		return nil, 0, careplan.ErrCareWithdrawalNotFound
	}
	return s.listWithdrawals(ctx, careplan.WithdrawalStatePending, filter.Normalized())
}

// ListResolvedWithdrawals keeps deleted completions visible without the
// deleted child's personal data.
func (s *CareLifecycle) ListResolvedWithdrawals(ctx context.Context, filter careplan.CareWithdrawalFilter) ([]careplan.WithdrawalCompletion, int, error) {
	return s.listWithdrawals(ctx, careplan.WithdrawalStateResolved, filter.Normalized())
}

// listWithdrawals takes the children's names and classes from the People
// Directory: the tasks page and search over the directory rows of every
// child they name.
func (s *CareLifecycle) listWithdrawals(ctx context.Context, state string, filter careplan.CareWithdrawalFilter) ([]careplan.WithdrawalCompletion, int, error) {
	studentIDs, err := s.records.ListWithdrawalStudentIDs(ctx, state, filter.StudentID)
	if err != nil {
		return nil, 0, fmt.Errorf("care lifecycle: list %s withdrawal children: %w", state, err)
	}
	students := []careplan.WithdrawalStudent{}
	if len(studentIDs) > 0 {
		if students, err = s.owners.Students.ListWithdrawalStudents(ctx, studentIDs); err != nil {
			return nil, 0, fmt.Errorf("care lifecycle: list %s withdrawal children: %w", state, err)
		}
	}
	ownerFilter := careplan.WithdrawalListFilter{
		Search: filter.Search, StudentID: filter.StudentID, Page: filter.Page, PageSize: filter.PageSize, Students: students,
	}
	var values []careplan.WithdrawalCompletion
	var total int
	if state == careplan.WithdrawalStatePending {
		values, total, err = s.records.ListPendingWithdrawals(ctx, ownerFilter)
	} else {
		values, total, err = s.records.ListResolvedWithdrawals(ctx, ownerFilter)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("care lifecycle: list %s withdrawals: %w", state, err)
	}
	return values, total, nil
}

func (s *CareLifecycle) GetPendingWithdrawal(ctx context.Context, id int64) (careplan.WithdrawalCompletion, error) {
	if id <= 0 {
		return careplan.WithdrawalCompletion{}, careplan.ErrCareWithdrawalNotFound
	}
	completion, err := s.records.FindWithdrawalCompletion(ctx, id, false)
	if err != nil {
		return careplan.WithdrawalCompletion{}, withdrawalLookupError(err, careplan.ErrCareWithdrawalNotFound)
	}
	if completion.State != careplan.WithdrawalStatePending || completion.StudentID == nil {
		return careplan.WithdrawalCompletion{}, careplan.ErrCareWithdrawalAlreadyResolved
	}
	return completion, nil
}

// withdrawalLookupError turns an unknown task into the caller's sentinel.
func withdrawalLookupError(err, notFound error) error {
	if errors.Is(err, careplan.ErrWithdrawalNotFound) {
		return notFound
	}
	return fmt.Errorf("care lifecycle: find care withdrawal completion: %w", err)
}

func (s *CareLifecycle) PreviewWithdrawalCareEnd(ctx context.Context, completionID int64, input careplan.CareExitInput) (*careplan.CareExitPreview, error) {
	completion, err := s.GetPendingWithdrawal(ctx, completionID)
	if err != nil {
		return nil, err
	}
	input.StudentIDs = []int64{*completion.StudentID}
	normalized, err := domain.NormalizeCareExitInput(input, true, s.today())
	if err != nil {
		return nil, err
	}
	if err := s.validateWithdrawalCareEnd(ctx, completion, normalized); err != nil {
		return nil, err
	}
	return s.buildPreview(ctx, normalized, false, withdrawalOfferings(&completion))
}

func (s *CareLifecycle) ConfirmWithdrawalCareEnd(
	ctx context.Context, completionID int64, token string, input careplan.CareExitInput, actorAccountID int64,
) (*careplan.CareExitResult, error) {
	completion, err := s.GetPendingWithdrawal(ctx, completionID)
	if err != nil {
		return nil, err
	}
	input.StudentIDs = []int64{*completion.StudentID}
	return s.confirm(ctx, &completion, token, input, actorAccountID, true)
}

// validateWithdrawalCareEnd keeps a retroactive last care day before the
// first bookingless day, on or after the enrolment start and not before the
// latest recorded attendance.
func (s *CareLifecycle) validateWithdrawalCareEnd(ctx context.Context, completion careplan.WithdrawalCompletion, input careplan.CareExitInput) error {
	if completion.StudentID == nil {
		return careplan.ErrCareWithdrawalNotFound
	}
	studentID := *completion.StudentID
	if input.LastCareDay.After(calendar.Date(completion.FirstBookinglessDay).AddDays(-1)) {
		return careplan.ErrCareWithdrawalAfterGap
	}
	students, err := s.owners.Students.FindCareStudents(ctx, []int64{studentID}, false)
	if err != nil {
		return err
	}
	student, found := students[studentID]
	if !found {
		return careplan.ErrCareWithdrawalNotFound
	}
	if student.EnrolledFrom != nil && input.LastCareDay.Before(*student.EnrolledFrom) {
		return &careplan.CareWithdrawalDateError{Message: fmt.Sprintf(careplan.CareBlockerBeforeStart, student.EnrolledFrom.Format("02.01.2006"))}
	}
	latest, err := s.latestAttendanceDate(ctx, studentID)
	if err != nil {
		return err
	}
	if latest != nil && input.LastCareDay.Before(*latest) {
		return &careplan.CareWithdrawalDateError{Message: fmt.Sprintf("Eine Anwesenheit ist bis zum %s erfasst. Der letzte Betreuungstag darf nicht davor liegen.", latest.Format("02.01.2006"))}
	}
	return nil
}

// withdrawalSourceOfferings decodes the offerings a task recorded. The JSON
// is Care Plan's own and written from the same type.
func withdrawalSourceOfferings(completion careplan.WithdrawalCompletion) []careplan.CareExitSourceOffering {
	var offerings []careplan.CareExitSourceOffering
	if len(completion.SourceOfferings) > 0 {
		if err := json.Unmarshal(completion.SourceOfferings, &offerings); err != nil {
			return nil
		}
	}
	return offerings
}

// ReconcileAuthoritativeBookingChange keeps the withdrawal tasks in step with
// one booking mutation. In weekly-plan mode only a rebooking can make a task
// obsolete; in booking-led mode the child's bookings are evaluated again.
func (s *CareLifecycle) ReconcileAuthoritativeBookingChange(ctx context.Context, change careplan.CareWithdrawalBookingChange) error {
	authoritative, err := s.bookingsAuthoritative(ctx)
	if err != nil {
		return err
	}
	if !authoritative {
		return s.reconcileWeeklyPlanRebooking(ctx, change)
	}
	effectiveOn := s.today()
	if !change.FirstBookinglessDay.IsZero() {
		effectiveOn = change.FirstBookinglessDay
	}
	facts, err := s.listCareBookingFacts(ctx, effectiveOn, []int64{change.StudentID})
	if err != nil {
		return err
	}
	if len(facts) != 1 {
		return fmt.Errorf("care lifecycle: booking facts for student %d not found", change.StudentID)
	}
	if change.WasCompleteWithdrawal {
		facts[0].ConfirmedBookinglessDay = &change.FirstBookinglessDay
	}
	evaluation := domain.EvaluateCareBookingStates(facts, effectiveOn)[0]
	pending, err := s.pendingWithdrawals(ctx, []int64{change.StudentID})
	if err != nil {
		return err
	}
	return s.reconcileBookingEvaluation(ctx, evaluation, change, effectiveOn, pending[change.StudentID])
}

func (s *CareLifecycle) reconcileWeeklyPlanRebooking(ctx context.Context, change careplan.CareWithdrawalBookingChange) error {
	if change.WasCompleteWithdrawal {
		return nil
	}
	pending, err := s.pendingWithdrawals(ctx, []int64{change.StudentID})
	if err != nil {
		return err
	}
	completion := pending[change.StudentID]
	if completion == nil {
		return nil
	}
	firstBookinglessDay := calendar.Date(completion.FirstBookinglessDay)
	facts, err := s.listCareBookingFacts(ctx, firstBookinglessDay, []int64{change.StudentID})
	if err != nil {
		return err
	}
	if len(facts) != 1 {
		return fmt.Errorf("care lifecycle: booking facts for student %d not found", change.StudentID)
	}
	if evaluation := domain.EvaluateCareBookingStates(facts, firstBookinglessDay)[0]; !evaluation.HasCareDays {
		return nil
	}
	_, err = s.records.MarkWithdrawalObsoleteForRebooking(ctx, change.StudentID, completion.FirstBookinglessDay, time.Now())
	return err
}

func (s *CareLifecycle) reconcileBookingEvaluations(ctx context.Context, evaluations []domain.CareBookingEvaluation, on calendar.Date) error {
	studentIDs := make([]int64, len(evaluations))
	for i, evaluation := range evaluations {
		studentIDs[i] = evaluation.StudentID
	}
	pending, err := s.pendingWithdrawals(ctx, studentIDs)
	if err != nil {
		return err
	}
	for _, evaluation := range evaluations {
		change := careplan.CareWithdrawalBookingChange{StudentID: evaluation.StudentID, ConfirmedRole: "system"}
		if err := s.reconcileBookingEvaluation(ctx, evaluation, change, on, pending[evaluation.StudentID]); err != nil {
			return err
		}
	}
	return nil
}

func (s *CareLifecycle) reconcileBookingEvaluation(
	ctx context.Context, evaluation domain.CareBookingEvaluation, change careplan.CareWithdrawalBookingChange,
	obsoleteFrom calendar.Date, pending *careplan.WithdrawalCompletion,
) error {
	if evaluation.FirstBookinglessDay == nil {
		if pending == nil || !evaluation.HasCareDays {
			return nil
		}
		_, err := s.records.MarkWithdrawalObsoleteForRebooking(ctx, evaluation.StudentID, careplan.Date(obsoleteFrom), time.Now())
		return err
	}
	if pending != nil {
		if *evaluation.FirstBookinglessDay != calendar.Date(pending.FirstBookinglessDay) {
			changed, err := s.records.MarkWithdrawalObsoleteForRebooking(ctx, evaluation.StudentID, careplan.Date(obsoleteFrom), time.Now())
			if err != nil {
				return err
			}
			if !changed {
				return errors.New("care lifecycle: pending booking completion changed during reconciliation")
			}
		} else if !change.WasCompleteWithdrawal {
			return nil
		}
	}
	return s.upsertBookingCompletion(ctx, evaluation, change)
}

func (s *CareLifecycle) upsertBookingCompletion(ctx context.Context, evaluation domain.CareBookingEvaluation, change careplan.CareWithdrawalBookingChange) error {
	studentID := change.StudentID
	var actorID *int64
	if change.ConfirmedBy > 0 {
		actorID = &change.ConfirmedBy
	}
	trigger := careplan.WithdrawalTriggerBookingExpired
	if change.WasCompleteWithdrawal {
		trigger = careplan.WithdrawalTriggerDirectSchool
	}
	role := strings.TrimSpace(change.ConfirmedRole)
	if role == "" {
		role = "system"
	}
	sourceChildID := evaluation.SourceRequestChildID
	if sourceChildID == 0 {
		sourceChildID = change.SourceRequestChildID
	}
	sourceOfferings := evaluation.SourceOfferings
	if len(sourceOfferings) == 0 {
		sourceOfferings = change.SourceOfferings
	}
	if sourceOfferings == nil {
		sourceOfferings = []careplan.CareExitSourceOffering{}
	}
	encoded, err := json.Marshal(sourceOfferings)
	if err != nil {
		return fmt.Errorf("care lifecycle: encode care withdrawal source offerings: %w", err)
	}
	_, err = s.records.UpsertPendingWithdrawal(ctx, careplan.WithdrawalCompletion{
		StudentID:               &studentID,
		FirstBookinglessDay:     careplan.Date(*evaluation.FirstBookinglessDay),
		Trigger:                 trigger,
		SourceAdjustmentID:      optionalPositiveID(change.SourceAdjustmentID),
		SourceRequestChildID:    optionalPositiveID(sourceChildID),
		WithdrawalConfirmedBy:   actorID,
		WithdrawalConfirmedRole: role,
		WithdrawalConfirmedAt:   time.Now(),
		SourceOfferings:         encoded,
		State:                   careplan.WithdrawalStatePending,
	})
	if err != nil {
		return fmt.Errorf("care lifecycle: upsert pending care withdrawal completion: %w", err)
	}
	return nil
}

// pendingWithdrawals maps each given child to its pending task.
func (s *CareLifecycle) pendingWithdrawals(ctx context.Context, studentIDs []int64) (map[int64]*careplan.WithdrawalCompletion, error) {
	result := make(map[int64]*careplan.WithdrawalCompletion, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	values, err := s.records.ListPendingWithdrawalsByStudent(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: list pending care withdrawal completions by students: %w", err)
	}
	for studentID, value := range values {
		result[studentID] = &value
	}
	return result, nil
}

// listCareBookingFacts reads the facts the booking evaluation interprets,
// without interpreting them: the given children, or every child in care on
// the day, with their care-counting booking windows.
func (s *CareLifecycle) listCareBookingFacts(ctx context.Context, on calendar.Date, studentIDs []int64) ([]domain.CareBookingFacts, error) {
	students, err := s.directory.ListCareBookingStudents(ctx, careplan.Date(on), studentIDs)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: list current care students for booking evaluation: %w", err)
	}
	if len(students) == 0 {
		return []domain.CareBookingFacts{}, nil
	}
	loadedIDs := make([]int64, 0, len(students))
	for _, student := range students {
		loadedIDs = append(loadedIDs, student.StudentID)
	}
	sources, err := s.loadCareExitSources(ctx, loadedIDs)
	if err != nil {
		return nil, err
	}
	periods := domain.CareBookingPeriods(loadedIDs, sources.applications, sources.offerings, sources.links)
	facts := make([]domain.CareBookingFacts, 0, len(students))
	for _, student := range students {
		facts = append(facts, domain.CareBookingFacts{
			StudentID: student.StudentID, FirstName: student.FirstName, LastName: student.LastName,
			SchoolClass: student.SchoolClass, EnrolledUntil: student.EnrolledUntil,
			Periods: periods[student.StudentID],
		})
	}
	return facts, nil
}

func (s *CareLifecycle) evaluateCareBookings(ctx context.Context, on calendar.Date) ([]domain.CareBookingEvaluation, error) {
	facts, err := s.listCareBookingFacts(ctx, on, nil)
	if err != nil {
		return nil, err
	}
	return domain.EvaluateCareBookingStates(facts, on), nil
}

func optionalPositiveID(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}
