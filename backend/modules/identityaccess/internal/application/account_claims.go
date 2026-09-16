package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// LoadAccountClaims assembles the tenant-scoped claims material of an
// account on the caller's transaction: the retained admin staff-view preview
// (#2893) mints its read-only token from it after taking its own locks. A
// missing or soft-deleted school is ErrTenantNotFound; a query failure
// propagates.
func (s *AccountAuthentication) LoadAccountClaims(ctx context.Context, accountID, tenantID int64) (*domain.AccountClaimsPayload, error) {
	account, found, _, err := s.store.FindLoginAccount(ctx, accountID, false)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, domain.ErrAccountNotFound
	}
	return s.loadAccountMetadataForTenantInTx(ctx, account, tenantID)
}
