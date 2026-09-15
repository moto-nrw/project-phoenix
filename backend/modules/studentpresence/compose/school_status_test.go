package compose_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchoolStatusesCheckoutOverridesRetainedYardHistory(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, db, "school-status-yard-history")
	at := time.Now().Add(-time.Hour)
	yard, checkout := at.Add(10*time.Minute), at.Add(20*time.Minute)
	day := testpkg.TodayDate().String()
	for _, scenario := range []struct {
		name           string
		yard, checkout *time.Time
		status         string
	}{
		{name: "building", status: "checked_in"},
		{name: "yard", yard: &yard, status: "on_yard"},
		{name: "checkout with yard history", yard: &yard, checkout: &checkout, status: "checked_out"},
		{name: "checkout without yard history", checkout: &checkout, status: "checked_out"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Yard", scenario.name, "3a")
			_, err := module.RecordAttendance(ctx, studentpresence.Attendance{
				StudentID: student.ID, Date: day, CheckInTime: at, DeviceID: device.ID,
				YardSince: scenario.yard, CheckOutTime: scenario.checkout,
			})
			require.NoError(t, err)
			statuses, err := module.ListSchoolStatuses(ctx, []int64{student.ID}, day)
			require.NoError(t, err)
			require.Len(t, statuses, 1)
			assert.Equal(t, scenario.status, statuses[0].Status)
			assert.Equal(t, scenario.yard == nil, statuses[0].YardSince == nil)
			assert.Equal(t, scenario.checkout == nil, statuses[0].CheckOutTime == nil)
		})
	}
}

func TestSchoolStatusesUseLatestStayAndKeepYardAndCheckoutSemantics(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	var observed []compose.Observation
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(o compose.Observation) { observed = append(observed, o) }})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "School", "Status", "3a")
	absent := testpkg.CreateTestStudent(t, db, "Absent", "Status", "3a")
	device := testpkg.CreateTestDevice(t, db, "school-status")
	at := time.Now().Add(-time.Hour)
	day := testpkg.TodayDate().String()
	row, err := module.RecordAttendance(ctx, studentpresence.Attendance{StudentID: student.ID, Date: day, CheckInTime: at, DeviceID: device.ID})
	require.NoError(t, err)
	assertStatus := func(want string) {
		t.Helper()
		statuses, err := module.ListSchoolStatuses(ctx, []int64{student.ID, absent.ID}, day)
		require.NoError(t, err)
		require.Len(t, statuses, 2)
		assert.Equal(t, want, statuses[0].Status)
		assert.Equal(t, day, statuses[0].Date)
		assert.Equal(t, "not_checked_in", statuses[1].Status)
		assert.Nil(t, statuses[1].CheckInTime)
		last := observed[len(observed)-1]
		assert.Equal(t, "list_school_statuses", last.Operation)
		assert.Equal(t, 1, last.Queries, "all student statuses use one query")
		require.NoError(t, last.Err)
	}
	assertStatus("checked_in")
	yard := at.Add(10 * time.Minute)
	row.YardSince = &yard
	_, err = module.ReviseAttendance(ctx, row)
	require.NoError(t, err)
	assertStatus("on_yard")
	_, err = module.CloseAttendance(ctx, studentpresence.AttendanceCheckout{StudentIDs: []int64{student.ID}, Date: day, At: at.Add(20 * time.Minute)})
	require.NoError(t, err)
	assertStatus("checked_out")
	_, err = module.RecordAttendance(ctx, studentpresence.Attendance{StudentID: student.ID, Date: day, CheckInTime: at.Add(30 * time.Minute), DeviceID: device.ID})
	require.NoError(t, err)
	assertStatus("checked_in")
}
