// Package checkin internal tests for handler-layer helper functions.
// These are pure helper tests that don't need a database. The DB-backed
// auto-create and workflow tests moved to services/iot/checkin alongside the
// extracted CheckinService (issue #575 B8); the daily-checkout / checkout-gate
// policy tests moved there too when that policy was extracted into the service.
package checkin

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	checkinsvc "github.com/moto-nrw/project-phoenix/services/iot/checkin"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// =============================================================================
// selectPickupNote TESTS
// =============================================================================

func TestSelectPickupNote_PreservesDayNoteOrder(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	effectivePickup := &scheduleSvc.EffectivePickupTime{
		Notes: "Wait at the side entrance",
	}

	result := selectPickupNote(effectivePickup)

	assert.Equal(t, "Wait at the side entrance", result)
}

func TestSelectPickupNote_NilInput(t *testing.T) {
	t.Parallel()

	result := selectPickupNote(nil)
	assert.Equal(t, "", result)
}

func TestSelectPickupNote_AllWhitespaceDayNotesFallsBackToRecurring(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	response := map[string]interface{}{}
	attachPickupInfoToResponse(response, nil)

	_, hasTime := response["pickup_time"]
	_, hasNote := response["pickup_note"]
	assert.False(t, hasTime, "Should not set pickup_time for nil pickup")
	assert.False(t, hasNote, "Should not set pickup_note for nil pickup")
}

func TestAttachPickupInfoToResponse_WithPickupTimeOnly(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
// buildCheckinResponse DailyCheckoutAvailable TESTS
// =============================================================================

func TestBuildCheckinResponse_DailyCheckoutAvailable(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/checkin", nil)
	r.Header.Set("Content-Type", "application/json")

	result := parseCheckinRequest(r.Context(), w, r, slog.Default(), "test-device")

	assert.Nil(t, result, "Should return nil for nil body")
}
