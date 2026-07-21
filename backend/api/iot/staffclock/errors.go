package staffclock

import (
	"errors"
	"strconv"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	staffclockSvc "github.com/moto-nrw/project-phoenix/services/iot/staffclock"
)

func classifyError(err error) render.Renderer {
	switch {
	case errors.Is(err, staffclockSvc.ErrInvalidRFIDTag):
		return common.ErrorInvalidRequestWithCode(err, "invalid_rfid_tag")
	case errors.Is(err, staffclockSvc.ErrRFIDTagNotFound):
		return common.ErrorNotFoundWithCode(err, "rfid_tag_not_found")
	case errors.Is(err, staffclockSvc.ErrRFIDTagInactive):
		return common.ErrorConflictWithCode(err, "rfid_tag_inactive")
	case errors.Is(err, staffclockSvc.ErrRFIDTagNotStaff):
		return common.ErrorConflictWithCode(err, "rfid_tag_not_staff")
	case errors.Is(err, staffclockSvc.ErrInvalidAction), errors.Is(err, staffclockSvc.ErrStatusRequired):
		return common.ErrorInvalidRequestWithCode(err, "invalid_staff_clock_request")
	case errors.Is(err, staffclockSvc.ErrCheckInRaced):
		// A concurrent scan won the insert: the same state conflict the kiosk
		// already knows how to recover from, not a server fault.
		return common.ErrorConflictWithCode(err, "invalid_staff_clock_state")
	}

	var reopenConflict *activeSvc.ReopenStatusConflictError
	if errors.As(err, &reopenConflict) {
		return common.ErrorConflictWithDetails(err, "reopen_status_conflict", map[string]any{
			"session_id":       strconv.FormatInt(reopenConflict.SessionID, 10),
			"existing_status":  reopenConflict.ExistingStatus,
			"requested_status": reopenConflict.RequestedStatus,
		})
	}
	var plannedStart *activeSvc.PlannedStartNotReachedError
	if errors.As(err, &plannedStart) {
		return common.ErrorConflictWithDetails(err, "planned_start_not_reached", map[string]any{
			"planned_start_time": plannedStart.PlannedStartTime,
			"current_time":       plannedStart.CurrentTime,
		})
	}
	var deviation *activeSvc.DeviationReasonRequiredError
	if errors.As(err, &deviation) {
		return common.ErrorConflictWithDetails(err, "deviation_reason_required", map[string]any{
			"action":            deviation.Action,
			"planned_time":      deviation.PlannedTime,
			"actual_time":       deviation.ActualTime,
			"deviation_minutes": strconv.Itoa(deviation.DeviationMinutes),
		})
	}

	switch err.Error() {
	case "already checked in", "already checked out today", "break already active",
		"no active session found", "no session found for today", "no active break found":
		return common.ErrorConflictWithCode(err, "invalid_staff_clock_state")
	}
	if strings.HasPrefix(err.Error(), "planned_duration_minutes must be") {
		return common.ErrorInvalidRequestWithCode(err, "invalid_staff_clock_request")
	}
	return common.ErrorInternalServerWrap("staff clock operation failed", err)
}
