// Package checkin internal tests for helper functions and auto-create paths.
// Pure helper tests don't need DB; auto-create tests use real database via testutil.
package checkin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// getStudentDailyCheckoutTime TESTS
// =============================================================================

// helperResource creates a Resource with nil services for testing helper methods.
func helperResource() *Resource {
	return &Resource{}
}

func TestGetStudentDailyCheckoutTime_NoConfig_ReturnsNil(t *testing.T) {
	// Clear any existing env var
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := helperResource()
	checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
	require.NoError(t, err)

	// No time configured — daily checkout is always available
	assert.Nil(t, checkoutTime, "should return nil when no checkout time is configured")
}

func TestGetStudentDailyCheckoutTime_CustomValid(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "14:30"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := helperResource()
	checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)

	assert.Equal(t, 14, checkoutTime.Hour())
	assert.Equal(t, 30, checkoutTime.Minute())
}

func TestGetStudentDailyCheckoutTime_InvalidFormat(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "invalid"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := helperResource()
	_, err := rs.getStudentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid checkout time format")
}

func TestGetStudentDailyCheckoutTime_InvalidHour(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "25:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := helperResource()
	_, err := rs.getStudentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hour")
}

func TestGetStudentDailyCheckoutTime_InvalidMinute(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "12:99"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := helperResource()
	_, err := rs.getStudentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid minute")
}

func TestGetStudentDailyCheckoutTime_NegativeHour(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "-1:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := helperResource()
	_, err := rs.getStudentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hour")
}

func TestGetStudentDailyCheckoutTime_NegativeMinute(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "12:-5"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := helperResource()
	_, err := rs.getStudentDailyCheckoutTime(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid minute")
}

func TestGetStudentDailyCheckoutTime_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		envVar  string
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"midnight", "00:00", 0, 0, false},
		{"end of day", "23:59", 23, 59, false},
		{"noon", "12:00", 12, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", tt.envVar))
			defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

			rs := helperResource()
			checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, checkoutTime)
				assert.Equal(t, tt.wantH, checkoutTime.Hour())
				assert.Equal(t, tt.wantM, checkoutTime.Minute())
			}
		})
	}
}

// =============================================================================
// Settings service wiring tests
// =============================================================================

// mockSettingsService is a minimal mock that supports ResolveString, ResolveBool, and ResolveInt.
type mockSettingsService struct {
	values     map[string]string
	boolValues map[string]bool
	intValues  map[string]int
}

func (m *mockSettingsService) GetSchema(_ context.Context, _ []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (m *mockSettingsService) GetSchemaForOperator(_ context.Context, _ []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (m *mockSettingsService) Resolve(_ context.Context, key string) (any, error) {
	if v, ok := m.values[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockSettingsService) ResolveString(_ context.Context, key string) (string, error) {
	if v, ok := m.values[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("not found")
}
func (m *mockSettingsService) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	return m.ResolveString(context.Background(), key)
}
func (m *mockSettingsService) ResolveBool(_ context.Context, key string) (bool, error) {
	if m.boolValues != nil {
		if v, ok := m.boolValues[key]; ok {
			return v, nil
		}
	}
	return false, nil
}
func (m *mockSettingsService) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	return m.ResolveBool(context.Background(), key)
}
func (m *mockSettingsService) ResolveInt(_ context.Context, key string) (int, error) {
	if m.intValues != nil {
		if v, ok := m.intValues[key]; ok {
			return v, nil
		}
	}
	return 0, nil
}
func (m *mockSettingsService) ResolveIntForTenant(_ context.Context, _ int64, key string) (int, error) {
	return m.ResolveInt(context.Background(), key)
}
func (m *mockSettingsService) HasTenantOverride(_ context.Context, key string) (bool, error) {
	_, exists := m.values[key]
	return exists, nil
}
func (m *mockSettingsService) SetValue(_ context.Context, _ string, _ any, _ *int64, _ []string) error {
	return nil
}
func (m *mockSettingsService) ResetValue(_ context.Context, _ string, _ *int64, _ []string) error {
	return nil
}
func (m *mockSettingsService) GetLoginImageURL(_ context.Context, _ int64) (string, error) {
	return "", nil
}
func (m *mockSettingsService) SetLoginImageURL(_ context.Context, _ int64, _ string) (string, error) {
	return "", nil
}
func (m *mockSettingsService) ClearLoginImageURL(_ context.Context, _ int64) (string, error) {
	return "", nil
}

func TestGetStudentDailyCheckoutTime_UsesSettingsService(t *testing.T) {
	// Clear env var so only the settings service provides the value
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: &mockSettingsService{
			values: map[string]string{
				"operations.student_daily_checkout_time": "16:45",
			},
		},
	}

	checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)
	assert.Equal(t, 16, checkoutTime.Hour())
	assert.Equal(t, 45, checkoutTime.Minute())
}

func TestGetStudentDailyCheckoutTime_SettingsServiceFallsBackToEnv(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "13:15"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	// Settings service returns empty — should fall back to env var
	rs := &Resource{
		SettingsService: &mockSettingsService{
			values: map[string]string{},
		},
	}

	checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)
	assert.Equal(t, 13, checkoutTime.Hour())
	assert.Equal(t, 15, checkoutTime.Minute())
}

func TestGetStudentDailyCheckoutTime_NilSettingsServiceUsesEnv(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "17:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := &Resource{
		SettingsService: nil,
	}

	checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)
	assert.Equal(t, 17, checkoutTime.Hour())
	assert.Equal(t, 0, checkoutTime.Minute())
}

func TestGetStudentDailyCheckoutTime_NoConfigAnywhere_ReturnsNil(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Settings service exists but has no override
	rs := &Resource{
		SettingsService: &mockSettingsService{
			values: map[string]string{},
		},
	}

	checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	assert.Nil(t, checkoutTime, "should return nil when no time is configured anywhere")
}

// =============================================================================
// getRoomNameFromVisit TESTS
// =============================================================================

func TestGetRoomNameFromVisit_NilVisit(t *testing.T) {
	result := getRoomNameFromVisit(nil)
	assert.Equal(t, "", result)
}

func TestGetRoomNameFromVisit_NilActiveGroup(t *testing.T) {
	visit := &active.Visit{}
	result := getRoomNameFromVisit(visit)
	assert.Equal(t, "", result)
}

func TestGetRoomNameFromVisit_NilRoom(t *testing.T) {
	visit := &active.Visit{
		ActiveGroup: &active.Group{},
	}
	result := getRoomNameFromVisit(visit)
	assert.Equal(t, "", result)
}

func TestGetRoomNameFromVisit_WithRoom(t *testing.T) {
	visit := &active.Visit{
		ActiveGroup: &active.Group{
			Room: &facilities.Room{Name: "Test Room"},
		},
	}
	result := getRoomNameFromVisit(visit)
	assert.Equal(t, "Test Room", result)
}

// =============================================================================
// shouldSkipCheckin TESTS
// =============================================================================

func TestShouldSkipCheckin_NilRoomID(t *testing.T) {
	result := shouldSkipCheckin(nil, true, &active.Visit{ActiveGroup: &active.Group{RoomID: 1}})
	assert.False(t, result)
}

func TestShouldSkipCheckin_NotCheckedOut(t *testing.T) {
	roomID := int64(1)
	result := shouldSkipCheckin(&roomID, false, &active.Visit{ActiveGroup: &active.Group{RoomID: 1}})
	assert.False(t, result)
}

