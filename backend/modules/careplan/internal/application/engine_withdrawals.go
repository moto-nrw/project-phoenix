package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The withdrawal commands deliberately do not open a transaction: like the
// retained repository they replace, each statement joins the caller's
// transaction when one is active and otherwise runs on its own (#3221).

func (e Engine) FindWithdrawalCompletion(ctx context.Context, id int64, lock bool) (careplan.WithdrawalCompletion, error) {
	value, err := e.service.FindWithdrawalCompletion(ctx, id, lock)
	return value, MapError(err)
}

func (e Engine) ListPendingWithdrawals(ctx context.Context, filter careplan.WithdrawalListFilter) ([]careplan.WithdrawalCompletion, int, error) {
	values, total, err := ListWithdrawals(ctx, e.service, e.withdrawals, careplan.WithdrawalStatePending, filter)
	return values, total, MapError(err)
}

func (e Engine) ListResolvedWithdrawals(ctx context.Context, filter careplan.WithdrawalListFilter) ([]careplan.WithdrawalCompletion, int, error) {
	values, total, err := ListWithdrawals(ctx, e.service, e.withdrawals, careplan.WithdrawalStateResolved, filter)
	return values, total, MapError(err)
}

func (e Engine) ListPendingWithdrawalsByStudent(ctx context.Context, studentIDs []int64) (map[int64]careplan.WithdrawalCompletion, error) {
	values, err := e.service.ListPendingWithdrawalsByStudent(ctx, studentIDs)
	return values, MapError(err)
}

func (e Engine) ListPendingWithdrawalStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	values, err := e.service.ListPendingWithdrawalStudentIDs(ctx, studentIDs)
	return values, MapError(err)
}

func (e Engine) ListPendingWithdrawalBoundaries(ctx context.Context, studentIDs []int64, includeBookingExpired bool) (map[int64]careplan.Date, error) {
	values, err := e.service.ListPendingWithdrawalBoundaries(ctx, studentIDs, includeBookingExpired)
	return values, MapError(err)
}

func (e Engine) UpsertPendingWithdrawal(ctx context.Context, value careplan.WithdrawalCompletion) (careplan.WithdrawalCompletion, error) {
	stored, err := e.service.UpsertPendingWithdrawal(ctx, value)
	return stored, MapError(err)
}

func (e Engine) ResolveWithdrawal(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	changed, err := e.service.ResolveWithdrawal(ctx, id, actorAccountID, at)
	return changed, MapError(err)
}

func (e Engine) MarkWithdrawalObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn careplan.Date, at time.Time) (bool, error) {
	changed, err := e.service.MarkWithdrawalObsoleteForRebooking(ctx, studentID, careStartsOn, at)
	return changed, MapError(err)
}

func (e Engine) MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error) {
	changed, err := e.service.MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx, at)
	return changed, MapError(err)
}

func (e Engine) ReopenWithdrawalAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, error) {
	changed, err := e.service.ReopenWithdrawalAfterCancelledExit(ctx, completionID, studentID, at)
	return changed, MapError(err)
}
