package staffstore

import (
	"context"

	staffinbox "github.com/moto-nrw/project-phoenix/modules/staffmessageinbox/contract"
)

type StaffInbox interface {
	UnreadCount(ctx context.Context, accountID int64) (int, error)
	ListInbox(ctx context.Context, accountID int64, onlyUnread bool) ([]*staffinbox.StaffInboxThread, error)
}
