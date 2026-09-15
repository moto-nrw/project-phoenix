package enrollment

import "context"

// LinkCreatedStudent records the student materialized by the approval workflow.
func (m *Module) LinkCreatedStudent(ctx context.Context, childID, studentID int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.LinkCreatedStudent(ctx, childID, studentID) })
}

// UpdateMatchedStudent changes or clears an existing-student match. Relationship
// authorization and identity matching remain with the calling workflow.
func (m *Module) UpdateMatchedStudent(ctx context.Context, childID int64, studentID *int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.UpdateMatchedStudent(ctx, childID, studentID) })
}
