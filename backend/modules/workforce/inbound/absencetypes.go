// Package inbound composes the HTTP adapters that serve Workforce
// capabilities on the absence-type and substitution routes (#2688). The
// adapters themselves must not own the HTTP platform, so this package
// supplies authentication, tenant transaction scoping, permission names,
// caller resolution and the shared response envelope through their Runtime.
package inbound

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	absencetypesHTTP "github.com/moto-nrw/project-phoenix/api/absence-types"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/uptrace/bun"
)

// NewAbsenceTypesResource wires /api/absence-types over the absence-type
// administration capability. resolveActor identifies the staff member behind
// an allowance change.
func NewAbsenceTypesResource(types workforce.AbsenceTypeAdministration, db *bun.DB, resolveActor func(context.Context) (int64, error)) *absencetypesHTTP.Resource {
	if types == nil || db == nil || resolveActor == nil {
		panic("absence-types HTTP composition: all dependencies are required")
	}
	return absencetypesHTTP.NewResource(types, absenceTypesRuntime(db, resolveActor))
}

func absenceTypesRuntime(db *bun.DB, resolveActor func(context.Context) (int64, error)) absencetypesHTTP.Runtime {
	return absencetypesHTTP.Runtime{
		Protected: func(router chi.Router, routes func(chi.Router, absencetypesHTTP.Middleware)) {
			common.ProtectedTenantGroup(router, db, routes)
		},
		Permission:         common.RequiresPermission,
		AnyPermission:      common.RequiresAnyPermission,
		ParseID:            common.ParseID,
		ParseIDParam:       common.ParseIDParam,
		Success:            common.Respond,
		Failure:            renderAbsenceTypesFailure,
		ResolveActor:       resolveActor,
		TimeTrackingOwn:    permissions.TimeTrackingOwn,
		TimeTrackingManage: permissions.TimeTrackingManage,
		VacationApprove:    permissions.VacationApprove,
	}
}

// renderAbsenceTypesFailure renders the failure with the shared envelope; the
// error's own wording is the message, as it was before the cutover.
func renderAbsenceTypesFailure(w http.ResponseWriter, r *http.Request, kind absencetypesHTTP.FailureKind, err error) {
	switch kind {
	case absencetypesHTTP.FailureInvalid:
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case absencetypesHTTP.FailureNotFound:
		common.RenderError(w, r, common.ErrorNotFound(err))
	case absencetypesHTTP.FailureConflict:
		common.RenderError(w, r, common.ErrorConflict(err))
	case absencetypesHTTP.FailureUnauthorized:
		common.RenderError(w, r, common.ErrorUnauthorized(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
