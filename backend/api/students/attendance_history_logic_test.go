package students

// Unit tests for the pure logic functions used by the attendance-history handler.
// These tests live in the internal package (no _test suffix on the package name)
// so they can exercise the unexported helpers directly. Full HTTP-level
// integration tests live in attendance_history_handlers_test.go (package
// students_test) and require a test database.

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAttendanceHistoryRange_Defaults(t *testing.T) {
	t.Parallel()

	defaultStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	defaultEnd := time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)

	req := httptest.NewRequest("GET", "/students/1/attendance-history", nil)
	start, end, err := parseAttendanceHistoryRange(req, defaultStart, defaultEnd)
	require.NoError(t, err)
	assert.Equal(t, defaultStart, start)
	assert.Equal(t, defaultEnd, end)
}

func TestParseAttendanceHistoryRange_ExplicitParams(t *testing.T) {
	t.Parallel()

	defaultStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	defaultEnd := time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)

	req := httptest.NewRequest("GET", "/students/1/attendance-history?start=2026-03-10T00:00:00Z&end=2026-03-20T23:59:59Z", nil)
	start, end, err := parseAttendanceHistoryRange(req, defaultStart, defaultEnd)
	require.NoError(t, err)
	assert.Equal(t, 2026, start.Year())
	assert.Equal(t, time.March, start.Month())
	assert.Equal(t, 10, start.Day())
	assert.Equal(t, 20, end.Day())
}

func TestParseAttendanceHistoryRange_InvalidStart(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/students/1/attendance-history?start=not-a-date", nil)
	_, _, err := parseAttendanceHistoryRange(req, time.Now(), time.Now())
	require.Error(t, err)
}

func TestParseAttendanceHistoryRange_StartAfterEnd(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/students/1/attendance-history?start=2026-03-20T00:00:00Z&end=2026-03-10T00:00:00Z", nil)
	_, _, err := parseAttendanceHistoryRange(req, time.Now(), time.Now())
	require.Error(t, err)
}

func TestClampAttendanceHistoryRange_ClampsFutureEndAndVisibilityCap(t *testing.T) {
	t.Parallel()

	endOfToday := time.Date(2026, 3, 20, 23, 59, 59, 0, time.UTC)
	start := endOfToday.AddDate(0, 0, -60)
	end := endOfToday.AddDate(0, 0, 5)

	gotStart, gotEnd, clamped, err := clampAttendanceHistoryRange(start, end, endOfToday, 30)
	require.NoError(t, err)
	assert.True(t, clamped)
	assert.Equal(t, endOfToday, gotEnd)
	assert.Equal(t, endOfToday.Add(-30*24*time.Hour).Add(time.Second), gotStart)
}

func TestClampAttendanceHistoryRange_RejectsFullyFutureRange(t *testing.T) {
	t.Parallel()

	endOfToday := time.Date(2026, 3, 20, 23, 59, 59, 0, time.UTC)
	start := endOfToday.Add(time.Second)
	end := start.Add(24 * time.Hour)

	_, _, _, err := clampAttendanceHistoryRange(start, end, endOfToday, 30)
	require.EqualError(t, err, "start must not be in the future")
}

func TestBuildAttendanceHistoryDays_EmptyRows(t *testing.T) {
	t.Parallel()

	days := buildAttendanceHistoryDays(nil, nil, nil, time.Now(), false)
	assert.Empty(t, days)
}

