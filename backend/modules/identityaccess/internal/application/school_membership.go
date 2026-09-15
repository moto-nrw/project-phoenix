package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// HasActiveSchoolMembership reports whether the account holds an active
// mapping to the school in context. A token can outlive a membership change,
// so consumers that gate school data on it ask here instead of trusting the
// tenant claim alone.
func (s *Service) HasActiveSchoolMembership(ctx context.Context, accountID int64) (active bool, err error) {
	err = s.run(ctx, s.withinSchoolRead, "has_active_school_membership", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		active, queryStats, queryErr = s.store.HasActiveAccountTenant(txCtx, accountID, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return active, err
}

// ListAccountRoleIDs returns the roles the account holds at the school in
// context. Roles held at another school are never included.
func (s *Service) ListAccountRoleIDs(ctx context.Context, accountID int64) (ids []int64, err error) {
	err = s.run(ctx, s.withinSchoolRead, "list_account_role_ids", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		ids, queryStats, queryErr = s.store.ListAccountRoleIDs(txCtx, accountID, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return ids, err
}

// ListActiveSchoolAccountIDs returns every account with an active mapping to
// the school in context.
func (s *Service) ListActiveSchoolAccountIDs(ctx context.Context) (ids []int64, err error) {
	err = s.run(ctx, s.withinSchoolRead, "list_active_school_account_ids", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		ids, queryStats, queryErr = s.store.ListActiveAccountIDs(txCtx, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return ids, err
}
