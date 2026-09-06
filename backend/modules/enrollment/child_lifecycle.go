package enrollment

import "context"

// DeleteRequestChildren removes intake children, not materialized students.
func (m *Module) DeleteRequestChildren(ctx context.Context, requestID int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.DeleteRequestChildren(ctx, requestID) })
}

func (m *Module) CountCreatedStudentsByPhase(ctx context.Context, phaseID int64) (int, error) {
	var count int
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		count, err = m.engine.CountCreatedStudentsByPhase(ctx, phaseID)
		return err
	})
	return count, err
}
