package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type closureCalendarReads struct {
	schools map[int64]schoolcalendar.HolidayClosureCalendar
	calls   int
}

func (r *closureCalendarReads) HolidayClosureCalendars(context.Context, int64, []int64, calendar.Date, calendar.Date) (map[int64]schoolcalendar.HolidayClosureCalendar, error) {
	r.calls++
	return r.schools, nil
}

type closureCalendarHolidays struct {
	calendarDaysEngine
	calls int
}

func (h *closureCalendarHolidays) ListHolidays(ctx context.Context, region, from, to string) ([]schoolcalendar.Holiday, error) {
	h.calls++
	return h.calendarDaysEngine.ListHolidays(ctx, region, from, to)
}

func TestHolidayClosureCalendarOwnsRevisionAndRegionGuards(t *testing.T) {
	t.Parallel()
	school := testpkg.UniqueTestTenantID(t)
	organization := testpkg.UniqueTestTenantID(t)
	from, to := calendar.NewDate(2030, 7, 8), calendar.NewDate(2030, 7, 9)
	for _, tc := range []struct {
		name, region, fallback string
		present                bool
		revision               int64
		want                   error
	}{
		{name: "foreign school", fallback: "DE-NW", revision: 4, want: schoolcalendar.ErrHolidayClosureNoticeNotFound},
		{name: "changed revision", present: true, region: "DE-NW", revision: 3, want: schoolcalendar.ErrHolidayClosureCalendarChanged},
		{name: "unknown state", present: true, region: "unknown", fallback: "DE-NW", revision: 4, want: schoolcalendar.ErrFederalStateUnavailable},
		{name: "missing default", present: true, revision: 4, want: schoolcalendar.ErrFederalStateUnavailable},
		{name: "registry default", present: true, fallback: "DE-NW", revision: 4},
		{name: "stored state", present: true, region: "DE-SN", revision: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := &closureCalendarReads{schools: map[int64]schoolcalendar.HolidayClosureCalendar{}}
			if tc.present {
				reads.schools[school] = schoolcalendar.HolidayClosureCalendar{SchoolID: school, FederalState: tc.region, Revision: tc.revision}
			}
			holidays := &closureCalendarHolidays{}
			query := NewHolidayClosureCalendar(reads, holidays, tc.fallback)
			_, err := query.HolidayClosureNoCareDays(t.Context(), organization, []int64{school}, from, to, map[int64]int64{school: 4})
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, holidays.calls)
			}
			assert.Equal(t, 1, reads.calls)
		})
	}
}

func TestHolidayClosureCalendarMergesSourcesWithinTheRequestedDates(t *testing.T) {
	t.Parallel()
	school, other, organization := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
	source, traegerSource := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
	from, to := calendar.NewDate(2030, 7, 8), calendar.NewDate(2030, 7, 9)
	reads := &closureCalendarReads{schools: map[int64]schoolcalendar.HolidayClosureCalendar{
		school: {SchoolID: school, Revision: 4, ClosingDays: []schoolcalendar.HolidayClosureCalendarDay{
			{SourceID: source, StartsOn: from.AddDays(-1), EndsOn: to.AddDays(1), Reason: "School closure"},
			{SourceID: traegerSource, Traeger: true, StartsOn: from, EndsOn: from, Reason: "Office closure"},
		}},
		other: {SchoolID: other, FederalState: "DE-NW"},
	}}
	holidays := &closureCalendarHolidays{calendarDaysEngine: calendarDaysEngine{holidays: []schoolcalendar.Holiday{{Date: from.String(), Name: "Holiday"}}}}
	result, err := NewHolidayClosureCalendar(reads, holidays, "DE-NW").HolidayClosureNoCareDays(t.Context(), organization, []int64{school, other}, from, to, map[int64]int64{school: 4})
	require.NoError(t, err)
	require.Len(t, result[school], 2)
	assert.Equal(t, from, result[school][0].Date)
	assert.Equal(t, schoolcalendar.HolidayClosureNoCareHoliday, result[school][0].Kind, "statutory holidays take precedence")
	assert.Equal(t, []schoolcalendar.HolidayClosureSource{{ID: source}, {ID: traegerSource, Traeger: true}}, result[school][0].ClosingDays)
	assert.Equal(t, to, result[school][1].Date)
	assert.Equal(t, schoolcalendar.HolidayClosureNoCareClosingDay, result[school][1].Kind)
	assert.Equal(t, []schoolcalendar.HolidayClosureSource{{ID: source}}, result[school][1].ClosingDays)
	require.Len(t, result[other], 1)
	assert.Empty(t, result[other][0].ClosingDays, "a school's sources cannot leak into another school's result")
	assert.Equal(t, 1, reads.calls, "school calendars are read together")
	assert.Equal(t, 1, holidays.calls, "schools in the same region share holiday computation")
}
