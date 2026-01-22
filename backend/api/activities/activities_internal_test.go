// Package activities internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package activities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// =============================================================================
// convertWeekdayToString Tests
// =============================================================================

func TestConvertWeekdayToString_AllDays(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MON", "Monday"},
		{"TUE", "Tuesday"},
		{"WED", "Wednesday"},
		{"THU", "Thursday"},
		{"FRI", "Friday"},
		{"SAT", "Saturday"},
		{"SUN", "Sunday"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := convertWeekdayToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertWeekdayToString_Unknown(t *testing.T) {
	// Unknown input should be returned as-is
	tests := []string{
		"MONDAY",
		"monday",
		"Mon",
		"unknown",
		"",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			result := convertWeekdayToString(input)
			assert.Equal(t, input, result)
		})
	}
}

// =============================================================================
// formatEndTime Tests
// =============================================================================

func TestFormatEndTime_WithTime(t *testing.T) {
	endTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	result := formatEndTime(&endTime)
	assert.Equal(t, "14:30", result)
}

func TestFormatEndTime_NilTime(t *testing.T) {
	result := formatEndTime(nil)
	assert.Equal(t, "", result)
}

func TestFormatEndTime_Midnight(t *testing.T) {
	endTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := formatEndTime(&endTime)
	assert.Equal(t, "00:00", result)
}

func TestFormatEndTime_BeforeMidnight(t *testing.T) {
	endTime := time.Date(2024, 1, 15, 23, 59, 0, 0, time.UTC)
	result := formatEndTime(&endTime)
	assert.Equal(t, "23:59", result)
}

// =============================================================================
// generateSlotName Tests
// =============================================================================

func TestGenerateSlotName_WithEndTime(t *testing.T) {
	startTime := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	result := generateSlotName(startTime, &endTime)
	assert.Equal(t, "08:00 - 10:30", result)
}

func TestGenerateSlotName_NilEndTime(t *testing.T) {
	startTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)

	result := generateSlotName(startTime, nil)
	assert.Equal(t, "From 14:00", result)
}

func TestGenerateSlotName_MorningSlot(t *testing.T) {
	startTime := time.Date(2024, 1, 15, 8, 30, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 15, 9, 15, 0, 0, time.UTC)

	result := generateSlotName(startTime, &endTime)
	assert.Equal(t, "08:30 - 09:15", result)
}

func TestGenerateSlotName_AfternoonSlot(t *testing.T) {
	startTime := time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC)

	result := generateSlotName(startTime, &endTime)
	assert.Equal(t, "13:00 - 16:00", result)
}

// =============================================================================
// Request Type Tests
// =============================================================================

