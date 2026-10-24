package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// checkIoTErrorCodes drives the kiosk refusals of #3939 through the assembled
// production router: the device middleware, the IoT resources and their
// composition-root renderers. Each refusal keeps its status, answers in the
// RFC 9457 envelope with its own iot.* code and carries its diagnostic text
// in detail (#2508, #2520).
func checkIoTErrorCodes(t *testing.T, api *goldenRuntime) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	const pin = "1234"

	t.Run("device authentication", func(t *testing.T) {
		apiKey := *testpkg.CreateTestDevice(t, db, "codes-auth").APIKey
		inactive := testpkg.CreateTestDevice(t, db, "codes-inactive")
		_, err := db.NewRaw("UPDATE iot.devices SET status = 'inactive' WHERE id = ?", inactive.ID).Exec(context.Background())
		require.NoError(t, err)

		for _, refusal := range []struct {
			name, authorization, pin string
			status                   int
			code, text               string
		}{
			{"missing API key", "", pin, http.StatusUnauthorized, "iot.device_api_key_required", "device API key is required"},
			{"empty bearer token", "Bearer ", pin, http.StatusUnauthorized, "iot.device_api_key_required", "device API key is required"},
			{"not a bearer token", "Token " + apiKey, pin, http.StatusUnauthorized, "iot.device_api_key_format_invalid", "invalid API key format - use Bearer token"},
			{"unknown API key", "Bearer unknown-" + apiKey, pin, http.StatusUnauthorized, "iot.device_api_key_invalid", "invalid device API key"},
			{"missing PIN", "Bearer " + apiKey, "", http.StatusUnauthorized, "iot.device_pin_required", "staff PIN is required"},
			{"wrong PIN", "Bearer " + apiKey, "0000", http.StatusUnauthorized, "iot.device_pin_invalid", "invalid staff PIN"},
			{"inactive device", "Bearer " + *inactive.APIKey, pin, http.StatusForbidden, "iot.device_inactive", "device is not active"},
		} {
			response := iotCodeRequest(t, api, http.MethodPost, "/api/iot/checkin", refusal.authorization, refusal.pin, map[string]any{"student_rfid": "TAG", "action": "checkin"})
			assertIoTRefusal(t, response, refusal.status, refusal.code, refusal.text, refusal.name)
		}

		// The device-only routes run the same key check without a PIN.
		response := iotCodeRequest(t, api, http.MethodPost, "/api/iot/error-reports", "Token "+apiKey, "", nil)
		assertIoTRefusal(t, response, http.StatusUnauthorized, "iot.device_api_key_format_invalid", "invalid API key format - use Bearer token", "device-only route")
	})

	t.Run("kiosk refusals", func(t *testing.T) {
		authorization := "Bearer " + *testpkg.CreateTestDevice(t, db, "codes-kiosk").APIKey

		staff := testpkg.CreateTestStaff(t, db, "Kiosk", "Betreuung")
		staffTag := iotCodeTag(t, db, staff.PersonID, "CODESTAFF")
		plain := testpkg.CreateTestPerson(t, db, "Kiosk", "Gast")
		plainTag := iotCodeTag(t, db, plain.ID, "CODEPLAIN")
		childTag, _ := openRoomKioskChild(t, db, "Codes")
		emptyRoom := testpkg.CreateTestRoom(t, db, "Leerer Raum")
		activity := testpkg.CreateTestActivityGroup(t, db, "Ohne Raum")
		unknownTag := fmt.Sprintf("CODEUNKNOWN%d", time.Now().UnixNano())

		for _, refusal := range []struct {
			name, method, path string
			body               map[string]any
			status             int
			code, text         string
		}{
			{"neither student nor staff", http.MethodPost, "/api/iot/checkin", map[string]any{"student_rfid": plainTag, "action": "checkin", "room_id": emptyRoom.ID},
				http.StatusNotFound, "iot.rfid_tag_not_student_or_staff", "RFID tag not assigned to student or staff"},
			{"supervisor scan without session", http.MethodPost, "/api/iot/checkin", map[string]any{"student_rfid": staffTag, "action": "checkin", "room_id": emptyRoom.ID},
				http.StatusNotFound, "iot.no_active_session", "no active session - please start an activity first"},
			{"room without groups", http.MethodPost, "/api/iot/checkin", map[string]any{"student_rfid": childTag, "action": "checkin", "room_id": emptyRoom.ID},
				http.StatusNotFound, "iot.no_active_groups_in_room", "no active groups in specified room"},
			{"pickup query for staff card", http.MethodPost, "/api/iot/pickup-query", map[string]any{"student_rfid": staffTag},
				http.StatusBadRequest, "iot.pickup_student_rfid_required", "student RFID tag required for pickup query"},
			{"pickup query for neither", http.MethodPost, "/api/iot/pickup-query", map[string]any{"student_rfid": plainTag},
				http.StatusNotFound, "iot.rfid_tag_not_student_or_staff", "RFID tag not assigned to student or staff"},
			{"attendance status of unknown card", http.MethodGet, "/api/iot/attendance/status/" + unknownTag, nil,
				http.StatusNotFound, "iot.rfid_tag_not_found", "RFID tag not found"},
			{"attendance status of staff card", http.MethodGet, "/api/iot/attendance/status/" + staffTag, nil,
				http.StatusNotFound, "iot.person_not_student", "person is not a student"},
			{"attendance toggle of unknown card", http.MethodPost, "/api/iot/attendance/toggle", map[string]any{"rfid": unknownTag, "action": "confirm"},
				http.StatusNotFound, "iot.rfid_tag_not_found", "RFID tag not found"},
			{"daily checkout of unknown card", http.MethodPost, "/api/iot/attendance/toggle", map[string]any{"rfid": unknownTag, "action": "confirm_daily_checkout", "destination": "zuhause"},
				http.StatusNotFound, "iot.rfid_tag_not_found", "RFID tag not found"},
			{"daily checkout without attendance", http.MethodPost, "/api/iot/attendance/toggle", map[string]any{"rfid": childTag, "action": "confirm_daily_checkout", "destination": "zuhause"},
				http.StatusNotFound, "iot.no_attendance_record_today", "student has no attendance record for today"},
			{"end without session", http.MethodPost, "/api/iot/session/end", nil,
				http.StatusBadRequest, "iot.no_session_to_end", "no active session to end"},
			{"timeout without session", http.MethodPost, "/api/iot/session/timeout", nil,
				http.StatusNotFound, "iot.no_active_session", "no active session - please start an activity first"},
			{"start without room", http.MethodPost, "/api/iot/session/start", map[string]any{"activity_id": activity.ID, "supervisor_ids": []int64{staff.ID}},
				http.StatusBadRequest, "iot.no_room_available", "no room available for this activity"},
			{"unknown staff", http.MethodDelete, "/api/iot/staff/999999999/rfid", nil,
				http.StatusNotFound, "iot.staff_not_found", "staff not found"},
			{"staff without bracelet", http.MethodDelete, fmt.Sprintf("/api/iot/staff/%d/rfid", testpkg.CreateTestStaff(t, db, "Ohne", "Band").ID), nil,
				http.StatusNotFound, "iot.staff_rfid_tag_unassigned", "staff has no RFID tag assigned"},
		} {
			response := iotCodeRequest(t, api, refusal.method, refusal.path, authorization, pin, refusal.body)
			assertIoTRefusal(t, response, refusal.status, refusal.code, refusal.text, refusal.name)
		}
	})
}

