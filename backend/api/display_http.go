package api

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

// newDisplayHTTPResource wires the display routes to the shared inbound middleware.
// displayEnabled resolves the current tenant's display.enabled toggle; the
// routes run inside the tenant transaction middleware.
func newDisplayHTTPResource(displays devicefleet.Capability, displayEnabled func(context.Context) (bool, error)) *displayHTTP.Resource {
	if displays == nil || displayEnabled == nil {
		panic("display HTTP composition: all dependencies are required")
	}
	return displayHTTP.NewResource(displays, displayHTTPRuntime(displayEnabled))
}

func displayHTTPRuntime(displayEnabled func(context.Context) (bool, error)) displayHTTP.Runtime {
	return displayHTTP.Runtime{
		Protected:      displayProtectedRoutes,
		AnyPermission:  apiCommon.RequiresAnyPermission,
		Permission:     apiCommon.RequiresPermission,
		ParseID:        apiCommon.ParseInt64IDWithError,
		Failure:        renderDisplayFailure,
		FeatureEnabled: displayEnabled,
		Read:           permissions.DisplayRead,
		Manage:         permissions.DisplayManage,
	}
}

// displayProtectedRoutes mounts the JWT-authenticated admin group and hands the
// tenant-transaction middleware to the routes that need it.
func displayProtectedRoutes(r chi.Router, register func(chi.Router, displayHTTP.Middleware)) {
	r.Group(func(r chi.Router) {
		r.Use(sessiontoken.Authenticator)
		r.Use(ReadOnlyPreviewMiddleware)
		r.Use(TenantScopeMiddleware)
		r.Use(SecurityPrincipalMiddleware)
		register(r, apiCommon.TenantTxMiddleware)
	})
}

func renderDisplayFailure(
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
