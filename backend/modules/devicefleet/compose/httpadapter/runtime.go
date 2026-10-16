// Package httpadapter binds the established display HTTP contract to the
// shared authentication, tenant-transaction, permission, and rendering
// middleware. The handler package itself stays free of those dependencies.
package httpadapter

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	displayHTTP "github.com/moto-nrw/project-phoenix/api/display"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// Resource is the composed display HTTP resource.
type Resource = displayHTTP.Resource

// NewResource wires the display routes to the shared inbound middleware.
// displayEnabled resolves the current tenant's display.enabled toggle; the
// routes run inside the tenant transaction middleware.
func NewResource(displays devicefleet.Capability, displayEnabled func(context.Context) (bool, error)) *Resource {
	if displays == nil || displayEnabled == nil {
		panic("display HTTP composition: all dependencies are required")
	}
	return displayHTTP.NewResource(displays, runtime(displayEnabled))
}

func runtime(displayEnabled func(context.Context) (bool, error)) displayHTTP.Runtime {
	return displayHTTP.Runtime{
		Protected:      protectedRoutes,
		AnyPermission:  apiCommon.RequiresAnyPermission,
		Permission:     apiCommon.RequiresPermission,
		ParseID:        apiCommon.ParseInt64IDWithError,
		Failure:        renderFailure,
		FeatureEnabled: displayEnabled,
		Read:           permissions.DisplayRead,
		Manage:         permissions.DisplayManage,
	}
}

// protectedRoutes mounts the JWT-authenticated admin group and hands the
// tenant-transaction middleware to the routes that need it.
func protectedRoutes(r chi.Router, register func(chi.Router, displayHTTP.Middleware)) {
	r.Group(func(r chi.Router) {
		r.Use(sessiontoken.Authenticator)
		r.Use(apiCommon.ReadOnlyPreviewMiddleware)
		r.Use(apiCommon.TenantScopeMiddleware)
		r.Use(apiCommon.SecurityPrincipalMiddleware)
		register(r, apiCommon.TenantTxMiddleware)
	})
}

func renderFailure(
	w http.ResponseWriter,
	r *http.Request,
	kind displayHTTP.FailureKind,
	err error,
	message string,
) {
	if err == nil {
		err = errors.New("display request failed")
	}
	switch kind {
	case displayHTTP.FailureInvalid:
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case displayHTTP.FailureForbidden:
		if errors.Is(err, displayHTTP.ErrFeatureDisabled) {
			apiCommon.RenderError(w, r, apiCommon.ErrorForbiddenWithCode(err, apiCommon.CodeDisplayDisabled))
			return
		}
		apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
	case displayHTTP.FailureNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	default:
		if message == "" {
			message = "display operation failed"
		}
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap(message, err))
	}
}