func TestQuickActivityRequest_Fields(t *testing.T) {
	roomID := int64(10)
	req := QuickActivityRequest{
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
	req := QuickActivityRequest{
		Name:            "Test Activity",
		CategoryID:      5,
		RoomID:          nil,
		MaxParticipants: 15,
	}

	assert.Equal(t, "Test Activity", req.Name)
	assert.Nil(t, req.RoomID)
}

func TestActivityRequest_Fields(t *testing.T) {
	roomID := int64(20)
	req := ActivityRequest{
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
	timeframeID := int64(5)
	req := ScheduleRequest{
		Weekday:     1, // Monday
		TimeframeID: &timeframeID,
	}

	assert.Equal(t, 1, req.Weekday)
	assert.Equal(t, int64(5), *req.TimeframeID)
}

func TestScheduleRequest_NilTimeframeID(t *testing.T) {
	req := ScheduleRequest{
		Weekday:     2, // Tuesday
		TimeframeID: nil,
	}

	assert.Equal(t, 2, req.Weekday)
	assert.Nil(t, req.TimeframeID)
}

func TestSupervisorRequest_Fields(t *testing.T) {
	req := SupervisorRequest{
		StaffID:   123,
		IsPrimary: true,
	}

	assert.Equal(t, int64(123), req.StaffID)
	assert.True(t, req.IsPrimary)
}

func TestBatchEnrollmentRequest_Fields(t *testing.T) {
	req := BatchEnrollmentRequest{
		StudentIDs: []int64{1, 2, 3},
	}

	assert.Len(t, req.StudentIDs, 3)
	assert.Equal(t, int64(1), req.StudentIDs[0])
}

func TestBatchEnrollmentRequest_EmptyList(t *testing.T) {
	req := BatchEnrollmentRequest{
		StudentIDs: []int64{},
	}

	assert.Empty(t, req.StudentIDs)
}

// =============================================================================
// NewResource Tests
// =============================================================================

func TestNewResource_ReturnsResource(t *testing.T) {
	resource := NewResource(nil, nil, nil, nil)
	assert.NotNil(t, resource)
}

// =============================================================================
// addSupervisorsToResponse Tests
// =============================================================================

func TestAddSupervisorsToResponse_EmptyList(t *testing.T) {
	response := &ActivityResponse{}
	addSupervisorsToResponse(response, []*activitiesModel.SupervisorPlanned{})
	assert.Nil(t, response.SupervisorIDs)
	assert.Nil(t, response.Supervisors)
}

func TestAddSupervisorsToResponse_NilList(t *testing.T) {
	response := &ActivityResponse{}
	addSupervisorsToResponse(response, nil)
	assert.Nil(t, response.SupervisorIDs)
	assert.Nil(t, response.Supervisors)
}

func TestAddSupervisorsToResponse_WithNilSupervisorInList(t *testing.T) {
	response := &ActivityResponse{}
	sup := &activitiesModel.SupervisorPlanned{
		Model:     base.Model{ID: 1},
		StaffID:   1,
		IsPrimary: false,
	}
	supervisors := []*activitiesModel.SupervisorPlanned{
		nil,
		sup,
		nil,
	}
	addSupervisorsToResponse(response, supervisors)
	assert.Len(t, response.SupervisorIDs, 1)
	assert.Equal(t, int64(1), response.SupervisorIDs[0])
}

func TestAddSupervisorsToResponse_WithPrimarySupervisor(t *testing.T) {
	response := &ActivityResponse{}
	supervisors := []*activitiesModel.SupervisorPlanned{
		{Model: base.Model{ID: 1}, StaffID: 1, IsPrimary: false},
		{Model: base.Model{ID: 2}, StaffID: 2, IsPrimary: true},
		{Model: base.Model{ID: 3}, StaffID: 3, IsPrimary: false},
	}
	addSupervisorsToResponse(response, supervisors)
	assert.Len(t, response.SupervisorIDs, 3)
	assert.NotNil(t, response.SupervisorID)
	assert.Equal(t, int64(2), *response.SupervisorID)
}

func TestAddSupervisorsToResponse_NoPrimarySupervisor(t *testing.T) {
	response := &ActivityResponse{}
	supervisors := []*activitiesModel.SupervisorPlanned{
		{Model: base.Model{ID: 1}, StaffID: 1, IsPrimary: false},
		{Model: base.Model{ID: 2}, StaffID: 2, IsPrimary: false},
	}
	addSupervisorsToResponse(response, supervisors)
	assert.Len(t, response.SupervisorIDs, 2)
	assert.Nil(t, response.SupervisorID)
}

// =============================================================================
// addSchedulesToResponse Tests
// =============================================================================

func TestAddSchedulesToResponse_EmptyList(t *testing.T) {
	response := &ActivityResponse{Schedules: []ScheduleResponse{}}
	addSchedulesToResponse(response, []*activitiesModel.Schedule{})
	assert.Empty(t, response.Schedules)
}

func TestAddSchedulesToResponse_NilList(t *testing.T) {
	response := &ActivityResponse{Schedules: []ScheduleResponse{}}
	addSchedulesToResponse(response, nil)
	assert.Empty(t, response.Schedules)
}

func TestAddSchedulesToResponse_WithNilScheduleInList(t *testing.T) {
	response := &ActivityResponse{Schedules: []ScheduleResponse{}}
	sch := &activitiesModel.Schedule{
		Model:   base.Model{ID: 1},
		Weekday: 1,
	}
	schedules := []*activitiesModel.Schedule{
		nil,
		sch,
		nil,
	}
	addSchedulesToResponse(response, schedules)
	assert.Len(t, response.Schedules, 1)
	assert.Equal(t, int64(1), response.Schedules[0].ID)
}

func TestAddSchedulesToResponse_WithMultipleSchedules(t *testing.T) {
	response := &ActivityResponse{Schedules: []ScheduleResponse{}}
	timeframeID := int64(5)
	schedules := []*activitiesModel.Schedule{
		{Model: base.Model{ID: 1}, Weekday: 1, TimeframeID: &timeframeID},
		{Model: base.Model{ID: 2}, Weekday: 3, TimeframeID: nil},
	}
	addSchedulesToResponse(response, schedules)
	assert.Len(t, response.Schedules, 2)
}

// =============================================================================
// addCategoryToResponse Tests
// =============================================================================

func TestAddCategoryToResponse_NilCategory(t *testing.T) {
	response := &ActivityResponse{}
	group := &activitiesModel.Group{Category: nil}
	addCategoryToResponse(response, group)
	assert.Nil(t, response.Category)
}

func TestAddCategoryToResponse_WithCategory(t *testing.T) {
	response := &ActivityResponse{}
	group := &activitiesModel.Group{
		Category: &activitiesModel.Category{
			TenantModel: base.TenantModel{Model: base.Model{ID: 1}},
			Name:        "Test Category",
		},
	}
	addCategoryToResponse(response, group)
	assert.NotNil(t, response.Category)
	assert.Equal(t, int64(1), response.Category.ID)
	assert.Equal(t, "Test Category", response.Category.Name)
}

// =============================================================================
// buildBaseActivityResponse Tests
// =============================================================================

func TestBuildBaseActivityResponse_BasicFields(t *testing.T) {
	roomID := int64(10)
	now := time.Now()
	group := &activitiesModel.Group{
		TenantModel:     base.TenantModel{Model: base.Model{ID: 1, CreatedAt: now, UpdatedAt: now}},
		Name:            "Test Activity",
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      5,
		PlannedRoomID:   &roomID,
	}

	response := buildBaseActivityResponse(group, 15)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "Test Activity", response.Name)
	assert.Equal(t, 20, response.MaxParticipants)
	assert.True(t, response.IsOpen)
	assert.Equal(t, int64(5), response.CategoryID)
	assert.Equal(t, int64(10), *response.PlannedRoomID)
	assert.Equal(t, 15, response.EnrollmentCount)
	assert.Empty(t, response.Schedules)
}

func TestBuildBaseActivityResponse_NilRoomID(t *testing.T) {
	group := &activitiesModel.Group{
		TenantModel:   base.TenantModel{Model: base.Model{ID: 1}},
		Name:          "Test Activity",
		PlannedRoomID: nil,
	}

	response := buildBaseActivityResponse(group, 0)
	assert.Nil(t, response.PlannedRoomID)
}

// =============================================================================
// updateGroupFields Tests
// =============================================================================

func TestUpdateGroupFields_AllFields(t *testing.T) {
	roomID := int64(25)
	group := &activitiesModel.Group{}
	req := &ActivityRequest{
		Name:            "Updated Name",
		MaxParticipants: 50,
		IsOpen:          true,
		CategoryID:      10,
		PlannedRoomID:   &roomID,
	}

	updateGroupFields(group, req)

	assert.Equal(t, "Updated Name", group.Name)
	assert.Equal(t, 50, group.MaxParticipants)
	assert.True(t, group.IsOpen)
	assert.Equal(t, int64(10), group.CategoryID)
	assert.Equal(t, int64(25), *group.PlannedRoomID)
}

func TestUpdateGroupFields_NilRoomID(t *testing.T) {
	roomID := int64(10)
	group := &activitiesModel.Group{
		PlannedRoomID: &roomID,
	}

	req := &ActivityRequest{
		Name:          "Test",
		PlannedRoomID: nil,
	}

	updateGroupFields(group, req)
	assert.Nil(t, group.PlannedRoomID)
}

// =============================================================================
// newCategoryResponse Tests
// =============================================================================

func TestNewCategoryResponse_FullCategory(t *testing.T) {
	category := &activitiesModel.Category{
		TenantModel: base.TenantModel{Model: base.Model{ID: 1}},
		Name:        "Sports",
		Description: "Sports activities",
		Color:       "#FF0000",
	}

	response := newCategoryResponse(category)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "Sports", response.Name)
	assert.Equal(t, "Sports activities", response.Description)
	assert.Equal(t, "#FF0000", response.Color)
}

