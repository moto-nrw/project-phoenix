package checkin_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Issue #2062: the check-in handler logged the kiosk greeting ("Hallo Max!") on
// its Info line, so every scan wrote a child's first name into journald and from
// there into Loki for the full retention window.
//
// The logger is injected into the route so the handler and every service the
// request touches log into the buffer. LevelInfo mirrors the production
// LOG_LEVEL — names at Debug are allowed and must not fail this test.
func TestDeviceCheckin_LogsOmitStudentNameAndGreeting(t *testing.T) {
	t.Parallel()
	const (
		firstName = "Loggingprobe"
		lastName  = "Namenspruefung"
	)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := setupCheckinRoute(t, logger)

	device := testpkg.CreateTestDevice(t, ctx.db, "gdpr-log-checkin")

	staff := testpkg.CreateTestStaff(t, ctx.db, "Gdpr", "Staff")

	student := testpkg.CreateTestStudent(t, ctx.db, firstName, lastName, "1a")

	card := testpkg.CreateTestRFIDCard(t, ctx.db, fmt.Sprintf("GDPRLOG%d", time.Now().UnixNano()))
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, card.ID)

	room := testpkg.CreateTestRoom(t, ctx.db, "Gdpr Log Room")

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "Gdpr Log Activity")

	_ = testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// scan performs one RFID scan and returns the response payload plus
	// everything logged at Info or above while serving it.
	scan := func(body map[string]any) (map[string]any, string) {
		t.Helper()
		logs.Reset()
		rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "POST", "/checkin", body,
			testutil.WithDeviceContext(createTestDeviceContext(device)),
			testutil.WithStaffContext(staff),
		))
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		data, ok := testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].(map[string]any)
		require.True(t, ok, "response should have a data object")
		return data, logs.String()
	}

	// assertLogsClean pins the whole Info+ output: no name, no greeting, and
	// neither of the two attributes removed in #2062 ("message", "class").
	assertLogsClean := func(out string) {
		t.Helper()
		require.NotEmpty(t, out, "expected the request to emit at least one Info record")
		require.Contains(t, out, `"msg":"checkin complete"`)
		for _, forbidden := range []string{firstName, lastName, "Hallo ", "Tschüss ", `"message"`, `"class"`} {
			assert.NotContainsf(t, out, forbidden, "Info logs must not carry %q:\n%s", forbidden, out)
		}
	}

	data, out := scan(map[string]any{"student_rfid": card.ID, "action": "checkin", "room_id": room.ID})
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "Hallo "+firstName+"!", data["message"], "kiosk greeting must survive the log change")
	assertLogsClean(out)
	// The diagnostic value stays: action, student_id and room are still there.
	assert.Contains(t, out, `"action":"checked_in"`)
	assert.Contains(t, out, fmt.Sprintf(`"student_id":%d`, student.ID))
	assert.Contains(t, out, fmt.Sprintf(`"room":%q`, room.Name))

	// Check-out builds a different greeting ("Tschüss X!") on the same log line.
	data, out = scan(map[string]any{"student_rfid": card.ID, "action": "checkout"})
	assert.Equal(t, "Tschüss "+firstName+"!", data["message"], "kiosk farewell must survive the log change")
	assertLogsClean(out)
	assert.Contains(t, out, fmt.Sprintf(`"student_id":%d`, student.ID))
}
