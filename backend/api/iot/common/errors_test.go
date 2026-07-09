package common_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Error Variables
func TestErrorVariables(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrRoomCapacityExceeded", iotCommon.ErrRoomCapacityExceeded},
		{"ErrActivityCapacityExceeded", iotCommon.ErrActivityCapacityExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}

// Test Error Message Constants
func TestErrorMessageConstants(t *testing.T) {
	assert.NotEmpty(t, iotCommon.ErrMsgPersonNotStudent)
	assert.NotEmpty(t, iotCommon.ErrMsgRFIDTagNotFound)
}

// Test RoomCapacityExceededError
func TestRoomCapacityExceededError_Error(t *testing.T) {
	err := &iotCommon.RoomCapacityExceededError{
		RoomID:           1,
		RoomName:         "Test Room",
		CurrentOccupancy: 15,
		MaxCapacity:      10,
	}

	errMsg := err.Error()
	assert.Contains(t, errMsg, "Test Room")
	assert.Contains(t, errMsg, "15/10")
	assert.Contains(t, errMsg, "room capacity exceeded")
}

// Test ActivityCapacityExceededError
func TestActivityCapacityExceededError_Error(t *testing.T) {
	err := &iotCommon.ActivityCapacityExceededError{
		ActivityID:       1,
		ActivityName:     "Test Activity",
		CurrentOccupancy: 25,
		MaxCapacity:      20,
	}

	errMsg := err.Error()
	assert.Contains(t, errMsg, "Test Activity")
	assert.Contains(t, errMsg, "25/20")
	assert.Contains(t, errMsg, "activity capacity exceeded")
}

// Test ErrorRoomCapacityExceeded Builder
func TestErrorRoomCapacityExceeded(t *testing.T) {
	renderer := iotCommon.ErrorRoomCapacityExceeded(42, "Test Room", 15, 10)
	resp, ok := renderer.(*iotCommon.CapacityErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "Room capacity exceeded", resp.Message)
	assert.Equal(t, "ROOM_CAPACITY_EXCEEDED", resp.Code)
	assert.NotNil(t, resp.Details)
	assert.Equal(t, int64(42), resp.Details.RoomID)
	assert.Equal(t, "Test Room", resp.Details.RoomName)
	assert.Equal(t, 15, resp.Details.CurrentOccupancy)
	assert.Equal(t, 10, resp.Details.MaxCapacity)
}

// Test ErrorActivityCapacityExceeded Builder
func TestErrorActivityCapacityExceeded(t *testing.T) {
	renderer := iotCommon.ErrorActivityCapacityExceeded(77, "Test Activity", 25, 20)
	resp, ok := renderer.(*iotCommon.ActivityCapacityErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "Activity capacity exceeded", resp.Message)
	assert.Equal(t, "ACTIVITY_CAPACITY_EXCEEDED", resp.Code)
	assert.NotNil(t, resp.Details)
	assert.Equal(t, int64(77), resp.Details.ActivityID)
	assert.Equal(t, "Test Activity", resp.Details.ActivityName)
	assert.Equal(t, 25, resp.Details.CurrentOccupancy)
	assert.Equal(t, 20, resp.Details.MaxCapacity)
}

// Test ErrorRenderer for IoT Service Errors
func TestErrorRenderer_IoTErrors(t *testing.T) {
	tests := []struct {
		name               string
		err                error
		expectedStatusCode int
	}{
		{"ErrDeviceNotFound", &iotSvc.IoTError{Err: iotSvc.ErrDeviceNotFound}, http.StatusNotFound},
		{"ErrInvalidDeviceData", &iotSvc.IoTError{Err: iotSvc.ErrInvalidDeviceData}, http.StatusBadRequest},
		{"ErrDuplicateDeviceID", &iotSvc.IoTError{Err: iotSvc.ErrDuplicateDeviceID}, http.StatusConflict},
		{"ErrInvalidStatus", &iotSvc.IoTError{Err: iotSvc.ErrInvalidStatus}, http.StatusBadRequest},
		{"ErrDeviceOffline", &iotSvc.IoTError{Err: iotSvc.ErrDeviceOffline}, http.StatusConflict},
		{"ErrNetworkScanFailed", &iotSvc.IoTError{Err: iotSvc.ErrNetworkScanFailed}, http.StatusInternalServerError},
		{"ErrDatabaseOperation", &iotSvc.IoTError{Err: iotSvc.ErrDatabaseOperation}, http.StatusInternalServerError},
		{"ErrDeviceProtected", &iotSvc.IoTError{Err: iotSvc.ErrDeviceProtected}, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := iotCommon.ErrorRenderer(tt.err)
			assert.NotNil(t, renderer)
			// Verify it's a render.Renderer
			_, ok := renderer.(interface {
				Render(http.ResponseWriter, *http.Request) error
			})
			assert.True(t, ok)
		})
	}
}

