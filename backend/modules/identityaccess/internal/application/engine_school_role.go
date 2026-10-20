package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

func (e engine) ListSchoolRoles(ctx context.Context) ([]*identityaccess.SchoolRole, error) {
	roles, err := e.Service.ListSchoolRoles(ctx)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]*identityaccess.SchoolRole, len(roles))
	for i, role := range roles {
		value := identityaccess.SchoolRole(*role)
		result[i] = &value
	}
	return result, nil
}

func (e engine) FindSchoolRoleByName(ctx context.Context, name string) (*identityaccess.SchoolRole, error) {
	role, err := e.Service.FindSchoolRoleByName(ctx, name)
	if err != nil {
		return nil, MapError(err)
	}
	value := identityaccess.SchoolRole(*role)
	return &value, nil
}

func (e engine) FindRolePermissions(ctx context.Context, id int64) ([]string, error) {
	names, err := e.Service.FindRolePermissions(ctx, id)
	return names, MapError(err)
}