func iotCodeRequest(t *testing.T, api *goldenRuntime, method, path, authorization, pin string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload := ""
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		payload = string(encoded)
	}
	request := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	if pin != "" {
		request.Header.Set("X-Staff-PIN", pin)
	}
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	return response
}

// assertIoTRefusal checks the HTTP status, that the envelope repeats it as
// its status number, the code, and the diagnostic text in detail; the
// pre-#2520 `error` member is gone.
func assertIoTRefusal(t *testing.T, response *httptest.ResponseRecorder, status int, code, text, name string) {
	t.Helper()
	var problem map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem), "%s: %s", name, response.Body.String())
	assert.Equal(t, status, response.Code, "%s: %s", name, response.Body.String())
	assert.Equal(t, float64(status), problem["status"], "%s: %s", name, response.Body.String())
	assert.Equal(t, code, problem["code"], "%s: %s", name, response.Body.String())
	assert.Contains(t, problem["detail"], text, name)
	assert.NotContains(t, problem, "error", name)
}

// iotCodeTag links a fresh card to the person.
func iotCodeTag(t *testing.T, db *testpkg.DB, personID int64, prefix string) string {
	t.Helper()
	card := testpkg.CreateTestRFIDCard(t, db, prefix)
	testpkg.LinkRFIDToStudent(t, db, personID, card.ID)
	return card.ID
}
