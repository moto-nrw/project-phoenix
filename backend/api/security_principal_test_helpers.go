//go:build testhooks

package api

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

func principalTestCurrentPrincipal(ctx context.Context) (permissions.Principal, error) {
	return common.CurrentPrincipal(ctx)
}

func principalTestRequiresPermission(permission string) func(http.Handler) http.Handler {
	return common.RequiresPermission(permission)
}

func principalTestRequiresAnyPermission(required ...string) func(http.Handler) http.Handler {
	return common.RequiresAnyPermission(required...)
}

func principalTestRequiresAllPermissions(required ...string) func(http.Handler) http.Handler {
	return common.RequiresAllPermissions(required...)
}
