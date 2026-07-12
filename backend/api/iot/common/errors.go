package common

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
)

// RenderError renders an error response and logs any render failures.
// For server errors (5xx), it delegates to common.RenderError which handles
// slog logging and Sentry capture. For non-5xx errors, it renders directly.
// Exported for use by sub-packages (devices, checkin, etc.)
func RenderError(w http.ResponseWriter, r *http.Request, renderer render.Renderer) {
	common.RenderError(w, r, renderer)
}

// Common error variables
var (
	ErrRoomCapacityExceeded     = errors.New("room capacity exceeded")
	ErrActivityCapacityExceeded = errors.New("activity capacity exceeded")
)

// Error message constants for reuse across handlers
const (
	ErrMsgPersonNotStudent = "person is not a student"
	ErrMsgRFIDTagNotFound  = "RFID tag not found"
)

// RoomCapacityExceededError represents detailed information about a capacity exceeded error
type RoomCapacityExceededError struct {
	RoomID           int64  `json:"room_id"`
	RoomName         string `json:"room_name"`
	CurrentOccupancy int    `json:"current_occupancy"`
	MaxCapacity      int    `json:"max_capacity"`
}

// Error implements the error interface for RoomCapacityExceededError
func (e *RoomCapacityExceededError) Error() string {
	return fmt.Sprintf("room capacity exceeded: %s (%d/%d)", e.RoomName, e.CurrentOccupancy, e.MaxCapacity)
}

// CapacityErrorResponse is a structured error response for capacity exceeded errors
type CapacityErrorResponse struct {
	Status  string                     `json:"status"`
	Message string                     `json:"message"`
	Code    string                     `json:"code"`
	Details *RoomCapacityExceededError `json:"details"`
}

// Render implements the render.Renderer interface
func (e *CapacityErrorResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusConflict)
	return nil
}

// ErrorRoomCapacityExceeded returns a 409 Conflict error response with capacity details
func ErrorRoomCapacityExceeded(roomID int64, roomName string, currentOccupancy, maxCapacity int) render.Renderer {
	return &CapacityErrorResponse{
		Status:  "error",
		Message: "Room capacity exceeded",
		Code:    "ROOM_CAPACITY_EXCEEDED",
		Details: &RoomCapacityExceededError{
			RoomID:           roomID,
			RoomName:         roomName,
			CurrentOccupancy: currentOccupancy,
			MaxCapacity:      maxCapacity,
		},
	}
}

// ActivityCapacityExceededError represents detailed information about an activity capacity exceeded error
type ActivityCapacityExceededError struct {
	ActivityID       int64  `json:"activity_id"`
	ActivityName     string `json:"activity_name"`
	CurrentOccupancy int    `json:"current_occupancy"`
	MaxCapacity      int    `json:"max_capacity"`
}

// Error implements the error interface for ActivityCapacityExceededError
func (e *ActivityCapacityExceededError) Error() string {
	return fmt.Sprintf("activity capacity exceeded: %s (%d/%d)", e.ActivityName, e.CurrentOccupancy, e.MaxCapacity)
}

// ActivityCapacityErrorResponse is a structured error response for activity capacity exceeded errors
type ActivityCapacityErrorResponse struct {
	Status  string                         `json:"status"`
	Message string                         `json:"message"`
	Code    string                         `json:"code"`
	Details *ActivityCapacityExceededError `json:"details"`
}

// Render implements the render.Renderer interface
func (e *ActivityCapacityErrorResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusConflict)
	return nil
}

// ErrorActivityCapacityExceeded returns a 409 Conflict error response with activity capacity details
func ErrorActivityCapacityExceeded(activityID int64, activityName string, currentOccupancy, maxCapacity int) render.Renderer {
	return &ActivityCapacityErrorResponse{
		Status:  "error",
		Message: "Activity capacity exceeded",
		Code:    "ACTIVITY_CAPACITY_EXCEEDED",
		Details: &ActivityCapacityExceededError{
			ActivityID:       activityID,
			ActivityName:     activityName,
			CurrentOccupancy: currentOccupancy,
			MaxCapacity:      maxCapacity,
		},
	}
}

