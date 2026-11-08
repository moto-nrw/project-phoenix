package compose_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type presenceHistoryFixture struct {
	device, activity, roomA, roomB int64
	berlin                         *time.Location
}

func newPresenceHistoryFixture(t *testing.T, name string) presenceHistoryFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	return presenceHistoryFixture{
		device:   testpkg.CreateTestDevice(t, db, name).ID,
		activity: testpkg.CreateTestActivityGroup(t, db, name).ID,
		roomA:    testpkg.CreateTestRoom(t, db, name+" A").ID,
		roomB:    testpkg.CreateTestRoom(t, db, name+" B").ID,
		berlin:   berlin,
	}
}

func (f presenceHistoryFixture) day(studentID int64, date calendar.Date, roomID int64) compose.PastPresenceDay {
	at := func(hour, minute int) time.Time {
		return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, f.berlin)
	}
	day := compose.PastPresenceDay{StudentID: studentID, Date: date, DeviceID: f.device, CheckIn: at(11, 50), CheckOut: at(16, 0)}
	if roomID != 0 {
		day.Visit = &compose.PastVisit{ActivityGroupID: f.activity, RoomID: roomID,
			SessionStart: at(12, 0), SessionEnd: at(15, 45), Entry: at(12, 5), Exit: at(15, 40)}
	}
	return day
}

// On each date two children share the session of room A, a third has room B
// to itself and a fourth stays without a visit (#5850).
func (f presenceHistoryFixture) days(students [4]int64, dates int) []compose.PastPresenceDay {
	days := make([]compose.PastPresenceDay, 0, 4*dates)
	for offset := range dates {
		date := calendar.NewDate(2026, time.March, 2+offset)
		days = append(days, f.day(students[0], date, f.roomA), f.day(students[1], date, f.roomA),
			f.day(students[2], date, f.roomB), f.day(students[3], date, 0))
	}
	return days
}

// The statement count of a history batch does not grow with its days, and
// every row, shared session and creation instant stays as before (#5850).
func TestRecordPresenceHistoryStatementsDoNotGrowWithDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	history := compose.NewPresenceHistory(compose.Dependencies{DB: db})
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	fixture := newPresenceHistoryFixture(t, "presence-history-batch")

	statements := map[int]int{}
	for _, dates := range []int{3, 8} {
		var students [4]int64
		for i := range students {
			students[i] = testpkg.CreateTestStudent(t, db, "Batch", fmt.Sprintf("History %d %d", dates, i), "3a").ID
		}
		days := fixture.days(students, dates)
		counter := testpkg.CaptureQueriesForContext(t, db)
		written, err := history.RecordPresenceHistory(counter.Context(testpkg.Ctx(t)), days)
		counter.Stop()
		require.NoError(t, err)
		assert.Equal(t, len(days), written)
		statements[dates] = counter.Total()
		t.Logf("%d dates, %d days: %d statements", dates, len(days), counter.Total())

		ctx := testpkg.Ctx(t)
		attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: students[:]})
		require.NoError(t, err)
		require.Len(t, attendance, len(days))
		for _, row := range attendance {
			require.NotNil(t, row.CheckOutTime)
			assert.True(t, row.CreatedAt.Equal(row.CheckInTime))
			assert.True(t, row.UpdatedAt.Equal(*row.CheckOutTime))
			assert.Equal(t, fixture.device, row.DeviceID)
			require.NotNil(t, row.CheckedOutDeviceID)
			assert.Equal(t, fixture.device, *row.CheckedOutDeviceID)
		}
		visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: students[:]})
		require.NoError(t, err)
		require.Len(t, visits, 3*dates)
		sessionOf := map[string]int64{}
		sessions := map[int64]bool{}
		for _, visit := range visits {
			require.NotNil(t, visit.ExitTime)
			assert.True(t, visit.CreatedAt.Equal(visit.EntryTime))
			assert.True(t, visit.UpdatedAt.Equal(*visit.ExitTime))
			sessionOf[fmt.Sprintf("%d/%s", visit.StudentID, calendar.DateFromTime(visit.EntryTime))] = visit.ActiveGroupID
			sessions[visit.ActiveGroupID] = true
		}
		require.Len(t, sessions, 2*dates, "one session per date, activity and room")
		for offset := range dates {
			date := calendar.NewDate(2026, time.March, 2+offset)
			shared := sessionOf[fmt.Sprintf("%d/%s", students[0], date)]
			assert.Equal(t, shared, sessionOf[fmt.Sprintf("%d/%s", students[1], date)])
			assert.NotEqual(t, shared, sessionOf[fmt.Sprintf("%d/%s", students[2], date)])
		}
		ids := make([]int64, 0, len(sessions))
		for id := range sessions {
			ids = append(ids, id)
		}
		groups, err := module.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{IDs: ids})
		require.NoError(t, err)
		require.Len(t, groups, len(ids))
		withTimeout, err := db.NewSelect().Table("active.groups").
			Where("tenant_id = ?", testpkg.Tenant(t)).Where("id IN (?)", bun.List(ids)).
			Where("timeout_minutes IS NOT NULL").Count(ctx)
		require.NoError(t, err)
		assert.Zero(t, withTimeout, "past sessions store NULL, not the database timeout default")
		for _, group := range groups {
			assert.Zero(t, group.TimeoutMinutes)
			require.NotNil(t, group.EndTime)
			require.NotNil(t, group.ActivityGroupID)
			assert.Equal(t, fixture.activity, *group.ActivityGroupID)
			require.NotNil(t, group.DeviceID)
			assert.Equal(t, fixture.device, *group.DeviceID)
			assert.True(t, group.CreatedAt.Equal(group.StartTime))
			assert.True(t, group.UpdatedAt.Equal(*group.EndTime))
			assert.True(t, group.LastActivity.Equal(*group.EndTime))
		}

		written, err = history.RecordPresenceHistory(ctx, days)
		require.NoError(t, err, "a retry reuses the deterministic history")
		assert.Equal(t, len(days), written)
		again, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: students[:]})
		require.NoError(t, err)
		assert.Len(t, again, len(visits))
	}
	assert.Equal(t, statements[3], statements[8], "the statements do not grow with the days")
}

