package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanLegacy "github.com/moto-nrw/project-phoenix/modules/careplan/legacy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// careWithdrawalCompletionRepository preserves the retained users model
// contract over the Care Plan owner, which persists
// users.care_withdrawal_completions (#3221). The list queries take the
// children's names and classes from the People Directory capability. It
// contains no persistence of its own.
type careWithdrawalCompletionRepository struct {
	carePlan func() careplan.Capability
	students func() peopledirectory.StudentQuery
}

var _ userModels.CareWithdrawalCompletionRepository = careWithdrawalCompletionRepository{}

// newCareWithdrawalCompletionRepository adapts the owner capabilities to the
// legacy model contract. Both are read on every call rather than captured, so
// the factory may bind them after construction and a later BindCarePlan or
// BindPeopleDirectory reaches this adapter without rewiring it. An unbound
// capability fails the call in owners(); it is not a construction error.
func newCareWithdrawalCompletionRepository(
	carePlan func() careplan.Capability, students func() peopledirectory.StudentQuery,
) userModels.CareWithdrawalCompletionRepository {
	if carePlan == nil || students == nil {
		panic("care withdrawal repository: Care Plan and People Directory bindings are required")
	}
	return careWithdrawalCompletionRepository{carePlan: carePlan, students: students}
}

func (r careWithdrawalCompletionRepository) owners() (careplan.Capability, peopledirectory.StudentQuery, error) {
	carePlan, students := r.carePlan(), r.students()
	if carePlan == nil || students == nil {
		return nil, nil, errors.New("care withdrawal repository: Care Plan and People Directory capabilities are not bound")
	}
	return carePlan, students, nil
}

func (r careWithdrawalCompletionRepository) owner() (careplan.Capability, error) {
	carePlan, _, err := r.owners()
	return carePlan, err
}

func (r careWithdrawalCompletionRepository) UpsertPending(ctx context.Context, completion *userModels.CareWithdrawalCompletion) error {
	if completion.StudentID == nil || *completion.StudentID <= 0 {
		return errors.New("care withdrawal completion requires a student")
	}
	carePlan, err := r.owner()
	if err != nil {
		return err
	}
	completion.State = userModels.CareWithdrawalStatePending
	if completion.SourceOfferings == nil {
		completion.SourceOfferings = []userModels.CareExitSourceOffering{}
	}
	value, err := withdrawalCompletionToPublic(completion)
	if err != nil {
		return err
	}
	stored, err := carePlan.UpsertPendingWithdrawal(ctx, value)
	if err != nil {
		return fmt.Errorf("upsert pending care withdrawal completion: %w", err)
	}
	completion.ID, completion.TenantID = stored.ID, stored.TenantID
	completion.CreatedAt, completion.UpdatedAt = stored.CreatedAt, stored.UpdatedAt
	return nil
}

// FindByID preserves this contract's nil-on-not-found behavior.
func (r careWithdrawalCompletionRepository) FindByID(ctx context.Context, id int64) (*userModels.CareWithdrawalCompletion, error) {
	return r.find(ctx, id, false)
}

// FindByIDForUpdate locks the task and returns nil for a stale task id so
// concurrent completions can return a stable conflict.
func (r careWithdrawalCompletionRepository) FindByIDForUpdate(ctx context.Context, id int64) (*userModels.CareWithdrawalCompletion, error) {
	return r.find(ctx, id, true)
}

func (r careWithdrawalCompletionRepository) find(ctx context.Context, id int64, lock bool) (*userModels.CareWithdrawalCompletion, error) {
	carePlan, err := r.owner()
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
	return withdrawalCompletionToLegacy(value)
}

func (r careWithdrawalCompletionRepository) ListPending(
	ctx context.Context, filter userModels.CareWithdrawalCompletionFilter,
) ([]*userModels.CareWithdrawalCompletion, int, error) {
	return r.list(ctx, careplan.WithdrawalStatePending, filter, "list pending care withdrawal completions")
}

// ListResolved keeps deleted completions visible without retaining child PII.
func (r careWithdrawalCompletionRepository) ListResolved(
	ctx context.Context, filter userModels.CareWithdrawalCompletionFilter,
) ([]*userModels.CareWithdrawalCompletion, int, error) {
	return r.list(ctx, careplan.WithdrawalStateResolved, filter, "list resolved care withdrawal completions")
}