// StudentAlreadyActiveError carries the existing-visit context that the
// kiosk surfaces to the user when a duplicate-checkin is rejected.
//
// All fields except StudentID are optional: ExistingVisitID is omitted when
// the lookup couldn't load the prior visit (race window between the
// failing INSERT and the response build), RoomID/RoomName are omitted when
// the prior visit's active group has no associated room.
//
// EntryTime is *time.Time rather than time.Time because encoding/json's
// `omitempty` tag does not skip zero-valued struct fields — a value-typed
// time.Time would marshal to "0001-01-01T00:00:00Z" in the degraded path,
// breaking the optional-field contract and feeding clients a bogus
// timestamp. A nil pointer is omitted correctly.
type StudentAlreadyActiveError struct {
	StudentID       int64      `json:"student_id"`
	ExistingVisitID int64      `json:"existing_visit_id,omitempty"`
	EntryTime       *time.Time `json:"entry_time,omitempty"`
	RoomID          *int64     `json:"room_id,omitempty"`
	RoomName        string     `json:"room_name,omitempty"`
}

// Error implements the error interface for StudentAlreadyActiveError.
// The canonical phrasing is the active-service sentinel — PyrePortal
// substring-matches on it for the German UI translation, so both layers
// must emit byte-identical text; referencing the sentinel makes that a
// compile-time fact instead of comment discipline.
func (e *StudentAlreadyActiveError) Error() string {
	return activeSvc.ErrStudentAlreadyActive.Error()
}

// StudentAlreadyActiveErrorResponse is the structured 409 body returned
// when a duplicate-active-visit attempt is rejected. The shape mirrors
// CapacityErrorResponse so PyrePortal can parse all checkin-conflict
// responses through a single decoder.
type StudentAlreadyActiveErrorResponse struct {
	Status  string                     `json:"status"`
	Message string                     `json:"message"`
	Code    string                     `json:"code"`
	Details *StudentAlreadyActiveError `json:"details"`
}

// Render implements render.Renderer
func (e *StudentAlreadyActiveErrorResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusConflict)
	return nil
}

// ErrorStudentAlreadyActive returns a 409 Conflict response with details
// about the existing visit. existingVisitID, entryTime, roomID and
// roomName may all be zero-valued — only studentID is required. Pass
// entryTime as nil (not &time.Time{}) when the visit lookup couldn't
// resolve the entry timestamp; the field is dropped from the JSON body
// rather than serialized as the Go zero value.
func ErrorStudentAlreadyActive(studentID, existingVisitID int64, entryTime *time.Time, roomID *int64, roomName string) render.Renderer {
	return &StudentAlreadyActiveErrorResponse{
		Status:  "error",
		Message: activeSvc.ErrStudentAlreadyActive.Error(),
		Code:    "STUDENT_ALREADY_ACTIVE",
		Details: &StudentAlreadyActiveError{
			StudentID:       studentID,
			ExistingVisitID: existingVisitID,
			EntryTime:       entryTime,
			RoomID:          roomID,
			RoomName:        roomName,
		},
	}
}

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return common.ErrorInvalidRequest(err)
}

// ErrorInternalServer returns a 500 Internal Server Error response
func ErrorInternalServer(err error) render.Renderer {
	return common.ErrorInternalServer(err)
}

// ErrorInternalServerWrap returns a 500 response with a stable client-facing message
// while preserving the full error chain for Sentry and slog.
func ErrorInternalServerWrap(clientMsg string, cause error) render.Renderer {
	return common.ErrorInternalServerWrap(clientMsg, cause)
}

// ErrorNotFound returns a 404 Not Found error response
func ErrorNotFound(err error) render.Renderer {
	return common.ErrorNotFound(err)
}