// The first failing write in input order names the error, and the batch
// writes nothing (#5850): a session of day 2 with a foreign room fails before
// the attendance of day 3 with a foreign child.
func TestRecordPresenceHistoryReportsTheFirstFailingWriteAndRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	history := compose.NewPresenceHistory(compose.Dependencies{DB: db})
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	fixture := newPresenceHistoryFixture(t, "presence-history-first-failure")
	own := testpkg.CreateTestStudent(t, db, "Batch", "Own", "3a")
	var foreignStudent, foreignRoom int64
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignStudent = testpkg.CreateTestStudent(t, db, "Batch", "Foreign", "3a").ID
		foreignRoom = testpkg.CreateTestRoom(t, db, "presence-history-foreign-room").ID
	})
	first := calendar.NewDate(2026, time.March, 2)
	second := calendar.NewDate(2026, time.March, 3)
	third := calendar.NewDate(2026, time.March, 4)
	days := []compose.PastPresenceDay{
		fixture.day(own.ID, first, fixture.roomA),
		fixture.day(own.ID, second, foreignRoom),
		fixture.day(foreignStudent, third, 0),
	}

	_, err = history.RecordPresenceHistory(ctx, days)

	require.ErrorIs(t, err, studentpresence.ErrPresenceHistoryInvalid)
	require.True(t, transactionruntime.IsConstraintViolation(err))
	assert.Contains(t, err.Error(), fmt.Sprintf("record past session in room %d on %s: record group: ", foreignRoom, second))
	assert.NotContains(t, err.Error(), "record past attendance")
	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{own.ID}})
	require.NoError(t, err)
	assert.Empty(t, attendance, "a failed batch rolls back the days before the failure")
	visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{own.ID}})
	require.NoError(t, err)
	assert.Empty(t, visits)

	_, err = history.RecordPresenceHistory(ctx, days[2:])
	require.ErrorIs(t, err, studentpresence.ErrPresenceHistoryInvalid)
	assert.Contains(t, err.Error(), fmt.Sprintf("record past attendance of student %d on %s: ", foreignStudent, third))
}

// Two visits of one date, activity and room must name the same session.
func TestRecordPresenceHistoryRejectsDifferentBoundsOfOneSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	history := compose.NewPresenceHistory(compose.Dependencies{DB: db})
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	fixture := newPresenceHistoryFixture(t, "presence-history-session-bounds")
	first := testpkg.CreateTestStudent(t, db, "Bounds", "First", "3a")
	second := testpkg.CreateTestStudent(t, db, "Bounds", "Second", "3a")
	date := calendar.NewDate(2026, time.March, 2)
	other := fixture.day(second.ID, date, fixture.roomA)
	other.Visit.SessionEnd = other.Visit.SessionEnd.Add(time.Minute)

	_, err = history.RecordPresenceHistory(ctx, []compose.PastPresenceDay{fixture.day(first.ID, date, fixture.roomA), other})

	require.ErrorIs(t, err, studentpresence.ErrPresenceHistoryInvalid)
	assert.Equal(t, fmt.Sprintf("%s: visits in room %d on %s name different session bounds",
		studentpresence.ErrPresenceHistoryInvalid, fixture.roomA, date), err.Error())
	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{first.ID, second.ID}})
	require.NoError(t, err)
	assert.Empty(t, attendance)
}
