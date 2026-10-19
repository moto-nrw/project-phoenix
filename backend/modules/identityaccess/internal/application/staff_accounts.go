package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Staff projections retain the caller's database scope. RunPlatform does not
// elevate or replace an ambient transaction; school-specific reads also carry
// an explicit tenant predicate in persistence.

func (q *AccountRoleQueries) ClassifySchoolRoles(ctx context.Context, tenantID int64, accountIDs []int64) (result []domain.SchoolRoleClass, err error) {
	if tenantID <= 0 {
		return nil, domain.ErrTenantRequired
	}
	if len(accountIDs) == 0 {
		return []domain.SchoolRoleClass{}, nil
	}
	err = q.service.run(ctx, q.service.tx.RunRead, "classify_school_roles", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = q.store.ClassifySchoolRoles(txCtx, tenantID, accountIDs)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}

func (q *AccountRoleQueries) ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) (result []int64, err error) {
	if tenantID <= 0 {
		return nil, domain.ErrTenantRequired
	}
	if len(accountIDs) == 0 {
		return []int64{}, nil
	}
	err = q.service.run(ctx, q.service.tx.RunRead, "list_active_account_i_ds_for_tenant", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = q.store.ListActiveAccountIDsForTenant(txCtx, tenantID, accountIDs)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}

func (q *AccountRoleQueries) FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (result map[int64][]string, err error) {
	if tenantID <= 0 {
		return nil, domain.ErrTenantRequired
	}
	if len(accountIDs) == 0 {
		return map[int64][]string{}, nil
	}
	err = q.service.run(ctx, q.service.tx.RunRead, "find_effective_permission_names_by_account_i_ds_for_tenant", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = q.store.FindEffectivePermissionNamesByAccountIDsForTenant(txCtx, accountIDs, tenantID)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}

func (q *AccountRoleQueries) CountRoleNameMatchesByAccountIDs(ctx context.Context, accountIDs []int64, roleNames []string) (result map[int64]int, err error) {
	if len(accountIDs) == 0 || len(roleNames) == 0 {
		return map[int64]int{}, nil
	}
	err = q.service.run(ctx, q.service.tx.RunAdmin, "count_role_name_matches_by_account_i_ds", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = q.store.CountRoleNameMatchesByAccountIDs(txCtx, accountIDs, roleNames)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}

func (q *AccountRoleQueries) ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) (result []int64, err error) {
	if tenantID <= 0 {
		return nil, domain.ErrTenantRequired
	}
	if len(accountIDs) == 0 || len(roleNames) == 0 {
		return []int64{}, nil
	}
	err = q.service.run(ctx, q.service.tx.RunRead, "list_account_i_ds_with_system_role_names", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = q.store.ListAccountIDsWithSystemRoleNames(txCtx, accountIDs, roleNames, tenantID)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}

func (q *AccountRoleQueries) ListAccountEmails(ctx context.Context, accountIDs []int64) (result map[int64]string, err error) {
	if len(accountIDs) == 0 {
		return map[int64]string{}, nil
	}
	err = q.service.run(ctx, q.service.tx.RunPlatform, "list_account_emails", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = q.store.ListAccountEmails(txCtx, accountIDs)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}
