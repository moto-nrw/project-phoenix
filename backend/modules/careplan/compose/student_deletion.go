package compose

import (
	"context"
	"time"
)

func (e engine) FindPendingWithdrawalStudent(ctx context.Context, completionID int64, lock bool) (int64, error) {
	value, err := e.service.FindPendingWithdrawalStudent(ctx, completionID, lock)
	return value, mapError(err)
}

func (e engine) ResolvePendingWithdrawalAsDeleted(ctx context.Context, completionID, actorAccountID int64, at time.Time) (bool, error) {
	value, err := e.service.ResolvePendingWithdrawalAsDeleted(ctx, completionID, actorAccountID, at)
	return value, mapError(err)
}

func (e engine) RedactWithdrawalsForDeletedStudent(ctx context.Context, studentID, actorAccountID int64, at time.Time) (int, error) {
	value, err := e.service.RedactWithdrawalsForDeletedStudent(ctx, studentID, actorAccountID, at)
	return value, mapError(err)
}

func (e engine) QueueCareDocumentCleanupForDeletedStudent(ctx context.Context, studentID int64, at time.Time) (int, error) {
	value, err := e.service.QueueCareDocumentCleanupForDeletedStudent(ctx, studentID, at)
	return value, mapError(err)
}