func TestShouldSkipCheckin_NilCurrentVisit(t *testing.T) {
	roomID := int64(1)
	result := shouldSkipCheckin(&roomID, true, nil)
	assert.False(t, result)
}

func TestShouldSkipCheckin_NilActiveGroup(t *testing.T) {
	roomID := int64(1)
	result := shouldSkipCheckin(&roomID, true, &active.Visit{})
	assert.False(t, result)
}

func TestShouldSkipCheckin_SameRoom(t *testing.T) {
	roomID := int64(1)
	result := shouldSkipCheckin(&roomID, true, &active.Visit{ActiveGroup: &active.Group{RoomID: 1}})
	assert.True(t, result)
}

func TestShouldSkipCheckin_DifferentRoom(t *testing.T) {
	roomID := int64(2)
	result := shouldSkipCheckin(&roomID, true, &active.Visit{ActiveGroup: &active.Group{RoomID: 1}})
	assert.False(t, result)
}

func TestSelectPickupNote_PreservesDayNoteOrder(t *testing.T) {
	effectivePickup := &scheduleSvc.EffectivePickupTime{
		Notes: "Recurring note should be ignored when day notes exist",
		DayNotes: []scheduleSvc.NoteData{
			{ID: 42, Content: "Ring bell at side entrance"},
			{ID: 7, Content: "Grandma is picking up today"},
		},
	}

	result := selectPickupNote(effectivePickup)

	assert.Equal(t, "Ring bell at side entrance\nGrandma is picking up today", result)
}

func TestSelectPickupNote_FallsBackToEffectiveNotes(t *testing.T) {
	effectivePickup := &scheduleSvc.EffectivePickupTime{
		Notes: "Wait at the side entrance",
	}

	result := selectPickupNote(effectivePickup)

	assert.Equal(t, "Wait at the side entrance", result)
}

func TestSelectPickupNote_NilInput(t *testing.T) {
	result := selectPickupNote(nil)
	assert.Equal(t, "", result)
}

func TestSelectPickupNote_AllWhitespaceDayNotesFallsBackToRecurring(t *testing.T) {
	effectivePickup := &scheduleSvc.EffectivePickupTime{
		Notes: "Recurring fallback note",
		DayNotes: []scheduleSvc.NoteData{
			{ID: 1, Content: "   "},
			{ID: 2, Content: "\t"},
		},
	}

	result := selectPickupNote(effectivePickup)

	assert.Equal(t, "Recurring fallback note", result)
}

func TestSelectPickupNote_EmptyDayNotesAndEmptyRecurring(t *testing.T) {
	effectivePickup := &scheduleSvc.EffectivePickupTime{
		Notes:    "  ",
		DayNotes: []scheduleSvc.NoteData{},
	}

	result := selectPickupNote(effectivePickup)

	assert.Equal(t, "", result)
}

// =============================================================================
// attachPickupInfoToResponse TESTS
// =============================================================================

func TestAttachPickupInfoToResponse_NilPickup(t *testing.T) {
	response := map[string]interface{}{}
	attachPickupInfoToResponse(response, nil)

	_, hasTime := response["pickup_time"]
	_, hasNote := response["pickup_note"]
	assert.False(t, hasTime, "Should not set pickup_time for nil pickup")
	assert.False(t, hasNote, "Should not set pickup_note for nil pickup")
}

func TestAttachPickupInfoToResponse_WithPickupTimeOnly(t *testing.T) {
	pickupTime := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
	effectivePickup := &scheduleSvc.EffectivePickupTime{
		PickupTime: &pickupTime,
	}

	response := map[string]interface{}{}
	attachPickupInfoToResponse(response, effectivePickup)

	assert.Equal(t, "15:30", response["pickup_time"])
	_, hasNote := response["pickup_note"]
	assert.False(t, hasNote, "Should not set pickup_note when no notes exist")
}

func TestAttachPickupInfoToResponse_WithPickupNoteOnly(t *testing.T) {
	effectivePickup := &scheduleSvc.EffectivePickupTime{
		Notes: "Bitte klingeln",
	}

	response := map[string]interface{}{}
	attachPickupInfoToResponse(response, effectivePickup)

	_, hasTime := response["pickup_time"]
	assert.False(t, hasTime, "Should not set pickup_time when nil")
	assert.Equal(t, "Bitte klingeln", response["pickup_note"])
}

func TestAttachPickupInfoToResponse_WithPickupTimeAndNote(t *testing.T) {
	pickupTime := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
	effectivePickup := &scheduleSvc.EffectivePickupTime{
		PickupTime: &pickupTime,
		Notes:      "Seiteneingang",
	}

	response := map[string]interface{}{}
	attachPickupInfoToResponse(response, effectivePickup)

	assert.Equal(t, "14:00", response["pickup_time"])
	assert.Equal(t, "Seiteneingang", response["pickup_note"])
}

func TestAttachPickupInfoToResponse_WithDayNotes(t *testing.T) {
	pickupTime := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
	effectivePickup := &scheduleSvc.EffectivePickupTime{
		PickupTime: &pickupTime,
		Notes:      "Should be ignored",
		DayNotes: []scheduleSvc.NoteData{
			{ID: 1, Content: "Day-specific note"},
		},
	}

	response := map[string]interface{}{}
	attachPickupInfoToResponse(response, effectivePickup)

	assert.Equal(t, "16:00", response["pickup_time"])
	assert.Equal(t, "Day-specific note", response["pickup_note"])
}

func TestAttachPickupInfoToResponse_EmptyNotesOmitsKey(t *testing.T) {
	effectivePickup := &scheduleSvc.EffectivePickupTime{
		Notes: "",
	}

	response := map[string]interface{}{}
	attachPickupInfoToResponse(response, effectivePickup)

	_, hasNote := response["pickup_note"]
	assert.False(t, hasNote, "Should not set pickup_note for empty notes")
}

// =============================================================================
// buildCheckinResult TESTS
// =============================================================================

func TestBuildCheckinResult_CheckedOutAndCheckedIn_Transfer(t *testing.T) {
	newVisitID := int64(123)
	checkoutVisitID := int64(100)

	input := &checkinResultInput{
		Student:          &users.Student{Model: base.Model{ID: 1}},
		Person:           &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut:       true,
		NewVisitID:       &newVisitID,
		CheckoutVisitID:  &checkoutVisitID,
		RoomName:         "Room B",
		PreviousRoomName: "Room A",
	}

	result := buildCheckinResult(input)

	assert.Equal(t, "transferred", result.Action)
	assert.Equal(t, "Gewechselt von Room A zu Room B!", result.GreetingMsg)
	assert.Equal(t, &newVisitID, result.VisitID)
	assert.Equal(t, "Room B", result.RoomName)
	assert.Equal(t, "Room A", result.PreviousRoomName)
}

func TestBuildCheckinResult_CheckedOutAndCheckedIn_SameRoom(t *testing.T) {
	newVisitID := int64(123)
	checkoutVisitID := int64(100)

	input := &checkinResultInput{
		Student:          &users.Student{Model: base.Model{ID: 1}},
		Person:           &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut:       true,
		NewVisitID:       &newVisitID,
		CheckoutVisitID:  &checkoutVisitID,
		RoomName:         "Room A",
		PreviousRoomName: "Room A", // Same room
	}

	result := buildCheckinResult(input)

	assert.Equal(t, "checked_in", result.Action)
	assert.Equal(t, "Hallo Max!", result.GreetingMsg)
}

