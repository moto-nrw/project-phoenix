package application

import (
	"context"
)

// CountUnreadRequests counts the unread enrollments of the account.
func (m *Service) CountUnreadRequests(ctx context.Context, accountID int64) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.CountUnreadRequests(ctx, accountID)
}

// UnreadRequestIDs keeps the ids among requestIDs the account has not read.
func (m *Service) UnreadRequestIDs(ctx context.Context, accountID int64, requestIDs []int64) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.UnreadRequestIDs(ctx, accountID, requestIDs)
}

// MarkRequestsRead marks the requests read for the account.
func (m *Service) MarkRequestsRead(ctx context.Context, accountID int64, requestIDs []int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.MarkRequestsRead(ctx, accountID, requestIDs)
}

// MarkAllRequestsRead marks every unread enrollment of the account read.
func (m *Service) MarkAllRequestsRead(ctx context.Context, accountID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.MarkAllRequestsRead(ctx, accountID)
}

// MarkRequestUnread makes one request unread again for the account.
func (m *Service) MarkRequestUnread(ctx context.Context, accountID, requestID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.MarkRequestUnread(ctx, accountID, requestID)
}

// MarkRequestParentChanged makes the request unread for every account: a
// parent submitted, edited or confirmed it.
func (m *Service) MarkRequestParentChanged(ctx context.Context, requestID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.MarkRequestParentChanged(ctx, requestID)
}
