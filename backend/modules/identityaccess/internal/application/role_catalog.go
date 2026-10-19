package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// RoleCatalog reads the role and permission catalogs without session or
// account-lifecycle dependencies. Mutations still use RoleAdministration.
type RoleCatalog struct {
	store ports.RoleCatalogStore
	run   func(context.Context, func(context.Context) error) error
}

func NewRoleCatalog(store ports.RoleCatalogStore, run func(context.Context, func(context.Context) error) error) *RoleCatalog {
	return &RoleCatalog{store: store, run: run}
}

func (r *RoleCatalog) inUnitListRoles(ctx context.Context, filter domain.RoleFilter) ([]domain.ManagedRole, error) {
	roles, err := r.store.ListRoles(ctx, filter)
	if err != nil {
		return nil, failed("list roles", err)
	}
	return roles, nil
}

func (r *RoleCatalog) inUnitListPermissions(ctx context.Context, filter domain.PermissionFilter) ([]domain.ManagedPermission, error) {
	permissions, err := r.store.ListPermissions(ctx, filter)
	if err != nil {
		return nil, failed("list permissions", err)
	}
	return permissions, nil
}
