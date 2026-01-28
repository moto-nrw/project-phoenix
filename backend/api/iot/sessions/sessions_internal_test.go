// Package sessions internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package sessions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// =============================================================================
// filterActiveSupervisors TESTS
// =============================================================================

func TestFilterActiveSupervisors_AllActive(t *testing.T) {
	rs := &Resource{}
	supervisors := []*active.GroupSupervisor{
		{StaffID: 1, EndDate: nil},
		{StaffID: 2, EndDate: nil},
		{StaffID: 3, EndDate: nil},
	}

	result := rs.filterActiveSupervisors(supervisors)

	assert.Len(t, result, 3)
}

func TestFilterActiveSupervisors_SomeEnded(t *testing.T) {
	rs := &Resource{}
	endDate := time.Now()
	supervisors := []*active.GroupSupervisor{
		{StaffID: 1, EndDate: nil},
		{StaffID: 2, EndDate: &endDate}, // Ended
		{StaffID: 3, EndDate: nil},
	}

	result := rs.filterActiveSupervisors(supervisors)

	assert.Len(t, result, 2)
	assert.Equal(t, int64(1), result[0].StaffID)
	assert.Equal(t, int64(3), result[1].StaffID)
}

func TestFilterActiveSupervisors_AllEnded(t *testing.T) {
	rs := &Resource{}
	endDate := time.Now()
	supervisors := []*active.GroupSupervisor{
		{StaffID: 1, EndDate: &endDate},
		{StaffID: 2, EndDate: &endDate},
	}

	result := rs.filterActiveSupervisors(supervisors)

	assert.Empty(t, result)
}

func TestFilterActiveSupervisors_Empty(t *testing.T) {
	rs := &Resource{}

	result := rs.filterActiveSupervisors([]*active.GroupSupervisor{})

	assert.Empty(t, result)
}

func TestFilterActiveSupervisors_ZeroStaffID(t *testing.T) {
	rs := &Resource{}
	supervisors := []*active.GroupSupervisor{
		{StaffID: 1, EndDate: nil},
		{StaffID: 0, EndDate: nil}, // Invalid StaffID
		{StaffID: 2, EndDate: nil},
	}

	result := rs.filterActiveSupervisors(supervisors)

	assert.Len(t, result, 2)
}

// =============================================================================
// countActiveStudents TESTS
// =============================================================================

func TestCountActiveStudents_AllActive(t *testing.T) {
	visits := []*active.Visit{
		{Model: base.Model{ID: 1}, StudentID: 1, ExitTime: nil},
		{Model: base.Model{ID: 2}, StudentID: 2, ExitTime: nil},
		{Model: base.Model{ID: 3}, StudentID: 3, ExitTime: nil},
	}

	count := countActiveStudents(visits)

	assert.Equal(t, 3, count)
}

func TestCountActiveStudents_SomeExited(t *testing.T) {
	exitTime := time.Now()
	visits := []*active.Visit{
		{Model: base.Model{ID: 1}, StudentID: 1, ExitTime: nil},
		{Model: base.Model{ID: 2}, StudentID: 2, ExitTime: &exitTime}, // Exited
		{Model: base.Model{ID: 3}, StudentID: 3, ExitTime: nil},
	}

	count := countActiveStudents(visits)

	assert.Equal(t, 2, count)
}

func TestCountActiveStudents_AllExited(t *testing.T) {
	exitTime := time.Now()
	visits := []*active.Visit{
		{Model: base.Model{ID: 1}, StudentID: 1, ExitTime: &exitTime},
		{Model: base.Model{ID: 2}, StudentID: 2, ExitTime: &exitTime},
	}

	count := countActiveStudents(visits)

	assert.Equal(t, 0, count)
}

func TestCountActiveStudents_Empty(t *testing.T) {
	count := countActiveStudents([]*active.Visit{})

	assert.Equal(t, 0, count)
}

func TestCountActiveStudents_Nil(t *testing.T) {
	count := countActiveStudents(nil)

	assert.Equal(t, 0, count)
}

// =============================================================================
// Types TESTS
// =============================================================================

func TestSessionStartRequest_Fields(t *testing.T) {
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
	now := time.Now()
	req := SessionActivityRequest{
		ActivityType: "rfid_scan",
		Timestamp:    now,
	}

	assert.Equal(t, "rfid_scan", req.ActivityType)
	assert.Equal(t, now, req.Timestamp)
}

func TestUpdateSupervisorsRequest_Fields(t *testing.T) {
	req := UpdateSupervisorsRequest{
		SupervisorIDs: []int64{1, 2, 3},
	}

	assert.Equal(t, []int64{1, 2, 3}, req.SupervisorIDs)
}

func TestSupervisorInfo_Fields(t *testing.T) {
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
	now := time.Now()
	supervisors := []SupervisorInfo{
		{StaffID: 1, DisplayName: "Test User"},
	}
	conflictInfo := &ConflictInfoResponse{HasConflict: false}

	resp := SessionStartResponse{
		ActiveGroupID: 123,
		ActivityID:    456,
		DeviceID:      789,
		StartTime:     now,
		Status:        "started",
		Message:       "Session started",
		Supervisors:   supervisors,
		ConflictInfo:  conflictInfo,
	}

	assert.Equal(t, int64(123), resp.ActiveGroupID)
	assert.Equal(t, int64(456), resp.ActivityID)
	assert.Equal(t, int64(789), resp.DeviceID)
	assert.Equal(t, now, resp.StartTime)
	assert.Equal(t, "started", resp.Status)
	assert.Equal(t, "Session started", resp.Message)
	assert.Len(t, resp.Supervisors, 1)
	assert.NotNil(t, resp.ConflictInfo)
}
