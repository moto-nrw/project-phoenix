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
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ActiveDuplicateChildren(ctx, phaseID, email, children, excludedRequestID)
}

func (m *Module) HasActiveRequestForMatchedStudent(ctx context.Context, phaseID, studentID, excludedChildID int64) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.engine.HasActiveRequestForMatchedStudent(ctx, phaseID, studentID, excludedChildID)
}
