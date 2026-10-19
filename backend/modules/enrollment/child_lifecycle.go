package enrollment

import "context"

// DeleteRequestChildren removes intake children, not materialized students.
func (m *Module) DeleteRequestChildren(ctx context.Context, requestID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.DeleteRequestChildren(ctx, requestID)
}

func (m *Module) CountCreatedStudentsByPhase(ctx context.Context, phaseID int64) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.CountCreatedStudentsByPhase(ctx, phaseID)
}
