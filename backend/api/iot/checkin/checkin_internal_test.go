// Package checkin internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package checkin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// =============================================================================
// getStudentDailyCheckoutTime TESTS
// =============================================================================

func TestGetStudentDailyCheckoutTime_Default(t *testing.T) {
	// Clear any existing env var
	_ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME")

	checkoutTime, err := getStudentDailyCheckoutTime()
	require.NoError(t, err)

	// Default should be 15:00
	assert.Equal(t, 15, checkoutTime.Hour())
	assert.Equal(t, 0, checkoutTime.Minute())
}

func TestGetStudentDailyCheckoutTime_CustomValid(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "14:30"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	checkoutTime, err := getStudentDailyCheckoutTime()
	require.NoError(t, err)

	assert.Equal(t, 14, checkoutTime.Hour())
	assert.Equal(t, 30, checkoutTime.Minute())
}

func TestGetStudentDailyCheckoutTime_InvalidFormat(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "invalid"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	_, err := getStudentDailyCheckoutTime()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid checkout time format")
}

func TestGetStudentDailyCheckoutTime_InvalidHour(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "25:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	_, err := getStudentDailyCheckoutTime()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hour")
}

func TestGetStudentDailyCheckoutTime_InvalidMinute(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "12:99"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	_, err := getStudentDailyCheckoutTime()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid minute")
}

func TestGetStudentDailyCheckoutTime_NegativeHour(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "-1:00"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	_, err := getStudentDailyCheckoutTime()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hour")
}

func TestGetStudentDailyCheckoutTime_NegativeMinute(t *testing.T) {
	require.NoError(t, os.Setenv("STUDENT_DAILY_CHECKOUT_TIME", "12:-5"))
	defer func() { _ = os.Unsetenv("STUDENT_DAILY_CHECKOUT_TIME") }()

	_, err := getStudentDailyCheckoutTime()
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

			checkoutTime, err := getStudentDailyCheckoutTime()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantH, checkoutTime.Hour())
				assert.Equal(t, tt.wantM, checkoutTime.Minute())
			}
		})
	}
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
// roomNameByID TESTS (additional edge cases)
// =============================================================================

func TestRoomNameByID_WithRoomObject(t *testing.T) {
	rs := &Resource{}
	room := &facilities.Room{Name: "Test Room"}
	name := rs.roomNameByID(context.Background(), room, 1)
	assert.Equal(t, "Test Room", name)
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
