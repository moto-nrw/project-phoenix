package application

import (
	"context"
	"time"
)

// SetRequestWithdrawal updates the request marker after child states change.
// A nil timestamp clears the marker when the request is restored.
func (m *Service) SetRequestWithdrawal(ctx context.Context, id int64, at *time.Time) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.SetRequestWithdrawal(ctx, id, at)
}

// CountPhaseRequests counts submissions, not children, in the caller's school.
func (m *Service) CountPhaseRequests(ctx context.Context, phaseID int64) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.CountPhaseRequests(ctx, phaseID)
}

// DeletePhaseRequests deletes submissions and their dependent intake rows.
// Materialized students are not owned by, or deleted with, an application.
func (m *Service) DeletePhaseRequests(ctx context.Context, phaseID int64) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.DeletePhaseRequests(ctx, phaseID)
}

func (m *Service) FullyRejectedRequestsBefore(ctx context.Context, cutoff time.Time) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.FullyRejectedRequestsBefore(ctx, cutoff)
}

// SettledRequestsBefore lists the requests settled without a child: every
// child withdrawn or rejected and none of them created a child record, at
// least one withdrawn, each withdrawal before withdrawnCutoff and each
// rejection before rejectedCutoff (#3960).
func (m *Service) SettledRequestsBefore(ctx context.Context, withdrawnCutoff, rejectedCutoff time.Time) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.SettledRequestsBefore(ctx, withdrawnCutoff, rejectedCutoff)
}

func (m *Service) DeleteRequest(ctx context.Context, id int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.DeleteRequest(ctx, id)
}

func (m *Service) HasSchemaRequests(ctx context.Context, id int64) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	count, err := m.store.CountRequestSchemaReferences(ctx, []int64{id})
	return count > 0, err
}
