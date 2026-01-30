package common_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/stretchr/testify/assert"
)

// Test Error Variables
func TestErrorVariables(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidRequest", iotCommon.ErrInvalidRequest},
		{"ErrInternalServer", iotCommon.ErrInternalServer},
		{"ErrResourceNotFound", iotCommon.ErrResourceNotFound},
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
	assert.NotEmpty(t, iotCommon.ErrMsgInvalidDeviceID)
	assert.NotEmpty(t, iotCommon.ErrMsgDeviceIDRequired)
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

// Test Error Builder Functions
func TestErrorInvalidRequest(t *testing.T) {
	testErr := errors.New("invalid input")
	renderer := iotCommon.ErrorInvalidRequest(testErr)
	assert.NotNil(t, renderer)
}

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("database error")
	renderer := iotCommon.ErrorInternalServer(testErr)
	assert.NotNil(t, renderer)
}

func TestErrorNotFound(t *testing.T) {
	testErr := errors.New("not found")
	renderer := iotCommon.ErrorNotFound(testErr)
	assert.NotNil(t, renderer)
}

func TestErrorConflict(t *testing.T) {
	testErr := errors.New("conflict")
	renderer := iotCommon.ErrorConflict(testErr)
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
