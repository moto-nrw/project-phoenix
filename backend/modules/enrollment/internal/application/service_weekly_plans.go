package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// SetChildWeeklyPlan plans the weekly plan an approved re-enrollment applies
// at its start; nil clears it (#6043).
func (m *Service) SetChildWeeklyPlan(ctx context.Context, childID int64, plan *enrollment.WeeklyPlan) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.SetChildWeeklyPlan(ctx, childID, plan)
}

// ChildWeeklyPlan returns the planned weekly plan of a request child, or nil.
func (m *Service) ChildWeeklyPlan(ctx context.Context, childID int64) (*enrollment.WeeklyPlan, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChildWeeklyPlan(ctx, childID)
}

// DueWeeklyPlans lists the planned weekly plans due on or before asOf.
func (m *Service) DueWeeklyPlans(ctx context.Context, asOf enrollment.Date) ([]enrollment.WeeklyPlan, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.DueWeeklyPlans(ctx, asOf)
}
