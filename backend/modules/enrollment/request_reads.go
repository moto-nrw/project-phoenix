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

var _ RequestReads = (*Module)(nil)

// CountUnreadRequests counts the unread enrollments of the account.
func (m *Module) CountUnreadRequests(ctx context.Context, accountID int64) (int, error) {
	var count int
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		count, err = m.engine.CountUnreadRequests(ctx, accountID)
		return err
	})
	return count, err
}

// UnreadRequestIDs keeps the ids among requestIDs the account has not read.
func (m *Module) UnreadRequestIDs(ctx context.Context, accountID int64, requestIDs []int64) ([]int64, error) {
	var ids []int64
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		ids, err = m.engine.UnreadRequestIDs(ctx, accountID, requestIDs)
		return err
	})
	return ids, err
}

// MarkRequestsRead marks the requests read for the account.
func (m *Module) MarkRequestsRead(ctx context.Context, accountID int64, requestIDs []int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.MarkRequestsRead(ctx, accountID, requestIDs)
	})
}

// MarkAllRequestsRead marks every unread enrollment of the account read.
func (m *Module) MarkAllRequestsRead(ctx context.Context, accountID int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.MarkAllRequestsRead(ctx, accountID)
	})
}

// MarkRequestUnread makes one request unread again for the account.
func (m *Module) MarkRequestUnread(ctx context.Context, accountID, requestID int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.MarkRequestUnread(ctx, accountID, requestID)
	})
}

// MarkRequestParentChanged makes the request unread for every account: a
// parent submitted, edited or confirmed it.
func (m *Module) MarkRequestParentChanged(ctx context.Context, requestID int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.MarkRequestParentChanged(ctx, requestID)
	})
}
