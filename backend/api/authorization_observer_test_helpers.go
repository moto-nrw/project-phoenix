//go:build testhooks

package api

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

func authorizationObserverTestHandler() http.Handler {
	return SecurityPrincipalMiddleware(common.RequiresPermission("users:read")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
}
