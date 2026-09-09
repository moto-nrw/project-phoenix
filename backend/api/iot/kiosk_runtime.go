package iot

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	staffclockAPI "github.com/moto-nrw/project-phoenix/api/iot/staffclock"
)

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
