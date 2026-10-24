package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// BindParentGroupThreads binds the parent portal's group thread port to
// Communication's guardian side of the group threads (#4784). Every method
// maps Communication's group thread errors onto the workflow's.
func BindParentGroupThreads(source communication.PortalGroupThreads) parentportal.GroupThreads {
	if source == nil {
		return nil
	}
	return parentGroupThreads{source: source}
}

type parentGroupThreads struct {
	source communication.PortalGroupThreads
}

func (p parentGroupThreads) ListGroupThreads(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) ([]parentportal.GroupThreadSummary, error) {
	rows, err := p.source.PortalListGroupThreads(ctx, accountID, studentIDs, tenantIDs)
	if err != nil {
		return nil, parentGroupThreadError(err)
	}
	out := make([]parentportal.GroupThreadSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, parentportal.GroupThreadSummary{
			Thread: parentGroupThread(row.Thread), LastMessageBody: row.LastMessageBody,
			LastMessageAt: row.LastMessageAt, UnreadCount: row.UnreadCount, StudentIDs: row.StudentIDs,
		})
	}
	return out, nil
}

func (p parentGroupThreads) CountUnreadGroupThreads(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) (int, error) {
	count, err := p.source.PortalCountUnreadGroupThreads(ctx, accountID, studentIDs, tenantIDs)
	return count, parentGroupThreadError(err)
}

func (p parentGroupThreads) ReadGroupThread(ctx context.Context, threadID, accountID int64, studentIDs []int64) (*parentportal.GroupThreadView, bool, error) {
	view, advanced, err := p.source.PortalReadGroupThread(ctx, threadID, accountID, studentIDs)
	if err != nil {
		return nil, false, parentGroupThreadError(err)
	}
	messages := make([]parentportal.GroupMessage, 0, len(view.Messages))
	for _, message := range view.Messages {
		messages = append(messages, parentportal.GroupMessage{ID: message.ID, Body: message.Body, CreatedAt: message.CreatedAt})
	}
	return &parentportal.GroupThreadView{Thread: parentGroupThread(view.Thread), Messages: messages, StudentIDs: view.StudentIDs}, advanced, nil
}

func (p parentGroupThreads) GroupReplyTarget(ctx context.Context, threadID, accountID, studentID int64) (*parentportal.GroupThread, error) {
	thread, err := p.source.PortalGroupReplyTarget(ctx, threadID, accountID, studentID)
	if err != nil {
		return nil, parentGroupThreadError(err)
	}
	value := parentGroupThread(*thread)
	return &value, nil
}

func (p parentGroupThreads) GroupThreadTitles(ctx context.Context, threadIDs []int64) (map[int64]string, error) {
	titles, err := p.source.PortalGroupThreadTitles(ctx, threadIDs)
	return titles, parentGroupThreadError(err)
}

func parentGroupThread(thread communication.ParentGroupThread) parentportal.GroupThread {
	return parentportal.GroupThread{
		ID: thread.ID, TenantID: thread.TenantID, Title: thread.Title, TargetLabel: thread.Target.Label,
		ClosedAt: thread.ClosedAt, LastMessageAt: thread.LastMessageAt,
	}
}

func parentGroupThreadError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, communication.ErrParentGroupThreadNotFound):
		return parentportal.ErrGroupThreadNotFound
	case errors.Is(err, communication.ErrParentGroupThreadClosed):
		return parentportal.ErrGroupThreadClosed
	default:
		return err
	}
}
