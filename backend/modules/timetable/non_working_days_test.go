package timetable_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type calendarStub struct {
	holidays   map[string]bool
	closing    map[string]bool
	err        error
	gotFrom    string
	gotTo      string
	closingErr error
}

func (c *calendarStub) TenantHolidayDates(_ context.Context, from, to string) (map[string]bool, error) {
	c.gotFrom, c.gotTo = from, to
	return c.holidays, c.err
}

func (c *calendarStub) ClosingDayDates(context.Context, string, string) (map[string]bool, error) {
	return c.closing, c.closingErr
}

func TestNonWorkingDays_Skip(t *testing.T) {
	t.Parallel()

	calendar := &calendarStub{
		holidays: map[string]bool{"2026-10-03": true, "2026-12-25": true},
		closing:  map[string]bool{"2026-10-12": true, "2026-12-25": true},
	}
	days, err := timetable.LoadNonWorkingDays(context.Background(), calendar, "2026-10-01", "2026-12-31")
	require.NoError(t, err)
	assert.Equal(t, "2026-10-01", calendar.gotFrom)
	assert.Equal(t, "2026-12-31", calendar.gotTo)

	cases := []struct {
		name    string
		date    string
		include bool
		want    timetable.OccurrenceSkip
	}{
		{"regular day", "2026-10-05", false, timetable.SkipNone},
		{"holiday", "2026-10-03", false, timetable.SkipHoliday},
		{"holiday for holiday care", "2026-10-03", true, timetable.SkipHoliday},
		{"closing day", "2026-10-12", false, timetable.SkipClosingDay},
		{"closing day for holiday care", "2026-10-12", true, timetable.SkipNone},
		{"holiday inside a closure", "2026-12-25", true, timetable.SkipHoliday},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, days.Skip(tc.date, tc.include), tc.name)
	}
}

func TestLoadNonWorkingDays_WithoutCalendarSkipsNothing(t *testing.T) {
	t.Parallel()

	days, err := timetable.LoadNonWorkingDays(context.Background(), nil, "2026-10-01", "2026-10-31")
	require.NoError(t, err)
	assert.Equal(t, timetable.SkipNone, days.Skip("2026-10-03", false))
}

func TestLoadNonWorkingDays_PropagatesCalendarErrors(t *testing.T) {
	t.Parallel()

	failure := errors.New("calendar down")
	_, err := timetable.LoadNonWorkingDays(context.Background(), &calendarStub{err: failure}, "2026-10-01", "2026-10-31")
	require.ErrorIs(t, err, failure)
	_, err = timetable.LoadNonWorkingDays(context.Background(), &calendarStub{closingErr: failure}, "2026-10-01", "2026-10-31")
	require.ErrorIs(t, err, failure)
}

func TestValidateBulkCancelRange(t *testing.T) {
	t.Parallel()

	require.NoError(t, timetable.ValidateBulkCancelRange("2026-10-12", "2026-10-12"))
	require.NoError(t, timetable.ValidateBulkCancelRange("2026-08-01", "2027-07-31"), "a school year fits")
	for name, window := range map[string][2]string{
		"reversed": {"2026-10-16", "2026-10-12"},
		"too long": {"2026-08-01", "2027-08-02"}, // 367 days

		"bad from":  {"12.10.2026", "2026-10-16"},
		"bad to":    {"2026-10-12", ""},
		"not a day": {"2026-02-30", "2026-03-01"},
	} {
		assert.ErrorIs(t, timetable.ValidateBulkCancelRange(window[0], window[1]), timetable.ErrInvalidBulkCancelRange, name)
	}
}

func TestSelectBulkCancel(t *testing.T) {
	t.Parallel()

	candidates := []timetable.BulkCancelCandidate{
		{InstanceID: 30, Date: "2026-10-13", Status: timetable.InstanceStatusPlanned},
		{InstanceID: 10, Date: "2026-10-12", Status: timetable.InstanceStatusPlanned},
		{InstanceID: 11, Date: "2026-10-12", Status: timetable.InstanceStatusPlanned},
		{InstanceID: 12, Date: "2026-10-12", Status: timetable.InstanceStatusCancelled},
		{InstanceID: 13, Date: "2026-10-12", Status: "active"},
		{InstanceID: 14, Date: "2026-10-12", Status: timetable.InstanceStatusPlanned, SeriesIncludesClosingDays: true},
		{InstanceID: 5, Date: "2026-10-09", Status: timetable.InstanceStatusPlanned},
		{InstanceID: 40, Date: "2026-10-19", Status: timetable.InstanceStatusPlanned},
	}

	ids, result := timetable.SelectBulkCancel(candidates, "2026-10-01", "2026-10-16", "2026-10-12")

	assert.Equal(t, []int64{10, 11, 30}, ids, "planned, from today, inside the range, date order")
	assert.Equal(t, 3, result.Count)
	assert.Equal(t, "2026-10-01", result.From)
	assert.Equal(t, "2026-10-16", result.To)
	assert.Equal(t, []timetable.BulkCancelDay{{Date: "2026-10-12", Count: 2}, {Date: "2026-10-13", Count: 1}}, result.Days)
}

func TestSelectBulkCancel_EmptyRangeHasNoDays(t *testing.T) {
	t.Parallel()

	ids, result := timetable.SelectBulkCancel(nil, "2026-10-12", "2026-10-16", "2026-10-01")
	assert.Empty(t, ids)
	assert.Zero(t, result.Count)
	assert.NotNil(t, result.Days, "the dialog reads an empty list, never null")
}
