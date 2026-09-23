package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type RoleCatalogStore interface {
	ListRoles(context.Context, domain.RoleFilter) ([]domain.ManagedRole, error)
	ListPermissions(context.Context, domain.PermissionFilter) ([]domain.ManagedPermission, error)
}