func TestBuildAttendanceHistoryDays_WithinRoomCap_IncludesVisits(t *testing.T) {
	t.Parallel()

	date := timezone.Today()
	roomCutoff := date.AddDate(0, 0, -6)
	checkIn := date.Add(8 * time.Hour)
	checkOut := checkIn.Add(5*time.Hour + 30*time.Minute)

	row := &active.Attendance{
		TenantModel:  base.TenantModel{TenantID: testpkg.Tenant(t)},
		StudentID:    10,
		Date:         timezone.DateFromTime(date),
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		CheckedInBy:  42,
		DeviceID:     7,
	}
	exit := checkIn.Add(90 * time.Minute)
	visit := &active.Visit{
		TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)},
		StudentID:   10,
		EntryTime:   checkIn,
		ExitTime:    &exit,
		ActiveGroup: &active.Group{
			RoomID: 5,
			Room:   &facilities.Room{Name: "Gruppenraum A"},
		},
	}
	key := timezone.DateOf(date).Format("2006-01-02")
	days := buildAttendanceHistoryDays(
		[]*active.Attendance{row},
		nil,
		map[string][]*active.Visit{key: {visit}},
		roomCutoff,
		false,
	)

	require.Len(t, days, 1)
	day := days[0]
	assert.Equal(t, key, day.Date)
	assert.True(t, day.RoomDetailAvailable)
	require.NotNil(t, day.Attendance)
	require.NotNil(t, day.Attendance.DurationMinutes)
	assert.Equal(t, 330, *day.Attendance.DurationMinutes, "duration should be (checkout-checkin) in minutes")
	require.Len(t, day.Visits, 1)
	assert.Equal(t, "Gruppenraum A", day.Visits[0].RoomName)
	require.NotNil(t, day.Visits[0].DurationMinutes)
	assert.Equal(t, 90, *day.Visits[0].DurationMinutes)
}

func TestBuildAttendanceHistoryDays_ExactlyOnRoomCutoff_IncludesVisits(t *testing.T) {
	t.Parallel()

	// Attendance date exactly equals roomCutoff → should still include room detail.
	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)
	checkIn := roomCutoff.Add(8 * time.Hour)
	checkOut := roomCutoff.Add(15 * time.Hour)

	row := &active.Attendance{
		TenantModel:  base.TenantModel{TenantID: testpkg.Tenant(t)},
		StudentID:    10,
		Date:         timezone.DateFromTime(roomCutoff),
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		CheckedInBy:  42,
		DeviceID:     7,
	}
	exit := checkIn.Add(time.Hour)
	visit := &active.Visit{
		TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)},
		StudentID:   10,
		EntryTime:   checkIn,
		ExitTime:    &exit,
		ActiveGroup: &active.Group{
			RoomID: 5,
			Room:   &facilities.Room{Name: "Boundary Room"},
		},
	}
	key := timezone.DateOf(roomCutoff).Format("2006-01-02")
	days := buildAttendanceHistoryDays(
		[]*active.Attendance{row},
		nil,
		map[string][]*active.Visit{key: {visit}},
		roomCutoff,
		false,
	)
	require.Len(t, days, 1)
	assert.True(t, days[0].RoomDetailAvailable, "date ON cutoff should have room detail available")
	assert.Len(t, days[0].Visits, 1)
}

func TestBuildAttendanceHistoryDays_VisitWithNilActiveGroup(t *testing.T) {
	t.Parallel()

	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)
	checkIn := today.Add(8 * time.Hour)

	row := &active.Attendance{
		TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)},
		StudentID:   10,
		Date:        timezone.DateFromTime(today),
		CheckInTime: checkIn,
		CheckedInBy: 42,
		DeviceID:    7,
	}
	exit := checkIn.Add(time.Hour)
	visit := &active.Visit{
		TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)},
		StudentID:   10,
		EntryTime:   checkIn,
		ExitTime:    &exit,
		ActiveGroup: nil, // No group info
	}
	key := timezone.DateOf(today).Format("2006-01-02")
	days := buildAttendanceHistoryDays(
		[]*active.Attendance{row},
		nil,
		map[string][]*active.Visit{key: {visit}},
		roomCutoff,
		false,
	)
	require.Len(t, days, 1)
	require.Len(t, days[0].Visits, 1)
	assert.Nil(t, days[0].Visits[0].RoomID, "nil ActiveGroup should yield nil RoomID")
	assert.Equal(t, "", days[0].Visits[0].RoomName)
}

