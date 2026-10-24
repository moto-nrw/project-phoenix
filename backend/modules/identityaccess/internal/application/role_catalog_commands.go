package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (r *RoleCatalog) listRoles(ctx context.Context, filter domain.RoleFilter) (value0 []domain.ManagedRole, err error) {
	err = r.run(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitListRoles(txCtx, filter)
		return err
	})
	return
}

func (r *RoleCatalog) listPermissions(ctx context.Context, filter domain.PermissionFilter) (value0 []domain.ManagedPermission, err error) {
	err = r.run(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitListPermissions(txCtx, filter)
		return err
	})
	return
}
