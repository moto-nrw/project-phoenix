package enrollment

import "context"

// CreatedStudentRequestChildIDs identifies source applications for care-exit
// cleanup. Matched-only links deliberately do not establish the created-student
// ownership used by that workflow.
func (m *Module) CreatedStudentRequestChildIDs(ctx context.Context, studentIDs []int64) ([]int64, error) {
	var ids []int64
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		ids, err = m.engine.CreatedStudentRequestChildIDs(txCtx, studentIDs)
		return err
	})
	return ids, err
}

// CountStudentReferences counts request children that retain a created or matched
// student link. A child referencing the student through both links counts once.
func (m *Module) CountStudentReferences(ctx context.Context, studentID int64) (int, error) {
	var count int
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		count, err = m.engine.CountStudentReferences(txCtx, studentID)
		return err
	})
	return count, err
}