func TestBuildAttendanceHistoryDays_MultipleDays(t *testing.T) {
	t.Parallel()

	today := timezone.Today()
	yesterday := today.AddDate(0, 0, -1)
	roomCutoff := today.AddDate(0, 0, -6)

	rows := []*active.Attendance{
		{TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: today.Add(8 * time.Hour), CheckedInBy: 42, DeviceID: 7},
		{TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)}, StudentID: 10, Date: timezone.DateFromTime(yesterday), CheckInTime: yesterday.Add(8 * time.Hour), CheckedInBy: 42, DeviceID: 7},
	}
	days := buildAttendanceHistoryDays(rows, nil, map[string][]*active.Visit{}, roomCutoff, false)
	assert.Len(t, days, 2, "should return one day entry per unique date")
}

func TestBuildAttendanceHistoryDays_StatusOnlyDay(t *testing.T) {
	t.Parallel()

	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)
	reportedAt := today.Add(7 * time.Hour)
	clearedAt := today.Add(16 * time.Hour)

	statusRows := []*active.StudentStatusDay{
		{
			TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)},
			StudentID:   10,
			Date:        timezone.DateFromTime(today),
			Status:      active.StudentStatusDaySick,
			ReportedAt:  reportedAt,
			ClearedAt:   &clearedAt,
			Source:      active.StudentStatusSourceEndOfDay,
		},
	}

	days := buildAttendanceHistoryDays(nil, statusRows, map[string][]*active.Visit{}, roomCutoff, false)

	require.Len(t, days, 1)
	assert.Equal(t, timezone.DateFromTime(today).String(), days[0].Date)
	assert.Nil(t, days[0].Attendance)
	require.Len(t, days[0].StatusEntries, 1)
	assert.Equal(t, "Krank", days[0].StatusEntries[0].Label)
	assert.Equal(t, reportedAt, days[0].StatusEntries[0].ReportedAt)
	require.NotNil(t, days[0].StatusEntries[0].ClearedAt)
	assert.Equal(t, clearedAt, *days[0].StatusEntries[0].ClearedAt)
}

func TestBuildAttendanceHistoryDays_MultipleRowsSameDay_Consolidated(t *testing.T) {
	t.Parallel()

	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)

	// Two attendance rows on the same day: morning session + afternoon re-check-in.
	morningIn := today.Add(8 * time.Hour)
	morningOut := today.Add(12 * time.Hour)
	afternoonIn := today.Add(13 * time.Hour)
	afternoonOut := today.Add(16 * time.Hour)

	rows := []*active.Attendance{
		{TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: morningIn, CheckOutTime: &morningOut, CheckedInBy: 42, DeviceID: 7},
		{TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: afternoonIn, CheckOutTime: &afternoonOut, CheckedInBy: 43, DeviceID: 8},
	}
	days := buildAttendanceHistoryDays(rows, nil, map[string][]*active.Visit{}, roomCutoff, false)

	require.Len(t, days, 1, "two rows on same day must consolidate into one entry")
	day := days[0]
	assert.Equal(t, morningIn, day.Attendance.CheckInTime, "earliest check-in wins")
	require.NotNil(t, day.Attendance.CheckOutTime)
	assert.Equal(t, afternoonOut, *day.Attendance.CheckOutTime, "latest check-out wins")
	assert.Equal(t, int64(42), day.Attendance.CheckedInBy, "checked_in_by from earliest row")
	require.NotNil(t, day.Attendance.DurationMinutes)
	assert.Equal(t, 420, *day.Attendance.DurationMinutes, "duration sums sessions without counting the school-time gap")
	require.Len(t, day.Attendance.Sessions, 2)
}

func TestBuildAttendanceHistoryDays_MultipleRowsSameDay_OneOpenSession(t *testing.T) {
	t.Parallel()

	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)

	morningIn := today.Add(8 * time.Hour)
	morningOut := today.Add(12 * time.Hour)
	afternoonIn := today.Add(13 * time.Hour)
	// Second session still open (no check-out).

	rows := []*active.Attendance{
		{TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: morningIn, CheckOutTime: &morningOut, CheckedInBy: 42, DeviceID: 7},
		{TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: afternoonIn, CheckedInBy: 43, DeviceID: 8},
	}
	days := buildAttendanceHistoryDays(rows, nil, map[string][]*active.Visit{}, roomCutoff, false)

	require.Len(t, days, 1)
	assert.Nil(t, days[0].Attendance.CheckOutTime, "nil check-out (still present) takes precedence")
	assert.Nil(t, days[0].Attendance.DurationMinutes, "no duration when session is still open")
}

