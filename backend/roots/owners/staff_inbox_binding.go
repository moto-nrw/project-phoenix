package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	staffInboxContract "github.com/moto-nrw/project-phoenix/modules/staffmessageinbox/contract"
)

type communicationStaffInbox struct{ source staffStoreProjection }
type staffStoreProjection interface {
	UnreadCount(context.Context, int64) (int, error)
	ListInbox(context.Context, int64, bool) ([]*staffInboxContract.StaffInboxThread, error)
}

func (r communicationStaffInbox) UnreadCount(ctx context.Context, accountID int64) (int, error) {
	return r.source.UnreadCount(ctx, accountID)
}
func (r communicationStaffInbox) ListInbox(ctx context.Context, accountID int64, unread bool) ([]*communication.StaffInboxRecord, error) {
	values, err := r.source.ListInbox(ctx, accountID, unread)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.StaffInboxRecord, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.StaffInboxRecord{ThreadID: value.ThreadID, TenantID: value.TenantID, CounterpartAccountID: value.CounterpartAccountID, CounterpartName: value.CounterpartName, LastMessageAt: value.LastMessageAt, LastMessageBody: value.LastMessageBody, LastSenderAccountID: value.LastSenderAccountID, UnreadCount: value.UnreadCount, Kind: value.Kind, GroupName: value.GroupName, MemberCount: value.MemberCount, LastSenderName: value.LastSenderName})
	}
	return rows, nil
}
