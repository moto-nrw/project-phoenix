// Package activities internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package httpadapter_test

import (
	"testing"
	"time"

	activityContract "github.com/moto-ogs/project-phoenix/modules/timetable/activitycontract"

	httpadapter "github.com/moto-ogs/project-phoenix/modules/timetable/http/activities"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// FormatEndTimeForTest Tests
// =============================================================================

func TestFormatEndTime_WithTime(t *testing.T) {
	t.Parallel()

	endTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	result := httpadapter.FormatEndTimeForTest(&endTime)
	assert.Equal(t, "14:30", result)
}

func TestFormatEndTime_NilTime(t *testing.T) {
	t.Parallel()

	result := httpadapter.FormatEndTimeForTest(nil)
	assert.Equal(t, "", result)
}

func TestFormatEndTime_Midnight(t *testing.T) {
	t.Parallel()

	endTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := httpadapter.FormatEndTimeForTest(&endTime)
	assert.Equal(t, "00:00", result)
}

func TestFormatEndTime_BeforeMidnight(t *testing.T) {
	t.Parallel()

	endTime := time.Date(2024, 1, 15, 23, 59, 0, 0, time.UTC)
	result := httpadapter.FormatEndTimeForTest(&endTime)
	assert.Equal(t, "23:59", result)
}

// =============================================================================
// Request Type Tests
// =============================================================================

func TestQuickActivityRequest_Fields(t *testing.T) {
	t.Parallel()

	roomID := int64(10)
	req := httpadapter.QuickActivityRequest{
		Name:            "Test Activity",
		CategoryID:      5,
		RoomID:          &roomID,
		MaxParticipants: 20,
	}

	assert.Equal(t, "Test Activity", req.Name)
	assert.Equal(t, int64(5), req.CategoryID)
	assert.Equal(t, int64(10), *req.RoomID)
	assert.Equal(t, 20, req.MaxParticipants)
}

func TestQuickActivityRequest_NilRoomID(t *testing.T) {
	t.Parallel()

	req := httpadapter.QuickActivityRequest{
		Name:            "Test Activity",
		CategoryID:      5,
		RoomID:          nil,
		MaxParticipants: 15,
	}

	assert.Equal(t, "Test Activity", req.Name)
	assert.Nil(t, req.RoomID)
}

func TestActivityRequest_Fields(t *testing.T) {
	t.Parallel()

	roomID := int64(20)
	req := httpadapter.ActivityRequest{
		Name:            "Test Activity",
		MaxParticipants: 30,
		IsOpen:          true,
		CategoryID:      10,
		PlannedRoomID:   &roomID,
		SupervisorIDs:   []int64{1, 2},
	}

	assert.Equal(t, "Test Activity", req.Name)
	assert.Equal(t, 30, req.MaxParticipants)
	assert.True(t, req.IsOpen)
	assert.Equal(t, int64(10), req.CategoryID)
	assert.Equal(t, int64(20), *req.PlannedRoomID)
	assert.Len(t, req.SupervisorIDs, 2)
}

func TestScheduleRequest_Fields(t *testing.T) {
	t.Parallel()

	timeframeID := int64(5)
	req := httpadapter.ScheduleRequest{
		Weekday:     1, // Monday
		TimeframeID: &timeframeID,
	}

	assert.Equal(t, 1, req.Weekday)
	assert.Equal(t, int64(5), *req.TimeframeID)
}

func TestScheduleRequest_NilTimeframeID(t *testing.T) {
	t.Parallel()

	req := httpadapter.ScheduleRequest{
		Weekday:     2, // Tuesday
		TimeframeID: nil,
	}

	assert.Equal(t, 2, req.Weekday)
	assert.Nil(t, req.TimeframeID)
}

func TestSupervisorRequest_Fields(t *testing.T) {
	t.Parallel()

	req := httpadapter.SupervisorRequest{
		StaffID:   123,
		IsPrimary: true,
	}

	assert.Equal(t, int64(123), req.StaffID)
	assert.True(t, req.IsPrimary)
}

func TestBatchEnrollmentRequest_Fields(t *testing.T) {
	t.Parallel()

	req := httpadapter.BatchEnrollmentRequest{
		StudentIDs: []int64{1, 2, 3},
	}

	assert.Len(t, req.StudentIDs, 3)
	assert.Equal(t, int64(1), req.StudentIDs[0])
}

func TestBatchEnrollmentRequest_EmptyList(t *testing.T) {
	t.Parallel()

	req := httpadapter.BatchEnrollmentRequest{
		StudentIDs: []int64{},
	}

	assert.Empty(t, req.StudentIDs)
}

// =============================================================================
// AddSupervisorsToResponseForTest Tests
// =============================================================================

func TestAddSupervisorsToResponse_EmptyList(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{}
	httpadapter.AddSupervisorsToResponseForTest(response, []*activityContract.SupervisorPlanned{})
	assert.Nil(t, response.SupervisorIDs)
	assert.Nil(t, response.Supervisors)
}

func TestAddSupervisorsToResponse_NilList(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{}
	httpadapter.AddSupervisorsToResponseForTest(response, nil)
	assert.Nil(t, response.SupervisorIDs)
	assert.Nil(t, response.Supervisors)
}

func TestAddSupervisorsToResponse_WithNilSupervisorInList(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{}
	sup := &activityContract.SupervisorPlanned{
		ID:        1,
		StaffID:   1,
		IsPrimary: false,
	}
	supervisors := []*activityContract.SupervisorPlanned{
		nil,
		sup,
		nil,
	}
	httpadapter.AddSupervisorsToResponseForTest(response, supervisors)
	assert.Len(t, response.SupervisorIDs, 1)
	assert.Equal(t, int64(1), response.SupervisorIDs[0])
}

func TestAddSupervisorsToResponse_WithPrimarySupervisor(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{}
	supervisors := []*activityContract.SupervisorPlanned{
		{ID: 1, StaffID: 1, IsPrimary: false},
		{ID: 2, StaffID: 2, IsPrimary: true},
		{ID: 3, StaffID: 3, IsPrimary: false},
	}
	httpadapter.AddSupervisorsToResponseForTest(response, supervisors)
	assert.Len(t, response.SupervisorIDs, 3)
	assert.NotNil(t, response.SupervisorID)
	assert.Equal(t, int64(2), *response.SupervisorID)
}

func TestAddSupervisorsToResponse_NoPrimarySupervisor(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{}
	supervisors := []*activityContract.SupervisorPlanned{
		{ID: 1, StaffID: 1, IsPrimary: false},
		{ID: 2, StaffID: 2, IsPrimary: false},
	}
	httpadapter.AddSupervisorsToResponseForTest(response, supervisors)
	assert.Len(t, response.SupervisorIDs, 2)
	assert.Nil(t, response.SupervisorID)
}

// =============================================================================
// AddSchedulesToResponseForTest Tests
// =============================================================================

func TestAddSchedulesToResponse_EmptyList(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{Schedules: []httpadapter.ScheduleResponse{}}
	httpadapter.AddSchedulesToResponseForTest(response, []*activityContract.Schedule{})
	assert.Empty(t, response.Schedules)
}

func TestAddSchedulesToResponse_NilList(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{Schedules: []httpadapter.ScheduleResponse{}}
	httpadapter.AddSchedulesToResponseForTest(response, nil)
	assert.Empty(t, response.Schedules)
}

func TestAddSchedulesToResponse_WithNilScheduleInList(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{Schedules: []httpadapter.ScheduleResponse{}}
	sch := &activityContract.Schedule{
		ID:      1,
		Weekday: 1,
	}
	schedules := []*activityContract.Schedule{
		nil,
		sch,
		nil,
	}
	httpadapter.AddSchedulesToResponseForTest(response, schedules)
	assert.Len(t, response.Schedules, 1)
	assert.Equal(t, int64(1), response.Schedules[0].ID)
}

func TestAddSchedulesToResponse_WithMultipleSchedules(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{Schedules: []httpadapter.ScheduleResponse{}}
	timeframeID := int64(5)
	schedules := []*activityContract.Schedule{
		{ID: 1, Weekday: 1, TimeframeID: &timeframeID},
		{ID: 2, Weekday: 3, TimeframeID: nil},
	}
	httpadapter.AddSchedulesToResponseForTest(response, schedules)
	assert.Len(t, response.Schedules, 2)
}

// =============================================================================
// AddCategoryToResponseForTest Tests
// =============================================================================

func TestAddCategoryToResponse_NilCategory(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{}
	group := &activityContract.Group{Category: nil}
	httpadapter.AddCategoryToResponseForTest(response, group)
	assert.Nil(t, response.Category)
}

func TestAddCategoryToResponse_WithCategory(t *testing.T) {
	t.Parallel()

	response := &httpadapter.ActivityResponse{}
	group := &activityContract.Group{
		Category: &activityContract.Category{
			ID:   1,
			Name: "Test Category",
		},
	}
	httpadapter.AddCategoryToResponseForTest(response, group)
	assert.NotNil(t, response.Category)
	assert.Equal(t, int64(1), response.Category.ID)
	assert.Equal(t, "Test Category", response.Category.Name)
}

// =============================================================================
// BuildBaseActivityResponseForTest Tests
// =============================================================================

func TestBuildBaseActivityResponse_BasicFields(t *testing.T) {
	t.Parallel()

	roomID := int64(10)
	now := time.Now()
	group := &activityContract.Group{
		ID: 1, CreatedAt: now, UpdatedAt: now,
		Name:            "Test Activity",
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      5,
		PlannedRoomID:   &roomID,
	}

	response := httpadapter.BuildBaseActivityResponseForTest(group, 15)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "Test Activity", response.Name)
	assert.NotNil(t, response.MaxParticipants)
	assert.Equal(t, 20, *response.MaxParticipants)
	assert.True(t, response.IsOpen)
	assert.Equal(t, int64(5), response.CategoryID)
	assert.Equal(t, int64(10), *response.PlannedRoomID)
	assert.Equal(t, 15, response.EnrollmentCount)
	assert.Empty(t, response.Schedules)
}

