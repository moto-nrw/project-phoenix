package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

// ---- holiday reader mock ----------------------------------------------------

type wtmMockHolidayReader struct {
	dates map[timezone.Date]bool
	err   error
}

func (m *wtmMockHolidayReader) HolidayDates(_ context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make(map[timezone.Date]bool)
	for d := range m.dates {
		if !d.Before(from) && !d.After(to) {
			result[d] = true
		}
	}
	return result, nil
}

// ---- tests (#1418 3a: public holidays zero the Soll) ------------------------

// A holiday on a scheduled workday zeroes that day's Soll in the daily table.
func TestWTMDailyTargets_HolidayZeroesSoll(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	// Fixture schedule: Mondays 480 min. 2026-06-08 is a Monday — declare it
	// a holiday.
	holiday := timezone.NewDate(2026, time.June, 8)
	f.svc.SetHolidayReader(&wtmMockHolidayReader{dates: map[timezone.Date]bool{holiday: true}})

	targets, err := f.svc.GetDailyTargets(context.Background(), wtmStaffID,
		timezone.NewDate(2026, time.June, 1), timezone.NewDate(2026, time.June, 15))
	require.NoError(t, err)

	byDate := make(map[timezone.Date]int, len(targets))
	for _, target := range targets {
		byDate[target.Date] = target.TargetMinutes
	}
	assert.Equal(t, 480, byDate[timezone.NewDate(2026, time.June, 1)], "regular Monday keeps its Soll")
	assert.Equal(t, 0, byDate[holiday], "holiday Monday must have Soll 0")
	assert.Equal(t, 480, byDate[timezone.NewDate(2026, time.June, 15)], "Monday after the holiday keeps its Soll")
}

// The Monatskarte target shrinks by the holiday's contractual minutes, and
// the month balance no longer shows minus hours for not working the holiday.
func TestWTMMonthSummary_HolidayReducesTarget(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	// June 2026 Mondays: 1, 8, 15, 22, 29 → 5 × 480 without holidays.
	holiday := timezone.NewDate(2026, time.June, 8)
	f.svc.SetHolidayReader(&wtmMockHolidayReader{dates: map[timezone.Date]bool{holiday: true}})

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)

	assert.Equal(t, 4*480, summary.TargetMinutes, "holiday Monday must not count toward the month Soll")
	assert.Equal(t, 4*480, summary.TargetMinutesToDate, "June is fully in the past (today = 2026-07-15)")
}

// A nil reader (not wired) keeps the pre-holiday behavior — every Monday
// counts.
func TestWTMMonthSummary_NoHolidayReaderKeepsSoll(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)

	assert.Equal(t, 5*480, summary.TargetMinutes)
}

// A vacation day falling on a holiday credits nothing: the holiday already
// zeroed the day's Soll, so crediting it again would double-pay the day.
func TestWTMMonthSummary_AbsenceOnHolidayCreditsNothing(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	holiday := timezone.NewDate(2026, time.June, 8)
	f.svc.SetHolidayReader(&wtmMockHolidayReader{dates: map[timezone.Date]bool{holiday: true}})
	f.absences.absences = []*activeModels.StaffAbsence{{
		StaffID:     wtmStaffID,
		AbsenceType: activeModels.AbsenceTypeVacation,
		DateStart:   holiday,
		DateEnd:     holiday,
		Status:      activeModels.AbsenceStatusApproved,
	}}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)

	assert.Equal(t, 0, summary.CreditedVacationMinutes, "vacation on a holiday must not credit minutes")
	assert.Equal(t, 4*480, summary.TargetMinutes)
}

// A failing holiday resolver fails the read: silently ignoring holidays
// would show a wrong Soll as authoritative.
func TestWTMMonthSummary_HolidayReaderErrorPropagates(t *testing.T) {
	t.Parallel()

	f := newWTMFixture()
	f.svc.SetHolidayReader(&wtmMockHolidayReader{err: errors.New("settings unavailable")})

	_, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "public holidays")
}

// ---- weekly helper arithmetic (work_session_service) ------------------------

func TestHolidayScheduleMinutes(t *testing.T) {
	t.Parallel()

	entries := []*configModels.StaffWorkSchedule{
		{StaffID: wtmStaffID, DayOfWeek: configModels.DayMonday, TargetMinutes: 480, RotationLength: 1, ValidFrom: timezone.NewDate(2020, time.January, 1)},
		{StaffID: wtmStaffID, DayOfWeek: configModels.DayThursday, TargetMinutes: 240, RotationLength: 1, ValidFrom: timezone.NewDate(2020, time.January, 1)},
	}
	weekStart := timezone.NewDate(2026, time.June, 8) // Monday

	// Holiday on the Monday only: the week loses 480 of its 720 minutes.
	holidaySet := map[timezone.Date]bool{weekStart: true}
	assert.Equal(t, 480, holidayScheduleMinutes(entries, nil, weekStart, holidaySet))

	// Holiday on a day without Soll (Saturday): nothing changes.
	holidaySet = map[timezone.Date]bool{weekStart.AddDays(5): true}
	assert.Equal(t, 0, holidayScheduleMinutes(entries, nil, weekStart, holidaySet))

	// No holidays: zero reduction.
	assert.Equal(t, 0, holidayScheduleMinutes(entries, nil, weekStart, nil))
}

func TestHolidayModelMinutes(t *testing.T) {
	t.Parallel()

	anchor := timezone.NewDate(2020, time.January, 6)
	model := &configModels.WorkTimeModel{
		RotationLength: 1,
		Entries: []*configModels.WorkTimeModelEntry{
			{WeekIndex: 0, DayOfWeek: configModels.DayMonday, TargetMinutes: 480},
		},
	}
	weekStart := timezone.NewDate(2026, time.June, 8)

	assert.Equal(t, 480, holidayModelMinutes(model, anchor, weekStart, map[timezone.Date]bool{weekStart: true}))
	assert.Equal(t, 0, holidayModelMinutes(model, anchor, weekStart, nil))
}
