package users

import (
	"context"

	peopleBindingPorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

func NewStaffMessageRoles(roles func(context.Context, int64, []int64) ([]peopleBindingPorts.StaffMessageRole, error)) func(ctx context.Context, tenantID int64, accountIDs []int64) ([]SchoolRoleClass, error) {
	return func(ctx context.Context, tenantID int64, accountIDs []int64) ([]SchoolRoleClass, error) {
		roles, err := roles(ctx, tenantID, accountIDs)
		if err != nil {
			return nil, err
		}
		result := make([]SchoolRoleClass, 0, len(roles))
		for _, role := range roles {
			result = append(result, SchoolRoleClass(role))
		}
		return result, nil
	}
}
