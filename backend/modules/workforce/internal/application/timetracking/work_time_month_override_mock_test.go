package timetracking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wtmMockOverrideReader serves Sonderarbeitszeit days (#3259) the way the
// Workforce capability does: already without weekends and statutory holidays.
type wtmMockOverrideReader struct {
	days workforce.TargetOverrideDays
	err  error
}

func (m *wtmMockOverrideReader) StaffTargetOverrideDays(_ context.Context, staffIDs []int64, _, _ string) (workforce.TargetOverrideDays, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make(workforce.TargetOverrideDays, len(staffIDs))
	for _, staffID := range staffIDs {
		result[staffID] = m.days[staffID]
	}
	return result, nil
}

// overrideDays lists every day in [from, to] that is Monday to Friday.
func overrideDays(from, to Date, minutes int) map[string]int {
	days := make(map[string]int)
	for d := from; !d.After(to); d = d.AddDays(1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			days[d.String()] = minutes
		}
	}
	return days
}

// newWissingenFixture is the OGS Wissingen autumn (#3259): a Mon–Fri 8h
// schedule, the autumn holidays entered as one closure 12.–25.10.2026, and a
// holiday-care Sonderarbeitszeit of 8.5h per day in KW 43 (19.–23.10.).
func newWissingenFixture(t *testing.T, withOverride bool) *wtmFixture {
	t.Helper()
	f := newWTMFixture()
	f.schedules.entries = nil
	for day := DayMonday; day <= DayFriday; day++ {
		f.schedules.entries = append(f.schedules.entries, &WorkScheduleRow{
			StaffID: wtmStaffID, DayOfWeek: day, TargetMinutes: 480, RotationLength: 1, ValidFrom: NewDate(2020, time.January, 1),
		})
	}
	closure := map[Date]bool{}
	for d := NewDate(2026, time.October, 12); !d.After(NewDate(2026, time.October, 25)); d = d.AddDays(1) {
		closure[d] = true
	}
	f.svc.holidayReader = &wtmMockHolidayReader{dates: closure}
	reader := &wtmMockOverrideReader{days: workforce.TargetOverrideDays{}}
	if withOverride {
		reader.days[wtmStaffID] = overrideDays(NewDate(2026, time.October, 19), NewDate(2026, time.October, 23), 510)
	}
	f.svc.overrideReader = reader
	f.svc.todayFunc = func() Date { return NewDate(2026, time.November, 15) }
	f.svc.nowFunc = func() time.Time { return time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC) }
	f.settings.accountStart = "2026-10-01"
	return f
}

// octoberSessions records 8h on every regular October workday outside the
// closure and, when holidayCare is set, 8.5h on each day of KW 43.
func octoberSessions(holidayCare bool) []*WorkSession {
	var sessions []*WorkSession
	for d := NewDate(2026, time.October, 1); !d.After(NewDate(2026, time.October, 31)); d = d.AddDays(1) {
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		inClosure := !d.Before(NewDate(2026, time.October, 12)) && !d.After(NewDate(2026, time.October, 25))
		inCare := !d.Before(NewDate(2026, time.October, 19)) && !d.After(NewDate(2026, time.October, 23))
		switch {
		case inCare && holidayCare:
			sessions = append(sessions, wtmSession(d, 8, 510, 30))
		case !inClosure:
			sessions = append(sessions, wtmSession(d, 8, 480, 30))
		}
	}
	return sessions
}

func projectionByDate(t *testing.T, f *wtmFixture, from, to Date) map[Date]DailyProjection {
	t.Helper()
	days, err := f.svc.GetDailyProjection(context.Background(), wtmStaffID, from, to)
	require.NoError(t, err)
	result := make(map[Date]DailyProjection, len(days))
	for _, day := range days {
		result[day.Date] = day
	}
	return result
}

// The override beats the closure day: KW 43 carries 8.5h Soll per day and is
// marked as a Sonderarbeitszeit, KW 42 stays at zero.
func TestWTMTargetOverride_BeatsClosureDay(t *testing.T) {
	t.Parallel()

	f := newWissingenFixture(t, true)
	days := projectionByDate(t, f, NewDate(2026, time.October, 12), NewDate(2026, time.October, 25))

	assert.Equal(t, 0, days[NewDate(2026, time.October, 14)].TargetMinutes, "closure day without override stays 0")
	assert.Empty(t, days[NewDate(2026, time.October, 14)].TargetSource)
	assert.Equal(t, 510, days[NewDate(2026, time.October, 21)].TargetMinutes, "override wins over the closure day")
	assert.Equal(t, workforce.TargetSourceOverride, days[NewDate(2026, time.October, 21)].TargetSource)
	assert.Equal(t, 0, days[NewDate(2026, time.October, 24)].TargetMinutes, "Saturday inside the range carries no Soll")
}

// After the range the normal work-time model applies again by itself.
func TestWTMTargetOverride_NormalScheduleResumesAfterRange(t *testing.T) {
	t.Parallel()

	f := newWissingenFixture(t, true)
	days := projectionByDate(t, f, NewDate(2026, time.October, 23), NewDate(2026, time.October, 27))

	assert.Equal(t, 510, days[NewDate(2026, time.October, 23)].TargetMinutes)
	assert.Equal(t, 480, days[NewDate(2026, time.October, 26)].TargetMinutes, "Monday after the closure is back on the schedule")
	assert.Empty(t, days[NewDate(2026, time.October, 26)].TargetSource)
}