func TestBuildCheckinResult_CheckedOutOnly(t *testing.T) {
	checkoutVisitID := int64(100)

	input := &checkinResultInput{
		Student:         &users.Student{Model: base.Model{ID: 1}},
		Person:          &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut:      true,
		NewVisitID:      nil, // No checkin
		CheckoutVisitID: &checkoutVisitID,
		RoomName:        "Room A",
	}

	result := buildCheckinResult(input)

	assert.Equal(t, "checked_out", result.Action)
	assert.Equal(t, "Tschüss Max!", result.GreetingMsg)
	assert.Equal(t, &checkoutVisitID, result.VisitID)
}

func TestBuildCheckinResult_CheckedInOnly(t *testing.T) {
	newVisitID := int64(123)

	input := &checkinResultInput{
		Student:    &users.Student{Model: base.Model{ID: 1}},
		Person:     &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut: false,
		NewVisitID: &newVisitID,
		RoomName:   "Room A",
	}

	result := buildCheckinResult(input)

	assert.Equal(t, "checked_in", result.Action)
	assert.Equal(t, "Hallo Max!", result.GreetingMsg)
	assert.Equal(t, &newVisitID, result.VisitID)
}

func TestBuildCheckinResult_NoAction(t *testing.T) {
	input := &checkinResultInput{
		Student:    &users.Student{Model: base.Model{ID: 1}},
		Person:     &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut: false,
		NewVisitID: nil,
	}

	result := buildCheckinResult(input)

	// No action - empty action field
	assert.Equal(t, "", result.Action)
	assert.Equal(t, "", result.GreetingMsg)
}

func TestBuildCheckinResult_TransferNoPreviousRoom(t *testing.T) {
	newVisitID := int64(123)
	checkoutVisitID := int64(100)

	input := &checkinResultInput{
		Student:          &users.Student{Model: base.Model{ID: 1}},
		Person:           &users.Person{FirstName: "Max", LastName: "Test"},
		CheckedOut:       true,
		NewVisitID:       &newVisitID,
		CheckoutVisitID:  &checkoutVisitID,
		RoomName:         "Room B",
		PreviousRoomName: "", // No previous room
	}

	result := buildCheckinResult(input)

	// No previous room, so treated as regular checkin
	assert.Equal(t, "checked_in", result.Action)
	assert.Equal(t, "Hallo Max!", result.GreetingMsg)
}

// =============================================================================
// buildCheckinResponse TESTS
// =============================================================================

func TestBuildCheckinResponse_BasicFields(t *testing.T) {
	now := time.Now()
	visitID := int64(123)
	student := &users.Student{
		Model:  base.Model{ID: 1},
		Person: &users.Person{FirstName: "Max", LastName: "Test"},
	}
	result := &checkinResult{
		Action:      "checked_in",
		VisitID:     &visitID,
		RoomName:    "Room A",
		GreetingMsg: "Hallo Max!",
	}

	response := buildCheckinResponse(student, result, now)

	assert.Equal(t, int64(1), response["student_id"])
	assert.Equal(t, "Max Test", response["student_name"])
	assert.Equal(t, "checked_in", response["action"])
	assert.Equal(t, &visitID, response["visit_id"])
	assert.Equal(t, "Room A", response["room_name"])
	assert.Equal(t, now, response["processed_at"])
	assert.Equal(t, "Hallo Max!", response["message"])
	assert.Equal(t, "success", response["status"])
}

func TestBuildCheckinResponse_Transfer(t *testing.T) {
	now := time.Now()
	visitID := int64(123)
	student := &users.Student{
		Model:  base.Model{ID: 1},
		Person: &users.Person{FirstName: "Max", LastName: "Test"},
	}
	result := &checkinResult{
		Action:           "transferred",
		VisitID:          &visitID,
		RoomName:         "Room B",
		PreviousRoomName: "Room A",
		GreetingMsg:      "Gewechselt!",
	}

	response := buildCheckinResponse(student, result, now)

	assert.Equal(t, "transferred", response["action"])
	assert.Equal(t, "Room A", response["previous_room"])
}

func TestBuildCheckinResponse_NoTransferNoPreviousRoom(t *testing.T) {
	now := time.Now()
	student := &users.Student{
		Model:  base.Model{ID: 1},
		Person: &users.Person{FirstName: "Max", LastName: "Test"},
	}
	result := &checkinResult{
		Action:   "checked_out",
		RoomName: "Room A",
	}

	response := buildCheckinResponse(student, result, now)

	// No previous_room field for non-transfer actions
	_, exists := response["previous_room"]
	assert.False(t, exists)
}

// =============================================================================
// shouldUpgradeToDailyCheckout TESTS
// =============================================================================

func TestShouldUpgradeToDailyCheckout_NotCheckedOutAction(t *testing.T) {
	rs := &Resource{}
	// Pass a valid student to avoid nil dereference on student.GroupID
	student := &users.Student{Model: base.Model{ID: 1}}
	result := rs.shouldUpgradeToDailyCheckout(context.Background(), "checked_in", student, nil)
	assert.False(t, result)
}

func TestShouldUpgradeToDailyCheckout_StudentNoGroupID(t *testing.T) {
	rs := &Resource{}
	student := &users.Student{Model: base.Model{ID: 1}}
	result := rs.shouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, nil)
	assert.False(t, result)
}

func TestShouldUpgradeToDailyCheckout_NilCurrentVisit(t *testing.T) {
	rs := &Resource{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	result := rs.shouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, nil)
	assert.False(t, result)
}

func TestShouldUpgradeToDailyCheckout_NilActiveGroup(t *testing.T) {
	rs := &Resource{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{}
	result := rs.shouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, visit)
	assert.False(t, result)
}

// =============================================================================
// shouldShowDailyCheckoutWithGroup TESTS (direct calls to test defensive guards)
// =============================================================================

func TestShouldShowDailyCheckoutWithGroup_NilGroupID(t *testing.T) {
	rs := &Resource{}
	student := &users.Student{Model: base.Model{ID: 1}} // GroupID is nil
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := rs.shouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result)
}

func TestShouldShowDailyCheckoutWithGroup_NilCurrentVisit(t *testing.T) {
	rs := &Resource{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	result := rs.shouldShowDailyCheckoutWithGroup(context.Background(), student, nil)
	assert.False(t, result)
}

func TestShouldShowDailyCheckoutWithGroup_NilActiveGroup(t *testing.T) {
	rs := &Resource{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{} // ActiveGroup is nil
	result := rs.shouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result)
}

func TestShouldShowDailyCheckoutWithGroup_BeforeCheckoutTime(t *testing.T) {
	// Set checkout time far in the future so we're always before it
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "23:59"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := &Resource{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := rs.shouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "Should return false before daily checkout time")
}

func TestShouldShowDailyCheckoutWithGroup_NilCheckoutTime_AlwaysAvailable(t *testing.T) {
	// No env var, no settings override → nil checkout time → time check skipped
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Education group has no room → daily checkout available from any room
	rs := &Resource{
		SettingsService:  &mockSettingsService{values: map[string]string{}},
		EducationService: &mockEducationService{group: &education.Group{Model: base.Model{ID: 1}}},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := rs.shouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.True(t, result, "Should return true when no checkout time is configured and group has no room")
}

func TestShouldShowDailyCheckoutWithGroup_NilCheckoutTime_MatchingRoom(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	roomID := int64(42)
	rs := &Resource{
		SettingsService:  &mockSettingsService{values: map[string]string{}},
		EducationService: &mockEducationService{group: &education.Group{Model: base.Model{ID: 1}, RoomID: &roomID}},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 42}}
	result := rs.shouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.True(t, result, "Should return true when rooms match and no time gate")
}

func TestShouldShowDailyCheckoutWithGroup_NilCheckoutTime_DifferentRoom(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	roomID := int64(42)
	rs := &Resource{
		SettingsService:  &mockSettingsService{values: map[string]string{}},
		EducationService: &mockEducationService{group: &education.Group{Model: base.Model{ID: 1}, RoomID: &roomID}},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 99}}
	result := rs.shouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "Should return false when student is in wrong room")
}

