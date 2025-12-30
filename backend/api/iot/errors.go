package iot

import (
	"errors"
	"fmt"
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
	ErrInvalidRequest           = errors.New("invalid request")
	ErrInternalServer           = errors.New("internal server error")
	ErrResourceNotFound         = errors.New("resource not found")
	ErrRoomCapacityExceeded     = errors.New("room capacity exceeded")
	ErrActivityCapacityExceeded = errors.New("activity capacity exceeded")
)

// Error message constants for reuse across handlers
const (
	ErrMsgInvalidDeviceID  = "invalid device ID"
	ErrMsgDeviceIDRequired = "device ID is required"
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
			if _, ok := iotErr.Err.(*iotSvc.DuplicateDeviceIDError); ok {
				return ErrorConflict(iotErr)
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
