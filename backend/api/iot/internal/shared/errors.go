// Package shared holds the pieces genuinely used by more than one IoT
// sub-package: the multi-domain ErrorRenderer and the unregistered-tag
// audit hook. It replaced api/iot/common in issue #575 B7 — the old package
// coupled every sub-package to a grab-bag hub; this one is fenced by Go's
// internal-package rule to api/iot/... and keeps a deliberately narrow
// surface. Generic error constructors are NOT re-exported here — handlers
// call api/common directly.
package shared

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
)

// Error message constants for reuse across IoT handlers. Both strings are
// part of the PyrePortal error contract (see the guard test at api/iot).
const (
	ErrMsgPersonNotStudent = "person is not a student"
	ErrMsgRFIDTagNotFound  = "RFID tag not found"
)

// ErrorRenderer renders an error to an HTTP response based on the IoT
// service error type. Deliberately hand-written, not a B2 rule table: it
// dispatches across three service domains (iot, active, feedback) with
// type-switch fallbacks, and its output strings are part of the PyrePortal
// contract.
func ErrorRenderer(err error) render.Renderer {
	if errors.Is(err, activeSvc.ErrRoomCapacityExceeded) {
		return common.ErrorConflict(err)
	}

	// The normal session-start path returns this sentinel unwrapped (see
	// determineRoomIDWithStrategy), so it never reaches handleActiveServiceError.
	if errors.Is(err, activeSvc.ErrNoRoomAvailable) {
		return common.ErrorInvalidRequest(err)
	}

	// Delegate to service-specific error handlers
	if iotErr, ok := err.(*iotSvc.IoTError); ok {
		return handleIoTServiceError(iotErr)
	}

	if activeErr, ok := err.(*activeSvc.ActiveError); ok {
		return handleActiveServiceError(activeErr)
	}

	if feedbackErr, ok := err.(*feedbackSvc.InvalidEntryDataError); ok {
		return common.ErrorInvalidRequest(feedbackErr)
	}

	if renderer := handleFeedbackServiceError(err); renderer != nil {
		return renderer
	}

	// For unknown errors, return a generic internal server error
	return common.ErrorInternalServer(err)
}

// handleIoTServiceError maps IoT service errors to HTTP responses
func handleIoTServiceError(iotErr *iotSvc.IoTError) render.Renderer {
	switch iotErr.Unwrap() {
	case iotSvc.ErrDeviceNotFound:
		return common.ErrorNotFound(iotErr)
	case iotSvc.ErrInvalidDeviceData:
		return common.ErrorInvalidRequest(iotErr)
	case iotSvc.ErrDuplicateDeviceID:
		return common.ErrorConflict(iotErr)
	case iotSvc.ErrInvalidStatus:
		return common.ErrorInvalidRequest(iotErr)
	case iotSvc.ErrDeviceProtected:
		return common.ErrorForbidden(iotErr)
	case iotSvc.ErrDeviceOffline:
		return common.ErrorConflict(iotErr)
	case iotSvc.ErrNetworkScanFailed:
		return common.ErrorInternalServer(iotErr)
	case iotSvc.ErrDatabaseOperation:
		return common.ErrorInternalServer(iotErr)
	default:
		return handleIoTErrorTypes(iotErr)
	}
}

// handleIoTErrorTypes handles specific IoT error types
func handleIoTErrorTypes(iotErr *iotSvc.IoTError) render.Renderer {
	switch iotErr.Err.(type) {
	case *iotSvc.DeviceNotFoundError:
		return common.ErrorNotFound(iotErr)
	case *iotSvc.DuplicateDeviceIDError:
		return common.ErrorConflict(iotErr)
	default:
		return common.ErrorInternalServer(iotErr)
	}
}

