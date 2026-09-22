package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The withdrawal commands deliberately do not open a transaction: like the
// retained repository they replace, each statement joins the caller's
// transaction when one is active and otherwise runs on its own (#3221).

func (e engine) FindWithdrawalCompletion(ctx context.Context, id int64, lock bool) (careplan.WithdrawalCompletion, error) {
	value, err := e.service.FindWithdrawalCompletion(ctx, id, lock)
	return value, mapError(err)
}

func (e engine) ListWithdrawalStudentIDs(ctx context.Context, state string, studentID int64) ([]int64, error) {
	values, err := e.service.ListWithdrawalStudentIDs(ctx, state, studentID)
	return values, mapError(err)
}

func (e engine) ListPendingWithdrawals(ctx context.Context, filter careplan.WithdrawalListFilter) ([]careplan.WithdrawalCompletion, int, error) {
	values, total, err := e.service.ListWithdrawals(ctx, careplan.WithdrawalStatePending, filter)
	return values, total, mapError(err)
}

func (e engine) ListResolvedWithdrawals(ctx context.Context, filter careplan.WithdrawalListFilter) ([]careplan.WithdrawalCompletion, int, error) {
	values, total, err := e.service.ListWithdrawals(ctx, careplan.WithdrawalStateResolved, filter)
	return values, total, mapError(err)
}

func (e engine) ListPendingWithdrawalsByStudent(ctx context.Context, studentIDs []int64) (map[int64]careplan.WithdrawalCompletion, error) {
	values, err := e.service.ListPendingWithdrawalsByStudent(ctx, studentIDs)
	return values, mapError(err)
}

func (e engine) ListPendingWithdrawalStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	values, err := e.service.ListPendingWithdrawalStudentIDs(ctx, studentIDs)
	return values, mapError(err)
}

func (e engine) ListPendingWithdrawalBoundaries(ctx context.Context, studentIDs []int64, includeBookingExpired bool) (map[int64]careplan.Date, error) {
	values, err := e.service.ListPendingWithdrawalBoundaries(ctx, studentIDs, includeBookingExpired)
	return values, mapError(err)
}

func (e engine) UpsertPendingWithdrawal(ctx context.Context, value careplan.WithdrawalCompletion) (careplan.WithdrawalCompletion, error) {
	stored, err := e.service.UpsertPendingWithdrawal(ctx, value)
	return stored, mapError(err)
}

func (e engine) ResolveWithdrawal(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	changed, err := e.service.ResolveWithdrawal(ctx, id, actorAccountID, at)
	return changed, mapError(err)
}

func (e engine) MarkWithdrawalObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn careplan.Date, at time.Time) (bool, error) {
	changed, err := e.service.MarkWithdrawalObsoleteForRebooking(ctx, studentID, careStartsOn, at)
	return changed, mapError(err)
}

func (e engine) MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error) {
	changed, err := e.service.MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx, at)
	return changed, mapError(err)
}

func (e engine) ReopenWithdrawalAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, error) {
	changed, err := e.service.ReopenWithdrawalAfterCancelledExit(ctx, completionID, studentID, at)
	return changed, mapError(err)
}
