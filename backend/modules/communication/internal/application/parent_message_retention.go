package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type retentionCounts interface{ CleanupCounts() (int, int, int) }
type parentRetention[R retentionCounts] interface {
	CleanupExpiredMessages(context.Context) (R, error)
}
type parentMessageRetention[R retentionCounts] struct{ retention parentRetention[R] }

func NewParentMessageRetention[R retentionCounts](retention parentRetention[R]) communication.ParentMessageCleanup {
	return parentMessageRetention[R]{retention: retention}
}
func (r parentMessageRetention[R]) CleanupExpiredParentMessages(ctx context.Context) (communication.ParentMessageCleanupResult, error) {
	result, err := r.retention.CleanupExpiredMessages(ctx)
	deleted, threads, days := result.CleanupCounts()
	return communication.ParentMessageCleanupResult{MessagesDeleted: deleted, ThreadsDeleted: threads, RetentionDays: days}, err
}
