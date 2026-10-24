package owners

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	owner0 "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// CareWithdrawalRecords preserves the retained users model
// contract over the Care Plan owner, which persists
// users.care_withdrawal_completions (#3221). The owner's list queries join
// the children's names and classes from the named student directory
// projection. It contains no persistence of its own.
type CareWithdrawalRecords struct {
	CarePlan careplan.Capability
}

// NewCareWithdrawalCompletionRepository adapts the Care Plan owner to the
// retained care withdrawal completion contract. The owner is required.
func NewCareWithdrawalCompletionRepository(carePlan careplan.Capability) CareWithdrawalRecords {
	if carePlan == nil {
		panic("care withdrawal repository: Care Plan binding is required")
	}
	return CareWithdrawalRecords{CarePlan: carePlan}
}

func (r CareWithdrawalRecords) Owner() (careplan.Capability, error) {
	return r.CarePlan, nil
}

// FindByID preserves this contract's nil-on-not-found behavior.
func (r CareWithdrawalRecords) FindByID(ctx context.Context, id int64) (*careplan.WithdrawalCompletion, error) {
	return r.find(ctx, id, false)
}

// FindByIDForUpdate locks the task and returns nil for a stale task id so
// concurrent completions can return a stable conflict.
func (r CareWithdrawalRecords) FindByIDForUpdate(ctx context.Context, id int64) (*careplan.WithdrawalCompletion, error) {
	return r.find(ctx, id, true)
}

func (r CareWithdrawalRecords) find(ctx context.Context, id int64, lock bool) (*careplan.WithdrawalCompletion, error) {
	carePlan, err := r.Owner()
	if err != nil {
		return nil, err
	}
	value, err := carePlan.FindWithdrawalCompletion(ctx, id, lock)
	if errors.Is(err, careplan.ErrWithdrawalNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find care withdrawal completion: %w", err)
	}
	return nativeWithdrawalCompletion(value)
}

func (r CareWithdrawalRecords) ListPending(
	ctx context.Context, filter careplan.WithdrawalListFilter,
) ([]*careplan.WithdrawalCompletion, int, error) {
	return r.list(ctx, careplan.WithdrawalStatePending, filter, "list pending care withdrawal completions")
}

// ListResolved keeps deleted completions visible without retaining child PII.
func (r CareWithdrawalRecords) ListResolved(
	ctx context.Context, filter careplan.WithdrawalListFilter,
) ([]*careplan.WithdrawalCompletion, int, error) {
	return r.list(ctx, careplan.WithdrawalStateResolved, filter, "list resolved care withdrawal completions")
}

func (r CareWithdrawalRecords) list(
	ctx context.Context, state string, filter careplan.WithdrawalListFilter, operation string,
) ([]*careplan.WithdrawalCompletion, int, error) {
	carePlan, err := r.Owner()
	if err != nil {
		return nil, 0, err
	}
	ownerFilter := careplan.WithdrawalListFilter{
		Search: filter.Search, StudentID: filter.StudentID, Page: filter.Page, PageSize: filter.PageSize,
	}
	var values []careplan.WithdrawalCompletion
	var total int
	if state == careplan.WithdrawalStatePending {
		values, total, err = carePlan.ListPendingWithdrawals(ctx, ownerFilter)
	} else {
		values, total, err = carePlan.ListResolvedWithdrawals(ctx, ownerFilter)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", operation, err)
	}
	var rows []*careplan.WithdrawalCompletion
	for _, value := range values {
		row, convertErr := nativeWithdrawalCompletion(value)
		if convertErr != nil {
			return nil, 0, fmt.Errorf("%s: %w", operation, convertErr)
		}
		rows = append(rows, row)
	}
	return rows, total, nil
}

func (r CareWithdrawalRecords) ListPendingByStudentIDs(
	ctx context.Context, studentIDs []int64,
) (map[int64]*careplan.WithdrawalCompletion, error) {
	result := make(map[int64]*careplan.WithdrawalCompletion)
	if len(studentIDs) == 0 {
		return result, nil
	}
	carePlan, err := r.Owner()
	if err != nil {
		return nil, err
	}
	values, err := carePlan.ListPendingWithdrawalsByStudent(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("list pending care withdrawal completions by students: %w", err)
	}
	for studentID, value := range values {
		row, convertErr := nativeWithdrawalCompletion(value)
		if convertErr != nil {
			return nil, fmt.Errorf("list pending care withdrawal completions by students: %w", convertErr)
		}
		result[studentID] = row
	}
	return result, nil
}

