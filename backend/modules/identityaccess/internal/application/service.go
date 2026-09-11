// Package application implements the guardian-access capability over the
// store and transaction ports.
package application

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// Service runs every operation inside the caller's ambient transaction (or
// one it opens for the tenant in context), records one observation per call
// and turns "no row" outcomes into the stable domain errors.
type Service struct {
	store     ports.Store
	operators ports.OperatorStore
	tx        ports.Transaction
	tenantOf  func(context.Context) int64
	observe   ports.Observer
}

func New(store ports.Store, operators ports.OperatorStore, tx ports.Transaction, tenantOf func(context.Context) int64, observe ports.Observer) *Service {
	if store == nil || operators == nil || tx == nil || tenantOf == nil || observe == nil {
		panic("identity access application: all dependencies are required")
	}
	return &Service{store: store, operators: operators, tx: tx, tenantOf: tenantOf, observe: observe}
}

func (s *Service) FindAccount(ctx context.Context, id int64) (result domain.Account, err error) {
	err = s.run(ctx, s.tx.RunRead, "find_account", func(txCtx context.Context, stats *domain.OperationStats) error {
		if id <= 0 {
			return domain.ErrAccountNotFound
		}
		account, found, queryStats, findErr := s.store.FindAccount(txCtx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrAccountNotFound
		}
		result = account
		return nil
	})
	return result, err
}

func (s *Service) FindAccountByEmail(ctx context.Context, email string) (result domain.Account, err error) {
	err = s.run(ctx, s.tx.RunRead, "find_account_by_email", func(txCtx context.Context, stats *domain.OperationStats) error {
		normalized := strings.TrimSpace(strings.ToLower(email))
		if normalized == "" {
			return domain.ErrAccountNotFound
		}
		account, found, queryStats, findErr := s.store.FindAccountByEmail(txCtx, normalized)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrAccountNotFound
		}
		result = account
		return nil
	})
	return result, err
}

// GrantGuardianTenantAccess reactivates or creates the tenant mapping first
// and assigns the guardian base role once. Reactivation, not creation, is
// the contract: a mapping left inactive by an offboarding must not survive
// an approval untouched. The role assignment is an insert-if-absent on the
// (account, role, tenant) key, so concurrent approvals for the same parent
// and school cannot collide on the unique index.
func (s *Service) GrantGuardianTenantAccess(ctx context.Context, accountID int64) (result domain.GuardianTenantAccess, err error) {
	err = s.run(ctx, s.tx.RunWrite, "grant_guardian_tenant_access", func(txCtx context.Context, stats *domain.OperationStats) error {
		tenantID := s.tenantOf(txCtx)
		if tenantID <= 0 {
			return domain.ErrTenantRequired
		}
		if accountID <= 0 {
			return domain.ErrAccountNotFound
		}
		mappingStats, mappingErr := s.store.EnsureActiveTenantMapping(txCtx, accountID, tenantID)
		stats.Add(mappingStats)
		if mappingErr != nil {
			return mappingErr
		}
		roleID, found, roleStats, roleErr := s.store.FindRoleByName(txCtx, domain.GuardianRoleName, tenantID)
		stats.Add(roleStats)
		if roleErr != nil {
			return roleErr
		}
		if !found {
			return domain.ErrGuardianRoleMissing
		}
		assigned, assignStats, assignErr := s.store.AssignAccountRole(txCtx, accountID, roleID, tenantID)
		stats.Add(assignStats)
		if assignErr != nil {
			return assignErr
		}
		result = domain.GuardianTenantAccess{AccountID: accountID, TenantID: tenantID, RoleID: roleID, RoleAssigned: assigned}
		return nil
	})
	if err != nil {
		return domain.GuardianTenantAccess{}, err
	}
	return result, nil
}

func (s *Service) run(
	ctx context.Context,
	within func(context.Context, func(context.Context) error) error,
	operation string,
	fn func(context.Context, *domain.OperationStats) error,
) error {
	started := time.Now()
	var stats domain.OperationStats
	err := within(ctx, func(txCtx context.Context) error {
		return fn(txCtx, &stats)
	})
	s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}
