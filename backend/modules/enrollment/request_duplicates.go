package enrollment

import "context"

// DuplicateChildKey identifies a child by normalized first and last name.
type DuplicateChildKey struct {
	FirstName string
	LastName  string
}

// ActiveDuplicateChildren excludes rejected and withdrawn children, but includes
// approved children. ExcludedRequestID permits rechecking an edited request.
func (m *Module) ActiveDuplicateChildren(ctx context.Context, phaseID int64, email string, children []DuplicateChildKey, excludedRequestID int64) ([]DuplicateChildKey, error) {
	var result []DuplicateChildKey
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.ActiveDuplicateChildren(ctx, phaseID, email, children, excludedRequestID)
		return err
	})
	return result, err
}

func (m *Module) HasActiveRequestForMatchedStudent(ctx context.Context, phaseID, studentID, excludedChildID int64) (bool, error) {
	var found bool
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		found, err = m.engine.HasActiveRequestForMatchedStudent(ctx, phaseID, studentID, excludedChildID)
		return err
	})
	return found, err
}
