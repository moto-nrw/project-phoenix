package compose_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The presence history of the demo seed (#5031) lands as finished days:
// attendance closed by the kiosk, the visit inside one ended session shared
// by the children of that room, every row created at its own instant.
func TestRecordPresenceHistoryWritesFinishedPastDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	history := compose.NewPresenceHistory(compose.Dependencies{DB: db})
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, db, "presence-history")
	activity := testpkg.CreateTestActivityGroup(t, db, "Presence history")
	room := testpkg.CreateTestRoom(t, db, "Presence history")
	first := testpkg.CreateTestStudent(t, db, "History", "First", "3a")
	second := testpkg.CreateTestStudent(t, db, "History", "Second", "3a")
	day := calendar.NewDate(2026, time.March, 10)
	at := func(hour, minute int) time.Time { return time.Date(2026, time.March, 10, hour, minute, 0, 0, berlin) }
	visit := func(entry, exit time.Time) *compose.PastVisit {
		return &compose.PastVisit{ActivityGroupID: activity.ID, RoomID: room.ID,
			SessionStart: at(12, 0), SessionEnd: at(16, 45), Entry: entry, Exit: exit}
	}

	days := []compose.PastPresenceDay{
		{StudentID: first.ID, Date: day, DeviceID: device.ID, CheckIn: at(11, 50), CheckOut: at(15, 30), Visit: visit(at(12, 0), at(15, 25))},
		{StudentID: second.ID, Date: day, DeviceID: device.ID, CheckIn: at(12, 10), CheckOut: at(16, 0), Visit: visit(at(12, 15), at(15, 55))},
	}
	written, err := history.RecordPresenceHistory(ctx, days)
	require.NoError(t, err)
	assert.Equal(t, 2, written)
	written, err = history.RecordPresenceHistory(ctx, days)
	require.NoError(t, err)
	assert.Equal(t, 2, written, "a retry reuses the deterministic history")

	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{
		StudentIDs: []int64{first.ID, second.ID}, FromDate: day.String(), UntilDate: day.String(), StudentOrder: true,
	})
	require.NoError(t, err)
	require.Len(t, attendance, 2)
	for _, row := range attendance {
		require.NotNil(t, row.CheckOutTime, "a past day is closed")
		assert.True(t, row.CreatedAt.Equal(row.CheckInTime), "created at the arrival")
		assert.Equal(t, device.ID, row.DeviceID)
	}
	visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{first.ID, second.ID}})
	require.NoError(t, err)
	require.Len(t, visits, 2)
	assert.Equal(t, visits[0].ActiveGroupID, visits[1].ActiveGroupID, "one session per day, activity and room")
	for _, row := range visits {
		require.NotNil(t, row.ExitTime)
		assert.True(t, row.CreatedAt.Equal(row.EntryTime), "created at the entry, so the retention counts from then")
	}
	sessions, err := module.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{IDs: []int64{visits[0].ActiveGroupID}})
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.NotNil(t, sessions[0].EndTime, "the session is finished")
	assert.True(t, sessions[0].StartTime.Equal(at(12, 0)))
	assert.Equal(t, room.ID, sessions[0].RoomID)
}

func TestRecordPresenceHistoryAcceptsAnEmptyBatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	history := compose.NewPresenceHistory(compose.Dependencies{DB: db})

	written, err := history.RecordPresenceHistory(testpkg.Ctx(t), nil)

	require.NoError(t, err)
	assert.Zero(t, written)
}

// A day in the future, or a visit outside its attendance, rejects the whole
// batch before anything is written.
func TestRecordPresenceHistoryRejectsDaysNoCareDayProduced(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	history := compose.NewPresenceHistory(compose.Dependencies{DB: db})
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, db, "presence-history-reject")
	activity := testpkg.CreateTestActivityGroup(t, db, "Presence history reject")
	room := testpkg.CreateTestRoom(t, db, "Presence history reject")
	student := testpkg.CreateTestStudent(t, db, "History", "Rejected", "3a")
	past := calendar.NewDate(2026, time.March, 10)
	valid := studentpresence.PastPresenceDay{StudentID: student.ID, Date: past, DeviceID: device.ID,
		CheckIn: time.Date(2026, time.March, 10, 12, 0, 0, 0, berlin), CheckOut: time.Date(2026, time.March, 10, 15, 0, 0, 0, berlin)}
	futureDate := calendar.NewDate(2099, time.January, 2)
	future := valid
	future.Date = futureDate
	future.CheckIn = time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 12, 0, 0, 0, berlin)
	future.CheckOut = future.CheckIn.Add(3 * time.Hour)
	outside := valid
	outside.Visit = &studentpresence.PastVisit{ActivityGroupID: activity.ID, RoomID: room.ID,
		SessionStart: valid.CheckIn, SessionEnd: valid.CheckOut.Add(time.Hour), Entry: valid.CheckIn, Exit: valid.CheckOut.Add(30 * time.Minute)}

	for name, day := range map[string]studentpresence.PastPresenceDay{"future": future, "visit after departure": outside} {
		_, err := history.RecordPresenceHistory(ctx, []studentpresence.PastPresenceDay{valid, day})
		require.Error(t, err, name)
	}
	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	assert.Empty(t, attendance, "a rejected batch writes nothing")
}

func TestRecordPresenceHistoryRejectsOpenAttendance(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	history := compose.NewPresenceHistory(compose.Dependencies{DB: db})
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, db, "presence-history-open")
	student := testpkg.CreateTestStudent(t, db, "History", "Open", "3a")
	day := calendar.NewDate(2026, time.March, 10)
	checkIn := time.Date(2026, time.March, 10, 12, 0, 0, 0, berlin)
	checkOut := checkIn.Add(3 * time.Hour)
	_, err = module.RecordAttendance(ctx, studentpresence.Attendance{StudentID: student.ID, Date: day.String(), DeviceID: device.ID, CheckInTime: checkIn})
	require.NoError(t, err)

	_, err = history.RecordPresenceHistory(ctx, []compose.PastPresenceDay{{
		StudentID: student.ID, Date: day, DeviceID: device.ID, CheckIn: checkIn, CheckOut: checkOut,
	}})

	require.ErrorContains(t, err, "already has open attendance")
	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}, FromDate: day.String(), UntilDate: day.String()})
	require.NoError(t, err)
	require.Len(t, attendance, 1)
	assert.Nil(t, attendance[0].CheckOutTime)
}