// ErrorConflict returns a 409 Conflict error response
func ErrorConflict(err error) render.Renderer {
	return common.ErrorConflict(err)
}

// ErrorRenderer renders an error to an HTTP response based on the IoT service error type
func ErrorRenderer(err error) render.Renderer {
	// Delegate to service-specific error handlers
	if iotErr, ok := err.(*iotSvc.IoTError); ok {
		return handleIoTServiceError(iotErr)
	}

	if activeErr, ok := err.(*activeSvc.ActiveError); ok {
		return handleActiveServiceError(activeErr)
	}

	if feedbackErr, ok := err.(*feedbackSvc.InvalidEntryDataError); ok {
		return ErrorInvalidRequest(feedbackErr)
	}

	if renderer := handleFeedbackServiceError(err); renderer != nil {
		return renderer
	}

	// For unknown errors, return a generic internal server error
	return ErrorInternalServer(err)
}

// handleIoTServiceError maps IoT service errors to HTTP responses
func handleIoTServiceError(iotErr *iotSvc.IoTError) render.Renderer {
	switch iotErr.Unwrap() {
	case iotSvc.ErrDeviceNotFound:
		return ErrorNotFound(iotErr)
	case iotSvc.ErrInvalidDeviceData:
		return ErrorInvalidRequest(iotErr)
	case iotSvc.ErrDuplicateDeviceID:
		return ErrorConflict(iotErr)
	case iotSvc.ErrInvalidStatus:
		return ErrorInvalidRequest(iotErr)
	case iotSvc.ErrDeviceProtected:
		return common.ErrorForbidden(iotErr)
	case iotSvc.ErrDeviceOffline:
		return ErrorConflict(iotErr)
	case iotSvc.ErrNetworkScanFailed:
		return ErrorInternalServer(iotErr)
	case iotSvc.ErrDatabaseOperation:
		return ErrorInternalServer(iotErr)
	default:
		return handleIoTErrorTypes(iotErr)
	}
}

// handleIoTErrorTypes handles specific IoT error types
func handleIoTErrorTypes(iotErr *iotSvc.IoTError) render.Renderer {
	switch iotErr.Err.(type) {
	case *iotSvc.DeviceNotFoundError:
		return ErrorNotFound(iotErr)
	case *iotSvc.DuplicateDeviceIDError:
		return ErrorConflict(iotErr)
	default:
		return ErrorInternalServer(iotErr)
	}
}

// handleActiveServiceError maps Active service errors to HTTP responses
func handleActiveServiceError(activeErr *activeSvc.ActiveError) render.Renderer {
	// Check for conflict errors (409)
	if isActiveConflictError(activeErr.Err) {
		return ErrorConflict(activeErr)
	}

	// Check for not found errors (404)
	if isActiveNotFoundError(activeErr.Err) {
		return ErrorNotFound(activeErr)
	}

	// Check for validation errors (400)
	if isActiveValidationError(activeErr.Err) {
		return ErrorInvalidRequest(activeErr)
	}

	// Check for database errors (500)
	if errors.Is(activeErr.Err, activeSvc.ErrDatabaseOperation) {
		return ErrorInternalServer(activeErr)
	}

	// Default to internal server error
	return ErrorInternalServer(activeErr)
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
		errors.Is(err, activeSvc.ErrInvalidActivitySession)
}

// handleFeedbackServiceError maps Feedback service errors to HTTP responses
func handleFeedbackServiceError(err error) render.Renderer {
	switch {
	case errors.Is(err, feedbackSvc.ErrEntryNotFound):
		return ErrorNotFound(err)
	case errors.Is(err, feedbackSvc.ErrInvalidEntryData):
		return ErrorInvalidRequest(err)
	case errors.Is(err, feedbackSvc.ErrStudentNotFound):
		return ErrorNotFound(err)
	case errors.Is(err, feedbackSvc.ErrInvalidDateRange):
		return ErrorInvalidRequest(err)
	default:
		return nil // Not a feedback error
	}
}
