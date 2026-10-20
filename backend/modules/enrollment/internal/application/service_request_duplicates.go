package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// ActiveDuplicateChildren excludes rejected and withdrawn children, but includes
// approved children. ExcludedRequestID permits rechecking an edited request.
func (m *Service) ActiveDuplicateChildren(ctx context.Context, phaseID int64, email string, children []enrollment.DuplicateChildKey, excludedRequestID int64) ([]enrollment.DuplicateChildKey, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ActiveDuplicateChildren(ctx, phaseID, email, children, excludedRequestID)
}

func (m *Service) HasActiveRequestForMatchedStudent(ctx context.Context, phaseID, studentID, excludedChildID int64) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.store.HasActiveRequestForMatchedStudent(ctx, phaseID, studentID, excludedChildID)
}
