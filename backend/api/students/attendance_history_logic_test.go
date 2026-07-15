package students

// Unit tests for the pure logic functions used by the attendance-history handler.
// These tests live in the internal package (no _test suffix on the package name)
// so they can exercise the unexported helpers directly. Full HTTP-level
// integration tests live in attendance_history_handlers_test.go (package
// students_test) and require a test database.

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAttendanceHistoryRange_Defaults(t *testing.T) {
	defaultStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	defaultEnd := time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)

	req := httptest.NewRequest("GET", "/students/1/attendance-history", nil)
	start, end, err := parseAttendanceHistoryRange(req, defaultStart, defaultEnd)
	require.NoError(t, err)
	assert.Equal(t, defaultStart, start)
	assert.Equal(t, defaultEnd, end)
}

func TestParseAttendanceHistoryRange_ExplicitParams(t *testing.T) {
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
	req := httptest.NewRequest("GET", "/students/1/attendance-history?start=not-a-date", nil)
	_, _, err := parseAttendanceHistoryRange(req, time.Now(), time.Now())
	require.Error(t, err)
}

func TestParseAttendanceHistoryRange_StartAfterEnd(t *testing.T) {
	req := httptest.NewRequest("GET", "/students/1/attendance-history?start=2026-03-20T00:00:00Z&end=2026-03-10T00:00:00Z", nil)
	_, _, err := parseAttendanceHistoryRange(req, time.Now(), time.Now())
	require.Error(t, err)
}

func TestBuildAttendanceHistoryDays_EmptyRows(t *testing.T) {
	days := buildAttendanceHistoryDays(nil, nil, nil, time.Now(), false)
	assert.Empty(t, days)
}

