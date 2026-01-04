package schedules

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// Common error variables
var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrInternalServer   = errors.New("internal server error")
	ErrResourceNotFound = errors.New("resource not found")
)

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return common.ErrorInvalidRequest(err)
}

// ErrorInternalServer returns a 500 Internal Server Error response
func ErrorInternalServer(err error) render.Renderer {
	return common.ErrorInternalServer(err)
}

// ErrorNotFound returns a 404 Not Found error response
func ErrorNotFound(err error) render.Renderer {
	return common.ErrorNotFound(err)
}

// ErrorRenderer renders an error to an HTTP response based on the schedule service error type
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific schedule service error
	if schedErr, ok := err.(*scheduleSvc.ScheduleError); ok {
		// Map specific schedule service errors to appropriate HTTP status codes
		switch schedErr.Unwrap() {
		case scheduleSvc.ErrDateframeNotFound:
			return ErrorNotFound(schedErr)
		case scheduleSvc.ErrTimeframeNotFound:
			return ErrorNotFound(schedErr)
		case scheduleSvc.ErrRecurrenceRuleNotFound:
			return ErrorNotFound(schedErr)
		case scheduleSvc.ErrInvalidDateRange:
			return ErrorInvalidRequest(schedErr)
		case scheduleSvc.ErrInvalidTimeRange:
			return ErrorInvalidRequest(schedErr)
		case scheduleSvc.ErrInvalidDuration:
			return ErrorInvalidRequest(schedErr)
		default:
			return ErrorInternalServer(schedErr)
		}
	}

	// For unknown errors, return a generic internal server error
	return ErrorInternalServer(err)
}
