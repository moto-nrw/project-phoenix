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
// it must have reached the guardian for that child. Closed threads accept
// only an already stored send; AppendMessage still checks its exact payload.
func (p GroupThreadPortal) PortalGroupReplyTarget(ctx context.Context, threadID, accountID, studentID int64, sendIDs ...string) (*communication.ParentGroupThread, error) {
	// Keep an open thread locked until the caller either appends the guardian
	// reply or rolls the transaction back. This makes the final authorization
	// check and a staff-side close mutually exclusive.
	thread, err := p.Store.FindThread(ctx, threadID, true)
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
		if len(sendIDs) == 0 || sendIDs[0] == "" {
			return nil, communication.ErrParentGroupThreadClosed
		}
		replay, err := p.Store.hasGuardianReplySend(ctx, threadID, accountID, studentID, sendIDs[0])
		if err != nil {
			return nil, err
		}
		if !replay {
			return nil, communication.ErrParentGroupThreadClosed
		}
	}
	return thread, nil
}

func (p GroupThreadPortal) PortalGroupThreadTitles(ctx context.Context, threadIDs []int64) (map[int64]string, error) {
	return p.Store.TitlesByIDs(ctx, threadIDs)
}

// The tenant and author namespace is the same one enforced by the send index.
func (s *GroupThreadStore) hasGuardianReplySend(ctx context.Context, threadID, accountID, studentID int64, key string) (bool, error) {
	db, tenantID, err := s.tenantDB(ctx, "find guardian group reply send")
	if err != nil {
		return false, err
	}
	return db.NewSelect().Model((*parentMessageRow)(nil)).
		Where("parent_message.tenant_id = ? AND parent_message.sender_account_id = ? AND parent_message.client_message_id = ?", tenantID, accountID, key).
		Where("parent_message.student_id = ? AND parent_message.group_thread_id = ? AND parent_message.sender_kind = ?", studentID, threadID, "guardian").Exists(ctx)
}
