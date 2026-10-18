package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	notificationsHTTP "github.com/moto-nrw/project-phoenix/modules/delivery/http/notifications"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func notificationHTTPRuntime() notificationsHTTP.Runtime {
	runtime := notificationProblems()
	runtime.ProtectedTenant = func(r chi.Router, register func(chi.Router, notificationsHTTP.Middleware)) {
		ProtectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) { register(r, withTx) })
	}
	runtime.ProtectedSchool = func(r chi.Router, register func(chi.Router, notificationsHTTP.Middleware)) {
		ProtectedSchoolRoutes(r, func(r chi.Router, withTx common.Middleware) { register(r, withTx) })
	}
	runtime.TenantOperation = common.TenantOperationMiddleware
	runtime.TenantID = tenant.FromContext
	runtime.ActorID = sessiontoken.ActorAccountIDFromCtx
	runtime.Caller = func(ctx context.Context) notificationsHTTP.Caller {
		claims := sessiontoken.ClaimsFromCtx(ctx)
		return notificationsHTTP.Caller{ID: claims.ID, FamilyID: claims.FamilyID}
	}
	runtime.Permissions = sessiontoken.PermissionsFromCtx
	runtime.Respond = common.Respond
	return runtime
}

func notificationProblems() notificationsHTTP.Runtime {
	return notificationsHTTP.Runtime{
		Internal: func(w http.ResponseWriter, r *http.Request, err error) {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		},
		Forbidden: func(w http.ResponseWriter, r *http.Request, err error) {
			common.RenderError(w, r, common.ErrorForbidden(err))
		},
		Unauthorized: func(w http.ResponseWriter, r *http.Request) {
			common.RenderError(w, r, common.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
		},
		Invalid: func(w http.ResponseWriter, r *http.Request, err error) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
		},
		Disabled: func(w http.ResponseWriter, r *http.Request) {
			common.RenderError(w, r, common.ErrorConflictWithCode(devices.ErrNotificationsDisabled, common.CodeCommunicationNotificationsDisabled))
		},
		Conflict: func(w http.ResponseWriter, r *http.Request, err error) {
			common.RenderError(w, r, common.ErrorConflict(err))
		},
		NotFound: func(w http.ResponseWriter, r *http.Request, err error) {
			common.RenderError(w, r, common.ErrorNotFound(err))
		},
		Wrapped: func(w http.ResponseWriter, r *http.Request, message string, err error) {
			common.RenderError(w, r, common.ErrorInternalServerWrap(message, err))
		},
	}
}
