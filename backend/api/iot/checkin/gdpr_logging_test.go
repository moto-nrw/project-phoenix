package checkin_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Issue #2062: the detailed check-in handler logged result.GreetingMsg ("Hallo
// Max!") on the "checkin complete" Info line, so every scan wrote a child's
// first name into journald and from there into Loki, where it sat for the full
// log retention window. These tests pin the structured log fields of a real
// check-in request: nothing at Info or above may carry a name or the greeting,
// while the technical fields the on-call queries rely on must stay.
//
// The capture is deliberately end-to-end. slog.SetDefault is installed *before*
// setupTestContext so the handler, the extracted CheckinService, and every
// service the request touches all resolve to the capturing logger; the buffer is
// reset immediately before the request so fixture setup noise stays out.
// LevelInfo mirrors the production LOG_LEVEL — names at Debug are allowed by
// the project rule and must not fail this test.

// distinctive fixture strings — chosen so a substring match cannot collide with
// SQL, table names, or any other log content.
const (
	gdprLogFirstName = "Loggingprobe"
	gdprLogLastName  = "Namenspruefung"
)

// installLogCapture redirects slog.Default() into a buffer for the duration of
// the test and returns the buffer.
func installLogCapture(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &buf
}

// decodeLogRecords parses the captured JSON lines. Non-JSON lines (nothing
// should emit them, but a stray writer would) fail the test rather than being
// silently skipped — an unparsed line could hide a name.
func decodeLogRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoErrorf(t, json.Unmarshal([]byte(line), &record), "captured log line is not JSON: %s", line)
		records = append(records, record)
	}
	return records
}

// assertNoPersonalDataInLogs fails when any captured record carries one of the
// forbidden values (in any field, at any nesting depth) or a "message"
// attribute — the exact field removed in #2062. slog writes the log message
// itself under "msg", so a "message" key can only come from an attribute.
func assertNoPersonalDataInLogs(t *testing.T, records []map[string]any, forbidden ...string) {
	t.Helper()

	for _, record := range records {
		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		line := string(encoded)

		_, hasMessageAttr := record["message"]
		assert.Falsef(t, hasMessageAttr,
			"log record carries a \"message\" attribute (removed in #2062): %s", line)

		for _, value := range forbidden {
			assert.NotContainsf(t, line, value,
				"log record leaks personal data %q: %s", value, line)
		}
	}
}

// findLogRecord returns the first captured record with the given msg.
func findLogRecord(records []map[string]any, msg string) map[string]any {
	for _, record := range records {
		if record["msg"] == msg {
			return record
		}
	}
	return nil
}

// TestDeviceCheckin_LogsOmitStudentNameAndGreeting drives a real check-in and a
// real check-out through the router and asserts on the structured log fields of
// both. Check-in and check-out produce different greetings ("Hallo X!" /
// "Tschüss X!"), so both are exercised.
func TestDeviceCheckin_LogsOmitStudentNameAndGreeting(t *testing.T) {
	logs := installLogCapture(t)

	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "gdpr-log-checkin")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, device.ID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Gdpr", "Staff")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	student := testpkg.CreateTestStudent(t, ctx.db, gdprLogFirstName, gdprLogLastName, "1a")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	tagID := fmt.Sprintf("GDPRLOG%d", time.Now().UnixNano())
	card := testpkg.CreateTestRFIDCard(t, ctx.db, tagID)
	defer testpkg.CleanupRFIDCards(t, ctx.db, card.ID)
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Gdpr Log Room")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, room.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Gdpr Log Activity")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activity.ID)

	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, activeGroup.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// --- check-in ---------------------------------------------------------
	logs.Reset()

	checkinReq := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", map[string]any{
		"student_rfid": card.ID,
		"action":       "checkin",
		"room_id":      room.ID,
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)
	checkinRR := testutil.ExecuteRequest(router, checkinReq)
	testutil.AssertSuccessResponse(t, checkinRR, http.StatusOK)

	checkinRecords := decodeLogRecords(t, logs)

	// The API response is unchanged: the kiosk still gets the personalized
	// greeting. Only the log lost it.
	checkinData := responseData(t, checkinRR.Body.Bytes())
	assert.Equal(t, "checked_in", checkinData["action"])
	assert.Equal(t, "Hallo "+gdprLogFirstName+"!", checkinData["message"],
		"kiosk greeting must survive the log change")
	assert.Equal(t, gdprLogFirstName+" "+gdprLogLastName, checkinData["student_name"])

	assertNoPersonalDataInLogs(t, checkinRecords,
		gdprLogFirstName,
		gdprLogLastName,
		"Hallo ",
		"Tschüss ",
	)

	// The diagnostic value has to survive: action + student_id + room stay.
	complete := findLogRecord(checkinRecords, "checkin complete")
	require.NotNil(t, complete, "expected a \"checkin complete\" Info record, got: %s", logs.String())
	assert.Equal(t, "checked_in", complete["action"])
	assert.EqualValues(t, student.ID, complete["student_id"])
	assert.Equal(t, room.Name, complete["room"])
	assert.NotNil(t, complete["visit_id"])

	// The school class was demoted to Debug along with the greeting removal.
	assert.Nil(t, findLogRecord(checkinRecords, "found student"),
		"\"found student\" carries the school class and must not reach Info")

	// --- check-out --------------------------------------------------------
	logs.Reset()

	checkoutReq := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", map[string]any{
		"student_rfid": card.ID,
		"action":       "checkout",
	},
		testutil.WithDeviceContext(createTestDeviceContext(device)),
		testutil.WithStaffContext(staff),
	)
	checkoutRR := testutil.ExecuteRequest(router, checkoutReq)
	testutil.AssertSuccessResponse(t, checkoutRR, http.StatusOK)

	checkoutRecords := decodeLogRecords(t, logs)

	checkoutData := responseData(t, checkoutRR.Body.Bytes())
	assert.Contains(t, []any{"checked_out", "checked_out_daily"}, checkoutData["action"])
	assert.Equal(t, "Tschüss "+gdprLogFirstName+"!", checkoutData["message"],
		"kiosk farewell must survive the log change")

	assertNoPersonalDataInLogs(t, checkoutRecords,
		gdprLogFirstName,
		gdprLogLastName,
		"Hallo ",
		"Tschüss ",
	)

	checkoutComplete := findLogRecord(checkoutRecords, "checkin complete")
	require.NotNil(t, checkoutComplete, "expected a \"checkin complete\" Info record, got: %s", logs.String())
	assert.EqualValues(t, student.ID, checkoutComplete["student_id"])
}

// responseData unwraps the {"data": {...}} envelope.
func responseData(t *testing.T, body []byte) map[string]any {
	t.Helper()

	response := testutil.ParseJSONResponse(t, body)
	data, ok := response["data"].(map[string]any)
	require.True(t, ok, "response should have a data object: %s", string(body))
	return data
}