func TestShouldShowDailyCheckoutWithGroup_GetCheckoutTimeError(t *testing.T) {
	// Set an invalid time format to trigger a parse error
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "not-a-time"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := &Resource{}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := rs.shouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "Should return false when checkout time parse fails")
}

func TestShouldShowDailyCheckoutWithGroup_EducationServiceError(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService:  &mockSettingsService{values: map[string]string{}},
		EducationService: &mockEducationService{err: fmt.Errorf("db error")},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := rs.shouldShowDailyCheckoutWithGroup(context.Background(), student, visit)
	assert.False(t, result, "Should return false when education service errors")
}

func TestShouldUpgradeToDailyCheckout_CheckedOut_NoTimeGate(t *testing.T) {
	// No checkout time configured → time check should pass
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService:  &mockSettingsService{values: map[string]string{}},
		EducationService: &mockEducationService{group: &education.Group{Model: base.Model{ID: 1}}},
	}
	groupID := int64(1)
	student := &users.Student{Model: base.Model{ID: 1}, GroupID: &groupID}
	visit := &active.Visit{ActiveGroup: &active.Group{RoomID: 1}}
	result := rs.shouldUpgradeToDailyCheckout(context.Background(), "checked_out", student, visit)
	assert.True(t, result, "Should upgrade when no time gate and group has no room")
}

// =============================================================================
// buildCheckinResponse DailyCheckoutAvailable TESTS
// =============================================================================

func TestBuildCheckinResponse_DailyCheckoutAvailable(t *testing.T) {
	now := time.Now()
	visitID := int64(100)
	student := &users.Student{
		Model:  base.Model{ID: 1},
		Person: &users.Person{FirstName: "Max", LastName: "Test"},
	}
	result := &checkinResult{
		Action:                 "checked_out",
		VisitID:                &visitID,
		RoomName:               "Klassenraum 1a",
		GreetingMsg:            "Tschüss Max!",
		DailyCheckoutAvailable: true,
	}

	response := buildCheckinResponse(student, result, now)

	assert.Equal(t, true, response["daily_checkout_available"])
	assert.Equal(t, "checked_out", response["action"])
}

func TestBuildCheckinResponse_DailyCheckoutNotAvailable(t *testing.T) {
	now := time.Now()
	visitID := int64(200)
	student := &users.Student{
		Model:  base.Model{ID: 2},
		Person: &users.Person{FirstName: "Anna", LastName: "Test"},
	}
	result := &checkinResult{
		Action:      "checked_in",
		VisitID:     &visitID,
		RoomName:    "Library",
		GreetingMsg: "Hallo Anna!",
		// DailyCheckoutAvailable defaults to false
	}

	response := buildCheckinResponse(student, result, now)

	assert.Equal(t, false, response["daily_checkout_available"])
	assert.Equal(t, "checked_in", response["action"])
}

// =============================================================================
// buildCheckinResponse PickupTime TESTS
// =============================================================================

func TestBuildCheckinResponse_WithPickupTime(t *testing.T) {
	now := time.Now()
	visitID := int64(300)
	pickupTime := "15:30"
	student := &users.Student{
		Model:  base.Model{ID: 3},
		Person: &users.Person{FirstName: "Lisa", LastName: "Test"},
	}
	result := &checkinResult{
		Action:      "checked_in",
		VisitID:     &visitID,
		RoomName:    "Klassenraum 2b",
		GreetingMsg: "Hallo Lisa!",
		PickupTime:  &pickupTime,
	}

	response := buildCheckinResponse(student, result, now)

	assert.Equal(t, "15:30", response["pickup_time"])
	assert.Equal(t, "checked_in", response["action"])
}

func TestBuildCheckinResponse_WithoutPickupTime(t *testing.T) {
	now := time.Now()
	visitID := int64(400)
	student := &users.Student{
		Model:  base.Model{ID: 4},
		Person: &users.Person{FirstName: "Tom", LastName: "Test"},
	}
	result := &checkinResult{
		Action:      "checked_in",
		VisitID:     &visitID,
		RoomName:    "Library",
		GreetingMsg: "Hallo Tom!",
		// PickupTime is nil
	}

	response := buildCheckinResponse(student, result, now)

	_, hasPickupTime := response["pickup_time"]
	assert.False(t, hasPickupTime, "pickup_time should be omitted when nil")
}

// =============================================================================
// roomNameByID TESTS (additional edge cases)
// =============================================================================

func TestRoomNameByID_WithRoomObject(t *testing.T) {
	rs := &Resource{}
	room := &facilities.Room{Name: "Test Room"}
	name := rs.roomNameByID(context.Background(), room, 1)
	assert.Equal(t, "Test Room", name)
}