func TestBuildAttendanceHistoryDays_OutsideRoomCap_HidesVisits(t *testing.T) {
	t.Parallel()

	// Attendance from 10 days ago, room cutoff at 7 days ago → no room detail.
	today := timezone.Today()
	oldDate := today.AddDate(0, 0, -10)
	roomCutoff := today.AddDate(0, 0, -6)
	checkIn := oldDate.Add(8 * time.Hour)

	row := &active.Attendance{
		TenantModel: base.TenantModel{TenantID: testpkg.Tenant(t)},
		StudentID:   10,
		Date:        timezone.DateFromTime(oldDate),
		CheckInTime: checkIn,
		CheckedInBy: 42,
		DeviceID:    7,
	}
	days := buildAttendanceHistoryDays(
		[]*active.Attendance{row},
		nil,
		map[string][]*active.Visit{},
		roomCutoff,
		false,
	)
	require.Len(t, days, 1)
	assert.False(t, days[0].RoomDetailAvailable, "older-than-room-cap day must hide visits")
	assert.Empty(t, days[0].Visits)
	assert.Nil(t, days[0].Attendance.DurationMinutes, "duration is nil when check_out_time is nil")
}

func TestAttachSlotAttendance_KeepsOpposingStatusesOnSameDay(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 7, 15)
	morning := &schedule.ActivityInstance{
		Date: schedule.Date(date), Title: "Morgenbetreuung",
		StartTime: time.Date(1, 1, 1, 7, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
	}
	morning.ID = 101
	afternoon := &schedule.ActivityInstance{
		Date: schedule.Date(date), Title: "Nachmittagsbetreuung",
		StartTime: time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
	}
	afternoon.ID = 102
	sick := schedule.AttendanceSubstatusSick

	days := attachSlotAttendance(nil, []*schedule.ScheduledInstanceRow{
		{Instance: morning, Attendance: &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent}},
		{Instance: afternoon, Attendance: &schedule.InstanceStudent{Status: schedule.AttendanceStatusAbsent, Substatus: &sick}},
	}, nil, date.BerlinMidnight(), false)

	require.Len(t, days, 1)
	assert.True(t, days[0].RoomDetailAvailable, "slot-only day within the retention window must expose room details")
	require.Len(t, days[0].Slots, 2)
	assert.Equal(t, "Morgenbetreuung", days[0].Slots[0].Title)
	assert.Equal(t, schedule.AttendanceStatusPresent, days[0].Slots[0].Status)
	assert.Equal(t, "Nachmittagsbetreuung", days[0].Slots[1].Title)
	assert.Equal(t, schedule.AttendanceStatusAbsent, days[0].Slots[1].Status)
	require.NotNil(t, days[0].Slots[1].Substatus)
	assert.Equal(t, schedule.AttendanceSubstatusSick, *days[0].Slots[1].Substatus)
}

func TestAttachSlotAttendance_SlotOnlyDayRespectsRoomRetention(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 7, 10)
	instance := &schedule.ActivityInstance{
		Date: schedule.Date(date), Title: "Morgenbetreuung",
		StartTime: time.Date(1, 1, 1, 7, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
	}
	instance.ID = 201
	rows := []*schedule.ScheduledInstanceRow{
		{Instance: instance, Attendance: &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent}},
	}

	t.Run("outside window stays unavailable", func(t *testing.T) {
		cutoff := date.AddDays(2).BerlinMidnight()
		days := attachSlotAttendance(nil, rows, nil, cutoff, false)
		require.Len(t, days, 1)
		assert.False(t, days[0].RoomDetailAvailable)
	})

	t.Run("failed visit query stays unavailable", func(t *testing.T) {
		days := attachSlotAttendance(nil, rows, nil, date.BerlinMidnight(), true)
		require.Len(t, days, 1)
		assert.False(t, days[0].RoomDetailAvailable)
	})

	t.Run("within window attaches visits for the date", func(t *testing.T) {
		entry := time.Date(2026, 7, 10, 9, 0, 0, 0, timezone.Berlin)
		visits := map[string][]*active.Visit{
			date.String(): {{StudentID: 1, EntryTime: entry}},
		}
		days := attachSlotAttendance(nil, rows, visits, date.BerlinMidnight(), false)
		require.Len(t, days, 1)
		assert.True(t, days[0].RoomDetailAvailable)
		require.Len(t, days[0].Visits, 1)
		assert.Equal(t, entry, days[0].Visits[0].EntryTime)
	})
}

