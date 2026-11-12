package grouplivehttp_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestOGSGroupLive_PlannedActivities pins the offerings on the child card
// (#6487): with the school setting on, every child carries today's planned,
// not cancelled blocks ordered by start; with it off (the default) none.
func TestOGSGroupLive_PlannedActivities(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t, fixedCalendarClock)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGSAct", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "OGSActGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	booked := testpkg.CreateTestStudent(t, tc.db, "OGSAct", "Gebucht", "OA1")
	free := testpkg.CreateTestStudent(t, tc.db, "OGSAct", "Frei", "OA1")
	testpkg.AssignStudentToGroup(t, tc.db, booked.ID, group.ID)
	testpkg.AssignStudentToGroup(t, tc.db, free.ID, group.ID)

	room := testpkg.CreateTestRoom(t, tc.db, "OGSActRoom")
	today := calendar.NewDate(2026, 8, 24)
	block := func(date calendar.Date, title, start, end, status string) {
		instance := testpkg.CreateTestActivityInstance(t, tc.db, date, room.ID, testpkg.ActivityInstanceOpts{
			Title: title, StartHHMM: start, EndHHMM: end, Status: status,
		})
		testpkg.CreateTestInstanceStudent(t, tc.db, instance.ID, booked.ID, "")
	}
	block(today, "Handball", "15:15", "16:00", "")
	block(today, "Töpfer-AG", "14:00", "15:00", "")
	block(today, "Chor", "13:00", "14:00", testpkg.ScheduleInstanceStatusCancelled)
	block(today.AddDays(1), "Schach", "14:00", "15:00", "")

	// activities returns each child's offerings and the request's statements.
	activities := func() (map[string]any, int) {
		counter := testpkg.CaptureQueriesForContext(t, tc.db)
		req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
		result := map[string]any{}
		for _, student := range decodeOGSLive(t, rr.Body.Bytes()).Data.Students {
			result[student["id"].(string)] = student["planned_activities"]
		}
		return result, counter.Total()
	}
	bookedID, freeID := strconv.FormatInt(booked.ID, 10), strconv.FormatInt(free.ID, 10)

	off, offStatements := activities()
	require.Len(t, off, 2)
	assert.Nil(t, off[bookedID], "off by default: the card stays as it was")

	ctx := testpkg.Ctx(t)
	require.NoError(t, tc.settings().SetValue(ctx, settings.KeyStudentCardActivitiesEnabled, true, nil, nil))
	t.Cleanup(func() { _ = tc.settings().ResetValue(ctx, settings.KeyStudentCardActivitiesEnabled, nil, nil) })

	on, onStatements := activities()
	// The budget scenario runs with the setting off; switched on, the whole
	// group costs two set reads (participants, then their blocks).
	assert.Equal(t, offStatements+2, onStatements, "the offerings are read set-based for the group")
	assert.Equal(t, []any{
		map[string]any{"title": "Töpfer-AG", "start_time": "14:00", "end_time": "15:00"},
		map[string]any{"title": "Handball", "start_time": "15:15", "end_time": "16:00"},
	}, on[bookedID], "today's blocks by start; the cancelled one and tomorrow's are left out")
	assert.Nil(t, on[freeID], "a child without a booking carries no offerings")
}
