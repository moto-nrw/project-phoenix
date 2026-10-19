package enrollment

import (
	"context"
	"time"
)

// SetRequestWithdrawal updates the request marker after child states change.
// A nil timestamp clears the marker when the request is restored.
func (m *Module) SetRequestWithdrawal(ctx context.Context, id int64, at *time.Time) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.SetRequestWithdrawal(ctx, id, at)
}

// CountPhaseRequests counts submissions, not children, in the caller's school.
func (m *Module) CountPhaseRequests(ctx context.Context, phaseID int64) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.CountPhaseRequests(ctx, phaseID)
}

// DeletePhaseRequests deletes submissions and their dependent intake rows.
// Materialized students are not owned by, or deleted with, an application.
func (m *Module) DeletePhaseRequests(ctx context.Context, phaseID int64) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.DeletePhaseRequests(ctx, phaseID)
}

func (m *Module) FullyRejectedRequestsBefore(ctx context.Context, cutoff time.Time) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.FullyRejectedRequestsBefore(ctx, cutoff)
}

// SettledRequestsBefore lists the requests settled without a child: every
// child withdrawn or rejected and none of them created a child record, at
// least one withdrawn, each withdrawal before withdrawnCutoff and each
// rejection before rejectedCutoff (#3960).
func (m *Module) SettledRequestsBefore(ctx context.Context, withdrawnCutoff, rejectedCutoff time.Time) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.SettledRequestsBefore(ctx, withdrawnCutoff, rejectedCutoff)
}

func (m *Module) DeleteRequest(ctx context.Context, id int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.DeleteRequest(ctx, id)
}

func (m *Module) HasSchemaRequests(ctx context.Context, id int64) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	count, err := m.engine.CountRequestSchemaReferences(ctx, []int64{id})
	return count > 0, err
}
