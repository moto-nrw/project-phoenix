package common

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

func RequireConfigRead() Middleware {
	return RequiresPermission(permissions.ConfigRead)
}

func RequireConfigWrite() Middleware {
	return RequiresAnyPermission(permissions.ConfigUpdate, permissions.ConfigManage)
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
