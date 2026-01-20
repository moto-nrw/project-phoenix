// Package activities internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package activities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
