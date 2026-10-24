package enrollment

import "context"

// RequestReads is the per-account read state of enrollments (#3778). An
// enrollment is unread for an account while it sits in an active phase, has
// a non-terminal child and the account has not read it since the last parent
// change.
type RequestReads interface {
	CountUnreadRequests(ctx context.Context, accountID int64) (int, error)
	UnreadRequestIDs(ctx context.Context, accountID int64, requestIDs []int64) ([]int64, error)
	MarkRequestsRead(ctx context.Context, accountID int64, requestIDs []int64) error
	MarkAllRequestsRead(ctx context.Context, accountID int64) error
	MarkRequestUnread(ctx context.Context, accountID, requestID int64) error
}

var _ RequestReads = Owner(nil)
