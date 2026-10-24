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

// checkOpenRoomKioskBooking drives the destination choice at a kiosk (#3067)
// through the assembled production router: real device authentication, the
// device-scan workflow, the open-room move and Student Presence over the
// test's own tenant.
func checkOpenRoomKioskBooking(t *testing.T, api *goldenRuntime) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	apiKey := *testpkg.CreateTestDevice(t, db, "open-room-kiosk").APIKey

	source := testpkg.CreateTestRoom(t, db, "Klassenraum")
	sourceActivity := testpkg.CreateTestActivityGroup(t, db, "Hausaufgaben")
	testpkg.CreateTestActiveGroup(t, db, sourceActivity.ID, source.ID)
	gym := testpkg.CreateTestOpenRoom(t, db, "Turnhalle")
	football := testpkg.CreateTestActivityGroup(t, db, "Fußball")
	footballSession := testpkg.CreateTestActiveGroup(t, db, football.ID, gym.ID)
	unreleased := testpkg.CreateTestRoom(t, db, "Werkraum")
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	foreignGym := testpkg.CreateTestOpenRoomForTenant(t, db, foreignTenant, "Fremde Turnhalle")

	tag, studentID := openRoomKioskChild(t, db, "Kiosk")
	checkin := postOpenRoomKiosk(t, api, apiKey, "/api/iot/checkin", map[string]any{"student_rfid": tag, "action": "checkin", "room_id": source.ID}, "1234")
	require.Equal(t, http.StatusOK, checkin.Code, checkin.Body.String())

	// The stay is booked in the gym's own session: no device, second scan or
	// supervision there, and no participation in the football running there.
	booked := postOpenRoomKiosk(t, api, apiKey, "/api/iot/move-to-room", map[string]any{"student_rfid": tag, "room_id": gym.ID}, "1234")
	require.Equal(t, http.StatusOK, booked.Code, booked.Body.String())
	stay := openRoomKioskData(t, booked)
	assert.Equal(t, "open_room_stay", stay["action"])
	assert.Equal(t, true, stay["moved"])
	assert.Equal(t, float64(gym.ID), stay["room_id"])
	roomSession := int64(stay["active_group_id"].(float64))
	assert.NotEqual(t, footballSession.ID, roomSession, "the child does not join the football")
	assert.Equal(t, []int64{roomSession}, openVisitSessions(t, db, studentID))
	var sessionRoom int64
	var sessionDevice *int64
	require.NoError(t, db.NewRaw("SELECT room_id, device_id FROM active.groups WHERE id = ?", roomSession).Scan(context.Background(), &sessionRoom, &sessionDevice))
	assert.Equal(t, gym.ID, sessionRoom)
	assert.Nil(t, sessionDevice, "the gym's own session belongs to no device")

	// A repeated booking records no second stay.
	again := postOpenRoomKiosk(t, api, apiKey, "/api/iot/move-to-room", map[string]any{"student_rfid": tag, "room_id": gym.ID}, "1234")
	require.Equal(t, http.StatusOK, again.Code, again.Body.String())
	assert.Equal(t, false, openRoomKioskData(t, again)["moved"])
	assert.Equal(t, []int64{roomSession}, openVisitSessions(t, db, studentID))

	// Refusals change nothing.
	for _, refusal := range []struct {
		name   string
		roomID int64
		status int
		code   string
	}{
		{"unreleased room", unreleased.ID, http.StatusConflict, "rooms.not_released"},
		{"foreign tenant's released room", foreignGym.ID, http.StatusNotFound, "iot.room_not_found"},
		{"unknown room", 999_999_999, http.StatusNotFound, "iot.room_not_found"},
	} {
		refused := postOpenRoomKiosk(t, api, apiKey, "/api/iot/move-to-room", map[string]any{"student_rfid": tag, "room_id": refusal.roomID}, "1234")
		assert.Equal(t, refusal.status, refused.Code, "%s: %s", refusal.name, refused.Body.String())
		assert.Contains(t, refused.Body.String(), `"code":"`+refusal.code+`"`, refusal.name)
		assert.Equal(t, []int64{roomSession}, openVisitSessions(t, db, studentID), refusal.name)
	}

	absentTag, absentID := openRoomKioskChild(t, db, "Absent")
	absent := postOpenRoomKiosk(t, api, apiKey, "/api/iot/move-to-room", map[string]any{"student_rfid": absentTag, "room_id": gym.ID}, "1234")
	assert.Equal(t, http.StatusConflict, absent.Code, absent.Body.String())
	assert.Contains(t, absent.Body.String(), `"code":"iot.student_not_present"`)
	assert.Empty(t, openVisitSessions(t, db, absentID))

	// The kiosk's device key and the school PIN stay required.
	wrongPIN := postOpenRoomKiosk(t, api, apiKey, "/api/iot/move-to-room", map[string]any{"student_rfid": tag, "room_id": gym.ID}, "0000")
	assert.Equal(t, http.StatusUnauthorized, wrongPIN.Code, wrongPIN.Body.String())
	anonymous := postOpenRoomKiosk(t, api, "", "/api/iot/move-to-room", map[string]any{"student_rfid": tag, "room_id": gym.ID}, "1234")
	assert.Equal(t, http.StatusUnauthorized, anonymous.Code, anonymous.Body.String())
	assert.Equal(t, []int64{roomSession}, openVisitSessions(t, db, studentID), "failed authentication leaves the stay unchanged")

	// KioskActor is school-wide, not cross-school: a valid device in another
	// school cannot resolve this child's scanned card, even with the right PIN.
	foreignKey := *testpkg.CreateTestDeviceForTenant(t, db, foreignTenant, "foreign-open-room-kiosk").APIKey
	foreignCard := postOpenRoomKiosk(t, api, foreignKey, "/api/iot/move-to-room", map[string]any{"student_rfid": tag, "room_id": foreignGym.ID}, "1234")
	assert.Equal(t, http.StatusNotFound, foreignCard.Code, foreignCard.Body.String())
	assert.Contains(t, foreignCard.Body.String(), `"code":"iot.rfid_tag_not_found"`)
	assert.Equal(t, []int64{roomSession}, openVisitSessions(t, db, studentID), "foreign device cannot move the child")
}

// postOpenRoomKiosk sends one kiosk request to handler, authenticated with
// the device key when apiKey is set.
func postOpenRoomKiosk(t *testing.T, handler http.Handler, apiKey, path string, body map[string]any, pin string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "http://localhost"+path, strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	request.Header.Set("X-Staff-PIN", pin)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// openRoomKioskChild creates a student with a linked card.
func openRoomKioskChild(t *testing.T, db *testpkg.DB, first string) (tag string, studentID int64) {
	t.Helper()
	student := testpkg.CreateTestStudent(t, db, first, "Kind", "3a")
	card := testpkg.CreateTestRFIDCard(t, db, fmt.Sprintf("OPENROOM%d", time.Now().UnixNano()))
	testpkg.LinkRFIDToStudent(t, db, student.PersonID, card.ID)
	return card.ID, student.ID
}

func openRoomKioskData(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.NotNil(t, envelope.Data, response.Body.String())
	return envelope.Data
}

// openVisitSessions lists the sessions of the child's open visits.
func openVisitSessions(t *testing.T, db *testpkg.DB, studentID int64) []int64 {
	t.Helper()
	var sessions []int64
	require.NoError(t, db.NewRaw("SELECT active_group_id FROM active.visits WHERE student_id = ? AND exit_time IS NULL ORDER BY id", studentID).Scan(context.Background(), &sessions))
	return sessions
}
