package enrollment

import "context"

func (m *Module) DeletionRequestCounts(ctx context.Context, requestID int64) (*DeletionRequestCounts, error) {
	var value *DeletionRequestCounts
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		value, err = m.engine.DeletionRequestCounts(ctx, requestID)
		return err
	})
	return value, err
}
func (m *Module) DeletionChildTarget(ctx context.Context, requestID, childID int64) (*DeletionChildTarget, error) {
	var value *DeletionChildTarget
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		value, err = m.engine.DeletionChildTarget(ctx, requestID, childID)
		return err
	})
	return value, err
}
func (m *Module) DeletionChildCounts(ctx context.Context, requestID, childID int64) (*DeletionChildCounts, error) {
	var value *DeletionChildCounts
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		value, err = m.engine.DeletionChildCounts(ctx, requestID, childID)
		return err
	})
	return value, err
}
func (m *Module) DeletionGuardianProfileIDs(ctx context.Context, requestID int64) ([]int64, error) {
	var value []int64
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		value, err = m.engine.DeletionGuardianProfileIDs(ctx, requestID)
		return err
	})
	return value, err
}
func (m *Module) DeletionBlockingStudentIDs(ctx context.Context, requestID int64, childID *int64) ([]int64, error) {
	var values []int64
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		values, err = m.engine.DeletionBlockingStudentIDs(ctx, requestID, childID)
		return err
	})
	return values, err
}
