package compose

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot"
	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	devicesAPI "github.com/moto-nrw/project-phoenix/api/iot/devices"
	sessionsAPI "github.com/moto-nrw/project-phoenix/api/iot/sessions"
	staffclockAPI "github.com/moto-nrw/project-phoenix/api/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func infoRuntime() iotAPI.Runtime {
	return iotAPI.Runtime{Authenticated: func(ctx context.Context) bool { return device.DeviceFromCtx(ctx) != nil }, Success: common.Respond, Failure: renderDataFailure}
}

func devicesRuntime() devicesAPI.Runtime {
	return devicesAPI.Runtime{ParseID: common.ParseIDParam, Permission: common.RequiresPermission, Success: common.Respond, Failure: renderDataFailure, ConstraintViolation: common.IsConstraintViolation}
}

func sessionRuntime() sessionsAPI.Runtime {
	return sessionsAPI.Runtime{ParseID: common.ParseIDParam,
		Authenticated: func(ctx context.Context) bool { return device.DeviceFromCtx(ctx) != nil },
		Success:       common.Respond,
		Failure:       renderDataFailure,
		MarkRollback:  tenant.MarkRollback,
	}
}

func dataRuntime() dataAPI.Runtime {
	return dataAPI.Runtime{ParseID: common.ParseIDParam,
		Device: func(ctx context.Context) (int64, string, bool) {
			principal := device.DeviceFromCtx(ctx)
			if principal == nil {
				return 0, "", false
			}
			return principal.ID, principal.DeviceID, true
		},
		Success: common.Respond,
		Failure: renderDataFailure,
	}
}

func renderDataFailure(w http.ResponseWriter, r *http.Request, status int, err error, clientMessage string) {
	switch status {
	case http.StatusUnauthorized:
		if render.Render(w, r, device.ErrDeviceUnauthorized(err)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
	case http.StatusBadRequest:
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case http.StatusNotFound:
		common.RenderError(w, r, common.ErrorNotFound(err))
	case http.StatusConflict:
		common.RenderError(w, r, common.ErrorConflict(err))
	case http.StatusForbidden:
		common.RenderError(w, r, common.ErrorForbidden(err))
	default:
		if clientMessage != "" {
			common.RenderError(w, r, common.ErrorInternalServerWrap(clientMessage, err))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
	}
}

// staffClockRuntime binds the kiosk staff clock resource to the shared
// response envelope. The resource classifies; this renders (#2690).
func staffClockRuntime() staffclockAPI.Runtime {
	return staffclockAPI.Runtime{
		Success: common.Respond,
		Failure: renderKioskFailure,
	}
}

// checkinRuntime binds the kiosk scan and attendance resources to the same
// envelope (#2698).
func checkinRuntime() checkinAPI.Runtime {
	return checkinAPI.Runtime{
		Success: common.Respond,
		Failure: renderKioskFailure,
	}
}

func renderKioskFailure(w http.ResponseWriter, r *http.Request, status int, code string, err error, details map[string]string, clientMessage string) {
	switch status {
	case http.StatusBadRequest:
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, code))
	case http.StatusUnauthorized:
		common.RenderError(w, r, common.ErrorUnauthorizedWithCode(err, code))
	case http.StatusNotFound:
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, code))
	case http.StatusConflict:
		if len(details) > 0 {
			payload := make(map[string]any, len(details))
			for key, value := range details {
				payload[key] = value
			}
			common.RenderError(w, r, common.ErrorConflictWithDetails(err, code, payload))
			return
		}
		common.RenderError(w, r, common.ErrorConflictWithCode(err, code))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap(clientMessage, err))
	}
}