// Wissingen: with the Sonderarbeitszeit, five holiday-care days of 8.5h are
// exactly the Soll, so the month ends at 0 instead of +42.5h.
func TestWTMTargetOverride_WissingenHolidayCareBalancesToZero(t *testing.T) {
	t.Parallel()

	withOverride := newWissingenFixture(t, true)
	withOverride.sessions.sessions = octoberSessions(true)
	summary, err := withOverride.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.BalanceMinutes, "holiday care matching the override must not create plus hours")

	withoutOverride := newWissingenFixture(t, false)
	withoutOverride.sessions.sessions = octoberSessions(true)
	summary, err = withoutOverride.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 10)
	require.NoError(t, err)
	assert.Equal(t, 2550, summary.BalanceMinutes, "the pre-#3259 closure-only picture: +42.5h")
}

// A colleague with contractual holiday hours who does not clock loses the
// override Soll: 5 × 8.5h = 2550 minutes.
func TestWTMTargetOverride_NoClockingDeductsTheOverrideSoll(t *testing.T) {
	t.Parallel()

	f := newWissingenFixture(t, true)
	f.sessions.sessions = octoberSessions(false)
	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 10)
	require.NoError(t, err)
	assert.Equal(t, -2550, summary.BalanceMinutes)
}

// A staff member without an override keeps the closure-day Soll of zero,
// even when colleagues have one.
func TestWTMTargetOverride_OtherStaffUnchanged(t *testing.T) {
	t.Parallel()

	f := newWissingenFixture(t, false)
	f.svc.overrideReader = &wtmMockOverrideReader{days: workforce.TargetOverrideDays{
		wtmStaffID + 1: overrideDays(NewDate(2026, time.October, 19), NewDate(2026, time.October, 23), 495),
	}}
	days := projectionByDate(t, f, NewDate(2026, time.October, 19), NewDate(2026, time.October, 23))
	for _, day := range days {
		assert.Zero(t, day.TargetMinutes, "closure day %s", day.Date)
		assert.Empty(t, day.TargetSource)
	}
}

// A 0-minute override on a normal workday sets the Soll to 0 and is marked.
func TestWTMTargetOverride_ZeroMinutesOnNormalWorkday(t *testing.T) {
	t.Parallel()

	f := newWissingenFixture(t, false)
	f.svc.overrideReader = &wtmMockOverrideReader{days: workforce.TargetOverrideDays{
		wtmStaffID: overrideDays(NewDate(2026, time.October, 5), NewDate(2026, time.October, 5), 0),
	}}
	days := projectionByDate(t, f, NewDate(2026, time.October, 5), NewDate(2026, time.October, 6))

	assert.Equal(t, 0, days[NewDate(2026, time.October, 5)].TargetMinutes)
	assert.Equal(t, workforce.TargetSourceOverride, days[NewDate(2026, time.October, 5)].TargetSource)
	assert.Equal(t, 480, days[NewDate(2026, time.October, 6)].TargetMinutes)
}

// A statutory holiday inside the range is never an override day (the
// capability leaves it out), so the holiday keeps Soll 0.
func TestWTMTargetOverride_PublicHolidayInsideRangeStaysZero(t *testing.T) {
	t.Parallel()

	f := newWissingenFixture(t, false)
	// 2026-12-25 is a Friday and a statutory holiday; the capability left it
	// out of the override days of 21.–31.12.
	days := overrideDays(NewDate(2026, time.December, 21), NewDate(2026, time.December, 31), 300)
	delete(days, "2026-12-25")
	f.svc.overrideReader = &wtmMockOverrideReader{days: workforce.TargetOverrideDays{wtmStaffID: days}}
	f.svc.holidayReader = &wtmMockHolidayReader{dates: map[Date]bool{NewDate(2026, time.December, 25): true}}
	f.svc.todayFunc = func() Date { return NewDate(2027, time.January, 15) }

	projection := projectionByDate(t, f, NewDate(2026, time.December, 21), NewDate(2026, time.December, 31))
	assert.Equal(t, 300, projection[NewDate(2026, time.December, 24)].TargetMinutes)
	assert.Equal(t, 0, projection[NewDate(2026, time.December, 25)].TargetMinutes)
	assert.Empty(t, projection[NewDate(2026, time.December, 25)].TargetSource)
}

// A failing override lookup fails the read instead of showing a Soll that
// ignores the Sonderarbeitszeit.
func TestWTMTargetOverride_ReaderErrorPropagates(t *testing.T) {
	t.Parallel()

	f := newWissingenFixture(t, true)
	f.svc.overrideReader = &wtmMockOverrideReader{err: errors.New("database unavailable")}
	_, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target overrides")
}

// A sick day inside the override credits the override's minutes, so an
// illness during holiday care is neutral.
func TestWTMTargetOverride_AbsenceCreditsOverrideMinutes(t *testing.T) {
	t.Parallel()

	f := newWissingenFixture(t, true)
	sickDay := NewDate(2026, time.October, 20)
	f.absences.absences = []*StaffAbsence{{
		StaffID: wtmStaffID, AbsenceType: AbsenceTypeSick, DateStart: sickDay, DateEnd: sickDay, Status: AbsenceStatusReported,
	}}
	days := projectionByDate(t, f, sickDay, sickDay)
	assert.Equal(t, 510, days[sickDay].CreditMinutes)
	assert.Zero(t, days[sickDay].BalanceMinutes)
}
