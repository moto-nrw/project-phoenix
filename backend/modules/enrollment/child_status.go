package enrollment

import "context"

// UpdateChildStatus records a decision or parent withdrawal and its reviewer.
func (m *Module) UpdateChildStatus(ctx context.Context, id int64, status string, reason *string, reviewedBy int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.UpdateChildStatus(ctx, id, status, reason, reviewedBy)
	})
}

// ReviewRolloverChild resolves the review marker and optionally changes the grade.
func (m *Module) ReviewRolloverChild(ctx context.Context, id int64, status string, reason *string, grade *int16, reviewedBy int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.ReviewRolloverChild(ctx, id, status, reason, grade, reviewedBy)
	})
}
