package enrollment

import "context"

// SetChildClassSwitch plans the class switch of an approved re-enrollment;
// nil clears it (#3917).
func (m *Module) SetChildClassSwitch(ctx context.Context, childID int64, change *ClassSwitch) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.SetChildClassSwitch(ctx, childID, change) })
}

// ChildClassSwitch returns the planned class switch of a request child, or nil.
func (m *Module) ChildClassSwitch(ctx context.Context, childID int64) (*ClassSwitch, error) {
	var result *ClassSwitch
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.ChildClassSwitch(ctx, childID)
		return err
	})
	return result, err
}

// DueClassSwitches lists the planned class switches due on or before asOf.
func (m *Module) DueClassSwitches(ctx context.Context, asOf Date) ([]DueClassSwitch, error) {
	var result []DueClassSwitch
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.DueClassSwitches(ctx, asOf)
		return err
	})
	return result, err
}
