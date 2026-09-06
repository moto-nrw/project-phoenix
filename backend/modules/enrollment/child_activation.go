package enrollment

import "context"

// UpdateChildActivationPlan persists the approval workflow's calendar-date plan.
func (m *Module) UpdateChildActivationPlan(ctx context.Context, id int64, mode string, on *Date) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.UpdateChildActivationPlan(ctx, id, mode, on)
	})
}
