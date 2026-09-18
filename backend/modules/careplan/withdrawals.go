package careplan

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Withdrawal completion vocabulary. users.care_withdrawal_completions belongs
// to Care Plan; the values mirror the table's CHECK constraints.
const (
	WithdrawalStatePending  = "pending"
	WithdrawalStateResolved = "resolved"
	WithdrawalStateObsolete = "obsolete"

	WithdrawalTriggerDirectSchool   = "direct_school"
	WithdrawalTriggerBookingExpired = "booking_expired"

	WithdrawalOutcomeCareEnded = "care_ended"
	WithdrawalOutcomeDeleted   = "deleted"

	WithdrawalObsoleteRebooked    = "rebooked_without_gap"
	WithdrawalObsoleteWeeklyPlans = "weekly_plan_mode"
)

var ErrInvalidWithdrawal = errors.New("invalid care withdrawal completion")

// WithdrawalCompletion is the owner view of one withdrawal task. The name and
// class fields are filled only by the list queries, from the student rows the
// caller supplied. SourceOfferings stays raw JSON so the capability does not
// leak a model package's types.
type WithdrawalCompletion struct {
	ID                      int64
	TenantID                int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
	StudentID               *int64
	FirstBookinglessDay     Date
	Trigger                 string
	SourceAdjustmentID      *int64
	SourceRequestChildID    *int64
	WithdrawalConfirmedBy   *int64
	WithdrawalConfirmedRole string
	WithdrawalConfirmedAt   time.Time
	SourceOfferings         json.RawMessage
	State                   string
	Outcome                 *string
	ObsoleteReason          *string
	ResolvedBy              *int64
	ResolvedAt              *time.Time

	FirstName   string
	LastName    string
	SchoolClass string
}

// WithdrawalStudent is the People Directory display row a withdrawal list
// joins. A nil name means the student has no person row.
type WithdrawalStudent struct {
	ID          int64   `json:"id"`
	FirstName   *string `json:"first_name"`
	LastName    *string `json:"last_name"`
	SchoolClass string  `json:"school_class"`
}

// WithdrawalListFilter selects one page of withdrawal tasks. Students are the
// directory rows of every child the tasks may name; the search matches their
// names and class.
type WithdrawalListFilter struct {
	Search    string
	StudentID int64
	Page      int
	PageSize  int
	Students  []WithdrawalStudent
}

// WithdrawalCompletionKey is the (child, first bookingless day) pair of one
// stored task in any state.
type WithdrawalCompletionKey struct {
	StudentID           int64
	FirstBookinglessDay Date
}

type WithdrawalQuery interface {
	// FindWithdrawalCompletion reads one task, optionally locking it FOR
	// UPDATE. ErrWithdrawalNotFound for an unknown task.
	FindWithdrawalCompletion(ctx context.Context, id int64, lock bool) (WithdrawalCompletion, error)
	// ListWithdrawalStudentIDs returns the distinct children named by tasks
	// in state, restricted to studentID when it is positive.
	ListWithdrawalStudentIDs(ctx context.Context, state string, studentID int64) ([]int64, error)
	// ListPendingWithdrawals pages the pending tasks whose child is among
	// filter.Students, ordered by first bookingless day and id.
	ListPendingWithdrawals(ctx context.Context, filter WithdrawalListFilter) ([]WithdrawalCompletion, int, error)
	// ListResolvedWithdrawals pages the resolved tasks, newest first. Tasks
	// whose child is not among filter.Students keep empty names.
	ListResolvedWithdrawals(ctx context.Context, filter WithdrawalListFilter) ([]WithdrawalCompletion, int, error)
	// ListPendingWithdrawalsByStudent maps each child to its pending task.
	ListPendingWithdrawalsByStudent(ctx context.Context, studentIDs []int64) (map[int64]WithdrawalCompletion, error)
	// ListPendingWithdrawalStudentIDs marks the children with a pending task.
	ListPendingWithdrawalStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
	// ListPendingWithdrawalBoundaries maps each child to the first
	// bookingless day of its pending task. Booking-expiry tasks count only
	// when includeBookingExpired is set.
	ListPendingWithdrawalBoundaries(ctx context.Context, studentIDs []int64, includeBookingExpired bool) (map[int64]Date, error)
	// ListWithdrawalCompletionKeys returns the key of every task, in any
	// state, that names one of the children.
	ListWithdrawalCompletionKeys(ctx context.Context, studentIDs []int64) ([]WithdrawalCompletionKey, error)
}