// TestAttachUnassignedAttendance_WindowMatchAndNeutralLeftover: a re-entry into
// the same slot re-stamps the slot's checked_in_at, so the earlier session must
// be claimed via the scheduled-window fallback. Sessions outside every window
// stay visible as neutral "Ohne Zuordnung" entries without an is_unplanned claim.
func TestAttachUnassignedAttendance_WindowMatchAndNeutralLeftover(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 7, 15, 8, 0, 0, 0, timezone.Berlin)
	reentry := time.Date(2026, 7, 15, 10, 0, 0, 0, timezone.Berlin)
	outside := time.Date(2026, 7, 15, 17, 30, 0, 0, timezone.Berlin)

	days := attachUnassignedAttendance([]attendanceHistoryDay{{
		Date: "2026-07-15",
		Attendance: &attendanceDayRecord{
			CheckInTime: first,
			Sessions: []attendanceSessionRecord{
				{CheckInTime: first},   // covered by the slot's scheduled window
				{CheckInTime: reentry}, // exact match on the re-stamped checked_in_at
				{CheckInTime: outside}, // after the window — stays unassigned
			},
		},
		Slots: []attendanceSlotEntry{{
			InstanceID: "301", Title: "Ganztagsbetreuung",
			StartTime: "07:00", EndTime: "16:00",
			Status: schedule.AttendanceStatusPresent, CheckedInAt: &reentry,
		}},
	}}, true)

	require.Len(t, days[0].Slots, 2, "only the out-of-window session may become a synthetic entry")
	synthetic := days[0].Slots[1]
	assert.Equal(t, "Ohne Zuordnung", synthetic.Title)
	assert.Equal(t, "17:30", synthetic.StartTime)
	assert.Equal(t, "-3", synthetic.InstanceID, "synthetic entries carry the negative sentinel")
	assert.False(t, synthetic.IsUnplanned, "unknown booking situation must not claim 'ungeplant'")
}

// TestAttachUnassignedAttendance_AbsentSlotDoesNotAbsorbSessions: only present
// slots may claim sessions via their scheduled window — an absence (sick) with
// an overlapping window must not swallow observed presence.
func TestAttachUnassignedAttendance_AbsentSlotDoesNotAbsorbSessions(t *testing.T) {
	t.Parallel()

	sick := schedule.AttendanceSubstatusSick
	checkIn := time.Date(2026, 7, 15, 9, 0, 0, 0, timezone.Berlin)

	days := attachUnassignedAttendance([]attendanceHistoryDay{{
		Date: "2026-07-15",
		Attendance: &attendanceDayRecord{
			CheckInTime: checkIn,
			Sessions:    []attendanceSessionRecord{{CheckInTime: checkIn}},
		},
		Slots: []attendanceSlotEntry{{
			InstanceID: "302", Title: "Morgenbetreuung",
			StartTime: "07:00", EndTime: "12:00",
			Status: schedule.AttendanceStatusAbsent, Substatus: &sick,
		}},
	}}, true)

	require.Len(t, days[0].Slots, 2, "the observed session must surface next to the absence")
	assert.Equal(t, "Ohne Zuordnung", days[0].Slots[1].Title)
}