func TestRoomNameByID_FallbackToLookup(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	room := testpkg.CreateTestRoom(t, tc.db, "LookupRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Pass nil room to force lookup by ID
	name := tc.rs.roomNameByID(context.Background(), nil, room.ID)
	assert.Contains(t, name, "LookupRoom")
}

func TestRoomNameByID_FallbackToFormattedID(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	// Use a non-existent room ID
	name := tc.rs.roomNameByID(context.Background(), nil, 999999)
	assert.Equal(t, "Room 999999", name)
}

func TestRoomNameForResponse_WithRoomID_NoVisit(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	room := testpkg.CreateTestRoom(t, tc.db, "ResponseRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	roomID := room.ID
	name := tc.rs.roomNameForResponse(context.Background(), nil, &roomID)
	assert.Contains(t, name, "ResponseRoom")
}

func TestRoomNameForResponse_VisitWithoutRoom_FallbackToRoomID(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	room := testpkg.CreateTestRoom(t, tc.db, "FallbackRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Visit without ActiveGroup.Room loaded
	visit := &active.Visit{ActiveGroup: &active.Group{}}
	roomID := room.ID
	name := tc.rs.roomNameForResponse(context.Background(), visit, &roomID)
	assert.Contains(t, name, "FallbackRoom")
}

// =============================================================================
// processStudentCheckin TESTS (DB-backed)
// =============================================================================

func TestProcessStudentCheckin_NoRoomNotCheckedOut(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/checkin", nil)
	student := &users.Student{Model: base.Model{ID: 1}}
	person := &users.Person{FirstName: "Test", LastName: "User"}

	input := &checkinProcessingInput{
		RoomID:      nil,
		SkipCheckin: false,
		CheckedOut:  false,
	}

	result := tc.rs.processStudentCheckin(r.Context(), w, r, student, person, input)
	require.NotNil(t, result)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "room_id is required")
}

func TestProcessStudentCheckin_SkippedCheckin_GetsRoomName(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	room := testpkg.CreateTestRoom(t, tc.db, "SkipCheckinRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/checkin", nil)
	student := &users.Student{Model: base.Model{ID: 1}}
	person := &users.Person{FirstName: "Test", LastName: "User"}

	roomID := room.ID
	input := &checkinProcessingInput{
		RoomID:      &roomID,
		SkipCheckin: true,
		CheckedOut:  true,
		CurrentVisit: &active.Visit{
			ActiveGroup: &active.Group{
				Room: &facilities.Room{Name: "SkipCheckinRoom"},
			},
		},
	}

	result := tc.rs.processStudentCheckin(r.Context(), w, r, student, person, input)
	require.NotNil(t, result)
	assert.Nil(t, result.Error)
	assert.Equal(t, "SkipCheckinRoom", result.RoomName)
	assert.Nil(t, result.NewVisitID, "Should not create a new visit when skipping checkin")
}

// =============================================================================
// sendCheckinResponse TESTS
// =============================================================================

func TestSendCheckinResponse(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/checkin", nil)

	response := map[string]interface{}{
		"student_id":   int64(123),
		"student_name": "Test Student",
		"action":       "checked_in",
		"status":       "success",
	}

	sendCheckinResponse(w, r, response, "checked_in")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "success", resp["status"])
}

// =============================================================================
// roomNameForResponse TESTS
// =============================================================================

func TestRoomNameForResponse_WithActiveGroupRoom(t *testing.T) {
	rs := &Resource{}
	currentVisit := &active.Visit{
		ActiveGroup: &active.Group{
			Room: &facilities.Room{Name: "Library"},
		},
	}

	name := rs.roomNameForResponse(context.Background(), currentVisit, nil)
	assert.Equal(t, "Library", name)
}

func TestRoomNameForResponse_NilVisit_NilRoomID(t *testing.T) {
	rs := &Resource{}

	name := rs.roomNameForResponse(context.Background(), nil, nil)
	assert.Equal(t, "", name)
}

// =============================================================================
// processStudentCheckin result struct TESTS
// =============================================================================

func TestCheckinProcessingResult_Struct(t *testing.T) {
	visitID := int64(123)
	result := &checkinProcessingResult{
		NewVisitID: &visitID,
		RoomName:   "Test Room",
		Error:      nil,
	}

	assert.Equal(t, &visitID, result.NewVisitID)
	assert.Equal(t, "Test Room", result.RoomName)
	assert.Nil(t, result.Error)
}

func TestCheckinProcessingInput_Struct(t *testing.T) {
	roomID := int64(1)
	input := &checkinProcessingInput{
		RoomID:       &roomID,
		SkipCheckin:  false,
		CheckedOut:   true,
		CurrentVisit: nil,
	}

	assert.Equal(t, &roomID, input.RoomID)
	assert.False(t, input.SkipCheckin)
	assert.True(t, input.CheckedOut)
	assert.Nil(t, input.CurrentVisit)
}

// =============================================================================
// countActiveStudentsInVisits TESTS
// =============================================================================

func TestCountActiveStudentsInVisits_EmptySlice(t *testing.T) {
	count := countActiveStudentsInVisits([]*active.Visit{})
	assert.Equal(t, 0, count)
}

func TestCountActiveStudentsInVisits_NilSlice(t *testing.T) {
	count := countActiveStudentsInVisits(nil)
	assert.Equal(t, 0, count)
}

func TestCountActiveStudentsInVisits_AllActive(t *testing.T) {
	visits := []*active.Visit{
		{Model: base.Model{ID: 1}, ExitTime: nil},
		{Model: base.Model{ID: 2}, ExitTime: nil},
		{Model: base.Model{ID: 3}, ExitTime: nil},
	}
	count := countActiveStudentsInVisits(visits)
	assert.Equal(t, 3, count)
}

func TestCountActiveStudentsInVisits_AllExited(t *testing.T) {
	now := time.Now()
	visits := []*active.Visit{
		{Model: base.Model{ID: 1}, ExitTime: &now},
		{Model: base.Model{ID: 2}, ExitTime: &now},
	}
	count := countActiveStudentsInVisits(visits)
	assert.Equal(t, 0, count)
}

func TestCountActiveStudentsInVisits_Mixed(t *testing.T) {
	now := time.Now()
	visits := []*active.Visit{
		{Model: base.Model{ID: 1}, ExitTime: nil},  // active
		{Model: base.Model{ID: 2}, ExitTime: &now}, // exited
		{Model: base.Model{ID: 3}, ExitTime: nil},  // active
		{Model: base.Model{ID: 4}, ExitTime: &now}, // exited
		{Model: base.Model{ID: 5}, ExitTime: nil},  // active
	}
	count := countActiveStudentsInVisits(visits)
	assert.Equal(t, 3, count)
}

// =============================================================================
// buildCheckinResponse ActiveStudents TESTS
// =============================================================================

func TestBuildCheckinResponse_WithActiveStudents(t *testing.T) {
	now := time.Now()
	visitID := int64(123)
	activeStudents := 5
	student := &users.Student{
		Model:  base.Model{ID: 1},
		Person: &users.Person{FirstName: "Max", LastName: "Test"},
	}
	result := &checkinResult{
		Action:         "checked_in",
		VisitID:        &visitID,
		RoomName:       "Room A",
		GreetingMsg:    "Hallo Max!",
		ActiveStudents: &activeStudents,
	}

	response := buildCheckinResponse(student, result, now)

	assert.Equal(t, 5, response["active_students"])
}

func TestBuildCheckinResponse_WithoutActiveStudents(t *testing.T) {
	now := time.Now()
	visitID := int64(123)
	student := &users.Student{
		Model:  base.Model{ID: 1},
		Person: &users.Person{FirstName: "Max", LastName: "Test"},
	}
	result := &checkinResult{
		Action:         "checked_in",
		VisitID:        &visitID,
		RoomName:       "Room A",
		GreetingMsg:    "Hallo Max!",
		ActiveStudents: nil,
	}

	response := buildCheckinResponse(student, result, now)

	_, exists := response["active_students"]
	assert.False(t, exists, "active_students should not be in response when nil")
}

func TestBuildCheckinResponse_ActiveStudentsZero(t *testing.T) {
	now := time.Now()
	visitID := int64(123)
	activeStudents := 0
	student := &users.Student{
		Model:  base.Model{ID: 1},
		Person: &users.Person{FirstName: "Max", LastName: "Test"},
	}
	result := &checkinResult{
		Action:         "checked_out",
		VisitID:        &visitID,
		RoomName:       "Room A",
		GreetingMsg:    "Tschüss Max!",
		ActiveStudents: &activeStudents,
	}

	response := buildCheckinResponse(student, result, now)

	assert.Equal(t, 0, response["active_students"])
}

// =============================================================================
// validateDeviceContext TESTS
// =============================================================================

func TestValidateDeviceContext_NilContext(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/checkin", nil)

	result := validateDeviceContext(w, r)

	assert.Nil(t, result, "Should return nil when no device in context")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// =============================================================================
// parseCheckinRequest TESTS
// =============================================================================

func TestParseCheckinRequest_NilBody(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/checkin", nil)
	r.Header.Set("Content-Type", "application/json")

	result := parseCheckinRequest(r.Context(), w, r, slog.Default(), "test-device")

	assert.Nil(t, result, "Should return nil for nil body")
}

// =============================================================================
// checkinResult struct TESTS
// =============================================================================

func TestCheckinResult_AllFields(t *testing.T) {
	visitID := int64(42)
	activeStudents := 7
	result := &checkinResult{
		Action:                 "transferred",
		VisitID:                &visitID,
		RoomName:               "Room B",
		PreviousRoomName:       "Room A",
		GreetingMsg:            "Gewechselt!",
		DailyCheckoutAvailable: true,
		ActiveStudents:         &activeStudents,
	}

	assert.Equal(t, "transferred", result.Action)
	assert.Equal(t, &visitID, result.VisitID)
	assert.Equal(t, "Room B", result.RoomName)
	assert.Equal(t, "Room A", result.PreviousRoomName)
	assert.Equal(t, "Gewechselt!", result.GreetingMsg)
	assert.True(t, result.DailyCheckoutAvailable)
	assert.Equal(t, &activeStudents, result.ActiveStudents)
}

// =============================================================================
// INTERNAL DB-BACKED TESTS: WC auto-create paths
// =============================================================================

// internalTestContext holds shared dependencies for internal DB-backed tests.
type internalTestContext struct {
	rs *Resource
	db *bun.DB
}

// setupInternalTestResource creates a Resource with real database services
// for internal unit tests that exercise auto-create code paths.
func setupInternalTestResource(t *testing.T) *internalTestContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Belt-and-suspenders: ensure the FK target row for tenant_id=1 exists
	// on the exact connection pool used by the services. SetupTestDB already
	// calls EnsureTestTenant, but connection pool semantics can cause a fresh
	// connection (without the SET search_path) to be used for subsequent queries.
	testpkg.EnsureTestTenant(t, db, 1)

	rs := &Resource{
		IoTService:            svc.IoT,
		UsersService:          svc.Users,
		ActiveService:         svc.Active,
		FacilityService:       svc.Facilities,
		ActivitiesService:     svc.Activities,
		EducationService:      svc.Education,
		PickupScheduleService: svc.PickupSchedule,
		logger:                slog.Default(),
	}

	return &internalTestContext{rs: rs, db: db}
}

// cleanupWCTestArtifacts removes WC infrastructure created by auto-create tests.
func cleanupWCTestArtifacts(t *testing.T, tc *internalTestContext) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmts := []string{
		fmt.Sprintf(`DELETE FROM active.attendance WHERE visit_id IN (SELECT v.id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.WCRoomName),
		fmt.Sprintf(`DELETE FROM active.visits WHERE active_group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.WCRoomName),
		fmt.Sprintf(`DELETE FROM active.group_supervisors WHERE group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.WCRoomName),
		fmt.Sprintf(`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE name = '%s')`, constants.WCRoomName),
		fmt.Sprintf(`DELETE FROM activities.schedules WHERE group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.WCActivityName),
		fmt.Sprintf(`DELETE FROM activities.student_enrollments WHERE group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.WCActivityName),
		fmt.Sprintf(`DELETE FROM activities.groups WHERE name = '%s'`, constants.WCActivityName),
		fmt.Sprintf(`DELETE FROM activities.categories WHERE name = '%s'`, constants.WCCategoryName),
		fmt.Sprintf(`DELETE FROM facilities.rooms WHERE name = '%s'`, constants.WCRoomName),
	}
	for _, stmt := range stmts {
		_, _ = tc.db.ExecContext(ctx, stmt)
	}
}

// cleanupSchulhofTestArtifacts removes Schulhof infrastructure created by auto-create tests.
func cleanupSchulhofTestArtifacts(t *testing.T, tc *internalTestContext) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmts := []string{
		fmt.Sprintf(`DELETE FROM active.attendance WHERE visit_id IN (SELECT v.id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.SchulhofRoomName),
		fmt.Sprintf(`DELETE FROM active.visits WHERE active_group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.SchulhofRoomName),
		fmt.Sprintf(`DELETE FROM active.group_supervisors WHERE group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.SchulhofRoomName),
		fmt.Sprintf(`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE name = '%s')`, constants.SchulhofRoomName),
		fmt.Sprintf(`DELETE FROM activities.schedules WHERE group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.SchulhofActivityName),
		fmt.Sprintf(`DELETE FROM activities.student_enrollments WHERE group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.SchulhofActivityName),
		fmt.Sprintf(`DELETE FROM activities.groups WHERE name = '%s'`, constants.SchulhofActivityName),
		fmt.Sprintf(`DELETE FROM activities.categories WHERE name = '%s'`, constants.SchulhofCategoryName),
		fmt.Sprintf(`DELETE FROM facilities.rooms WHERE name = '%s'`, constants.SchulhofRoomName),
	}
	for _, stmt := range stmts {
		_, _ = tc.db.ExecContext(ctx, stmt)
	}
}

