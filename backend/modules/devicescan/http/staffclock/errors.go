package staffclock

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// The codes below are a cross-repo contract: PyrePortal maps them to German
// UI text (docs/agents/contracts.md, Ecosystem and IoT). They are registered
// in error-registry.json; this package may not import api/common, so it
// names them once here.
const (
	codeInvalidRequest         = "iot.invalid_staff_clock_request"
	codeInvalidRFIDTag         = "iot.invalid_rfid_tag"
	codeRFIDTagNotFound        = "iot.rfid_tag_not_found"
	codeRFIDTagInactive        = "iot.rfid_tag_inactive"
	codeRFIDTagNotStaff        = "iot.rfid_tag_not_staff"
	codeInvalidState           = "iot.invalid_staff_clock_state"
	codePlannedStartNotReached = "iot.planned_start_not_reached"
	codeDeviationReason        = "iot.deviation_reason_required"
	messageOperationFailed     = "staff clock operation failed"
)

func classifyError(err error) Failure {
	switch {
	case errors.Is(err, devicescan.ErrInvalidRFIDTag):
		return Failure{Status: http.StatusBadRequest, Code: codeInvalidRFIDTag, Err: err}
	case errors.Is(err, devicescan.ErrRFIDTagNotFound):
		return Failure{Status: http.StatusNotFound, Code: codeRFIDTagNotFound, Err: err}
	case errors.Is(err, devicescan.ErrRFIDTagInactive):
		return Failure{Status: http.StatusConflict, Code: codeRFIDTagInactive, Err: err}
	case errors.Is(err, devicescan.ErrRFIDTagNotStaff):
		return Failure{Status: http.StatusConflict, Code: codeRFIDTagNotStaff, Err: err}
	case errors.Is(err, devicescan.ErrInvalidAction), errors.Is(err, devicescan.ErrStatusRequired), errors.Is(err, devicescan.ErrStaffClockInvalid):
		return Failure{Status: http.StatusBadRequest, Code: codeInvalidRequest, Err: err}
	case errors.Is(err, devicescan.ErrStaffClockRaced), errors.Is(err, devicescan.ErrStaffClockState):
		// A concurrent scan or a stamp that does not fit the current state is
		// the same state conflict the kiosk already knows how to recover from,
		// not a server fault.
		return Failure{Status: http.StatusConflict, Code: codeInvalidState, Err: err}
	}

	if plannedStart, ok := errors.AsType[*devicescan.PlannedStartNotReachedError](err); ok {
		return Failure{Status: http.StatusConflict, Code: codePlannedStartNotReached, Err: err, Details: map[string]string{
			"planned_start_time": plannedStart.PlannedStartTime,
			"current_time":       plannedStart.CurrentTime,
		}}
	}
	if deviation, ok := errors.AsType[*devicescan.DeviationReasonRequiredError](err); ok {
		return Failure{Status: http.StatusConflict, Code: codeDeviationReason, Err: err, Details: map[string]string{
			"action":            deviation.Action,
			"planned_time":      deviation.PlannedTime,
			"actual_time":       deviation.ActualTime,
			"deviation_minutes": strconv.Itoa(deviation.DeviationMinutes),
		}}
	}
	return Failure{Status: http.StatusInternalServerError, Err: err, ClientMessage: messageOperationFailed}
}