func TestAttachUnassignedAttendance_SortsMergedSlotsByDisplayedTime(t *testing.T) {
	t.Parallel()

	checkIn := time.Date(2026, 7, 15, 7, 0, 0, 0, timezone.Berlin)

	days := attachUnassignedAttendance([]attendanceHistoryDay{{
		Date: "2026-07-15",
		Attendance: &attendanceDayRecord{
			CheckInTime: checkIn,
			Sessions:    []attendanceSessionRecord{{CheckInTime: checkIn}},
		},
		Slots: []attendanceSlotEntry{
			{InstanceID: "401", Title: "Mittagsbetreuung", StartTime: "12:00", EndTime: "13:00"},
			{InstanceID: "402", Title: "Nachmittagsbetreuung", StartTime: "14:00", EndTime: "16:00"},
		},
	}}, true)

	require.Len(t, days[0].Slots, 3)
	assert.Equal(t, []string{"07:00", "12:00", "14:00"}, []string{
		days[0].Slots[0].StartTime,
		days[0].Slots[1].StartTime,
		days[0].Slots[2].StartTime,
	})
	assert.Equal(t, "Ohne Zuordnung", days[0].Slots[0].Title)
}

// TestAttachUnassignedAttendance_NoSlotExpectationStaysNeutral: at a school
// that does not keep a care plan the resolved expectation is false, so every
// session would otherwise be labelled "Ohne Zuordnung". The history must stay
// neutral — the day keeps its check-in/out times and gains no synthetic
// entries.
func TestAttachUnassignedAttendance_NoSlotExpectationStaysNeutral(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 7, 15, 8, 0, 0, 0, timezone.Berlin)
	second := time.Date(2026, 7, 14, 9, 0, 0, 0, timezone.Berlin)

	days := attachUnassignedAttendance([]attendanceHistoryDay{
		{
			Date: "2026-07-15",
			Attendance: &attendanceDayRecord{
				CheckInTime: first,
				Sessions:    []attendanceSessionRecord{{CheckInTime: first}},
			},
			Slots: []attendanceSlotEntry{},
		},
		{
			Date: "2026-07-14",
			Attendance: &attendanceDayRecord{
				CheckInTime: second,
				Sessions:    []attendanceSessionRecord{{CheckInTime: second}},
			},
			Slots: []attendanceSlotEntry{},
		},
	}, false)

	assert.Empty(t, days[0].Slots)
	assert.Empty(t, days[1].Slots)
	assert.Equal(t, first, days[0].Attendance.CheckInTime, "session times stay untouched")
}

// TestAttachUnassignedAttendance_ExpectationKeepsUnassignedRowOnSlotFreeDay:
// the expectation is resolved for the whole range (tenant-wide), not per day.
// A school with a care plan keeps its "Ohne Zuordnung" rows on days that hold
// no slot of their own.
func TestAttachUnassignedAttendance_ExpectationKeepsUnassignedRowOnSlotFreeDay(t *testing.T) {
	t.Parallel()

	planned := time.Date(2026, 7, 15, 7, 0, 0, 0, timezone.Berlin)
	walkIn := time.Date(2026, 7, 14, 9, 0, 0, 0, timezone.Berlin)

	days := attachUnassignedAttendance([]attendanceHistoryDay{
		{
			Date: "2026-07-15",
			Attendance: &attendanceDayRecord{
				CheckInTime: planned,
				Sessions:    []attendanceSessionRecord{{CheckInTime: planned}},
			},
			Slots: []attendanceSlotEntry{{
				InstanceID: "501", Title: "Morgenbetreuung",
				StartTime: "07:00", EndTime: "12:00",
				Status: schedule.AttendanceStatusPresent, CheckedInAt: &planned,
			}},
		},
		{
			Date: "2026-07-14",
			Attendance: &attendanceDayRecord{
				CheckInTime: walkIn,
				Sessions:    []attendanceSessionRecord{{CheckInTime: walkIn}},
			},
			Slots: []attendanceSlotEntry{},
		},
	}, true)

	require.Len(t, days[0].Slots, 1, "the booked slot claims its own session")
	require.Len(t, days[1].Slots, 1)
	assert.Equal(t, "Ohne Zuordnung", days[1].Slots[0].Title)
}

