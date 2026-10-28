package communication

import "context"

// StaffInboxReads supplies Communication's inbox rows through a root-bound projection reader.
type StaffInboxReads interface {
	UnreadCount(context.Context, int64) (int, error)
	ListInbox(context.Context, int64, bool) ([]*StaffInboxRecord, error)
}