// TestEnsureWCRoom_AutoCreatesWhenNotFound verifies that ensureWCRoom creates
// a new WC room when none exists in the database.
func TestEnsureWCRoom_AutoCreatesWhenNotFound(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	// Clean up any pre-existing WC infrastructure (from seed data)
	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)
	room, err := tc.rs.ensureWCRoom(ctx)

	require.NoError(t, err, "ensureWCRoom should not return error")
	require.NotNil(t, room, "ensureWCRoom should return a room")
	assert.Equal(t, constants.WCRoomName, room.Name)
	assert.NotZero(t, room.ID, "Room should have a valid ID after creation")
}

// TestEnsureWCRoom_FindsExistingRoom verifies that ensureWCRoom returns
// an existing WC room without creating a duplicate.
func TestEnsureWCRoom_FindsExistingRoom(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	// Create WC room first
	room1, err := tc.rs.ensureWCRoom(ctx)
	require.NoError(t, err)
	require.NotNil(t, room1)

	// Call again - should find existing room
	room2, err := tc.rs.ensureWCRoom(ctx)
	require.NoError(t, err)
	require.NotNil(t, room2)

	assert.Equal(t, room1.ID, room2.ID, "Should return same room, not create duplicate")
}

// TestEnsureWCCategory_AutoCreatesWhenNotFound verifies that ensureWCCategory
// creates a new WC category when none exists.
func TestEnsureWCCategory_AutoCreatesWhenNotFound(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)
	category, err := tc.rs.ensureWCCategory(ctx)

	require.NoError(t, err, "ensureWCCategory should not return error")
	require.NotNil(t, category, "ensureWCCategory should return a category")
	assert.Equal(t, constants.WCCategoryName, category.Name)
	assert.NotZero(t, category.ID)
}

// TestEnsureWCCategory_FindsExistingCategory verifies that ensureWCCategory
// returns an existing WC category without creating a duplicate.
func TestEnsureWCCategory_FindsExistingCategory(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	// Create category first
	cat1, err := tc.rs.ensureWCCategory(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat1)

	// Call again - should find existing
	cat2, err := tc.rs.ensureWCCategory(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat2)

	assert.Equal(t, cat1.ID, cat2.ID, "Should return same category, not create duplicate")
}

// TestWcActivityGroup_FullAutoCreate verifies that wcActivityGroup creates
// the full WC infrastructure (room, category, activity) when nothing exists.
func TestWcActivityGroup_FullAutoCreate(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	// Create staff for the created_by FK constraint
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	staff := testpkg.CreateTestStaff(t, db, "WCInternal", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Set staff context with tenant
	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), 1)

	group, err := tc.rs.wcActivityGroup(ctx)

	require.NoError(t, err, "wcActivityGroup should not return error")
	require.NotNil(t, group, "wcActivityGroup should return an activity group")
	assert.Equal(t, constants.WCActivityName, group.Name)
	assert.NotZero(t, group.ID)
}

// TestWcActivityGroup_FindsExisting verifies that wcActivityGroup returns
// an existing WC activity without creating duplicates.
func TestWcActivityGroup_FindsExisting(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	staff := testpkg.CreateTestStaff(t, db, "WCExist", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), 1)

	// First call - creates everything
	group1, err := tc.rs.wcActivityGroup(ctx)
	require.NoError(t, err)
	require.NotNil(t, group1)

	// Second call - should find existing
	group2, err := tc.rs.wcActivityGroup(ctx)
	require.NoError(t, err)
	require.NotNil(t, group2)

	assert.Equal(t, group1.ID, group2.ID, "Should return same activity group, not create duplicate")
}

