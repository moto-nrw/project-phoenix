package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// DeleteExpiredSubmissionRateLimits removes the school's buckets older than
// the longest submission window.
func (m *Service) DeleteExpiredSubmissionRateLimits(ctx context.Context) (int, int, error) {
	deleted, err := m.store.CleanupExpired(ctx)
	if err != nil {
		return 0, 0, err
	}
	return deleted, submissionRateLimitRetentionDays, nil
}

// Schema returns the immutable version pinned to a submission or phase.
func (m *Service) Schema(ctx context.Context, id int64) (*enrollment.FormSchema, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.Schema(ctx, id)
}

// ActiveSchema returns the active form visible in the caller's tenant.
func (m *Service) ActiveSchema(ctx context.Context) (*enrollment.FormSchema, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ActiveSchema(ctx)
}

// SchemaVersions lists versions newest first within the caller's tenant.
func (m *Service) SchemaVersions(ctx context.Context) ([]*enrollment.FormSchema, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.SchemaVersions(ctx)
}

// BackfillGuardianAccountID attaches previously anonymous requests after account
// verification. The caller supplies its authorized tenant or admin transaction.
func (m *Service) BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error) {
	if accountID <= 0 {
		return 0, fmt.Errorf("parent: account_id must be positive")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return 0, nil
	}
	return m.store.BackfillGuardianAccountID(ctx, accountID, email)
}
