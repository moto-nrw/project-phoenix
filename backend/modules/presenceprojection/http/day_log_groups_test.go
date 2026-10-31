package presenceprojectionhttp_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	presenceprojectionhttp "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The weekly protocol's evening run (#4769) reads today's day log of the
// groups without a request: the same verdicts as GET /day-log, the children
// of each group and the missing ones with their label.
func TestGroupDayLogsReportTodaysVerdicts(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Wochenprotokoll Gruppe")
	present := testpkg.CreateTestStudent(t, tc.db, "Paula", "Da", "1a")
	sick := testpkg.CreateTestStudent(t, tc.db, "Sina", "Krank", "1a")
	missing := testpkg.CreateTestStudent(t, tc.db, "Momo", "Fehlt", "1b")
	staff := testpkg.CreateTestStaff(t, tc.db, "Woche", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, "week-protocol-dev")
	for _, id := range []int64{present.ID, sick.ID, missing.ID} {
		testpkg.AssignStudentToGroup(t, tc.db, id, group.ID)
	}
	today := calendar.NewDate(2026, 8, 24)
	checkIn := today.BerlinMidnight().Add(8 * time.Hour)
	testpkg.CreateTestAttendanceForDate(t, tc.db, present.ID, staff.ID, device.ID, today, checkIn, nil)
	testpkg.CreateTestStudentStatusDay(t, tc.db, sick.ID, today, absencerecords.StudentStatusDaySick)

	// The evening run reads inside the school's transaction, like here.
	var logs []presenceprojectionhttp.GroupDayLog
	require.NoError(t, tenant.WithinCurrentTenant(testpkg.Ctx(t), func(ctx context.Context) error {
		_, err := tc.resource.GroupDayLogs(ctx, today.AddDays(-1), []int64{group.ID})
		require.ErrorIs(t, err, presenceprojectionhttp.ErrGroupDayLogNotToday, "a past day cannot be attributed to today's groups")
		logs, err = tc.resource.GroupDayLogs(ctx, today, []int64{group.ID})
		return err
	}))
	require.Len(t, logs, 1)
	got := logs[0]
	assert.Equal(t, group.ID, got.GroupID)
	assert.ElementsMatch(t, []int64{present.ID, sick.ID, missing.ID}, got.StudentIDs)
	assert.Equal(t, 1, got.Present)
	assert.Equal(t, 1, got.Sick)
	assert.Equal(t, 1, got.Absent)
	labels := map[int64]string{}
	for _, child := range got.MissingChild {
		labels[child.StudentID] = child.Label
	}
	assert.Equal(t, map[int64]string{sick.ID: "Krank", missing.ID: "Abwesend"}, labels, "the present child is not missing")
}