// TestAttachUnassignedAttendance_WalkInWithoutExpectationKeepsItsSlotOnly: a
// spontaneous walk-in at a school without a care plan renders as its persisted
// slot row (still marked is_unplanned), but must not resurrect synthetic
// "Ohne Zuordnung" entries for the remaining sessions.
func TestAttachUnassignedAttendance_WalkInWithoutExpectationKeepsItsSlotOnly(t *testing.T) {
	t.Parallel()

	walkIn := time.Date(2026, 7, 15, 9, 0, 0, 0, timezone.Berlin)
	later := time.Date(2026, 7, 15, 17, 0, 0, 0, timezone.Berlin)

	days := attachUnassignedAttendance([]attendanceHistoryDay{{
		Date: "2026-07-15",
		Attendance: &attendanceDayRecord{
			CheckInTime: walkIn,
			Sessions: []attendanceSessionRecord{
				{CheckInTime: walkIn},
				{CheckInTime: later},
			},
		},
		Slots: []attendanceSlotEntry{{
			InstanceID: "601", Title: "Spontan-AG",
			StartTime: "09:00", EndTime: "10:00",
			Status: schedule.AttendanceStatusPresent, CheckedInAt: &walkIn,
			IsUnplanned: true,
		}},
	}}, false)

	require.Len(t, days[0].Slots, 1, "no synthetic entries without a slot expectation")
	assert.Equal(t, "Spontan-AG", days[0].Slots[0].Title)
	assert.True(t, days[0].Slots[0].IsUnplanned, "the walk-in keeps its marking")
}

// TestHasPlannedSlotRow: only usable slot rows (instance + attendance) with a
// planned booking count. Walk-in rows (is_unplanned) are no plan evidence — a
// spontaneous drop-in also happens at schools that plan nothing — and the
// incomplete rows the assembly skips must not count either. Cancelled
// instances keep their assignment rows but are no evidence: a cancelled-only
// booking was never a usable slot.
func TestHasPlannedSlotRow(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 7, 15)
	assert.False(t, hasPlannedSlotRow(nil))
	assert.False(t, hasPlannedSlotRow([]*schedule.ScheduledInstanceRow{
		nil,
		{Instance: &schedule.ActivityInstance{Date: schedule.Date(date)}, Attendance: nil},
	}))
	assert.False(t, hasPlannedSlotRow([]*schedule.ScheduledInstanceRow{{
		Instance:   &schedule.ActivityInstance{Date: schedule.Date(date), Title: "Spontan-AG"},
		Attendance: &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent, IsUnplanned: true},
	}}), "walk-in rows must not count as care-plan evidence")
	assert.False(t, hasPlannedSlotRow([]*schedule.ScheduledInstanceRow{{
		Instance:   &schedule.ActivityInstance{Date: schedule.Date(date), Title: "Ausgefallene AG", Status: schedule.InstanceStatusCancelled},
		Attendance: &schedule.InstanceStudent{Status: schedule.AttendanceStatusExpected},
	}}), "cancelled instances must not count as care-plan evidence")
	assert.True(t, hasPlannedSlotRow([]*schedule.ScheduledInstanceRow{{
		Instance:   &schedule.ActivityInstance{Date: schedule.Date(date), Title: "Morgenbetreuung"},
		Attendance: &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent},
	}}))
}

func TestAttachSlotAttendance_SerializesInt64InstanceIDAsDecimalString(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 7, 15)
	instance := &schedule.ActivityInstance{
		Date:      schedule.Date(date),
		StartTime: time.Date(1, 1, 1, 7, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
	}
	instance.ID = math.MaxInt64

	days := attachSlotAttendance(nil, []*schedule.ScheduledInstanceRow{{
		Instance:   instance,
		Attendance: &schedule.InstanceStudent{Status: schedule.AttendanceStatusExpected},
	}}, nil, date.BerlinMidnight(), false)

	require.Len(t, days, 1)
	payload, err := json.Marshal(days[0].Slots[0])
	require.NoError(t, err)
	assert.JSONEq(t, `{"instance_id":"9223372036854775807","title":"","start_time":"07:00","end_time":"08:00","status":"expected","is_unplanned":false}`, string(payload))
}