func TestBuildBaseActivityResponse_UnlimitedCapacity(t *testing.T) {
	t.Parallel()

	group := &activityContract.Group{
		Name:            "Open Sports Hall",
		MaxParticipants: 0,
		CategoryID:      5,
	}

	response := httpadapter.BuildBaseActivityResponseForTest(group, 75)

	assert.Nil(t, response.MaxParticipants)
}

func TestBuildBaseActivityResponse_NilRoomID(t *testing.T) {
	t.Parallel()

	group := &activityContract.Group{
		ID:            1,
		Name:          "Test Activity",
		PlannedRoomID: nil,
	}

	response := httpadapter.BuildBaseActivityResponseForTest(group, 0)
	assert.Nil(t, response.PlannedRoomID)
}

// =============================================================================
// UpdateGroupFieldsForTest Tests
// =============================================================================

func TestUpdateGroupFields_AllFields(t *testing.T) {
	t.Parallel()

	roomID := int64(25)
	group := &activityContract.Group{}
	req := &httpadapter.ActivityRequest{
		Name:            "Updated Name",
		MaxParticipants: 50,
		IsOpen:          true,
		CategoryID:      10,
		PlannedRoomID:   &roomID,
	}

	httpadapter.UpdateGroupFieldsForTest(group, req)

	assert.Equal(t, "Updated Name", group.Name)
	assert.Equal(t, 50, group.MaxParticipants)
	assert.True(t, group.IsOpen)
	assert.Equal(t, int64(10), group.CategoryID)
	assert.Equal(t, int64(25), *group.PlannedRoomID)
}