func TestNewCategoryResponse_MinimalCategory(t *testing.T) {
	category := &activitiesModel.Category{
		TenantModel: base.TenantModel{Model: base.Model{ID: 1}},
		Name:        "Minimal",
	}

	response := newCategoryResponse(category)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "Minimal", response.Name)
	assert.Empty(t, response.Description)
	assert.Empty(t, response.Color)
}

func TestNewCategoryResponse_NilCategory(t *testing.T) {
	response := newCategoryResponse(nil)
	assert.Equal(t, "Unknown Category", response.Name)
}

// =============================================================================
// newScheduleResponse Tests
// =============================================================================

func TestNewScheduleResponse_Basic(t *testing.T) {
	schedule := &activitiesModel.Schedule{
		Model:   base.Model{ID: 1},
		Weekday: 1,
	}

	response := newScheduleResponse(schedule)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, 1, response.Weekday)
}

func TestNewScheduleResponse_WithTimeframeID(t *testing.T) {
	timeframeID := int64(5)
	schedule := &activitiesModel.Schedule{
		Model:       base.Model{ID: 1},
		Weekday:     1,
		TimeframeID: &timeframeID,
	}

	response := newScheduleResponse(schedule)

	assert.Equal(t, int64(1), response.ID)
	assert.NotNil(t, response.TimeframeID)
	assert.Equal(t, int64(5), *response.TimeframeID)
}