func (r careWithdrawalCompletionRepository) list(
	ctx context.Context, state string, filter userModels.CareWithdrawalCompletionFilter, operation string,
) ([]*userModels.CareWithdrawalCompletion, int, error) {
	carePlan, students, err := r.owners()
	if err != nil {
		return nil, 0, err
	}
	studentIDs, err := carePlan.ListWithdrawalStudentIDs(ctx, state, filter.StudentID)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", operation, err)
	}
	directory, err := withdrawalStudents(ctx, students, studentIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", operation, err)
	}
	ownerFilter := careplan.WithdrawalListFilter{
		Search: filter.Search, StudentID: filter.StudentID, Page: filter.Page, PageSize: filter.PageSize, Students: directory,
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
	var rows []*userModels.CareWithdrawalCompletion
	for _, value := range values {
		row, convertErr := withdrawalCompletionToLegacy(value)
		if convertErr != nil {
			return nil, 0, fmt.Errorf("%s: %w", operation, convertErr)
		}
		rows = append(rows, row)
	}
	return rows, total, nil
}

// withdrawalStudents resolves the directory rows the list queries join: every
// tenant student with its class, and its name when the person row exists.
func withdrawalStudents(ctx context.Context, students peopledirectory.StudentQuery, ids []int64) ([]careplan.WithdrawalStudent, error) {
	result := []careplan.WithdrawalStudent{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := students.ListStudentsByID(ctx, ids)
	if err != nil || len(rows) == 0 {
		return result, err
	}
	names, err := students.ListStudentNamesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	nameByStudent := make(map[int64]peopledirectory.StudentName, len(names))
	for _, name := range names {
		nameByStudent[name.StudentID] = name
	}
	for _, row := range rows {
		value := careplan.WithdrawalStudent{ID: row.ID, SchoolClass: row.SchoolClass}
		if name, ok := nameByStudent[row.ID]; ok {
			value.FirstName, value.LastName = &name.FirstName, &name.LastName
		}
		result = append(result, value)
	}
	return result, nil
}

func (r careWithdrawalCompletionRepository) ListPendingByStudentIDs(
	ctx context.Context, studentIDs []int64,
) (map[int64]*userModels.CareWithdrawalCompletion, error) {
	result := make(map[int64]*userModels.CareWithdrawalCompletion)
	if len(studentIDs) == 0 {
		return result, nil
	}
	carePlan, err := r.owner()
	if err != nil {
		return nil, err
	}
	values, err := carePlan.ListPendingWithdrawalsByStudent(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("list pending care withdrawal completions by students: %w", err)
	}
	for studentID, value := range values {
		row, convertErr := withdrawalCompletionToLegacy(value)
		if convertErr != nil {
			return nil, fmt.Errorf("list pending care withdrawal completions by students: %w", convertErr)
		}
		result[studentID] = row
	}
	return result, nil
}

// ListParticipationBoundaries returns the earliest day on which each of the
// given tenant students is no longer operationally participating: either the
// day after enrolled_until or a pending completion boundary, whichever comes
// first. Booking-led boundaries count only when includeBookingBoundaries is
// set. The caller passes the student rows it already read, so the enrolment
// end costs no extra statement.
func (r careWithdrawalCompletionRepository) ListParticipationBoundaries(
	ctx context.Context, students map[int64]*userModels.Student, includeBookingBoundaries bool,
) (map[int64]carePlanLegacy.ScheduleDate, error) {
	boundaries := make(map[int64]carePlanLegacy.ScheduleDate, len(students))
	if len(students) == 0 {
		return boundaries, nil
	}
	carePlan, err := r.owner()
	if err != nil {
		return nil, err
	}
	studentIDs := slices.Sorted(maps.Keys(students))
	pending, err := carePlan.ListPendingWithdrawalBoundaries(ctx, studentIDs, includeBookingBoundaries)
	if err != nil {
		return nil, fmt.Errorf("list care participation boundaries: %w", err)
	}
	for _, studentID := range studentIDs {
		var boundary carePlanLegacy.ScheduleDate
		found := false
		if student := students[studentID]; student != nil && student.EnrolledUntil != nil && !student.EnrolledUntil.IsZero() {
			boundary, found = student.EnrolledUntil.AddDays(1), true
		}
		if day, ok := pending[studentID]; ok {
			completion := carePlanLegacy.ScheduleDate(day)
			if !found || completion.Before(boundary) {
				boundary = completion
			}
			found = true
		}
		if found {
			boundaries[studentID] = boundary
		}
	}
	return boundaries, nil
}

func (r careWithdrawalCompletionRepository) ListPendingStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	if len(studentIDs) == 0 {
		return make(map[int64]bool), nil
	}
	carePlan, err := r.owner()
	if err != nil {
		return nil, err
	}
	result, err := carePlan.ListPendingWithdrawalStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("list pending care withdrawal student ids: %w", err)
	}
	return result, nil
}

