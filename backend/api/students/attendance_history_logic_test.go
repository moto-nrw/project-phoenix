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
	days := buildAttendanceHistoryDays(nil, nil, time.Now())
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
		Date:         date,
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
		map[string][]*active.Visit{key: {visit}},
		roomCutoff,
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

func TestBuildAttendanceHistoryDays_OutsideRoomCap_HidesVisits(t *testing.T) {
	// Attendance from 10 days ago, room cutoff at 7 days ago → no room detail.
	today := timezone.Today()
	oldDate := today.AddDate(0, 0, -10)
	roomCutoff := today.AddDate(0, 0, -6)
	checkIn := oldDate.Add(8 * time.Hour)

	row := &active.Attendance{
		TenantModel: base.TenantModel{TenantID: 1},
		StudentID:   10,
		Date:        oldDate,
		CheckInTime: checkIn,
		CheckedInBy: 42,
		DeviceID:    7,
	}
	days := buildAttendanceHistoryDays(
		[]*active.Attendance{row},
		map[string][]*active.Visit{},
		roomCutoff,
	)
	require.Len(t, days, 1)
	assert.False(t, days[0].RoomDetailAvailable, "older-than-room-cap day must hide visits")
	assert.Empty(t, days[0].Visits)
	assert.Nil(t, days[0].Attendance.DurationMinutes, "duration is nil when check_out_time is nil")
}
