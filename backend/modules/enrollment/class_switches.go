package enrollment

import "context"

// SetChildClassSwitch plans the class switch of an approved re-enrollment;
// nil clears it (#3917).
func (m *Module) SetChildClassSwitch(ctx context.Context, childID int64, change *ClassSwitch) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.SetChildClassSwitch(ctx, childID, change)
}

// ChildClassSwitch returns the planned class switch of a request child, or nil.
func (m *Module) ChildClassSwitch(ctx context.Context, childID int64) (*ClassSwitch, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChildClassSwitch(ctx, childID)
}

// DueClassSwitches lists the planned class switches due on or before asOf.
func (m *Module) DueClassSwitches(ctx context.Context, asOf Date) ([]DueClassSwitch, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.DueClassSwitches(ctx, asOf)
}
