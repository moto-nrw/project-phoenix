package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Service) ListSchoolRoles(ctx context.Context) ([]*identityaccess.SchoolRole, error) {
	var roles []*domain.SchoolRole
	err := s.run(ctx, s.withinSchoolRead, "list_school_roles", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		roles, queryStats, queryErr = s.store.ListSchoolRoles(txCtx, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: list school roles: %w", MapError(err))
	}
	return publicSchoolRoles(roles), nil
}

func (s *Service) FindSchoolRoleByName(ctx context.Context, name string) (*identityaccess.SchoolRole, error) {
	var role *domain.SchoolRole
	err := s.run(ctx, s.withinSchoolRead, "find_school_role", func(txCtx context.Context, stats *domain.OperationStats) error {
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
	if err != nil {
		return nil, fmt.Errorf("identity access: find school role: %w", MapError(err))
	}
	return publicSchoolRole(role), nil
}

func (s *Service) FindRolePermissions(ctx context.Context, id int64) ([]string, error) {
	var permissions []string
	err := s.run(ctx, s.withinSchoolRead, "find_role_permissions", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		permissions, queryStats, queryErr = s.store.FindRolePermissions(txCtx, id, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: find role permissions: %w", MapError(err))
	}
	return permissions, nil
}

func publicSchoolRoles(roles []*domain.SchoolRole) []*identityaccess.SchoolRole {
	result := make([]*identityaccess.SchoolRole, len(roles))
	for i, role := range roles {
		result[i] = publicSchoolRole(role)
	}
	return result
}

func publicSchoolRole(role *domain.SchoolRole) *identityaccess.SchoolRole {
	value := identityaccess.SchoolRole(*role)
	return &value
}
