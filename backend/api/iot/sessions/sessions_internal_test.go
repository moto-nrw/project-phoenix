package sessions

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSupervisorsResponse_EmptySupervisorSliceSerializesAsJSONArray(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(UpdateSupervisorsResponse{
		ActiveGroupID: 77,
		Supervisors:   []SupervisorInfo{},
		Status:        "success",
		Message:       "Supervisors updated successfully",
	})

	require.NoError(t, err)
	assert.Contains(t, string(payload), `"supervisors":[]`)
}

// =============================================================================
// Types TESTS
// =============================================================================

func TestSessionStartRequest_Fields(t *testing.T) {
	t.Parallel()

	roomID := int64(5)
	req := SessionStartRequest{
		ActivityID:    123,
		SupervisorIDs: []int64{1, 2, 3},
		Force:         true,
		RoomID:        &roomID,
	}

	assert.Equal(t, int64(123), req.ActivityID)
	assert.Equal(t, []int64{1, 2, 3}, req.SupervisorIDs)
	assert.True(t, req.Force)
	assert.Equal(t, int64(5), *req.RoomID)
}

func TestSessionActivityRequest_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	req := SessionActivityRequest{
		ActivityType: "rfid_scan",
		Timestamp:    now,
	}

	assert.Equal(t, "rfid_scan", req.ActivityType)
	assert.Equal(t, now, req.Timestamp)
}

func TestUpdateSupervisorsRequest_Fields(t *testing.T) {
	t.Parallel()

	req := UpdateSupervisorsRequest{
		SupervisorIDs: []int64{1, 2, 3},
	}

	assert.Equal(t, []int64{1, 2, 3}, req.SupervisorIDs)
}

func TestSupervisorInfo_Fields(t *testing.T) {
	t.Parallel()

	info := SupervisorInfo{
		StaffID:     789,
		FirstName:   "Test",
		LastName:    "Supervisor",
		DisplayName: "Test Supervisor",
		Role:        "lead",
	}

	assert.Equal(t, int64(789), info.StaffID)
	assert.Equal(t, "Test", info.FirstName)
	assert.Equal(t, "Supervisor", info.LastName)
	assert.Equal(t, "Test Supervisor", info.DisplayName)
	assert.Equal(t, "lead", info.Role)
}

func TestConflictInfoResponse_Fields(t *testing.T) {
	t.Parallel()

	deviceID := int64(100)
	info := ConflictInfoResponse{
		HasConflict:       true,
		ConflictMessage:   "Device is already active",
		CanOverride:       true,
		ConflictingDevice: &deviceID,
	}

	assert.True(t, info.HasConflict)
	assert.Equal(t, "Device is already active", info.ConflictMessage)
	assert.True(t, info.CanOverride)
	assert.Equal(t, int64(100), *info.ConflictingDevice)
}

func TestSessionStartResponse_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	supervisors := []SupervisorInfo{
		{StaffID: 1, DisplayName: "Test User"},
	}
	conflictInfo := &ConflictInfoResponse{HasConflict: false}

	activityID := int64(456)
	resp := SessionStartResponse{
		ActiveGroupID: 123,
		ActivityID:    &activityID,
		DeviceID:      789,
		StartTime:     now,
		Status:        "started",
		Message:       "Session started",
		Supervisors:   supervisors,
		ConflictInfo:  conflictInfo,
	}

	assert.Equal(t, int64(123), resp.ActiveGroupID)
	require.NotNil(t, resp.ActivityID)
	assert.Equal(t, int64(456), *resp.ActivityID)
	assert.Equal(t, int64(789), resp.DeviceID)
	assert.Equal(t, now, resp.StartTime)
	assert.Equal(t, "started", resp.Status)
	assert.Equal(t, "Session started", resp.Message)
	assert.Len(t, resp.Supervisors, 1)
	assert.NotNil(t, resp.ConflictInfo)
}