// Test ErrorRenderer for Active Service Errors
func TestErrorRenderer_ActiveErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		// Conflict errors
		{"ErrRoomConflict", &activeSvc.ActiveError{Err: activeSvc.ErrRoomConflict}},
		{"ErrSessionConflict", &activeSvc.ActiveError{Err: activeSvc.ErrSessionConflict}},
		{"ErrStudentAlreadyInGroup", &activeSvc.ActiveError{Err: activeSvc.ErrStudentAlreadyInGroup}},
		{"ErrGroupAlreadyInCombination", &activeSvc.ActiveError{Err: activeSvc.ErrGroupAlreadyInCombination}},
		{"ErrStudentAlreadyActive", &activeSvc.ActiveError{Err: activeSvc.ErrStudentAlreadyActive}},
		{"ErrStaffAlreadySupervising", &activeSvc.ActiveError{Err: activeSvc.ErrStaffAlreadySupervising}},
		// Not found errors
		{"ErrActiveGroupNotFound", &activeSvc.ActiveError{Err: activeSvc.ErrActiveGroupNotFound}},
		{"ErrVisitNotFound", &activeSvc.ActiveError{Err: activeSvc.ErrVisitNotFound}},
		{"ErrGroupSupervisorNotFound", &activeSvc.ActiveError{Err: activeSvc.ErrGroupSupervisorNotFound}},
		{"ErrCombinedGroupNotFound", &activeSvc.ActiveError{Err: activeSvc.ErrCombinedGroupNotFound}},
		{"ErrGroupMappingNotFound", &activeSvc.ActiveError{Err: activeSvc.ErrGroupMappingNotFound}},
		{"ErrNoActiveSession", &activeSvc.ActiveError{Err: activeSvc.ErrNoActiveSession}},
		{"ErrStaffNotFound", &activeSvc.ActiveError{Err: activeSvc.ErrStaffNotFound}},
		// Validation errors
		{"ErrActiveGroupAlreadyEnded", &activeSvc.ActiveError{Err: activeSvc.ErrActiveGroupAlreadyEnded}},
		{"ErrVisitAlreadyEnded", &activeSvc.ActiveError{Err: activeSvc.ErrVisitAlreadyEnded}},
		{"ErrSupervisionAlreadyEnded", &activeSvc.ActiveError{Err: activeSvc.ErrSupervisionAlreadyEnded}},
		{"ErrCombinedGroupAlreadyEnded", &activeSvc.ActiveError{Err: activeSvc.ErrCombinedGroupAlreadyEnded}},
		{"ErrInvalidTimeRange", &activeSvc.ActiveError{Err: activeSvc.ErrInvalidTimeRange}},
		{"ErrCannotDeleteActiveGroup", &activeSvc.ActiveError{Err: activeSvc.ErrCannotDeleteActiveGroup}},
		{"ErrInvalidData", &activeSvc.ActiveError{Err: activeSvc.ErrInvalidData}},
		// Database errors
		{"ErrDatabaseOperation", &activeSvc.ActiveError{Err: activeSvc.ErrDatabaseOperation}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := iotCommon.ErrorRenderer(tt.err)
			assert.NotNil(t, renderer)
		})
	}
}

// Test ErrorRenderer for Feedback Service Errors
func TestErrorRenderer_FeedbackErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrEntryNotFound", feedbackSvc.ErrEntryNotFound},
		{"ErrInvalidEntryData", feedbackSvc.ErrInvalidEntryData},
		{"ErrStudentNotFound", feedbackSvc.ErrStudentNotFound},
		{"ErrInvalidDateRange", feedbackSvc.ErrInvalidDateRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := iotCommon.ErrorRenderer(tt.err)
			assert.NotNil(t, renderer)
		})
	}
}

// Test ErrorRenderer for Unknown Errors
func TestErrorRenderer_UnknownError(t *testing.T) {
	unknownErr := errors.New("unknown error")
	renderer := iotCommon.ErrorRenderer(unknownErr)
	assert.NotNil(t, renderer)
}

