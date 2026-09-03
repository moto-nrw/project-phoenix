package common

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

func RequireConfigRead() Middleware {
	return RequiresPermission(permissions.ConfigRead)
}

// RequireMealParticipantsRead protects the kitchen list, which combines
// configuration-owned meal settings with student directory data.
func RequireMealParticipantsRead() Middleware {
	return RequiresAllPermissions(permissions.ConfigRead, permissions.UsersRead)
}

func RequireConfigWrite() Middleware {
	return RequiresAnyPermission(permissions.ConfigUpdate, permissions.ConfigManage)
}

func RequireConfigUpdate() Middleware {
	return RequiresPermission(permissions.ConfigUpdate)
}

func RequireConfigManage() Middleware {
	return RequiresPermission(permissions.ConfigManage)
}

func RequireConfigReadOrWrite() Middleware {
	return RequiresAnyPermission(permissions.ConfigRead, permissions.ConfigUpdate, permissions.ConfigManage)
}

func CanEditConfig(ctx context.Context) bool {
	principal, err := CurrentPrincipal(ctx)
	return err == nil && (principal.HasPermission(permissions.ConfigUpdate) ||
		principal.HasPermission(permissions.ConfigManage))
}