// WithdrawalCommand changes withdrawal tasks. Every command joins the
// caller's transaction when one is active.
type WithdrawalCommand interface {
	// UpsertPendingWithdrawal keeps one pending task per child. A school
	// confirmation survives a conflicting booking expiry; the boundary always
	// follows the incoming value. It returns the stored id and timestamps.
	UpsertPendingWithdrawal(ctx context.Context, value WithdrawalCompletion) (WithdrawalCompletion, error)
	// ResolveWithdrawal closes a pending task with the care-ended outcome;
	// false means it was no longer pending.
	ResolveWithdrawal(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error)
	// MarkWithdrawalObsoleteForRebooking closes the child's pending task when
	// care starts again on or before its first bookingless day.
	MarkWithdrawalObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn Date, at time.Time) (bool, error)
	// MarkPendingWithdrawalsObsoleteForWeeklyPlans closes every pending
	// booking-expiry task of the tenant and returns how many it closed.
	MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error)
	// ReopenWithdrawalAfterCancelledExit copies one resolved care-ended task
	// of the child into a new pending task.
	ReopenWithdrawalAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, error)
}

func (m *Module) FindWithdrawalCompletion(ctx context.Context, id int64, lock bool) (WithdrawalCompletion, error) {
	return m.engine.FindWithdrawalCompletion(ctx, id, lock)
}

func (m *Module) ListWithdrawalStudentIDs(ctx context.Context, state string, studentID int64) ([]int64, error) {
	return m.engine.ListWithdrawalStudentIDs(ctx, state, studentID)
}

func (m *Module) ListPendingWithdrawals(ctx context.Context, filter WithdrawalListFilter) ([]WithdrawalCompletion, int, error) {
	return m.engine.ListPendingWithdrawals(ctx, filter)
}

func (m *Module) ListResolvedWithdrawals(ctx context.Context, filter WithdrawalListFilter) ([]WithdrawalCompletion, int, error) {
	return m.engine.ListResolvedWithdrawals(ctx, filter)
}

func (m *Module) ListPendingWithdrawalsByStudent(ctx context.Context, studentIDs []int64) (map[int64]WithdrawalCompletion, error) {
	if len(studentIDs) == 0 {
		return map[int64]WithdrawalCompletion{}, nil
	}
	return m.engine.ListPendingWithdrawalsByStudent(ctx, studentIDs)
}

func (m *Module) ListPendingWithdrawalStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	if len(studentIDs) == 0 {
		return map[int64]bool{}, nil
	}
	return m.engine.ListPendingWithdrawalStudentIDs(ctx, studentIDs)
}

func (m *Module) ListPendingWithdrawalBoundaries(ctx context.Context, studentIDs []int64, includeBookingExpired bool) (map[int64]Date, error) {
	if len(studentIDs) == 0 {
		return map[int64]Date{}, nil
	}
	return m.engine.ListPendingWithdrawalBoundaries(ctx, studentIDs, includeBookingExpired)
}

func (m *Module) ListWithdrawalCompletionKeys(ctx context.Context, studentIDs []int64) ([]WithdrawalCompletionKey, error) {
	if len(studentIDs) == 0 {
		return []WithdrawalCompletionKey{}, nil
	}
	return m.engine.ListWithdrawalCompletionKeys(ctx, studentIDs)
}

func (m *Module) UpsertPendingWithdrawal(ctx context.Context, value WithdrawalCompletion) (WithdrawalCompletion, error) {
	if value.StudentID == nil || *value.StudentID <= 0 {
		return WithdrawalCompletion{}, invalid(ErrInvalidWithdrawal, "care withdrawal completion requires a student")
	}
	return m.engine.UpsertPendingWithdrawal(ctx, value)
}

func (m *Module) ResolveWithdrawal(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	return m.engine.ResolveWithdrawal(ctx, id, actorAccountID, at)
}

func (m *Module) MarkWithdrawalObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn Date, at time.Time) (bool, error) {
	return m.engine.MarkWithdrawalObsoleteForRebooking(ctx, studentID, careStartsOn, at)
}

func (m *Module) MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error) {
	return m.engine.MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx, at)
}

func (m *Module) ReopenWithdrawalAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, error) {
	return m.engine.ReopenWithdrawalAfterCancelledExit(ctx, completionID, studentID, at)
}
