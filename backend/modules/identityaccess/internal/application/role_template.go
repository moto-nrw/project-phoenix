package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

const opCreateRoleFromTemplate = "create role from template"

// createRoleFromTemplate creates a custom role that starts as a copy of the
// template system role (#4574): the same tier and the permissions the
// template effectively holds, minus the reserved permissions a school role may
// not hold. It runs inside
// CreateRole's transaction, so role and copy commit together.
func (r *RoleAdministration) createRoleFromTemplate(ctx context.Context, name, description string, requestedBase *string, templates []identityaccess.RoleTemplate) (domain.ManagedRole, error) {
	// The template sets the tier; a second template or an explicit tier
	// alongside it is a caller error.
	if requestedBase != nil || len(templates) != 1 {
		return domain.ManagedRole{}, failed(opCreateRoleFromTemplate, domain.ErrRoleNotFound)
	}
	baseRole, permissions, err := r.roleTemplate(ctx, templates[0])
	if err != nil {
		return domain.ManagedRole{}, err
	}
	// A copy of the Lehrkraft role opens moto schule like the original (#4574).
	schoolPortal := templates[0] == identityaccess.RoleTemplateLehrkraft
	role, err := r.store.CreateRole(ctx, domain.ManagedRole{Name: name, Description: description, BaseRole: &baseRole, SchoolPortal: schoolPortal})
	if err != nil {
		return domain.ManagedRole{}, failed(opCreateRoleFromTemplate, err)
	}
	// One statement copies the list, skipping a permission deleted since the
	// catalog was read, instead of failing the new role with a 500.
	permissionIDs := make([]int64, 0, len(permissions))
	for _, permission := range permissions {
		permissionIDs = append(permissionIDs, permission.ID)
	}
	if err := r.store.AssignRolePermissions(ctx, role.ID, permissionIDs); err != nil {
		return domain.ManagedRole{}, failed(opCreateRoleFromTemplate, err)
	}
	return role, nil
}

// roleTemplate resolves the template system role to the tier and the
// permissions a school role copies from it. The admin template stands for the
// whole catalog: admin holds many permissions only through admin:*, which
// stays reserved, so its explicit grants alone would fall short. Permissions
// a school grants only on purpose are never copied.
func (r *RoleAdministration) roleTemplate(ctx context.Context, template identityaccess.RoleTemplate) (string, []domain.ManagedPermission, error) {
	if !template.Valid() {
		return "", nil, failed(opCreateRoleFromTemplate, domain.ErrRoleNotFound)
	}
	source, found, err := r.store.FindSystemRoleByName(ctx, string(template))
	if err != nil {
		return "", nil, failed(opCreateRoleFromTemplate, err)
	}
	if !found {
		return "", nil, failed(opCreateRoleFromTemplate, domain.ErrRoleNotFound)
	}
	baseRole := identityaccess.EffectiveBaseRole(publicRoleFacts(source.Facts()))
	if baseRole == "" {
		return "", nil, failed(opCreateRoleFromTemplate, domain.ErrBaseRoleRequired)
	}
	var permissions []domain.ManagedPermission
	if template == identityaccess.RoleTemplateAdministration {
		permissions, err = r.store.ListPermissions(ctx, domain.PermissionFilter{})
	} else {
		permissions, err = r.store.ListRolePermissions(ctx, source.ID)
	}
	if err != nil {
		return "", nil, failed(opCreateRoleFromTemplate, err)
	}
	copied := make([]domain.ManagedPermission, 0, len(permissions))
	for _, permission := range permissions {
		name := permissionKey(permission)
		if !identityaccess.IsReservedPermission(name) && !identityaccess.IsExplicitGrantPermission(name) {
			copied = append(copied, permission)
		}
	}
	return baseRole, copied, nil
}
