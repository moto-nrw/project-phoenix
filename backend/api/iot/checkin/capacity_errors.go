package checkin

// Capacity and duplicate-visit conflict responses for the device checkin
// flow. Moved verbatim from the deleted api/iot/common in issue #575 B7 —
// checkin is the only flow that emits them.
//
// WIRE CONTRACT (PyrePortal): the Code values, Message strings, and details
// field names below are substring-matched by the kiosk
// (PyrePortal/src/services/apiErrors.ts) and pinned byte-for-byte by
// wire_format_test.go. Since issue #1879 PyrePortal reads the activity
// details via current_occupancy/max_capacity (same keys as the room error).
//
// Whether the capacity 409s carry the `details` object at all is gated per
// tenant by the checkin.*_capacity_details_enabled settings — see
// renderCheckinError in workflow.go. When a setting is off the *NoDetails
// renderer variants below are used and the kiosk falls back to its generic
// German message.

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// Common error variables
var (
	ErrRoomCapacityExceeded     = errors.New("room capacity exceeded")
	ErrActivityCapacityExceeded = errors.New("activity capacity exceeded")
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

// capacityErrorNoDetails is the details-omitted variant of the capacity 409
// bodies, emitted when the tenant's capacity-detail disclosure setting is off
// (issue #1879): same status/code/message, but no `details` field exists on
// the struct at all, so the key is absent from the JSON and PyrePortal falls
// back to its generic German message.
type capacityErrorNoDetails struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Render implements the render.Renderer interface
func (e *capacityErrorNoDetails) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusConflict)
	return nil
}

// ErrorRoomCapacityExceededNoDetails returns the ROOM_CAPACITY_EXCEEDED 409
// without the details object.
func ErrorRoomCapacityExceededNoDetails() render.Renderer {
	return &capacityErrorNoDetails{
		Status:  "error",
		Message: "Room capacity exceeded",
		Code:    "ROOM_CAPACITY_EXCEEDED",
	}
}

// ErrorActivityCapacityExceededNoDetails returns the ACTIVITY_CAPACITY_EXCEEDED
// 409 without the details object.
func ErrorActivityCapacityExceededNoDetails() render.Renderer {
	return &capacityErrorNoDetails{
		Status:  "error",
		Message: "Activity capacity exceeded",
		Code:    "ACTIVITY_CAPACITY_EXCEEDED",
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
