package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

func (e engine) ClassifySchoolRoles(ctx context.Context, tenantID int64, accountIDs []int64) ([]identityaccess.SchoolRoleClass, error) {
	rows, err := e.accountRoleQueries.ClassifySchoolRoles(ctx, tenantID, accountIDs)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]identityaccess.SchoolRoleClass, 0, len(rows))
	for _, row := range rows {
		result = append(result, identityaccess.SchoolRoleClass(row))
	}
	return result, nil
}

func (e engine) ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error) {
	value, err := e.accountRoleQueries.ListActiveAccountIDsForTenant(ctx, tenantID, accountIDs)
	return value, mapError(err)
}

func (e engine) FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error) {
	value, err := e.accountRoleQueries.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, accountIDs, tenantID)
	return value, mapError(err)
}

func (e engine) CountRoleNameMatchesByAccountIDs(ctx context.Context, accountIDs []int64, roleNames []string) (map[int64]int, error) {
	value, err := e.accountRoleQueries.CountRoleNameMatchesByAccountIDs(ctx, accountIDs, roleNames)
	return value, mapError(err)
}

func (e engine) ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) ([]int64, error) {
	value, err := e.accountRoleQueries.ListAccountIDsWithSystemRoleNames(ctx, accountIDs, roleNames, tenantID)
	return value, mapError(err)
}

func (e engine) ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	value, err := e.accountRoleQueries.ListAccountEmails(ctx, accountIDs)
	return value, mapError(err)
}
