// Capacity/conflict error tests moved verbatim from api/iot/common (issue #575 B7).
package checkin_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	checkin "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrRoomCapacityExceeded", checkin.ErrRoomCapacityExceeded},
		{"ErrActivityCapacityExceeded", checkin.ErrActivityCapacityExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}

// Test Error Message Constants
func TestRoomCapacityExceededError_Error(t *testing.T) {
	t.Parallel()

	err := &checkin.RoomCapacityExceededError{
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
	t.Parallel()

	err := &checkin.ActivityCapacityExceededError{
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
	t.Parallel()

	renderer := checkin.ErrorRoomCapacityExceeded(42, "Test Room", 15, 10)
	resp, ok := renderer.(*checkin.CapacityErrorResponse)
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
	t.Parallel()

	renderer := checkin.ErrorActivityCapacityExceeded(77, "Test Activity", 25, 20)
	resp, ok := renderer.(*checkin.ActivityCapacityErrorResponse)
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
func TestCapacityErrorResponse_Render(t *testing.T) {
	t.Parallel()

	resp := &checkin.CapacityErrorResponse{
		Status:  "error",
		Message: "Room capacity exceeded",
		Code:    "ROOM_CAPACITY_EXCEEDED",
		Details: &checkin.RoomCapacityExceededError{
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
	t.Parallel()

	resp := &checkin.ActivityCapacityErrorResponse{
		Status:  "error",
		Message: "Activity capacity exceeded",
		Code:    "ACTIVITY_CAPACITY_EXCEEDED",
		Details: &checkin.ActivityCapacityExceededError{
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
	t.Parallel()

	renderer := checkin.ErrorStudentAlreadyActive(int64(42), 0, nil, nil, "")
	resp, ok := renderer.(*checkin.StudentAlreadyActiveErrorResponse)
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
	t.Parallel()

	entryTime := time.Date(2026, 4, 29, 14, 16, 30, 0, time.UTC)
	roomID := int64(7)
	renderer := checkin.ErrorStudentAlreadyActive(int64(42), int64(101), &entryTime, &roomID, "Klassenzimmer 2")
	resp, ok := renderer.(*checkin.StudentAlreadyActiveErrorResponse)
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
