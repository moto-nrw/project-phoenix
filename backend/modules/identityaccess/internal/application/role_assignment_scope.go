package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Role assignment always targets one school, including when an operator
// supplies an administrative transaction. Shared system-role definitions
// remain resolvable; another school's custom role does not.
func (r *RoleAdministration) roleForAssignment(ctx context.Context, roleID int64) (domain.ManagedRole, error) {
	schoolID := r.runtime.TenantID(ctx)
	if schoolID <= 0 {
		return domain.ManagedRole{}, failed("assign role", domain.ErrTenantRequired)
	}
	role, found, err := r.store.FindRoleIgnoringTenant(ctx, roleID)
	if err != nil || !found {
		return domain.ManagedRole{}, failed("assign role", errors.New("role not found"))
	}
	if role.TenantID != nil && *role.TenantID != schoolID {
		return domain.ManagedRole{}, failed("assign role", errors.New("role not found"))
	}
	return role, nil
}
