//go:build testhooks

package owners

import (
	"context"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	owner0 "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// CareWithdrawalTestRows are the care-withdrawal completions the suites read
// back and resolve over the Care Plan owner (NewCareWithdrawalCompletionRepository).
type CareWithdrawalTestRows interface {
	FindByID(ctx context.Context, id int64) (*careplan.WithdrawalCompletion, error)
	ListPending(ctx context.Context, filter careplan.WithdrawalListFilter) ([]*careplan.WithdrawalCompletion, int, error)
	ListResolved(ctx context.Context, filter careplan.WithdrawalListFilter) ([]*careplan.WithdrawalCompletion, int, error)
	MarkResolved(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error)
	MarkObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn owner0.Date, at time.Time) (bool, error)
	MarkPendingObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error)
	ReopenAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, error)
	ListParticipationBoundaries(
		ctx context.Context, students map[int64]*peopleValues.StudentState, includeBookingBoundaries bool,
	) (map[int64]owner0.Date, error)
}
