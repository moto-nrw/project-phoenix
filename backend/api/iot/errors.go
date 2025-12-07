package iot

import (
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
)

// renderError renders an error response and logs any render failures.
// This helper consolidates the common pattern of rendering errors and
// logging render failures, addressing DRY and error handling concerns.
func renderError(w http.ResponseWriter, r *http.Request, renderer render.Renderer) {
	if err := render.Render(w, r, renderer); err != nil {
		log.Printf("Render error: %v", err)
	}
}

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

// ErrorConflict returns a 409 Conflict error response
func ErrorConflict(err error) render.Renderer {
	return common.ErrorConflict(err)
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return common.ErrorForbidden(err)
}

// ErrorRenderer renders an error to an HTTP response based on the IoT service error type
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific IoT service error
	if iotErr, ok := err.(*iotSvc.IoTError); ok {
		// Map specific IoT service errors to appropriate HTTP status codes
		switch iotErr.Unwrap() {
		case iotSvc.ErrDeviceNotFound:
			return ErrorNotFound(iotErr)
		case iotSvc.ErrInvalidDeviceData:
			return ErrorInvalidRequest(iotErr)
		case iotSvc.ErrDuplicateDeviceID:
			return ErrorConflict(iotErr)
		case iotSvc.ErrInvalidStatus:
			return ErrorInvalidRequest(iotErr)
		case iotSvc.ErrDeviceOffline:
			return ErrorConflict(iotErr)
		case iotSvc.ErrNetworkScanFailed:
			return ErrorInternalServer(iotErr)
		case iotSvc.ErrDatabaseOperation:
			return ErrorInternalServer(iotErr)
		default:
			// Check for specific error types
			if _, ok := iotErr.Err.(*iotSvc.DeviceNotFoundError); ok {
				return ErrorNotFound(iotErr)
			}
			if _, ok := iotErr.Err.(*iotSvc.InvalidDeviceDataError); ok {
				return ErrorInvalidRequest(iotErr)
			}
			if _, ok := iotErr.Err.(*iotSvc.DuplicateDeviceIDError); ok {
				return ErrorConflict(iotErr)
			}
			if _, ok := iotErr.Err.(*iotSvc.DeviceOfflineError); ok {
				return ErrorConflict(iotErr)
			}
			if _, ok := iotErr.Err.(*iotSvc.NetworkScanError); ok {
				return ErrorInternalServer(iotErr)
			}
			return ErrorInternalServer(iotErr)
		}
	}

	// Check for Active Service errors
	if activeErr, ok := err.(*activeSvc.ActiveError); ok {
		switch {
		// 409 Conflict - resource conflicts
		case errors.Is(activeErr.Err, activeSvc.ErrRoomConflict),
			errors.Is(activeErr.Err, activeSvc.ErrSessionConflict),
			errors.Is(activeErr.Err, activeSvc.ErrStudentAlreadyInGroup),
			errors.Is(activeErr.Err, activeSvc.ErrGroupAlreadyInCombination),
			errors.Is(activeErr.Err, activeSvc.ErrStudentAlreadyActive),
			errors.Is(activeErr.Err, activeSvc.ErrStaffAlreadySupervising),
			errors.Is(activeErr.Err, activeSvc.ErrDeviceAlreadyActive):
			return ErrorConflict(activeErr)

		// 404 Not Found
		case errors.Is(activeErr.Err, activeSvc.ErrActiveGroupNotFound),
			errors.Is(activeErr.Err, activeSvc.ErrVisitNotFound),
			errors.Is(activeErr.Err, activeSvc.ErrGroupSupervisorNotFound),
			errors.Is(activeErr.Err, activeSvc.ErrCombinedGroupNotFound),
			errors.Is(activeErr.Err, activeSvc.ErrGroupMappingNotFound),
			errors.Is(activeErr.Err, activeSvc.ErrNoActiveSession),
			errors.Is(activeErr.Err, activeSvc.ErrStaffNotFound):
			return ErrorNotFound(activeErr)

		// 400 Bad Request - validation errors
		case errors.Is(activeErr.Err, activeSvc.ErrActiveGroupAlreadyEnded),
			errors.Is(activeErr.Err, activeSvc.ErrVisitAlreadyEnded),
			errors.Is(activeErr.Err, activeSvc.ErrSupervisionAlreadyEnded),
			errors.Is(activeErr.Err, activeSvc.ErrCombinedGroupAlreadyEnded),
			errors.Is(activeErr.Err, activeSvc.ErrInvalidTimeRange),
			errors.Is(activeErr.Err, activeSvc.ErrCannotDeleteActiveGroup),
			errors.Is(activeErr.Err, activeSvc.ErrInvalidData),
			errors.Is(activeErr.Err, activeSvc.ErrInvalidActivitySession):
			return ErrorInvalidRequest(activeErr)

		// 500 Internal Server Error
		case errors.Is(activeErr.Err, activeSvc.ErrDatabaseOperation):
			return ErrorInternalServer(activeErr)

		default:
			return ErrorInternalServer(activeErr)
		}
	}

	// Check for Feedback Service errors
	if feedbackErr, ok := err.(*feedbackSvc.InvalidEntryDataError); ok {
		return ErrorInvalidRequest(feedbackErr)
	}

	// Check for other specific feedback errors
	switch {
	case errors.Is(err, feedbackSvc.ErrEntryNotFound):
		return ErrorNotFound(err)
	case errors.Is(err, feedbackSvc.ErrInvalidEntryData):
		return ErrorInvalidRequest(err)
	case errors.Is(err, feedbackSvc.ErrStudentNotFound):
		return ErrorNotFound(err)
	case errors.Is(err, feedbackSvc.ErrInvalidDateRange):
		return ErrorInvalidRequest(err)
	}

	// For unknown errors, return a generic internal server error
	return ErrorInternalServer(err)
}