// =============================================================================
// INTERNAL DB-BACKED TESTS: Schulhof auto-create paths
// =============================================================================

// TestEnsureSchulhofRoom_AutoCreatesWhenNotFound verifies that ensureSchulhofRoom
// creates a new Schulhof room when none exists in the database.
func TestEnsureSchulhofRoom_AutoCreatesWhenNotFound(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)
	room, err := tc.rs.ensureSchulhofRoom(ctx)

	require.NoError(t, err, "ensureSchulhofRoom should not return error")
	require.NotNil(t, room, "ensureSchulhofRoom should return a room")
	assert.Equal(t, constants.SchulhofRoomName, room.Name)
	assert.NotZero(t, room.ID)
}

// TestEnsureSchulhofRoom_FindsExistingRoom verifies that ensureSchulhofRoom
// returns an existing Schulhof room without creating a duplicate.
func TestEnsureSchulhofRoom_FindsExistingRoom(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	room1, err := tc.rs.ensureSchulhofRoom(ctx)
	require.NoError(t, err)
	require.NotNil(t, room1)

	room2, err := tc.rs.ensureSchulhofRoom(ctx)
	require.NoError(t, err)
	require.NotNil(t, room2)

	assert.Equal(t, room1.ID, room2.ID, "Should return same room, not create duplicate")
}

// TestEnsureSchulhofCategory_AutoCreatesWhenNotFound verifies that ensureSchulhofCategory
// creates a new Schulhof category when none exists.
func TestEnsureSchulhofCategory_AutoCreatesWhenNotFound(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)
	category, err := tc.rs.ensureSchulhofCategory(ctx)

	require.NoError(t, err, "ensureSchulhofCategory should not return error")
	require.NotNil(t, category, "ensureSchulhofCategory should return a category")
	assert.Equal(t, constants.SchulhofCategoryName, category.Name)
	assert.NotZero(t, category.ID)
}

// TestEnsureSchulhofCategory_FindsExistingCategory verifies that ensureSchulhofCategory
// returns an existing Schulhof category without creating a duplicate.
func TestEnsureSchulhofCategory_FindsExistingCategory(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	cat1, err := tc.rs.ensureSchulhofCategory(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat1)

	cat2, err := tc.rs.ensureSchulhofCategory(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat2)

	assert.Equal(t, cat1.ID, cat2.ID, "Should return same category, not create duplicate")
}

// TestSchulhofActivityGroup_FullAutoCreate verifies that schulhofActivityGroup creates
// the full Schulhof infrastructure (room, category, activity) when nothing exists.
func TestSchulhofActivityGroup_FullAutoCreate(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	staff := testpkg.CreateTestStaff(t, db, "SchulhofInt", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), 1)

	group, err := tc.rs.schulhofActivityGroup(ctx)

	require.NoError(t, err, "schulhofActivityGroup should not return error")
	require.NotNil(t, group, "schulhofActivityGroup should return an activity group")
	assert.Equal(t, constants.SchulhofActivityName, group.Name)
	assert.NotZero(t, group.ID)
}

// TestSchulhofActivityGroup_FindsExisting verifies that schulhofActivityGroup
// returns an existing Schulhof activity without creating duplicates.
func TestSchulhofActivityGroup_FindsExisting(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	staff := testpkg.CreateTestStaff(t, db, "SchulhofExist", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), 1)

	group1, err := tc.rs.schulhofActivityGroup(ctx)
	require.NoError(t, err)
	require.NotNil(t, group1)

	group2, err := tc.rs.schulhofActivityGroup(ctx)
	require.NoError(t, err)
	require.NotNil(t, group2)

	assert.Equal(t, group1.ID, group2.ID, "Should return same activity group, not create duplicate")
}

// =============================================================================
// NO STAFF CONTEXT TESTS: system-created auto-create paths (created_by = NULL)
// =============================================================================

// TestWcActivityGroup_NoStaffContext verifies that wcActivityGroup succeeds
// without any staff in the context (created_by = NULL = system-created).
func TestWcActivityGroup_NoStaffContext(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	// No staff context — tenant context only (no staff)
	ctx := tenant.WithTenantID(context.Background(), 1)

	group, err := tc.rs.wcActivityGroup(ctx)

	require.NoError(t, err, "wcActivityGroup should succeed without staff context")
	require.NotNil(t, group, "wcActivityGroup should return an activity group")
	assert.Equal(t, constants.WCActivityName, group.Name)
	assert.NotZero(t, group.ID)
	assert.Nil(t, group.CreatedBy, "system-created WC group should have NULL created_by")
}

// TestSchulhofActivityGroup_NoStaffContext verifies that schulhofActivityGroup succeeds
// without any staff in the context (created_by = NULL = system-created).
func TestSchulhofActivityGroup_NoStaffContext(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	// No staff context — tenant context only (no staff)
	ctx := tenant.WithTenantID(context.Background(), 1)

	group, err := tc.rs.schulhofActivityGroup(ctx)

	require.NoError(t, err, "schulhofActivityGroup should succeed without staff context")
	require.NotNil(t, group, "schulhofActivityGroup should return an activity group")
	assert.Equal(t, constants.SchulhofActivityName, group.Name)
	assert.NotZero(t, group.ID)
	assert.Nil(t, group.CreatedBy, "system-created Schulhof group should have NULL created_by")
}

// overrideTimeNow pins the package-level timeNow clock to fixed and returns a
// restore function that should be deferred by the caller.
func overrideTimeNow(fixed time.Time) func() {
	orig := timeNow
	timeNow = func() time.Time { return fixed }
	return func() { timeNow = orig }
}

// mockPickupScheduleService is a minimal mock for testing isAfterCheckoutTimeGate.
type mockPickupScheduleService struct {
	scheduleSvc.PickupScheduleService
	effectivePickupTime *scheduleSvc.EffectivePickupTime
	err                 error
}

func (m *mockPickupScheduleService) GetEffectivePickupTimeForDate(_ context.Context, _ int64, _ timezone.Date) (*scheduleSvc.EffectivePickupTime, error) {
	return m.effectivePickupTime, m.err
}

// mockEducationService is a minimal mock for testing shouldShowDailyCheckoutWithGroup.
type mockEducationService struct {
	educationSvc.Service
	group *education.Group
	err   error
}

func (m *mockEducationService) GetGroup(_ context.Context, _ int64) (*education.Group, error) {
	return m.group, m.err
}

// mockErrorSettingsService returns errors from HasTenantOverride.
type mockErrorSettingsService struct {
	mockSettingsService
}

func (m *mockErrorSettingsService) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return false, fmt.Errorf("db connection failed")
}

func TestGetStudentDailyCheckoutTime_HasTenantOverrideError(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: &mockErrorSettingsService{},
	}

	checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	// Should fall back to nil (no time configured) since env var is also unset
	assert.Nil(t, checkoutTime, "should return nil when HasTenantOverride errors and no env var")
}

func TestGetStudentDailyCheckoutTime_HasTenantOverrideError_FallsBackToEnv(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "15:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := &Resource{
		SettingsService: &mockErrorSettingsService{},
	}

	checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, checkoutTime)
	assert.Equal(t, 15, checkoutTime.Hour())
	assert.Equal(t, 0, checkoutTime.Minute())
}