// ListParticipationBoundaries returns the day each of the given tenant
// students stops participating, as Care Plan defines it. Booking-led boundaries count only when includeBookingBoundaries is
// set. The caller passes the student rows it already read, so the enrolment
// end costs no extra statement.
func (r CareWithdrawalRecords) ListParticipationBoundaries(
	ctx context.Context, students map[int64]*peopleValues.StudentState, includeBookingBoundaries bool,
) (map[int64]owner0.Date, error) {
	boundaries := make(map[int64]owner0.Date, len(students))
	if len(students) == 0 {
		return boundaries, nil
	}
	carePlan, err := r.Owner()
	if err != nil {
		return nil, err
	}
	studentIDs := slices.Sorted(maps.Keys(students))
	pending, err := carePlan.ListPendingWithdrawalBoundaries(ctx, studentIDs, includeBookingBoundaries)
	if err != nil {
		return nil, fmt.Errorf("list care participation boundaries: %w", err)
	}
	for _, studentID := range studentIDs {
		var enrolledUntil owner0.Date
		if student := students[studentID]; student != nil && student.EnrolledUntil != nil {
			enrolledUntil = *student.EnrolledUntil
		}
		var completion *careplan.Date
		if day, ok := pending[studentID]; ok {
			completion = &day
		}
		if boundary, found := careplan.ParticipationBoundary(enrolledUntil, completion); found {
			boundaries[studentID] = boundary
		}
	}
	return boundaries, nil
}

func (r CareWithdrawalRecords) ListPendingStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	if len(studentIDs) == 0 {
		return make(map[int64]bool), nil
	}
	carePlan, err := r.Owner()
	if err != nil {
		return nil, err
	}
	result, err := carePlan.ListPendingWithdrawalStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("list pending care withdrawal student ids: %w", err)
	}
	return result, nil
}

func (r CareWithdrawalRecords) MarkResolved(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	carePlan, err := r.Owner()
	if err != nil {
		return false, err
	}
	changed, err := carePlan.ResolveWithdrawal(ctx, id, actorAccountID, at)
	if err != nil {
		return false, fmt.Errorf("resolve care withdrawal completion: %w", err)
	}
	return changed, nil
}

// MarkDeleted resolves and redacts the completion before the student cascade.
// Its caller owns the surrounding transaction, so a failed deletion restores
// the pending task as well.
func (r CareWithdrawalRecords) MarkDeleted(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	carePlan, err := r.Owner()
	if err != nil {
		return false, err
	}
	changed, err := carePlan.ResolvePendingWithdrawalAsDeleted(ctx, id, actorAccountID, at)
	if err != nil {
		return false, fmt.Errorf("resolve and redact deleted care withdrawal completion: %w", err)
	}
	return changed, nil
}

// MarkObsoleteForRebooking atomically applies the no-gap domain predicate.
func (r CareWithdrawalRecords) MarkObsoleteForRebooking(
	ctx context.Context, studentID int64, careStartsOn owner0.Date, at time.Time,
) (bool, error) {
	carePlan, err := r.Owner()
	if err != nil {
		return false, err
	}
	changed, err := carePlan.MarkWithdrawalObsoleteForRebooking(ctx, studentID, careplan.Date(careStartsOn), at)
	if err != nil {
		return false, fmt.Errorf("obsolete care withdrawal completion after rebooking: %w", err)
	}
	return changed, nil
}

// MarkPendingObsoleteForWeeklyPlans closes booking-derived tasks when the
// school switches back to weekly-plan-driven care. Confirmed withdrawals stay.
func (r CareWithdrawalRecords) MarkPendingObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error) {
	carePlan, err := r.Owner()
	if err != nil {
		return 0, err
	}
	changed, err := carePlan.MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx, at)
	if err != nil {
		return 0, fmt.Errorf("obsolete pending care withdrawal completions for weekly plans: %w", err)
	}
	return changed, nil
}

// ReopenAfterCancelledExit copies one exact resolved event into a new event;
// the historical outcome is never mutated in place.
func (r CareWithdrawalRecords) ReopenAfterCancelledExit(
	ctx context.Context, completionID, studentID int64, at time.Time,
) (bool, error) {
	carePlan, err := r.Owner()
	if err != nil {
		return false, err
	}
	changed, err := carePlan.ReopenWithdrawalAfterCancelledExit(ctx, completionID, studentID, at)
	if err != nil {
		return false, fmt.Errorf("reopen care withdrawal after cancelled exit: %w", err)
	}
	return changed, nil
}

func nativeWithdrawalCompletion(value careplan.WithdrawalCompletion) (*careplan.WithdrawalCompletion, error) {
	return &value, nil
}
