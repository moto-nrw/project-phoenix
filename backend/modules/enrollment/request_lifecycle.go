package enrollment

import (
	"context"
	"time"
)

// SetRequestWithdrawal updates the request marker after child states change.
// A nil timestamp clears the marker when the request is restored.
func (m *Module) SetRequestWithdrawal(ctx context.Context, id int64, at *time.Time) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.SetRequestWithdrawal(ctx, id, at)
	})
}

// CountPhaseRequests counts submissions, not children, in the caller's school.
func (m *Module) CountPhaseRequests(ctx context.Context, phaseID int64) (int, error) {
	var count int
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		count, err = m.engine.CountPhaseRequests(ctx, phaseID)
		return err
	})
	return count, err
}

// DeletePhaseRequests deletes submissions and their dependent intake rows.
// Materialized students are not owned by, or deleted with, an application.
func (m *Module) DeletePhaseRequests(ctx context.Context, phaseID int64) (int, error) {
	var count int
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		count, err = m.engine.DeletePhaseRequests(ctx, phaseID)
		return err
	})
	return count, err
}

func (m *Module) FullyRejectedRequestsBefore(ctx context.Context, cutoff time.Time) ([]int64, error) {
	var ids []int64
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		ids, err = m.engine.FullyRejectedRequestsBefore(ctx, cutoff)
		return err
	})
	return ids, err
}

func (m *Module) DeleteRequest(ctx context.Context, id int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.DeleteRequest(ctx, id) })
}

func (m *Module) HasSchemaRequests(ctx context.Context, id int64) (bool, error) {
	var count int
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		count, err = m.engine.CountRequestSchemaReferences(ctx, []int64{id})
		return err
	})
	return count > 0, err
}
