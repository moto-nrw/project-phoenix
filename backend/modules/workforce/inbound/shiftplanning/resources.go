package shiftplanning

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	shifttypesHTTP "github.com/moto-nrw/project-phoenix/api/shift-types"
	staffshiftsHTTP "github.com/moto-nrw/project-phoenix/api/staff-shifts"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StaffShiftsDependencies are the collaborators the staff-shift route
// composition cannot own: the planning capability, the database the shared
// tenant middleware opens transactions on, and the actor lookups.
type StaffShiftsDependencies struct {
	Planning workforce.StaffShiftPlanning
	DB       *bun.DB
	// ResolveStaffID identifies the staff record of the acting admin.
	ResolveStaffID func(context.Context) (int64, error)
	// ActorAccountID names the acting account for audit entries; nil records
	// an actor-less event.
	ActorAccountID func(context.Context) *int64
}

// NewStaffShiftsResource wires /api/staff-shifts over the planning
// capability.
func NewStaffShiftsResource(deps StaffShiftsDependencies) *staffshiftsHTTP.Resource {
	if deps.Planning == nil || deps.DB == nil || deps.ResolveStaffID == nil || deps.ActorAccountID == nil {
		panic("staff-shifts HTTP composition: all dependencies are required")
	}
	return staffshiftsHTTP.NewResource(deps.Planning, staffShiftsRuntime(deps))
}

func staffShiftsRuntime(deps StaffShiftsDependencies) staffshiftsHTTP.Runtime {
	return staffshiftsHTTP.Runtime{
		Protected: func(router chi.Router, routes func(chi.Router, staffshiftsHTTP.Middleware)) {
			common.ProtectedTenantGroup(router, deps.DB, routes)
		},
		Permission: common.RequiresPermission,
		ParseID:    common.ParseID,
		Success:    common.Respond,
		Failure:    renderStaffShiftsFailure,
		ResolveActor: func(ctx context.Context) (staffshiftsHTTP.Actor, error) {
			staffID, err := deps.ResolveStaffID(ctx)
			if err != nil {
				return staffshiftsHTTP.Actor{}, err
			}
			if staffID <= 0 {
				return staffshiftsHTTP.Actor{}, errors.New("staff record not found")
			}
			return staffshiftsHTTP.Actor{StaffID: staffID, AccountID: deps.ActorAccountID(ctx)}, nil
		},
		MarkRollback:          tenant.MarkRollback,
		CanExportInternalPlan: common.CanExportInternalPlan,
		TimeTrackingManage:    permissions.TimeTrackingManage,
		SchedulesRead:         permissions.SchedulesRead,
		UsersRead:             permissions.UsersRead,
	}
}

// renderStaffShiftsFailure renders the failure with the shared envelope; the
// error's own wording is the message, as it was before the cutover. An
// internal failure carrying a client message keeps its cause in the log
// only.
func renderStaffShiftsFailure(w http.ResponseWriter, r *http.Request, kind staffshiftsHTTP.FailureKind, err error) {
	switch kind {
	case staffshiftsHTTP.FailureInvalid:
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case staffshiftsHTTP.FailureNotFound:
		common.RenderError(w, r, common.ErrorNotFound(err))
	case staffshiftsHTTP.FailureConflict:
		common.RenderError(w, r, common.ErrorConflict(err))
	case staffshiftsHTTP.FailureUnauthorized:
		common.RenderError(w, r, common.ErrorUnauthorized(err))
	case staffshiftsHTTP.FailureForbidden:
		common.RenderError(w, r, common.ErrorForbidden(err))
	default:
		var clientMessage *staffshiftsHTTP.ClientMessageError
		if errors.As(err, &clientMessage) {
			common.RenderError(w, r, common.ErrorInternalServerWrap(clientMessage.Message, clientMessage.Cause))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// NewShiftTypesResource wires /api/shift-types over the shift-type
// administration capability.
func NewShiftTypesResource(types workforce.ShiftTypeAdministration, db *bun.DB) *shifttypesHTTP.Resource {
	if types == nil || db == nil {
		panic("shift-types HTTP composition: all dependencies are required")
	}
	return shifttypesHTTP.NewResource(types, shiftTypesRuntime(db))
}

func shiftTypesRuntime(db *bun.DB) shifttypesHTTP.Runtime {
	return shifttypesHTTP.Runtime{
		Protected: func(router chi.Router, routes func(chi.Router, shifttypesHTTP.Middleware)) {
			common.ProtectedTenantGroup(router, db, routes)
		},
		Permission:         common.RequiresPermission,
		ParseID:            common.ParseID,
		Success:            common.Respond,
		Failure:            renderShiftTypesFailure,
		MarkRollback:       tenant.MarkRollback,
		TimeTrackingManage: permissions.TimeTrackingManage,
	}
}

func renderShiftTypesFailure(w http.ResponseWriter, r *http.Request, kind shifttypesHTTP.FailureKind, err error) {
	switch kind {
	case shifttypesHTTP.FailureInvalid:
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case shifttypesHTTP.FailureNotFound:
		common.RenderError(w, r, common.ErrorNotFound(err))
	case shifttypesHTTP.FailureConflict:
		common.RenderError(w, r, common.ErrorConflict(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