func (r careWithdrawalCompletionRepository) MarkResolved(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	carePlan, err := r.owner()
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
func (r careWithdrawalCompletionRepository) MarkDeleted(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	carePlan, err := r.owner()
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
func (r careWithdrawalCompletionRepository) MarkObsoleteForRebooking(
	ctx context.Context, studentID int64, careStartsOn carePlanLegacy.ScheduleDate, at time.Time,
) (bool, error) {
	carePlan, err := r.owner()
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
func (r careWithdrawalCompletionRepository) MarkPendingObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error) {
	carePlan, err := r.owner()
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
func (r careWithdrawalCompletionRepository) ReopenAfterCancelledExit(
	ctx context.Context, completionID, studentID int64, at time.Time,
) (bool, error) {
	carePlan, err := r.owner()
	if err != nil {
		return false, err
	}
	changed, err := carePlan.ReopenWithdrawalAfterCancelledExit(ctx, completionID, studentID, at)
	if err != nil {
		return false, fmt.Errorf("reopen care withdrawal after cancelled exit: %w", err)
	}
	return changed, nil
}

func withdrawalCompletionToPublic(completion *userModels.CareWithdrawalCompletion) (careplan.WithdrawalCompletion, error) {
	offerings, err := json.Marshal(completion.SourceOfferings)
	if err != nil {
		return careplan.WithdrawalCompletion{}, fmt.Errorf("encode care withdrawal source offerings: %w", err)
	}
	return careplan.WithdrawalCompletion{
		ID: completion.ID, TenantID: completion.TenantID, CreatedAt: completion.CreatedAt, UpdatedAt: completion.UpdatedAt,
		StudentID: completion.StudentID, FirstBookinglessDay: careplan.Date(completion.FirstBookinglessDay),
		Trigger: completion.Trigger, SourceAdjustmentID: completion.SourceAdjustmentID,
		SourceRequestChildID: completion.SourceRequestChildID, WithdrawalConfirmedBy: completion.WithdrawalConfirmedBy,
		WithdrawalConfirmedRole: completion.WithdrawalConfirmedRole, WithdrawalConfirmedAt: completion.WithdrawalConfirmedAt,
		SourceOfferings: offerings, State: completion.State, Outcome: completion.Outcome,
		ObsoleteReason: completion.ObsoleteReason, ResolvedBy: completion.ResolvedBy, ResolvedAt: completion.ResolvedAt,
	}, nil
}

func withdrawalCompletionToLegacy(value careplan.WithdrawalCompletion) (*userModels.CareWithdrawalCompletion, error) {
	row := &userModels.CareWithdrawalCompletion{
		StudentID: value.StudentID, FirstBookinglessDay: carePlanLegacy.ScheduleDate(value.FirstBookinglessDay),
		Trigger: value.Trigger, SourceAdjustmentID: value.SourceAdjustmentID,
		SourceRequestChildID: value.SourceRequestChildID, WithdrawalConfirmedBy: value.WithdrawalConfirmedBy,
		WithdrawalConfirmedRole: value.WithdrawalConfirmedRole, WithdrawalConfirmedAt: value.WithdrawalConfirmedAt,
		State: value.State, Outcome: value.Outcome, ObsoleteReason: value.ObsoleteReason,
		ResolvedBy: value.ResolvedBy, ResolvedAt: value.ResolvedAt,
		FirstName: value.FirstName, LastName: value.LastName, SchoolClass: value.SchoolClass,
	}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	if len(value.SourceOfferings) > 0 {
		if err := json.Unmarshal(value.SourceOfferings, &row.SourceOfferings); err != nil {
			return nil, fmt.Errorf("decode care withdrawal source offerings: %w", err)
		}
	}
	return row, nil
}
