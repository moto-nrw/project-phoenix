package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type AccountRoleQueryStore interface {
	ListActiveTenantIDs(context.Context, int64) ([]int64, domain.OperationStats, error)
	ListSchoolAccountListings(context.Context, []int64) ([]domain.SchoolAccountListing, domain.OperationStats, error)
	FindActiveGuardianMemberships(context.Context, []int64) (map[int64][]int64, domain.OperationStats, error)
	ClassifySchoolRoles(context.Context, int64, []int64) ([]domain.SchoolRoleClass, domain.OperationStats, error)
	ListSchoolAccountRoleNames(context.Context, int64, int64) ([]string, domain.OperationStats, error)
	FindSystemRoleID(context.Context, string) (int64, bool, domain.OperationStats, error)
	ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, domain.OperationStats, error)
	FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, domain.OperationStats, error)
	CountRoleNameMatchesByAccountIDs(ctx context.Context, accountIDs []int64, roleNames []string) (map[int64]int, domain.OperationStats, error)
	ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) ([]int64, domain.OperationStats, error)
	ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, domain.OperationStats, error)
}