func TestUpdateGroupFields_NilRoomID(t *testing.T) {
	t.Parallel()

	roomID := int64(10)
	group := &activityContract.Group{
		PlannedRoomID: &roomID,
	}

	req := &httpadapter.ActivityRequest{
		Name:          "Test",
		PlannedRoomID: nil,
	}

	httpadapter.UpdateGroupFieldsForTest(group, req)
	assert.Nil(t, group.PlannedRoomID)
}

// =============================================================================
// NewCategoryResponseForTest Tests
// =============================================================================

func TestNewCategoryResponse_FullCategory(t *testing.T) {
	t.Parallel()

	category := &activityContract.Category{
		ID:          1,
		Name:        "Sports",
		Description: "Sports activities",
		Color:       "#FF0000",
	}

	response := httpadapter.NewCategoryResponseForTest(category)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "Sports", response.Name)
	assert.Equal(t, "Sports activities", response.Description)
	assert.Equal(t, "#FF0000", response.Color)
}

func TestNewCategoryResponse_MinimalCategory(t *testing.T) {
	t.Parallel()

	category := &activityContract.Category{
		ID:   1,
		Name: "Minimal",
	}

	response := httpadapter.NewCategoryResponseForTest(category)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "Minimal", response.Name)
	assert.Empty(t, response.Description)
	assert.Empty(t, response.Color)
}

