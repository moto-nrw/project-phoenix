package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

type MessageInboxReads interface{ messaging.MessageInboxReads }
type MessageInboxReadsCallbacks struct {
	ListThreadsForGuardianStudent        func(ctx context.Context, accountID int64, studentID int64) ([]*InboxThread, error)
	ListThreadsForGuardianTenants        func(ctx context.Context, accountID int64, tenantIDs []int64) ([]*InboxThread, error)
	UnreadMessageCountForGuardianTenants func(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
}

func NewMessageInboxReads(c MessageInboxReadsCallbacks) MessageInboxReads {
	return messageInboxReadsBinding{c}
}

type messageInboxReadsBinding struct{ callbacks MessageInboxReadsCallbacks }

func (p messageInboxReadsBinding) ListThreadsForGuardianStudent(ctx context.Context, accountID int64, studentID int64) ([]*messaging.InboxThread, error) {
	return p.callbacks.ListThreadsForGuardianStudent(ctx, accountID, studentID)
}
func (p messageInboxReadsBinding) ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*messaging.InboxThread, error) {
	return p.callbacks.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
}
func (p messageInboxReadsBinding) UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	return p.callbacks.UnreadMessageCountForGuardianTenants(ctx, accountID, tenantIDs)
}
