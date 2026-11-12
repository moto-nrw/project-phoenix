package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Service) FindSchoolAccountByEmail(ctx context.Context, email string) (identityaccess.Account, error) {
	var result domain.Account
	err := s.run(ctx, s.withinSchoolRead, "find_school_account_by_email", func(txCtx context.Context, stats *domain.OperationStats) error {
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
	if err != nil {
		return identityaccess.Account{}, fmt.Errorf("identity access: find school account: %w", MapError(err))
	}
	return identityaccess.Account(result), nil
}

// withinSchoolRead rejects tenantless reads before the transaction adapter can
// select its platform-admin path. The outer operation still records a failure.
func (s *Service) withinSchoolRead(ctx context.Context, fn func(context.Context) error) error {
	if s.tenantOf(ctx) <= 0 {
		return domain.ErrTenantRequired
	}
	return s.tx.RunRead(ctx, fn)
}

// FindSchoolEmailIdentities batches the account and pending-invitation keys
// used by school imports without exposing accounts of another school.
func (s *Service) FindSchoolEmailIdentities(ctx context.Context, emails []string) (accounts map[string]int64, invited map[string][]int64, err error) {
	normalized := make([]string, 0, len(emails))
	for _, email := range emails {
		if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
			normalized = append(normalized, email)
		}
	}
	err = s.run(ctx, s.withinSchoolRead, "find_school_email_identities", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		accounts, invited, queryStats, queryErr = s.store.FindSchoolEmailIdentities(txCtx, normalized, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return nil, nil, fmt.Errorf("identity access: find school email identities: %w", MapError(err))
	}
	return accounts, invited, nil
}
