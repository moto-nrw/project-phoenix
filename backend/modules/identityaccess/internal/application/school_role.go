package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Service) ListSchoolRoles(ctx context.Context) (roles []*domain.SchoolRole, err error) {
	err = s.run(ctx, s.withinSchoolRead, "list_school_roles", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		roles, queryStats, queryErr = s.store.ListSchoolRoles(txCtx, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return roles, err
}

func (s *Service) FindSchoolRoleByName(ctx context.Context, name string) (role *domain.SchoolRole, err error) {
	err = s.run(ctx, s.withinSchoolRead, "find_school_role", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		role, queryStats, queryErr = s.store.FindSchoolRoleByName(txCtx, strings.ToLower(strings.TrimSpace(name)), s.tenantOf(txCtx))
		stats.Add(queryStats)
		if queryErr != nil {
			return queryErr
		}
		if role == nil {
			return domain.ErrRoleNotFound
		}
		return nil
	})
	return role, err
}

func (s *Service) FindRolePermissions(ctx context.Context, id int64) (permissions []string, err error) {
	err = s.run(ctx, s.withinSchoolRead, "find_role_permissions", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		permissions, queryStats, queryErr = s.store.FindRolePermissions(txCtx, id, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return permissions, err
}
