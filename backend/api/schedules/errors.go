package schedules

import (
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// ErrorRenderer renders an error to an HTTP response based on the schedule service error type
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific schedule service error
	if schedErr, ok := err.(*scheduleSvc.ScheduleError); ok {
		// Map specific schedule service errors to appropriate HTTP status codes
		switch schedErr.Unwrap() {
		case scheduleSvc.ErrDateframeNotFound:
			return common.ErrorNotFound(schedErr)
		case scheduleSvc.ErrTimeframeNotFound:
			return common.ErrorNotFound(schedErr)
		case scheduleSvc.ErrTimeframeRequiredByCareOffering:
			return common.ErrorConflict(schedErr)
		case scheduleSvc.ErrRecurrenceRuleNotFound:
			return common.ErrorNotFound(schedErr)
		case scheduleSvc.ErrInvalidDateRange:
			return common.ErrorInvalidRequest(schedErr)
		case scheduleSvc.ErrInvalidTimeRange:
			return common.ErrorInvalidRequest(schedErr)
		case scheduleSvc.ErrInvalidDuration:
			return common.ErrorInvalidRequest(schedErr)
		default:
			return common.ErrorInternalServer(schedErr)
		}
	}

	// For unknown errors, return a generic internal server error
	return common.ErrorInternalServer(err)
}