// handleActiveServiceError maps Active service errors to HTTP responses
func handleActiveServiceError(activeErr *activeSvc.ActiveError) render.Renderer {
	// A graduation committing between the kiosk's student lookup and the
	// attendance/visit write surfaces here as ErrStudentGraduated. It is a 404
	// like an unknown student — but the MESSAGE is what PyrePortal maps, and the
	// wrapped ActiveError renders "student has graduated and cannot check in",
	// a string the kiosk knows nothing about (it would show a generic error).
	// Render the documented contract string instead (#405 review).
	// The same holds for a child whose care ended between the lookup and the
	// write (#2487): the kiosk has one mapping for "this tag belongs to nobody
	// we care for", and both cases mean exactly that to the person at the
	// tablet. A second English string would arrive unmapped.
	if errors.Is(activeErr.Err, activeSvc.ErrStudentGraduated) ||
		errors.Is(activeErr.Err, activeSvc.ErrStudentCareEnded) {
		return common.ErrorNotFound(errors.New(ErrMsgPersonNotStudent))
	}

	// Check for conflict errors (409)
	if isActiveConflictError(activeErr.Err) {
		return common.ErrorConflict(activeErr)
	}

	// Check for not found errors (404)
	if isActiveNotFoundError(activeErr.Err) {
		return common.ErrorNotFound(activeErr)
	}

	// Check for validation errors (400)
	if isActiveValidationError(activeErr.Err) {
		return common.ErrorInvalidRequest(activeErr)
	}

	// Check for database errors (500)
	if errors.Is(activeErr.Err, activeSvc.ErrDatabaseOperation) {
		return common.ErrorInternalServer(activeErr)
	}

	// Default to internal server error
	return common.ErrorInternalServer(activeErr)
}

// isActiveConflictError checks if error is a conflict error
func isActiveConflictError(err error) bool {
	return errors.Is(err, activeSvc.ErrRoomConflict) ||
		errors.Is(err, activeSvc.ErrSessionConflict) ||
		errors.Is(err, activeSvc.ErrStudentAlreadyInGroup) ||
		errors.Is(err, activeSvc.ErrGroupAlreadyInCombination) ||
		errors.Is(err, activeSvc.ErrStudentAlreadyActive) ||
		errors.Is(err, activeSvc.ErrStaffAlreadySupervising) ||
		errors.Is(err, activeSvc.ErrDeviceAlreadyActive)
}

// isActiveNotFoundError checks if error is a not found error
func isActiveNotFoundError(err error) bool {
	return errors.Is(err, activeSvc.ErrActiveGroupNotFound) ||
		errors.Is(err, activeSvc.ErrVisitNotFound) ||
		errors.Is(err, activeSvc.ErrGroupSupervisorNotFound) ||
		errors.Is(err, activeSvc.ErrCombinedGroupNotFound) ||
		errors.Is(err, activeSvc.ErrGroupMappingNotFound) ||
		errors.Is(err, activeSvc.ErrNoActiveSession) ||
		errors.Is(err, activeSvc.ErrStaffNotFound)
	// ErrStudentGraduated is deliberately absent: it is also a 404, but
	// handleActiveServiceError renders it earlier with the PyrePortal contract
	// string rather than the wrapped error text (#405 review).
}

// isActiveValidationError checks if error is a validation error
func isActiveValidationError(err error) bool {
	return errors.Is(err, activeSvc.ErrActiveGroupAlreadyEnded) ||
		errors.Is(err, activeSvc.ErrVisitAlreadyEnded) ||
		errors.Is(err, activeSvc.ErrSupervisionAlreadyEnded) ||
		errors.Is(err, activeSvc.ErrCombinedGroupAlreadyEnded) ||
		errors.Is(err, activeSvc.ErrInvalidTimeRange) ||
		errors.Is(err, activeSvc.ErrCannotDeleteActiveGroup) ||
		errors.Is(err, activeSvc.ErrInvalidData) ||
		errors.Is(err, activeSvc.ErrInvalidActivitySession) ||
		errors.Is(err, activeSvc.ErrNoRoomAvailable)
}

// handleFeedbackServiceError maps Feedback service errors to HTTP responses
func handleFeedbackServiceError(err error) render.Renderer {
	switch {
	case errors.Is(err, feedbackSvc.ErrEntryNotFound):
		return common.ErrorNotFound(err)
	case errors.Is(err, feedbackSvc.ErrInvalidEntryData):
		return common.ErrorInvalidRequest(err)
	case errors.Is(err, feedbackSvc.ErrStudentNotFound):
		return common.ErrorNotFound(err)
	case errors.Is(err, feedbackSvc.ErrInvalidDateRange):
		return common.ErrorInvalidRequest(err)
	default:
		return nil // Not a feedback error
	}
}