func TestNewCategoryResponse_NilCategory(t *testing.T) {
	t.Parallel()

	response := httpadapter.NewCategoryResponseForTest(nil)
	assert.Equal(t, "Unknown Category", response.Name)
}

// =============================================================================
// NewScheduleResponseForTest Tests
// =============================================================================

func TestNewScheduleResponse_Basic(t *testing.T) {
	t.Parallel()

	schedule := &activityContract.Schedule{
		ID:      1,
		Weekday: 1,
	}

	response := httpadapter.NewScheduleResponseForTest(schedule)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, 1, response.Weekday)
}

func TestNewScheduleResponse_WithTimeframeID(t *testing.T) {
	t.Parallel()

	timeframeID := int64(5)
	schedule := &activityContract.Schedule{
		ID:          1,
		Weekday:     1,
		TimeframeID: &timeframeID,
	}

	response := httpadapter.NewScheduleResponseForTest(schedule)

	assert.Equal(t, int64(1), response.ID)
	assert.NotNil(t, response.TimeframeID)
	assert.Equal(t, int64(5), *response.TimeframeID)
}

func TestNewScheduleResponse_NilSchedule(t *testing.T) {
	t.Parallel()

	response := httpadapter.NewScheduleResponseForTest(nil)
	assert.Equal(t, int64(0), response.ID)
}

// =============================================================================
// NewSupervisorResponseForTest Tests
// =============================================================================

func TestNewSupervisorResponse_Complete(t *testing.T) {
	t.Parallel()

	supervisor := &activityContract.SupervisorPlanned{
		ID:        1,
		StaffID:   10,
		IsPrimary: true,
		FirstName: "John",
		LastName:  "Doe",
	}

	response := httpadapter.NewSupervisorResponseForTest(supervisor)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, int64(10), response.StaffID)
	assert.True(t, response.IsPrimary)
	assert.Equal(t, "John", response.FirstName)
	assert.Equal(t, "Doe", response.LastName)
}

func TestNewSupervisorResponse_NilStaff(t *testing.T) {
	t.Parallel()

	supervisor := &activityContract.SupervisorPlanned{
		ID:        1,
		StaffID:   10,
		IsPrimary: false,
	}

	response := httpadapter.NewSupervisorResponseForTest(supervisor)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, int64(10), response.StaffID)
	assert.False(t, response.IsPrimary)
	assert.Empty(t, response.FirstName)
	assert.Empty(t, response.LastName)
}

func TestNewSupervisorResponse_StaffWithNilPerson(t *testing.T) {
	t.Parallel()

	supervisor := &activityContract.SupervisorPlanned{
		ID:        1,
		StaffID:   10,
		IsPrimary: false,
	}

	response := httpadapter.NewSupervisorResponseForTest(supervisor)

	assert.Equal(t, int64(1), response.ID)
	assert.Empty(t, response.FirstName)
	assert.Empty(t, response.LastName)
}

func TestNewSupervisorResponse_NilSupervisor(t *testing.T) {
	t.Parallel()

	response := httpadapter.NewSupervisorResponseForTest(nil)
	assert.Equal(t, int64(0), response.ID)
}

// =============================================================================
// Response Type Tests
// =============================================================================

func TestActivityResponse_DefaultSchedules(t *testing.T) {
	t.Parallel()

	response := httpadapter.ActivityResponse{}
	assert.Nil(t, response.Schedules)

	// After init with empty slice
	response.Schedules = []httpadapter.ScheduleResponse{}
	assert.NotNil(t, response.Schedules)
	assert.Empty(t, response.Schedules)
}

func TestCategoryResponse_Fields(t *testing.T) {
	t.Parallel()

	response := httpadapter.CategoryResponse{
		ID:          1,
		Name:        "Test",
		Description: "Description",
		Color:       "#000000",
	}

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "Test", response.Name)
	assert.Equal(t, "Description", response.Description)
	assert.Equal(t, "#000000", response.Color)
}
