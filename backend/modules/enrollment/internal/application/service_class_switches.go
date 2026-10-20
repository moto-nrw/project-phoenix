package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// SetChildClassSwitch plans the class switch of an approved re-enrollment;
// nil clears it (#3917).
func (m *Service) SetChildClassSwitch(ctx context.Context, childID int64, change *enrollment.ClassSwitch) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.SetChildClassSwitch(ctx, childID, change)
}

// ChildClassSwitch returns the planned class switch of a request child, or nil.
func (m *Service) ChildClassSwitch(ctx context.Context, childID int64) (*enrollment.ClassSwitch, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChildClassSwitch(ctx, childID)
}

// DueClassSwitches lists the planned class switches due on or before asOf.
func (m *Service) DueClassSwitches(ctx context.Context, asOf enrollment.Date) ([]enrollment.DueClassSwitch, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.DueClassSwitches(ctx, asOf)
}
