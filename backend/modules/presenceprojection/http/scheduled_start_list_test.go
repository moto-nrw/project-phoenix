package presenceprojectionhttp_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// A child approved for a later care start waits as pending with that start
// (#6041). The running child list, its count and the export derived from it
// leave the child out until the start day; a chosen planning day from the
// start on, the child management and the view of every child still find it. An
// immediately activated child, a running renewal and a child actually
// present stay.
func TestStudentListCountsAScheduledChildOnlyFromItsCareStart(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)
	today := studentsTestToday // Monday 24.08.2026
	start := today.AddDays(3)  // Thursday, inside the planning horizon
	pastStart := today.AddDays(-30)
	until := today.AddDays(300)

	scheduled := testpkg.CreateTestStudent(t, tc.db, "Geplant", "Februarstart", "1a")
	testpkg.SetStudentLifecycle(t, tc.db, scheduled.ID, testpkg.StudentStatusPending, &start, &until)
	endedScheduled := testpkg.CreateTestStudent(t, tc.db, "Beendet", "VorBeginn", "1a")
	ended := today.AddDays(-1)
	testpkg.SetStudentLifecycle(t, tc.db, endedScheduled.ID, testpkg.StudentStatusPending, &start, &ended)
	immediate := testpkg.CreateTestStudent(t, tc.db, "Sofort", "Aktiviert", "1a")
	testpkg.SetStudentLifecycle(t, tc.db, immediate.ID, testpkg.StudentStatusActive, &start, &until)
	renewed := testpkg.CreateTestStudent(t, tc.db, "Laufend", "Verlaengert", "2a")
	testpkg.SetStudentLifecycle(t, tc.db, renewed.ID, testpkg.StudentStatusActive, &pastStart, &until)
	present := testpkg.CreateTestStudent(t, tc.db, "Schon", "Anwesend", "2a")
	testpkg.SetStudentLifecycle(t, tc.db, present.ID, testpkg.StudentStatusPending, &start, &until)
	staff := testpkg.CreateTestStaff(t, tc.db, "Betreuung", "Startliste")
	device := testpkg.CreateTestDevice(t, tc.db, "startliste-geraet")
	testpkg.CreateTestAttendanceForDate(t, tc.db, present.ID, staff.ID, device.ID, today, fixedCalendarClock(), nil)

	actor := testpkg.CreateTestAccount(t, tc.db, "startliste@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	listed := func(query string) (map[int64]bool, int) {
		t.Helper()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/"+query, nil)
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())
		var body struct {
			Data []struct {
				ID int64 `json:"id"`
			} `json:"data"`
			Pagination struct {
				TotalRecords int `json:"total_records"`
			} `json:"pagination"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		ids := map[int64]bool{}
		for _, row := range body.Data {
			ids[row.ID] = true
		}
		return ids, body.Pagination.TotalRecords
	}

	rows, total := listed("?page_size=500")
	assert.False(t, rows[scheduled.ID], "before its care start the scheduled child is not in the running list")
	assert.True(t, rows[immediate.ID], "an immediately activated child stays")
	assert.True(t, rows[renewed.ID], "a running renewal stays")
	assert.True(t, rows[present.ID], "a child actually present stays")
	assert.Equal(t, 3, total, "the count leaves the scheduled child out too")

	dayBefore, totalBefore := listed("?page_size=500&date=" + start.AddDays(-1).String())
	assert.False(t, dayBefore[scheduled.ID], "the day before the start does not count")
	assert.Equal(t, 2, totalBefore, "on a later planning day presence no longer keeps the waiting child")
	onStart, totalOnStart := listed("?page_size=500&date=" + start.String())
	assert.True(t, onStart[scheduled.ID], "the start day counts")
	assert.True(t, onStart[present.ID])
	assert.Equal(t, 4, totalOnStart)
	afterStart, _ := listed("?page_size=500&date=" + start.AddDays(1).String())
	assert.True(t, afterStart[scheduled.ID], "a planning day after the start counts")

	all, _ := listed("?page_size=500&care_status=all")
	assert.True(t, all[scheduled.ID], "the administrative view of every child keeps the scheduled child")
	assert.True(t, all[endedScheduled.ID], "the explicit view of all children still includes ended care")
	// The child management (Datenverwaltung → Kinder) asks with
	// include_pending_withdrawals and must still find the child to prepare it.
	managed, managedTotal := listed("?page_size=500&include_pending_withdrawals=true")
	assert.True(t, managed[scheduled.ID], "the child management keeps the scheduled child")
	assert.False(t, managed[endedScheduled.ID], "a scheduled start does not restore ended care without presence or a withdrawal task")
	assert.Equal(t, 4, managedTotal)

	exported := func(date string) string {
		t.Helper()
		body := map[string]any{"format": "xlsx", "preset": "blank_checklist", "filters": map[string]string{"date": date}}
		request := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/export", body)
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())
		// An xlsx file is a zip of XML parts; the names stand in their text.
		data := response.Body.Bytes()
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(t, err)
		var text strings.Builder
		for _, part := range archive.File {
			if !strings.HasSuffix(part.Name, ".xml") {
				continue
			}
			reader, err := part.Open()
			require.NoError(t, err)
			content, err := io.ReadAll(reader)
			require.NoError(t, reader.Close())
			require.NoError(t, err)
			text.Write(content)
		}
		return text.String()
	}
	todayExport := exported("")
	assert.NotContains(t, todayExport, "Februarstart", "the export leaves the scheduled child out before its start")
	assert.Contains(t, todayExport, "Verlaengert")
	assert.Contains(t, exported(start.String()), "Februarstart", "the export for the start day lists the child")
}
