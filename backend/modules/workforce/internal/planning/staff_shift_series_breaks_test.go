package planning

// Series skip statutory holidays always, and Ferien and closing days unless
// they opt in (#3820). The reader is a fake: what these tests pin is which
// occurrences the materializer hands to BulkCreate and what it reports. The
// School Calendar binding is covered by the integration suite.

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNonWorkingDays struct {
	days     SeriesNonWorkingDays
	err      error
	periodID int64
	from, to timezone.Date
}

func (f *fakeNonWorkingDays) SeriesNonWorkingDays(_ context.Context, from, to timezone.Date, seriesPeriodID int64) (SeriesNonWorkingDays, error) {
	f.from, f.to, f.periodID = from, to, seriesPeriodID
	return f.days, f.err
}

// The series test window runs from 2026-08-25 (tomorrow) to 2026-09-23.
func newBreakDays() *fakeNonWorkingDays {
	return &fakeNonWorkingDays{days: SeriesNonWorkingDays{
		Holidays: map[timezone.Date]bool{"2026-08-27": true},
		Breaks: map[timezone.Date]bool{
			"2026-08-28": true,                                                             // closing day
			"2026-09-01": true, "2026-09-02": true, "2026-09-03": true, "2026-09-04": true, // Ferien
		},
	}}
}

type plannedDates struct{ dates []timezone.Date }

func (p *plannedDates) bulkCreate(_ context.Context, created []*StaffShift) error {
	for _, shift := range created {
		p.dates = append(p.dates, shift.Date)
	}
	return nil
}

func newBreakSeriesService(reader SeriesNonWorkingDayReader, planned *plannedDates, series *seriesMockRepo) StaffShiftSeriesService {
	service := newSeriesServiceForTest(seriesServiceMocks{
		series: series,
		shifts: &seriesShiftMockRepo{bulkCreateFn: planned.bulkCreate},
	})
	WithStaffShiftSeriesNonWorkingDays(reader)(service.(*staffShiftSeriesService))
	return service
}

func TestCreateSeries_SkipsHolidaysFerienAndClosingDays(t *testing.T) {
	t.Parallel()

	reader := newBreakDays()
	planned := &plannedDates{}
	series := unitSeries(t)
	result, err := newBreakSeriesService(reader, planned, nil).CreateSeries(context.Background(), series)
	require.NoError(t, err)

	for _, day := range []timezone.Date{"2026-08-27", "2026-08-28", "2026-09-01", "2026-09-02", "2026-09-03", "2026-09-04"} {
		assert.NotContains(t, planned.dates, day, "%s must stay free", day)
	}
	assert.Contains(t, planned.dates, timezone.Date("2026-08-26"))
	assert.Contains(t, planned.dates, timezone.Date("2026-09-05"))
	assert.Equal(t, 6, result.SkippedNonWorkingDays, "1 holiday, 1 closing day, 4 Ferien days")
	assert.Equal(t, 30-6, result.Created)
	assert.Empty(t, result.SkippedDates, "non-working days are not overlap collisions")

	assert.Equal(t, timezone.Date("2026-08-25"), reader.from, "the window starts tomorrow")
	assert.Equal(t, timezone.Date("2026-09-23"), reader.to)
	assert.Equal(t, series.CalendarPeriodID, reader.periodID, "the series' own period is passed so it is never its own break")
}

func TestCreateSeries_IncludeSchoolBreaksStillSkipsHolidays(t *testing.T) {
	t.Parallel()

	planned := &plannedDates{}
	series := unitSeries(t)
	series.IncludeSchoolBreaks = true

	result, err := newBreakSeriesService(newBreakDays(), planned, nil).CreateSeries(context.Background(), series)
	require.NoError(t, err)

	assert.NotContains(t, planned.dates, timezone.Date("2026-08-27"), "a statutory holiday stays free")
	for _, day := range []timezone.Date{"2026-08-28", "2026-09-01", "2026-09-04"} {
		assert.Contains(t, planned.dates, day)
	}
	assert.Equal(t, 1, result.SkippedNonWorkingDays)
	assert.Equal(t, 29, result.Created)
}

func TestCreateSeries_WithoutCalendarPlansEveryOccurrence(t *testing.T) {
	t.Parallel()

	result, err := newSeriesServiceForTest(seriesServiceMocks{}).CreateSeries(context.Background(), unitSeries(t))
	require.NoError(t, err)
	assert.Equal(t, 30, result.Created)
	assert.Zero(t, result.SkippedNonWorkingDays)
}

func TestCreateSeries_CalendarFailureAborts(t *testing.T) {
	t.Parallel()

	reader := newBreakDays()
	reader.err = errors.New("federal state is not available")
	planned := &plannedDates{}

	_, err := newBreakSeriesService(reader, planned, nil).CreateSeries(context.Background(), unitSeries(t))
	require.ErrorIs(t, err, reader.err)
	assert.Empty(t, planned.dates, "no occurrence may be planned without knowing the free days")
}

func TestSplitSeries_SchoolBreakOptIn(t *testing.T) {
	t.Parallel()

	include, exclude := true, false
	tests := []struct {
		name      string
		stored    bool
		edit      *bool
		want      bool
		wantSkips int
	}{
		{name: "omitted keeps the stored opt-in", stored: true, edit: nil, want: true, wantSkips: 1},
		{name: "omitted keeps the stored default", stored: false, edit: nil, want: false, wantSkips: 6},
		{name: "explicit opt-in", stored: false, edit: &include, want: true, wantSkips: 1},
		{name: "explicit opt-out", stored: true, edit: &exclude, want: false, wantSkips: 6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			old := unitSeries(t)
			old.ID = 40
			old.IncludeSchoolBreaks = tc.stored
			var created *StaffShiftSeries
			service := newBreakSeriesService(newBreakDays(), &plannedDates{}, &seriesMockRepo{
				findByIDFn: func(context.Context, any) (*StaffShiftSeries, error) { return old, nil },
				createFn: func(_ context.Context, series *StaffShiftSeries) error {
					series.ID = 41
					created = series
					return nil
				},
			})

			result, err := service.SplitSeries(context.Background(), SplitSeriesInput{
				SeriesID: old.ID, EffectiveDate: seriesTestToday.AddDays(1),
				StartTime: old.StartTime, EndTime: old.EndTime, IncludeSchoolBreaks: tc.edit, ActorStaffID: 5,
			})
			require.NoError(t, err)
			require.NotNil(t, created)
			assert.Equal(t, tc.want, created.IncludeSchoolBreaks)
			assert.Equal(t, tc.wantSkips, result.SkippedNonWorkingDays)
		})
	}
}
