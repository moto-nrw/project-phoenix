package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// SetChildWeeklyPlanOn plans the day an approved re-enrollment's weekly plan
// applies; nil clears it (#6043).
func (m *Service) SetChildWeeklyPlanOn(ctx context.Context, childID int64, on *enrollment.Date) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.SetChildWeeklyPlanOn(ctx, childID, on)
}

// ChildWeeklyPlanOn returns the day a request child's planned weekly plan
// applies, or nil.
func (m *Service) ChildWeeklyPlanOn(ctx context.Context, childID int64) (*enrollment.Date, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChildWeeklyPlanOn(ctx, childID)
}

// DueWeeklyPlans lists the planned weekly plans due on or before asOf.
func (m *Service) DueWeeklyPlans(ctx context.Context, asOf enrollment.Date) ([]enrollment.DueWeeklyPlan, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.DueWeeklyPlans(ctx, asOf)
}