func TestNewScheduleResponse_NilSchedule(t *testing.T) {
	response := newScheduleResponse(nil)
	assert.Equal(t, int64(0), response.ID)
}

// =============================================================================
// newSupervisorResponse Tests
// =============================================================================

func TestNewSupervisorResponse_Complete(t *testing.T) {
	supervisor := &activitiesModel.SupervisorPlanned{
		Model:     base.Model{ID: 1},
		StaffID:   10,
		IsPrimary: true,
		Staff: &users.Staff{
			TenantModel: base.TenantModel{Model: base.Model{ID: 10}},
			Person: &users.Person{
				TenantModel: base.TenantModel{Model: base.Model{ID: 100}},
				FirstName:   "John",
				LastName:    "Doe",
			},
		},
	}

	response := newSupervisorResponse(supervisor)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, int64(10), response.StaffID)
	assert.True(t, response.IsPrimary)
	assert.Equal(t, "John", response.FirstName)
	assert.Equal(t, "Doe", response.LastName)
}

func TestNewSupervisorResponse_NilStaff(t *testing.T) {
	supervisor := &activitiesModel.SupervisorPlanned{
		Model:     base.Model{ID: 1},
		StaffID:   10,
		IsPrimary: false,
		Staff:     nil,
	}

	response := newSupervisorResponse(supervisor)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, int64(10), response.StaffID)
	assert.False(t, response.IsPrimary)
	assert.Empty(t, response.FirstName)
	assert.Empty(t, response.LastName)
}

func TestNewSupervisorResponse_StaffWithNilPerson(t *testing.T) {
	supervisor := &activitiesModel.SupervisorPlanned{
		Model:     base.Model{ID: 1},
		StaffID:   10,
		IsPrimary: false,
		Staff: &users.Staff{
			TenantModel: base.TenantModel{Model: base.Model{ID: 10}},
			Person:      nil,
		},
	}

	response := newSupervisorResponse(supervisor)

	assert.Equal(t, int64(1), response.ID)
	assert.Empty(t, response.FirstName)
	assert.Empty(t, response.LastName)
}

func TestNewSupervisorResponse_NilSupervisor(t *testing.T) {
	response := newSupervisorResponse(nil)
	assert.Equal(t, int64(0), response.ID)
}

// =============================================================================
// Response Type Tests
// =============================================================================

func TestActivityResponse_DefaultSchedules(t *testing.T) {
	response := ActivityResponse{}
	assert.Nil(t, response.Schedules)

	// After init with empty slice
	response.Schedules = []ScheduleResponse{}
	assert.NotNil(t, response.Schedules)
	assert.Empty(t, response.Schedules)
}

func TestCategoryResponse_Fields(t *testing.T) {
	response := CategoryResponse{
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