func TestBuildAttendanceHistoryDays_WithinRoomCap_IncludesVisits(t *testing.T) {
	date := timezone.Today()
	roomCutoff := date.AddDate(0, 0, -6)
	checkIn := date.Add(8 * time.Hour)
	checkOut := checkIn.Add(5*time.Hour + 30*time.Minute)

	row := &active.Attendance{
		TenantModel:  base.TenantModel{TenantID: 1},
		StudentID:    10,
		Date:         timezone.DateFromTime(date),
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		CheckedInBy:  42,
		DeviceID:     7,
	}
	exit := checkIn.Add(90 * time.Minute)
	visit := &active.Visit{
		TenantModel: base.TenantModel{TenantID: 1},
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
	// Attendance date exactly equals roomCutoff → should still include room detail.
	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)
	checkIn := roomCutoff.Add(8 * time.Hour)
	checkOut := roomCutoff.Add(15 * time.Hour)

	row := &active.Attendance{
		TenantModel:  base.TenantModel{TenantID: 1},
		StudentID:    10,
		Date:         timezone.DateFromTime(roomCutoff),
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		CheckedInBy:  42,
		DeviceID:     7,
	}
	exit := checkIn.Add(time.Hour)
	visit := &active.Visit{
		TenantModel: base.TenantModel{TenantID: 1},
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
	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)
	checkIn := today.Add(8 * time.Hour)

	row := &active.Attendance{
		TenantModel: base.TenantModel{TenantID: 1},
		StudentID:   10,
		Date:        timezone.DateFromTime(today),
		CheckInTime: checkIn,
		CheckedInBy: 42,
		DeviceID:    7,
	}
	exit := checkIn.Add(time.Hour)
	visit := &active.Visit{
		TenantModel: base.TenantModel{TenantID: 1},
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
	today := timezone.Today()
	yesterday := today.AddDate(0, 0, -1)
	roomCutoff := today.AddDate(0, 0, -6)

	rows := []*active.Attendance{
		{TenantModel: base.TenantModel{TenantID: 1}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: today.Add(8 * time.Hour), CheckedInBy: 42, DeviceID: 7},
		{TenantModel: base.TenantModel{TenantID: 1}, StudentID: 10, Date: timezone.DateFromTime(yesterday), CheckInTime: yesterday.Add(8 * time.Hour), CheckedInBy: 42, DeviceID: 7},
	}
	days := buildAttendanceHistoryDays(rows, nil, map[string][]*active.Visit{}, roomCutoff, false)
	assert.Len(t, days, 2, "should return one day entry per unique date")
}

func TestBuildAttendanceHistoryDays_StatusOnlyDay(t *testing.T) {
	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)
	reportedAt := today.Add(7 * time.Hour)
	clearedAt := today.Add(16 * time.Hour)

	statusRows := []*active.StudentStatusDay{
		{
			TenantModel: base.TenantModel{TenantID: 1},
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
	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)

	// Two attendance rows on the same day: morning session + afternoon re-check-in.
	morningIn := today.Add(8 * time.Hour)
	morningOut := today.Add(12 * time.Hour)
	afternoonIn := today.Add(13 * time.Hour)
	afternoonOut := today.Add(16 * time.Hour)

	rows := []*active.Attendance{
		{TenantModel: base.TenantModel{TenantID: 1}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: morningIn, CheckOutTime: &morningOut, CheckedInBy: 42, DeviceID: 7},
		{TenantModel: base.TenantModel{TenantID: 1}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: afternoonIn, CheckOutTime: &afternoonOut, CheckedInBy: 43, DeviceID: 8},
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
	today := timezone.Today()
	roomCutoff := today.AddDate(0, 0, -6)

	morningIn := today.Add(8 * time.Hour)
	morningOut := today.Add(12 * time.Hour)
	afternoonIn := today.Add(13 * time.Hour)
	// Second session still open (no check-out).

	rows := []*active.Attendance{
		{TenantModel: base.TenantModel{TenantID: 1}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: morningIn, CheckOutTime: &morningOut, CheckedInBy: 42, DeviceID: 7},
		{TenantModel: base.TenantModel{TenantID: 1}, StudentID: 10, Date: timezone.DateFromTime(today), CheckInTime: afternoonIn, CheckedInBy: 43, DeviceID: 8},
	}
	days := buildAttendanceHistoryDays(rows, nil, map[string][]*active.Visit{}, roomCutoff, false)

	require.Len(t, days, 1)
	assert.Nil(t, days[0].Attendance.CheckOutTime, "nil check-out (still present) takes precedence")
	assert.Nil(t, days[0].Attendance.DurationMinutes, "no duration when session is still open")
}

func TestBuildAttendanceHistoryDays_OutsideRoomCap_HidesVisits(t *testing.T) {
	// Attendance from 10 days ago, room cutoff at 7 days ago → no room detail.
	today := timezone.Today()
	oldDate := today.AddDate(0, 0, -10)
	roomCutoff := today.AddDate(0, 0, -6)
	checkIn := oldDate.Add(8 * time.Hour)

	row := &active.Attendance{
		TenantModel: base.TenantModel{TenantID: 1},
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
	date := timezone.NewDate(2026, 7, 15)
	morning := &schedule.ActivityInstance{
		Date: date, Title: "Morgenbetreuung",
		StartTime: time.Date(1, 1, 1, 7, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
	}
	morning.ID = 101
	afternoon := &schedule.ActivityInstance{
		Date: date, Title: "Nachmittagsbetreuung",
		StartTime: time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
	}
	afternoon.ID = 102
	sick := schedule.AttendanceSubstatusSick

	days := attachSlotAttendance(nil, []*schedule.ScheduledInstanceRow{
		{Instance: morning, Attendance: &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent}},
		{Instance: afternoon, Attendance: &schedule.InstanceStudent{Status: schedule.AttendanceStatusAbsent, Substatus: &sick}},
	})

	require.Len(t, days, 1)
	require.Len(t, days[0].Slots, 2)
	assert.Equal(t, "Morgenbetreuung", days[0].Slots[0].Title)
	assert.Equal(t, schedule.AttendanceStatusPresent, days[0].Slots[0].Status)
	assert.Equal(t, "Nachmittagsbetreuung", days[0].Slots[1].Title)
	assert.Equal(t, schedule.AttendanceStatusAbsent, days[0].Slots[1].Status)
	require.NotNil(t, days[0].Slots[1].Substatus)
	assert.Equal(t, schedule.AttendanceSubstatusSick, *days[0].Slots[1].Substatus)
}
