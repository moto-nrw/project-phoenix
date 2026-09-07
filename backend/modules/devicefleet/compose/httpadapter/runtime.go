// Package httpadapter binds the established display HTTP contract to the
// shared authentication, tenant-transaction, permission, and rendering
// middleware. The handler package itself stays free of those dependencies.
package httpadapter

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	displayHTTP "github.com/moto-nrw/project-phoenix/api/display"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	projectJWT "github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// Resource is the composed display HTTP resource.
type Resource = displayHTTP.Resource

// NewResource wires the display routes to the shared inbound middleware.
func NewResource(displays devicefleet.Capability, settings configService.SettingsService) *Resource {
	if displays == nil || settings == nil {
		panic("display HTTP composition: all dependencies are required")
	}
	return displayHTTP.NewResource(displays, runtime(settings))
}

func runtime(settings configService.SettingsService) displayHTTP.Runtime {
	return displayHTTP.Runtime{
		Protected:     protectedRoutes,
		AnyPermission: apiCommon.RequiresAnyPermission,
		Permission:    apiCommon.RequiresPermission,
		ParseID:       apiCommon.ParseInt64IDWithError,
		Failure:       renderFailure,
		FeatureEnabled: func(ctx context.Context) (bool, error) {
			// Routes run inside the tenant transaction middleware, so this
			// resolves against the current tenant.
			return settings.ResolveBool(ctx, configModel.KeyDisplayEnabled)
		},
		Read:   permissions.DisplayRead,
		Manage: permissions.DisplayManage,
	}
}

// protectedRoutes mounts the JWT-authenticated admin group and hands the
// tenant-transaction middleware to the routes that need it.
func protectedRoutes(r chi.Router, register func(chi.Router, displayHTTP.Middleware)) {
	tokenAuth := projectJWT.MustNewTokenAuth()
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(projectJWT.Authenticator)
		r.Use(apiCommon.ReadOnlyPreviewMiddleware)
		r.Use(projectJWT.TenantMiddleware)
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
