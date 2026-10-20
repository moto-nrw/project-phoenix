package careplan

import (
	"context"
	"errors"
	"time"
)

var (
	ErrWithdrawalNotFound   = errors.New("care withdrawal completion not found")
	ErrWithdrawalNotPending = errors.New("care withdrawal completion is not pending")
)

// StudentDeletionCommand is the Care Plan half of a permanent child deletion
// (#2710). Every command joins the caller's tenant transaction; the
// coordinating workflow holds the student locks and decides the order.
type StudentDeletionCommand interface {
	// FindPendingWithdrawalStudent resolves the child of a pending withdrawal
	// task, optionally locking the task row. ErrWithdrawalNotFound for an
	// unknown task, ErrWithdrawalNotPending once it is resolved or redacted.
	FindPendingWithdrawalStudent(ctx context.Context, completionID int64, lock bool) (int64, error)
	// ResolvePendingWithdrawalAsDeleted closes the task with the deleted
	// outcome; false means it was no longer pending.
	ResolvePendingWithdrawalAsDeleted(ctx context.Context, completionID, actorAccountID int64, at time.Time) (bool, error)
	// RedactWithdrawalsForDeletedStudent removes the child from every task
	// still naming them, resolving pending ones as deleted.
	RedactWithdrawalsForDeletedStudent(ctx context.Context, studentID, actorAccountID int64, at time.Time) (int, error)
	// QueueCareDocumentCleanupForDeletedStudent records an immediately
	// eligible cleanup intent for every stored document of the child and
	// returns how many it queued.
	QueueCareDocumentCleanupForDeletedStudent(ctx context.Context, studentID int64, at time.Time) (int, error)
}
