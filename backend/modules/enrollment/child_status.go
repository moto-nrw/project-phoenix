package enrollment

import "context"

// UpdateChildStatus records a decision or parent withdrawal and its reviewer.
func (m *Module) UpdateChildStatus(ctx context.Context, id int64, status string, reason *string, reviewedBy int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.UpdateChildStatus(ctx, id, status, reason, reviewedBy)
	})
}

// HoldAutoRenewedChild leaves an auto_renewed child to the school: it becomes
// submitted and carries the review reason the admin area shows. It reports
// false when the child is no longer auto_renewed.
func (m *Module) HoldAutoRenewedChild(ctx context.Context, id int64, reviewReason string) (bool, error) {
	var held bool
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		held, err = m.engine.HoldAutoRenewedChild(ctx, id, reviewReason)
		return err
	})
	return held, err
}

// ReviewRolloverChild resolves the review marker and optionally changes the grade.
func (m *Module) ReviewRolloverChild(ctx context.Context, id int64, status string, reason *string, grade *int16, reviewedBy int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.ReviewRolloverChild(ctx, id, status, reason, grade, reviewedBy)
	})
}
