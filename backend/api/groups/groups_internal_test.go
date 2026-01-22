// Package groups internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package groups

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Error Helper Tests
// =============================================================================

func TestErrorConflict_ReturnsCorrectStatus(t *testing.T) {
	err := errors.New("conflict error")
	response := ErrorConflict(err)

	errResp, ok := response.(*ErrorResponse)
	assert.True(t, ok, "Expected *ErrorResponse")
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Equal(t, "conflict error", errResp.ErrorText)
}

func TestErrorInternalServer_ReturnsCorrectStatus(t *testing.T) {
	err := errors.New("internal server error")
	response := ErrorInternalServer(err)

	errResp, ok := response.(*ErrorResponse)
	assert.True(t, ok, "Expected *ErrorResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Equal(t, "internal server error", errResp.ErrorText)
}

func TestErrorInternalServer_EmptyMessage(t *testing.T) {
	err := errors.New("")
	response := ErrorInternalServer(err)

	errResp, ok := response.(*ErrorResponse)
	assert.True(t, ok, "Expected *ErrorResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "", errResp.ErrorText)
}

// =============================================================================
// buildNoRoomResponse Tests
// =============================================================================

func TestBuildNoRoomResponse_EmptyStudents(t *testing.T) {
	students := []*users.Student{}

	result := buildNoRoomResponse(students)

	assert.False(t, result["group_has_room"].(bool))
	statusMap := result["student_room_status"].(map[string]interface{})
	assert.Empty(t, statusMap)
}

func TestBuildNoRoomResponse_SingleStudent(t *testing.T) {
	students := []*users.Student{
		{TenantModel: base.TenantModel{Model: base.Model{ID: 1}}},
	}

	result := buildNoRoomResponse(students)

	assert.False(t, result["group_has_room"].(bool))
	statusMap := result["student_room_status"].(map[string]interface{})
	assert.Len(t, statusMap, 1)

	studentStatus := statusMap["1"].(map[string]interface{})
	assert.False(t, studentStatus["in_group_room"].(bool))
	assert.Equal(t, "group_no_room", studentStatus["reason"])
}

func TestBuildNoRoomResponse_MultipleStudents(t *testing.T) {
	students := []*users.Student{
		{TenantModel: base.TenantModel{Model: base.Model{ID: 10}}},
		{TenantModel: base.TenantModel{Model: base.Model{ID: 20}}},
		{TenantModel: base.TenantModel{Model: base.Model{ID: 30}}},
	}

	result := buildNoRoomResponse(students)

	assert.False(t, result["group_has_room"].(bool))
	statusMap := result["student_room_status"].(map[string]interface{})
	assert.Len(t, statusMap, 3)

	// Verify all students have correct status
	for _, id := range []string{"10", "20", "30"} {
		studentStatus := statusMap[id].(map[string]interface{})
		assert.False(t, studentStatus["in_group_room"].(bool))
		assert.Equal(t, "group_no_room", studentStatus["reason"])
	}
}

func TestBuildNoRoomResponse_NilStudentsList(t *testing.T) {
	var students []*users.Student

	result := buildNoRoomResponse(students)

	assert.False(t, result["group_has_room"].(bool))
	statusMap := result["student_room_status"].(map[string]interface{})
	assert.Empty(t, statusMap)
}

// =============================================================================
// Request Type Tests
// =============================================================================

func TestGroupRequest_Fields(t *testing.T) {
	roomID := int64(5)
	req := GroupRequest{
		Name:       "Test Group",
		RoomID:     &roomID,
		TeacherIDs: []int64{10, 20},
	}

	assert.Equal(t, "Test Group", req.Name)
	assert.Equal(t, int64(5), *req.RoomID)
	assert.Len(t, req.TeacherIDs, 2)
}

func TestGroupRequest_Bind_Valid(t *testing.T) {
	req := GroupRequest{Name: "Valid Group"}
	err := req.Bind(nil)
	assert.NoError(t, err)
}

func TestGroupRequest_Bind_EmptyName(t *testing.T) {
	req := GroupRequest{Name: ""}
	err := req.Bind(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestTransferGroupRequest_Fields(t *testing.T) {
	req := TransferGroupRequest{
		TargetUserID: 100,
	}
	assert.Equal(t, int64(100), req.TargetUserID)
}

func TestTransferGroupRequest_Bind_Valid(t *testing.T) {
	req := TransferGroupRequest{TargetUserID: 100}
	err := req.Bind(nil)
	assert.NoError(t, err)
}

func TestTransferGroupRequest_Bind_ZeroID(t *testing.T) {
	req := TransferGroupRequest{TargetUserID: 0}
	err := req.Bind(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target_user_id is required")
}

func TestTransferGroupRequest_Bind_NegativeID(t *testing.T) {
	req := TransferGroupRequest{TargetUserID: -1}
	err := req.Bind(nil)
	assert.Error(t, err)
}

// =============================================================================
// Response Type Tests
// =============================================================================

func TestGroupResponse_Fields(t *testing.T) {
	roomID := int64(100)
	repID := int64(200)
	now := time.Now()

	resp := GroupResponse{
		ID:               1,
		Name:             "Response Group",
		RoomID:           &roomID,
		RepresentativeID: &repID,
		StudentCount:     25,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, "Response Group", resp.Name)
	assert.Equal(t, int64(100), *resp.RoomID)
	assert.Equal(t, int64(200), *resp.RepresentativeID)
	assert.Equal(t, 25, resp.StudentCount)
}

func TestTeacherResponse_Fields(t *testing.T) {
	resp := TeacherResponse{
		ID:             1,
		StaffID:        2,
		FirstName:      "John",
		LastName:       "Doe",
		Specialization: "Math",
		Role:           "lead",
		FullName:       "John Doe",
	}

	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, int64(2), resp.StaffID)
	assert.Equal(t, "John", resp.FirstName)
	assert.Equal(t, "Doe", resp.LastName)
	assert.Equal(t, "Math", resp.Specialization)
	assert.Equal(t, "lead", resp.Role)
}

func TestErrorResponse_Fields(t *testing.T) {
	errResp := &ErrorResponse{
		HTTPStatusCode: http.StatusBadRequest,
		Status:         "error",
		ErrorText:      "bad request",
	}

	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Equal(t, "bad request", errResp.ErrorText)
}

// NOTE: NewResource requires non-nil services, tested via integration tests in groups_test.go
