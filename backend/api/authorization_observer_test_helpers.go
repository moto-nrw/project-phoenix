//go:build testhooks

package api

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

func authorizationObserverTestHandler() http.Handler {
	return common.SecurityPrincipalMiddleware(common.RequiresPermission("users:read")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
}
