package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/api/common"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

func RequireConfigRead() common.Middleware {
	return common.RequiresPermission(permissions.ConfigRead)
}

// RequireMealParticipantsRead protects the kitchen list, which combines
// configuration-owned meal settings with student directory data.
func RequireMealParticipantsRead() common.Middleware {
	return common.RequiresAllPermissions(permissions.ConfigRead, permissions.UsersRead)
}

func RequireConfigWrite() common.Middleware {
	return common.RequiresAnyPermission(permissions.ConfigUpdate, permissions.ConfigManage)
}

func RequireConfigUpdate() common.Middleware {
	return common.RequiresPermission(permissions.ConfigUpdate)
}

func RequireConfigManage() common.Middleware {
	return common.RequiresPermission(permissions.ConfigManage)
}

func RequireConfigReadOrWrite() common.Middleware {
	return common.RequiresAnyPermission(permissions.ConfigRead, permissions.ConfigUpdate, permissions.ConfigManage)
}

func CanEditConfig(ctx context.Context) bool {
	principal, err := common.CurrentPrincipal(ctx)
	return err == nil && (principal.HasPermission(permissions.ConfigUpdate) ||
		principal.HasPermission(permissions.ConfigManage))
}
