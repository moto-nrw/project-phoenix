package application

import (
	"context"
)

// UpdateChildStatus records a decision or parent withdrawal and its reviewer.
func (m *Service) UpdateChildStatus(ctx context.Context, id int64, status string, reason *string, reviewedBy int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.UpdateChildStatus(ctx, id, status, reason, reviewedBy)
}

// HoldAutoRenewedChild leaves an auto_renewed child to the school: it becomes
// submitted and carries the review reason the admin area shows. It reports
// false when the child is no longer auto_renewed.
func (m *Service) HoldAutoRenewedChild(ctx context.Context, id int64, reviewReason string) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.store.HoldAutoRenewedChild(ctx, id, reviewReason)
}

// ReviewRolloverChild resolves the review marker and optionally changes the grade.
func (m *Service) ReviewRolloverChild(ctx context.Context, id int64, status string, reason *string, grade *int16, reviewedBy int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.ReviewRolloverChild(ctx, id, status, reason, grade, reviewedBy)
}
