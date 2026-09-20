package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// SeedGuardianAccount provisions a local development guardian login without an
// invitation. The CLI owns its environment guard and profile linking; Identity
// owns the account, membership and role writes in the same ambient transaction.
func (s *Service) SeedGuardianAccount(ctx context.Context, email, passwordHash string) (accountID int64, reused bool, err error) {
	err = s.run(ctx, s.tx.RunWrite, "seed_guardian_account", func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tenantOf(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || passwordHash == "" {
			return fmt.Errorf("guardian seed email and password hash are required")
		}
		account, found, lookupStats, lookupErr := s.store.FindAccountByEmail(txCtx, email)
		stats.Add(lookupStats)
		if lookupErr != nil {
			return fmt.Errorf("find guardian seed account: %w", lookupErr)
		}
		accountID, reused = account.ID, found
		if !found {
			created, insertStats, insertErr := s.insertGuardianSeedAccount(txCtx, email, passwordHash)
			stats.Add(insertStats)
			if insertErr != nil {
				return fmt.Errorf("create guardian seed account: %w", insertErr)
			}
			accountID = created.ID
		}
		if _, grantErr := s.GrantGuardianTenantAccess(txCtx, accountID); grantErr != nil {
			return failed("grant guardian access", grantErr)
		}
		return nil
	})
	if err != nil {
		return 0, false, err
	}
	return accountID, reused, nil
}

func (s *Service) insertGuardianSeedAccount(ctx context.Context, email, passwordHash string) (domain.LoginAccount, domain.OperationStats, error) {
	validatedEmail, err := domain.NormalizeAccountEmail(email)
	if err != nil {
		return domain.LoginAccount{}, domain.OperationStats{}, err
	}
	return s.store.InsertAccount(ctx, validatedEmail, passwordHash)
}
