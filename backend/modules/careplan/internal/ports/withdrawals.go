package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// WithdrawalStore persists users.care_withdrawal_completions (#3221).
type WithdrawalStore interface {
	FindWithdrawalCompletion(ctx context.Context, id int64, lock bool) (domain.WithdrawalCompletion, bool, domain.OperationStats, error)
	ListWithdrawalStudentIDs(ctx context.Context, state string, studentID int64) ([]int64, domain.OperationStats, error)
	ListWithdrawals(ctx context.Context, state string, filter domain.WithdrawalListFilter) ([]domain.WithdrawalCompletion, int, domain.OperationStats, error)
	ListPendingWithdrawalsByStudent(ctx context.Context, studentIDs []int64) (map[int64]domain.WithdrawalCompletion, domain.OperationStats, error)
	ListPendingWithdrawalStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, domain.OperationStats, error)
	ListPendingWithdrawalBoundaries(ctx context.Context, studentIDs []int64, includeBookingExpired bool) (map[int64]domain.Date, domain.OperationStats, error)
	UpsertPendingWithdrawal(ctx context.Context, value domain.WithdrawalCompletion) (domain.WithdrawalCompletion, domain.OperationStats, error)
	ResolveWithdrawal(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, domain.OperationStats, error)
	MarkWithdrawalObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn domain.Date, at time.Time) (bool, domain.OperationStats, error)
	MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, domain.OperationStats, error)
	ReopenWithdrawalAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, domain.OperationStats, error)
}
