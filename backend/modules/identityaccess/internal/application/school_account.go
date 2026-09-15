package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Service) FindSchoolAccountByEmail(ctx context.Context, email string) (result domain.Account, err error) {
	err = s.run(ctx, s.withinSchoolRead, "find_school_account_by_email", func(txCtx context.Context, stats *domain.OperationStats) error {
		account, found, queryStats, queryErr := s.store.FindAccountByEmail(txCtx, strings.ToLower(strings.TrimSpace(email)))
		stats.Add(queryStats)
		if queryErr != nil {
			return queryErr
		}
		if !found {
			return domain.ErrAccountNotFound
		}
		active, mappingStats, mappingErr := s.store.HasActiveAccountTenant(txCtx, account.ID, s.tenantOf(txCtx))
		stats.Add(mappingStats)
		if mappingErr != nil {
			return mappingErr
		}
		if !active {
			return domain.ErrAccountNotFound
		}
		result = account
		return nil
	})
	return result, err
}

// withinSchoolRead rejects tenantless reads before the transaction adapter can
// select its platform-admin path. The outer operation still records a failure.
func (s *Service) withinSchoolRead(ctx context.Context, fn func(context.Context) error) error {
	if s.tenantOf(ctx) <= 0 {
		return domain.ErrTenantRequired
	}
	return s.tx.RunRead(ctx, fn)
}
