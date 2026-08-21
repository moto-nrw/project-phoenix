package shared_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	shared "github.com/moto-nrw/project-phoenix/api/iot/internal/shared"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorMessageConstants(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, shared.ErrMsgPersonNotStudent)
	assert.NotEmpty(t, shared.ErrMsgRFIDTagNotFound)
}

// Test RoomCapacityExceededError
func TestErrorRenderer_IoTErrors(t *testing.T) {
	t.Parallel()

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
			renderer := shared.ErrorRenderer(tt.err)
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
	t.Parallel()

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
		{"ErrNoRoomAvailable", &activeSvc.ActiveError{Err: activeSvc.ErrNoRoomAvailable}},
		// Database errors
		{"ErrDatabaseOperation", &activeSvc.ActiveError{Err: activeSvc.ErrDatabaseOperation}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := shared.ErrorRenderer(tt.err)
			assert.NotNil(t, renderer)
		})
	}
}

func TestErrorRenderer_NoRoomAvailableUsesKioskContract(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/iot/sessions", nil)
	require.NoError(t, render.Render(rec, req, shared.ErrorRenderer(activeSvc.ErrNoRoomAvailable)))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "no room available for this activity")
}

// Test ErrorRenderer for Feedback Service Errors
func TestErrorRenderer_FeedbackErrors(t *testing.T) {
	t.Parallel()

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
			renderer := shared.ErrorRenderer(tt.err)
			assert.NotNil(t, renderer)
		})
	}
}

// Test ErrorRenderer for Unknown Errors
func TestErrorRenderer_UnknownError(t *testing.T) {
	t.Parallel()

	unknownErr := errors.New("unknown error")
	renderer := shared.ErrorRenderer(unknownErr)
	assert.NotNil(t, renderer)
}

func TestErrorRenderer_RoomCapacityExceeded(t *testing.T) {
	t.Parallel()

	renderer := shared.ErrorRenderer(&activeSvc.RoomCapacityError{
		RoomID:           42,
		RoomName:         "Mensa",
		CurrentOccupancy: 43,
		MaxCapacity:      43,
	})
	require.NotNil(t, renderer)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/iot/sessions", nil)
	require.NoError(t, render.Render(rec, req, renderer))

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "room capacity exceeded")
}

// Test Capacity Error Response Render

// TestErrorRenderer_StudentGraduatedUsesContractString covers the #405 review
// fix: when a graduation commits between the kiosk's student lookup and the
// attendance/visit write, the wrapped ActiveError carries "student has graduated
// and cannot check in" — a string PyrePortal has no mapping for. The response
// must be the documented 404 "person is not a student" instead, which the kiosk
// already renders in German.
func TestErrorRenderer_StudentGraduatedUsesContractString(t *testing.T) {
	t.Parallel()

	renderer := shared.ErrorRenderer(&activeSvc.ActiveError{
		Op:  "create visit",
		Err: activeSvc.ErrStudentGraduated,
	})
	require.NotNil(t, renderer)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/iot/checkin", nil)
	require.NoError(t, render.Render(rec, req, renderer))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), shared.ErrMsgPersonNotStudent)
	assert.NotContains(t, rec.Body.String(), "graduated",
		"the kiosk contract string must not leak the internal graduation wording")
}
