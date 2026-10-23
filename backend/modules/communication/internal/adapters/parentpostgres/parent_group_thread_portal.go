package parentpostgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// GroupThreadPortal is the guardian side of the group threads in the public
// vocabulary the parent portal workflow binds.
type GroupThreadPortal struct{ Store *GroupThreadStore }

var _ communication.PortalGroupThreads = GroupThreadPortal{}

func (p GroupThreadPortal) PortalListGroupThreads(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) ([]communication.PortalGuardianGroupThread, error) {
	return p.Store.ListThreadsForGuardian(ctx, accountID, studentIDs, tenantIDs)
}

func (p GroupThreadPortal) PortalCountUnreadGroupThreads(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) (int, error) {
	return p.Store.CountUnreadThreadsForGuardian(ctx, accountID, studentIDs, tenantIDs)
}

// PortalReadGroupThread returns the messages that reached the guardian and
// advances the guardian's cursor to the newest of them. It reports
// ErrParentGroupThreadNotFound when nothing of the thread reached them, so a
// guardian cannot probe for threads of other groups.
func (p GroupThreadPortal) PortalReadGroupThread(ctx context.Context, threadID, accountID int64, studentIDs []int64) (*communication.PortalGuardianGroupThreadView, bool, error) {
	thread, err := p.Store.FindThread(ctx, threadID, false)
	if err != nil {
		return nil, false, err
	}
	if thread == nil {
		return nil, false, communication.ErrParentGroupThreadNotFound
	}
	messages, children, err := p.Store.ListMessagesForGuardian(ctx, threadID, accountID, studentIDs)
	if err != nil {
		return nil, false, err
	}
	if len(messages) == 0 {
		return nil, false, communication.ErrParentGroupThreadNotFound
	}
	advanced, err := p.Store.MarkGuardianRead(ctx, threadID, accountID, messages[len(messages)-1].ID)
	if err != nil {
		return nil, false, err
	}
	return &communication.PortalGuardianGroupThreadView{Thread: *thread, Messages: messages, StudentIDs: children}, advanced, nil
}

// PortalGroupReplyTarget returns the thread a guardian answers for a child:
// it must exist, have reached the guardian for that child and be open.
func (p GroupThreadPortal) PortalGroupReplyTarget(ctx context.Context, threadID, accountID, studentID int64) (*communication.ParentGroupThread, error) {
	thread, err := p.Store.FindThread(ctx, threadID, false)
	if err != nil {
		return nil, err
	}
	if thread == nil {
		return nil, communication.ErrParentGroupThreadNotFound
	}
	recipient, err := p.Store.IsRecipient(ctx, threadID, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if !recipient {
		return nil, communication.ErrParentGroupThreadNotFound
	}
	if thread.ClosedAt != nil {
		return nil, communication.ErrParentGroupThreadClosed
	}
	return thread, nil
}

func (p GroupThreadPortal) PortalGroupThreadTitles(ctx context.Context, threadIDs []int64) (map[int64]string, error) {
	return p.Store.TitlesByIDs(ctx, threadIDs)
}
