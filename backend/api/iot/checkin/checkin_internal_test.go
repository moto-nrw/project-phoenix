// Package checkin internal tests for handler-layer helper functions.
// These are pure helper tests that don't need a database. The DB-backed
// auto-create and workflow tests moved to services/iot/checkin alongside the
// extracted CheckinService (issue #575 B8).
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

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	checkinsvc "github.com/moto-nrw/project-phoenix/services/iot/checkin"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
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

// newMockSettingsService builds a configtest.Mock reproducing the behavior
// of the former hand-rolled mockSettingsService stub: ResolveString/Resolve
// return a "not found" error for missing keys, while ResolveBool/ResolveInt
// return the zero value with no error for missing keys.
func newMockSettingsService(values map[string]string, boolValues map[string]bool, intValues map[string]int) *configtest.Mock {
	resolveString := func(_ context.Context, key string) (string, error) {
		if v, ok := values[key]; ok {
			return v, nil
		}
		return "", fmt.Errorf("not found")
	}
	resolveBool := func(_ context.Context, key string) (bool, error) {
		if boolValues != nil {
			if v, ok := boolValues[key]; ok {
				return v, nil
			}
		}
		return false, nil
	}
	resolveInt := func(_ context.Context, key string) (int, error) {
		if intValues != nil {
			if v, ok := intValues[key]; ok {
				return v, nil
			}
		}
		return 0, nil
	}
	return &configtest.Mock{
		ResolveFn: func(_ context.Context, key string) (any, error) {
			if v, ok := values[key]; ok {
				return v, nil
			}
			return nil, fmt.Errorf("not found")
		},
		ResolveStringFn: resolveString,
		ResolveStringForTenantFn: func(ctx context.Context, _ int64, key string) (string, error) {
			return resolveString(ctx, key)
		},
		ResolveBoolFn: resolveBool,
		ResolveBoolForTenantFn: func(ctx context.Context, _ int64, key string) (bool, error) {
			return resolveBool(ctx, key)
		},
		ResolveIntFn: resolveInt,
		ResolveIntForTenantFn: func(ctx context.Context, _ int64, key string) (int, error) {
			return resolveInt(ctx, key)
		},
		HasTenantOverrideFn: func(_ context.Context, key string) (bool, error) {
			_, exists := values[key]
			return exists, nil
		},
	}
}

func TestGetStudentDailyCheckoutTime_UsesSettingsService(t *testing.T) {
	// Clear env var so only the settings service provides the value
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: newMockSettingsService(map[string]string{
			"operations.student_daily_checkout_time": "16:45",
		}, nil, nil),
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
		SettingsService: newMockSettingsService(map[string]string{}, nil, nil),
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
		SettingsService: newMockSettingsService(map[string]string{}, nil, nil),
	}

	checkoutTime, err := rs.getStudentDailyCheckoutTime(context.Background())
	require.NoError(t, err)
	assert.Nil(t, checkoutTime, "should return nil when no time is configured anywhere")
}

// =============================================================================
// selectPickupNote TESTS
// =============================================================================

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
// buildCheckinResponse TESTS
// =============================================================================

func TestBuildCheckinResponse_BasicFields(t *testing.T) {
	now := time.Now()
	visitID := int64(123)
	student := &users.Student{
		Model:  base.Model{ID: 1},
		Person: &users.Person{FirstName: "Max", LastName: "Test"},
	}
	result := &checkinsvc.CheckinResult{
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
	result := &checkinsvc.CheckinResult{
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
	result := &checkinsvc.CheckinResult{
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
		SettingsService:  newMockSettingsService(map[string]string{}, nil, nil),
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
		SettingsService:  newMockSettingsService(map[string]string{}, nil, nil),
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
		SettingsService:  newMockSettingsService(map[string]string{}, nil, nil),
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
		SettingsService:  newMockSettingsService(map[string]string{}, nil, nil),
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
		SettingsService:  newMockSettingsService(map[string]string{}, nil, nil),
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
	result := &checkinsvc.CheckinResult{
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
	result := &checkinsvc.CheckinResult{
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
	result := &checkinsvc.CheckinResult{
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
	result := &checkinsvc.CheckinResult{
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
	result := &checkinsvc.CheckinResult{
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
	result := &checkinsvc.CheckinResult{
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
	result := &checkinsvc.CheckinResult{
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

// newMockErrorSettingsService builds a configtest.Mock reproducing the
// former mockErrorSettingsService stub: identical to an empty
// mockSettingsService, except HasTenantOverride always errors.
func newMockErrorSettingsService() *configtest.Mock {
	m := newMockSettingsService(nil, nil, nil)
	m.HasTenantOverrideFn = func(_ context.Context, _ string) (bool, error) {
		return false, fmt.Errorf("db connection failed")
	}
	return m
}

func TestGetStudentDailyCheckoutTime_HasTenantOverrideError(t *testing.T) {
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	rs := &Resource{
		SettingsService: newMockErrorSettingsService(),
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
		SettingsService: newMockErrorSettingsService(),
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
		SettingsService: newMockSettingsService(map[string]string{}, nil, nil),
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
		SettingsService: newMockSettingsService(map[string]string{}, nil, nil),
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
		SettingsService: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			map[string]int{"operations.per_student_checkout_delta_minutes": 15},
		),
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
		SettingsService: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			map[string]int{"operations.per_student_checkout_delta_minutes": 15},
		),
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
		SettingsService: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			nil,
		),
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
		SettingsService: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			nil,
		),
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
		SettingsService: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			nil,
		),
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
		SettingsService: newMockSettingsService(
			map[string]string{},
			map[string]bool{"operations.per_student_checkout_enabled": true},
			map[string]int{"operations.per_student_checkout_delta_minutes": 0},
		),
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
