package application

import (
	"context"
)

// LinkCreatedStudent records the student materialized by the approval workflow.
func (m *Service) LinkCreatedStudent(ctx context.Context, childID, studentID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.LinkCreatedStudent(ctx, childID, studentID)
}

// UpdateMatchedStudent changes or clears an existing-student match. Relationship
// authorization and identity matching remain with the calling workflow.
func (m *Service) UpdateMatchedStudent(ctx context.Context, childID int64, studentID *int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.UpdateMatchedStudent(ctx, childID, studentID)
}

// PinOfficeChosenStudent records the enrolled student the office chose at
// approval (#6053). The calling decision keeps the candidate check and the
// phase-wide uniqueness guard.
func (m *Service) PinOfficeChosenStudent(ctx context.Context, childID, studentID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.PinOfficeChosenStudent(ctx, childID, studentID)
}
