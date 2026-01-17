package common_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	activeSvc "github.com/moto-nrw/project-phoenix/internal/core/service/active"
	feedbackSvc "github.com/moto-nrw/project-phoenix/internal/core/service/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/internal/core/service/iot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// RoomCapacityExceededError Tests
// =============================================================================

func TestRoomCapacityExceededError_Error(t *testing.T) {
	err := &common.RoomCapacityExceededError{
		RoomID:           1,
		RoomName:         "Room A",
		CurrentOccupancy: 30,
		MaxCapacity:      25,
	}

	expected := "room capacity exceeded: Room A (30/25)"
	assert.Equal(t, expected, err.Error())
}

// =============================================================================
// ActivityCapacityExceededError Tests
// =============================================================================

func TestActivityCapacityExceededError_Error(t *testing.T) {
	err := &common.ActivityCapacityExceededError{
		ActivityID:       42,
		ActivityName:     "Basketball",
		CurrentOccupancy: 20,
		MaxCapacity:      15,
	}

	expected := "activity capacity exceeded: Basketball (20/15)"
	assert.Equal(t, expected, err.Error())
}

// =============================================================================
// CapacityErrorResponse Tests
// =============================================================================

func TestCapacityErrorResponse_Render(t *testing.T) {
	resp := &common.CapacityErrorResponse{
		Status:  "error",
		Message: "Room capacity exceeded",
		Code:    "ROOM_CAPACITY_EXCEEDED",
		Details: &common.RoomCapacityExceededError{
			RoomID:           1,
			RoomName:         "Room A",
			CurrentOccupancy: 30,
			MaxCapacity:      25,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := resp.Render(w, r)
	require.NoError(t, err)
}

// =============================================================================
// ActivityCapacityErrorResponse Tests
// =============================================================================

func TestActivityCapacityErrorResponse_Render(t *testing.T) {
	resp := &common.ActivityCapacityErrorResponse{
		Status:  "error",
		Message: "Activity capacity exceeded",
		Code:    "ACTIVITY_CAPACITY_EXCEEDED",
		Details: &common.ActivityCapacityExceededError{
			ActivityID:       42,
			ActivityName:     "Basketball",
			CurrentOccupancy: 20,
			MaxCapacity:      15,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := resp.Render(w, r)
	require.NoError(t, err)
}

// =============================================================================
// ErrorRoomCapacityExceeded Tests
// =============================================================================

func TestErrorRoomCapacityExceeded(t *testing.T) {
	renderer := common.ErrorRoomCapacityExceeded(1, "Room A", 30, 25)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// =============================================================================
// ErrorActivityCapacityExceeded Tests
// =============================================================================

func TestErrorActivityCapacityExceeded(t *testing.T) {
	renderer := common.ErrorActivityCapacityExceeded(42, "Basketball", 20, 15)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// =============================================================================
// Error Helper Function Tests
// =============================================================================

func TestErrorInvalidRequest(t *testing.T) {
	testErr := errors.New("invalid data")
	renderer := common.ErrorInvalidRequest(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("server error")
	renderer := common.ErrorInternalServer(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestErrorNotFound(t *testing.T) {
	testErr := errors.New("not found")
	renderer := common.ErrorNotFound(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestErrorConflict(t *testing.T) {
	testErr := errors.New("conflict")
	renderer := common.ErrorConflict(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// =============================================================================
// ErrorRenderer Tests
// =============================================================================

func TestErrorRenderer_IoTServiceError_NotFound(t *testing.T) {
	iotErr := &iotSvc.IoTError{Err: iotSvc.ErrDeviceNotFound}
	renderer := common.ErrorRenderer(iotErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestErrorRenderer_IoTServiceError_InvalidData(t *testing.T) {
	iotErr := &iotSvc.IoTError{Err: iotSvc.ErrInvalidDeviceData}
	renderer := common.ErrorRenderer(iotErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestErrorRenderer_IoTServiceError_Duplicate(t *testing.T) {
	iotErr := &iotSvc.IoTError{Err: iotSvc.ErrDuplicateDeviceID}
	renderer := common.ErrorRenderer(iotErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestErrorRenderer_IoTServiceError_DeviceOffline(t *testing.T) {
	iotErr := &iotSvc.IoTError{Err: iotSvc.ErrDeviceOffline}
	renderer := common.ErrorRenderer(iotErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestErrorRenderer_IoTServiceError_NetworkScan(t *testing.T) {
	iotErr := &iotSvc.IoTError{Err: iotSvc.ErrNetworkScanFailed}
	renderer := common.ErrorRenderer(iotErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestErrorRenderer_IoTServiceError_Database(t *testing.T) {
	iotErr := &iotSvc.IoTError{Err: errors.New("database failure")}
	renderer := common.ErrorRenderer(iotErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestErrorRenderer_ActiveServiceError_Conflict(t *testing.T) {
	activeErr := &activeSvc.ActiveError{Err: activeSvc.ErrRoomConflict}
	renderer := common.ErrorRenderer(activeErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestErrorRenderer_ActiveServiceError_NotFound(t *testing.T) {
	activeErr := &activeSvc.ActiveError{Err: activeSvc.ErrActiveGroupNotFound}
	renderer := common.ErrorRenderer(activeErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestErrorRenderer_ActiveServiceError_Validation(t *testing.T) {
	activeErr := &activeSvc.ActiveError{Err: activeSvc.ErrActiveGroupAlreadyEnded}
	renderer := common.ErrorRenderer(activeErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestErrorRenderer_ActiveServiceError_Database(t *testing.T) {
	activeErr := &activeSvc.ActiveError{Err: activeSvc.ErrDatabaseOperation}
	renderer := common.ErrorRenderer(activeErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestErrorRenderer_FeedbackServiceError_NotFound(t *testing.T) {
	renderer := common.ErrorRenderer(feedbackSvc.ErrEntryNotFound)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestErrorRenderer_FeedbackServiceError_InvalidData(t *testing.T) {
	renderer := common.ErrorRenderer(feedbackSvc.ErrInvalidEntryData)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestErrorRenderer_FeedbackServiceError_InvalidEntryDataError(t *testing.T) {
	feedbackErr := &feedbackSvc.InvalidEntryDataError{Err: errors.New("invalid")}
	renderer := common.ErrorRenderer(feedbackErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	unknownErr := errors.New("unknown error")
	renderer := common.ErrorRenderer(unknownErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// =============================================================================
// Error Constants Tests
// =============================================================================

func TestErrorConstants(t *testing.T) {
	assert.NotNil(t, common.ErrRoomCapacityExceeded)
	assert.NotNil(t, common.ErrActivityCapacityExceeded)
}

func TestErrorMessageConstants(t *testing.T) {
	assert.Equal(t, "person is not a student", common.ErrMsgPersonNotStudent)
	assert.Equal(t, "RFID tag not found", common.ErrMsgRFIDTagNotFound)
}

// =============================================================================
// RenderError Tests
// =============================================================================

func TestRenderError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	renderer := common.ErrorInvalidRequest(errors.New("test"))
	common.RenderError(w, r, renderer)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
