package enrollment

import "context"

// UpdateChildActivationPlan persists the approval workflow's calendar-date plan.
func (m *Module) UpdateChildActivationPlan(ctx context.Context, id int64, mode string, on *Date) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.UpdateChildActivationPlan(ctx, id, mode, on)
}
