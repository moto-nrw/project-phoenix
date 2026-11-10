package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// UpdateChildActivationPlan persists the approval workflow's calendar-date plan.
func (m *Service) UpdateChildActivationPlan(ctx context.Context, id int64, mode string, on *enrollment.Date) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.UpdateChildActivationPlan(ctx, id, mode, on)
}

// PhaseActivationPlans lists the activation plans a phase-start edit follows.
func (m *Service) PhaseActivationPlans(ctx context.Context, phaseID int64, on enrollment.Date) ([]enrollment.ActivationPlan, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PhaseActivationPlans(ctx, phaseID, on)
}