// =============================================================================
// isAfterCheckoutTimeGate TESTS
// =============================================================================

func TestIsAfterCheckoutTimeGate_PerStudentDisabled_FallsBackToGlobal(t *testing.T) {
	// Per-student disabled (default) → should use global checkout time
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: &mockSettingsService{values: map[string]string{}},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	// No global time configured → always available
	result := rs.isAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should return true when per-student disabled and no global time")
}

func TestIsAfterCheckoutTimeGate_PerStudentDisabled_GlobalTimeInFuture(t *testing.T) {
	// Set a global time far in the future
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "23:59"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	rs := &Resource{
		SettingsService: &mockSettingsService{values: map[string]string{}},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := rs.isAfterCheckoutTimeGate(context.Background(), student)
	assert.False(t, result, "should return false when per-student disabled and global time is in future")
}

func TestIsAfterCheckoutTimeGate_PerStudentEnabled_BeforeDelta(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Pin "now" to a deterministic midday so the +2h pickup offset never
	// wraps past midnight. The function projects pickup onto today's date,
	// so a wrapped hour would otherwise land in the past and flip the
	// assertion when CI runs late in the day.
	fixedNow := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	restore := overrideTimeNow(fixedNow)
	defer restore()

	// Pickup time 2 hours from fixed now, delta 15 min → too early.
	pickupTime := time.Date(2000, 1, 1, fixedNow.Hour()+2, 0, 0, 0, fixedNow.Location())

	rs := &Resource{
		SettingsService: &mockSettingsService{
			values:     map[string]string{},
			boolValues: map[string]bool{"operations.per_student_checkout_enabled": true},
			intValues:  map[string]int{"operations.per_student_checkout_delta_minutes": 15},
		},
		PickupScheduleService: &mockPickupScheduleService{
			effectivePickupTime: &scheduleSvc.EffectivePickupTime{PickupTime: &pickupTime},
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := rs.isAfterCheckoutTimeGate(context.Background(), student)
	assert.False(t, result, "should return false when current time is before pickup_time - delta")
}

func TestIsAfterCheckoutTimeGate_PerStudentEnabled_AfterDelta(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Pickup time 5 minutes from now, delta 15 min → already past threshold
	now := time.Now()
	pickupTime := time.Date(2000, 1, 1, now.Hour(), now.Minute()+5, 0, 0, now.Location())

	rs := &Resource{
		SettingsService: &mockSettingsService{
			values:     map[string]string{},
			boolValues: map[string]bool{"operations.per_student_checkout_enabled": true},
			intValues:  map[string]int{"operations.per_student_checkout_delta_minutes": 15},
		},
		PickupScheduleService: &mockPickupScheduleService{
			effectivePickupTime: &scheduleSvc.EffectivePickupTime{PickupTime: &pickupTime},
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := rs.isAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should return true when current time is after pickup_time - delta")
}

func TestIsAfterCheckoutTimeGate_PerStudentEnabled_NoPickupTime_FallsBackToGlobal(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Per-student enabled but student has no pickup time → fall back to global (no global = always available)
	rs := &Resource{
		SettingsService: &mockSettingsService{
			values:     map[string]string{},
			boolValues: map[string]bool{"operations.per_student_checkout_enabled": true},
		},
		PickupScheduleService: &mockPickupScheduleService{
			effectivePickupTime: &scheduleSvc.EffectivePickupTime{PickupTime: nil},
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := rs.isAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should fall back to global (no global = always available) when student has no pickup time")
}

func TestIsAfterCheckoutTimeGate_PerStudentEnabled_PickupServiceError_FallsBackToGlobal(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: &mockSettingsService{
			values:     map[string]string{},
			boolValues: map[string]bool{"operations.per_student_checkout_enabled": true},
		},
		PickupScheduleService: &mockPickupScheduleService{
			err: fmt.Errorf("db error"),
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := rs.isAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should fall back to global when pickup service errors")
}

func TestIsAfterCheckoutTimeGate_PerStudentEnabled_NilPickupService_FallsBackToGlobal(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: &mockSettingsService{
			values:     map[string]string{},
			boolValues: map[string]bool{"operations.per_student_checkout_enabled": true},
		},
		// PickupScheduleService is nil
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := rs.isAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should fall back to global when PickupScheduleService is nil")
}

func TestIsAfterCheckoutTimeGate_PerStudentEnabled_DeltaZero(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	// Pickup time 1 minute from now, delta 0 → not yet
	now := time.Now()
	pickupTime := time.Date(2000, 1, 1, now.Hour(), now.Minute()+1, 0, 0, now.Location())

	rs := &Resource{
		SettingsService: &mockSettingsService{
			values:     map[string]string{},
			boolValues: map[string]bool{"operations.per_student_checkout_enabled": true},
			intValues:  map[string]int{"operations.per_student_checkout_delta_minutes": 0},
		},
		PickupScheduleService: &mockPickupScheduleService{
			effectivePickupTime: &scheduleSvc.EffectivePickupTime{PickupTime: &pickupTime},
		},
	}
	student := &users.Student{Model: base.Model{ID: 1}}

	result := rs.isAfterCheckoutTimeGate(context.Background(), student)
	assert.False(t, result, "should return false when delta is 0 and current time is before pickup time")
}

func TestIsAfterCheckoutTimeGate_NilSettingsService(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{}
	student := &users.Student{Model: base.Model{ID: 1}}

	// No settings service → perStudentEnabled defaults to false → no global time → always available
	result := rs.isAfterCheckoutTimeGate(context.Background(), student)
	assert.True(t, result, "should return true when SettingsService is nil and no global time")
}

// =============================================================================
// IS_SYSTEM FLAG TESTS: auto-provisioned infrastructure is flagged (issue #923)
// =============================================================================

// TestWcProvisioning_SetsIsSystemFlag verifies that auto-created WC
// infrastructure (room, category, activity group) carries is_system = true
// so it stays hidden from staff-facing list endpoints.
func TestWcProvisioning_SetsIsSystemFlag(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	room, err := tc.rs.ensureWCRoom(ctx)
	require.NoError(t, err)
	require.NotNil(t, room)
	assert.True(t, room.IsSystem, "auto-created WC room must be flagged is_system")

	group, err := tc.rs.wcActivityGroup(ctx)
	require.NoError(t, err)
	require.NotNil(t, group)
	assert.True(t, group.IsSystem, "auto-created WC activity group must be flagged is_system")

	category, err := tc.rs.ensureWCCategory(ctx)
	require.NoError(t, err)
	require.NotNil(t, category)
	assert.True(t, category.IsSystem, "auto-created WC category must be flagged is_system")
}

// TestSchulhofProvisioning_SetsIsSystemFlag verifies that auto-created
// Schulhof infrastructure carries is_system = true.
func TestSchulhofProvisioning_SetsIsSystemFlag(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	room, err := tc.rs.ensureSchulhofRoom(ctx)
	require.NoError(t, err)
	require.NotNil(t, room)
	assert.True(t, room.IsSystem, "auto-created Schulhof room must be flagged is_system")

	group, err := tc.rs.schulhofActivityGroup(ctx)
	require.NoError(t, err)
	require.NotNil(t, group)
	assert.True(t, group.IsSystem, "auto-created Schulhof activity group must be flagged is_system")

	category, err := tc.rs.ensureSchulhofCategory(ctx)
	require.NoError(t, err)
	require.NotNil(t, category)
	assert.True(t, category.IsSystem, "auto-created Schulhof category must be flagged is_system")
}