// Test Capacity Error Response Render
func TestCapacityErrorResponse_Render(t *testing.T) {
	resp := &iotCommon.CapacityErrorResponse{
		Status:  "error",
		Message: "Room capacity exceeded",
		Code:    "ROOM_CAPACITY_EXCEEDED",
		Details: &iotCommon.RoomCapacityExceededError{
			RoomID:           1,
			RoomName:         "Test Room",
			CurrentOccupancy: 15,
			MaxCapacity:      10,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := resp.Render(w, r)
	assert.NoError(t, err)
}

// Test Activity Capacity Error Response Render
func TestActivityCapacityErrorResponse_Render(t *testing.T) {
	resp := &iotCommon.ActivityCapacityErrorResponse{
		Status:  "error",
		Message: "Activity capacity exceeded",
		Code:    "ACTIVITY_CAPACITY_EXCEEDED",
		Details: &iotCommon.ActivityCapacityExceededError{
			ActivityID:       1,
			ActivityName:     "Test Activity",
			CurrentOccupancy: 25,
			MaxCapacity:      20,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := resp.Render(w, r)
	assert.NoError(t, err)
}

// TestErrorStudentAlreadyActive_DegradedPathOmitsOptionalFields verifies
// the contract documented on StudentAlreadyActiveError: when the
// duplicate-visit lookup fails (race window between the failing INSERT
// and the response build), the 409 body must contain ONLY student_id —
// every other field carries the JSON `omitempty` semantics so kiosks
// don't render a bogus zero-valued timestamp ("0001-01-01T00:00:00Z")
// or a phantom existing_visit_id of 0.
//
// Issue #844 review feedback: the original implementation used
// EntryTime time.Time, which encoding/json does NOT skip for the zero
// value (omitempty only applies to nil pointers, empty strings/slices,
// 0 numbers, and false booleans — NOT struct zero values). Switching
// to *time.Time fixes the contract.
func TestErrorStudentAlreadyActive_DegradedPathOmitsOptionalFields(t *testing.T) {
	renderer := iotCommon.ErrorStudentAlreadyActive(int64(42), 0, nil, nil, "")
	resp, ok := renderer.(*iotCommon.StudentAlreadyActiveErrorResponse)
	require.True(t, ok)
	require.NotNil(t, resp.Details)

	body, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &decoded))

	assert.Equal(t, "STUDENT_ALREADY_ACTIVE", decoded["code"])
	details, ok := decoded["details"].(map[string]interface{})
	require.True(t, ok)

	assert.EqualValues(t, 42, details["student_id"], "student_id is the only required field")

	// All other fields must be omitted when not provided. A regression here
	// re-introduces the bogus "0001-01-01T00:00:00Z" / 0-valued IDs that
	// the cross-repo contract with PyrePortal explicitly forbids.
	assert.NotContains(t, details, "existing_visit_id", "must be omitted when 0")
	assert.NotContains(t, details, "entry_time", "must be omitted when nil — see *time.Time choice on the struct")
	assert.NotContains(t, details, "room_id", "must be omitted when nil")
	assert.NotContains(t, details, "room_name", "must be omitted when empty")
}

// TestErrorStudentAlreadyActive_FullPathPreservesAllFields verifies
// that when the existing-visit lookup succeeds, every detail field is
// serialized so the kiosk can render "Bereits angemeldet in <Raum>".
func TestErrorStudentAlreadyActive_FullPathPreservesAllFields(t *testing.T) {
	entryTime := time.Date(2026, 4, 29, 14, 16, 30, 0, time.UTC)
	roomID := int64(7)
	renderer := iotCommon.ErrorStudentAlreadyActive(int64(42), int64(101), &entryTime, &roomID, "Klassenzimmer 2")
	resp, ok := renderer.(*iotCommon.StudentAlreadyActiveErrorResponse)
	require.True(t, ok)

	body, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &decoded))

	details, ok := decoded["details"].(map[string]interface{})
	require.True(t, ok)

	assert.EqualValues(t, 42, details["student_id"])
	assert.EqualValues(t, 101, details["existing_visit_id"])
	assert.Equal(t, "2026-04-29T14:16:30Z", details["entry_time"])
	assert.EqualValues(t, 7, details["room_id"])
	assert.Equal(t, "Klassenzimmer 2", details["room_name"])
}
