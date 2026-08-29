package common

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

func RequireConfigRead() Middleware {
	return authorize.RequiresPermission(permissions.ConfigRead)
}

func RequireConfigWrite() Middleware {
	return authorize.RequiresAnyPermission(permissions.ConfigUpdate, permissions.ConfigManage)
}

func RequireConfigManage() Middleware {
	return authorize.RequiresPermission(permissions.ConfigManage)
}

func RequireConfigReadOrWrite() Middleware {
	return authorize.RequiresAnyPermission(permissions.ConfigRead, permissions.ConfigUpdate, permissions.ConfigManage)
}

func CanEditConfig(ctx context.Context) bool {
	claims := jwt.ClaimsFromCtx(ctx)
	return authorize.HasPermission(permissions.ConfigUpdate, claims.Permissions) ||
		authorize.HasPermission(permissions.ConfigManage, claims.Permissions)
}
