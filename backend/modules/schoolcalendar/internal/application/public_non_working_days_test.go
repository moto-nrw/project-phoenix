package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// calendarDaysEngine stubs the holiday and closing-day reads the tenant
// non-working-day queries compose; every other engine method is unreachable.
type calendarDaysEngine struct {
	recordingEngine
	region      string
	regionErr   error
	holidays    []schoolcalendar.Holiday
	dates       map[string]bool
	holidayErr  error
	closing     []schoolcalendar.ClosingDay
	closingErr  error
	seenRegion  string
	seenClosing schoolcalendar.ClosingDayFilter
}

func (e *calendarDaysEngine) FederalState(context.Context) (string, error) {
	return e.region, e.regionErr
}

func (e *calendarDaysEngine) ValidHolidayRegion(region string) bool {
	return region == "DE-NW" || region == "DE-SN"
}

func (e *calendarDaysEngine) ListHolidays(_ context.Context, region, _, _ string) ([]schoolcalendar.Holiday, error) {
	e.seenRegion = region
	return e.holidays, e.holidayErr
}

func (e *calendarDaysEngine) HolidayDates(_ context.Context, region, _, _ string) (map[string]bool, error) {
	e.seenRegion = region
	if e.holidayErr != nil {
		return nil, e.holidayErr
	}
	// Copy so the union in the module cannot leak into the stub.
	out := make(map[string]bool, len(e.dates))
	for day := range e.dates {
		out[day] = true
	}
	return out, nil
}

func (e *calendarDaysEngine) ListClosingDays(_ context.Context, filter schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error) {
	e.seenClosing = filter
	return e.closing, e.closingErr
}

func closingRange(start, end, reason string) schoolcalendar.ClosingDay {
	return schoolcalendar.ClosingDay{StartDate: start, EndDate: end, Reason: reason}
}

func TestTenantHolidaysResolveTheRegionFromTheTenant(t *testing.T) {
	t.Parallel()

	engine := &calendarDaysEngine{region: "DE-SN", holidays: []schoolcalendar.Holiday{{Date: "2026-11-18", Name: "Buß- und Bettag"}}}
	module := newTestModule(engine)

	list, err := module.TenantHolidays(context.Background(), "2026-11-01", "2026-11-30")
	require.NoError(t, err)
	assert.Equal(t, "DE-SN", engine.seenRegion)
	require.Len(t, list, 1)
	assert.Equal(t, "Buß- und Bettag", list[0].Name)
}

// The school checks its holidays against the state they are computed for
// (#4775); an unreadable or unsupported state is the same error the
// holiday reads report.
func TestTenantHolidayRegionNamesTheFederalState(t *testing.T) {
	t.Parallel()

	region, err := newTestModule(&calendarDaysEngine{region: "DE-SN"}).TenantHolidayRegion(context.Background())
	require.NoError(t, err)
	assert.Equal(t, schoolcalendar.HolidayRegion{Code: "DE-SN", Name: "Sachsen"}, region)

	_, err = newTestModule(&calendarDaysEngine{regionErr: errors.New("boom")}).TenantHolidayRegion(context.Background())
	require.ErrorIs(t, err, schoolcalendar.ErrFederalStateUnavailable)
	_, err = newTestModule(&calendarDaysEngine{region: "XX"}).TenantHolidayRegion(context.Background())
	require.ErrorIs(t, err, schoolcalendar.ErrFederalStateUnavailable)
}

func TestTenantHolidayDates(t *testing.T) {
	t.Parallel()

	module := newTestModule(&calendarDaysEngine{region: "DE-NW", dates: map[string]bool{"2026-05-01": true, "2026-05-25": true}})

	set, err := module.TenantHolidayDates(context.Background(), "2026-05-01", "2026-05-31")
	require.NoError(t, err)
	assert.True(t, set["2026-05-01"], "Tag der Arbeit")
	assert.True(t, set["2026-05-25"], "Pfingstmontag")
	assert.False(t, set["2026-05-02"])
}

func TestTenantHolidayErrors(t *testing.T) {
	t.Parallel()

	_, err := newTestModule(&calendarDaysEngine{regionErr: errors.New("boom")}).
		TenantHolidayDates(context.Background(), "2026-01-01", "2026-01-02")
	require.ErrorIs(t, err, schoolcalendar.ErrFederalStateUnavailable)

	_, err = newTestModule(&calendarDaysEngine{region: "XX"}).
		TenantHolidayDates(context.Background(), "2026-01-01", "2026-01-02")
	require.ErrorIs(t, err, schoolcalendar.ErrFederalStateUnavailable)
	assert.Contains(t, err.Error(), "unsupported region")

	_, err = newTestModule(&calendarDaysEngine{regionErr: errors.New("boom")}).
		TenantHolidays(context.Background(), "2026-01-01", "2026-01-02")
	require.ErrorIs(t, err, schoolcalendar.ErrFederalStateUnavailable)

	_, err = newTestModule(&calendarDaysEngine{region: "DE-NW"}).
		TenantHolidays(context.Background(), "2026-01-02", "2026-01-01")
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidHolidayRange, "the window is validated before the region is resolved")
}

func TestClosingDayDatesExpandsRanges(t *testing.T) {
	t.Parallel()

	engine := &calendarDaysEngine{closing: []schoolcalendar.ClosingDay{
		closingRange("2026-12-24", "2026-12-27", "Weihnachten"),
		closingRange("2026-12-31", "2026-12-31", "Silvester"),
	}}

	set, err := newTestModule(engine).ClosingDayDates(context.Background(), "2026-12-01", "2026-12-31")
	require.NoError(t, err)
	assert.Equal(t, schoolcalendar.ClosingDayFilter{OverlappingFrom: "2026-12-01", OverlappingTo: "2026-12-31"}, engine.seenClosing)
	assert.Len(t, set, 5)
	assert.True(t, set["2026-12-24"])
	assert.True(t, set["2026-12-27"])
	assert.True(t, set["2026-12-31"])
	assert.False(t, set["2026-12-28"])
}

func TestClosingDayDatesClampsToWindow(t *testing.T) {
	t.Parallel()

	// Range extends past both window edges; only in-window days may appear.
	module := newTestModule(&calendarDaysEngine{closing: []schoolcalendar.ClosingDay{closingRange("2026-07-20", "2026-08-07", "Sommerschließung")}})

	set, err := module.ClosingDayDates(context.Background(), "2026-08-01", "2026-08-03")
	require.NoError(t, err)
	assert.Len(t, set, 3)
	assert.True(t, set["2026-08-01"])
	assert.True(t, set["2026-08-03"])
	assert.False(t, set["2026-07-31"])
	assert.False(t, set["2026-08-04"])
}

func TestClosingDayDatesEmptyOnInvertedWindow(t *testing.T) {
	t.Parallel()

	engine := &calendarDaysEngine{closingErr: errors.New("must not be reached")}
	set, err := newTestModule(engine).ClosingDayDates(context.Background(), "2026-08-03", "2026-08-01")
	require.NoError(t, err)
	assert.Empty(t, set)
}

func TestClosingDayDatesRejectsInvalidWindow(t *testing.T) {
	t.Parallel()

	module := newTestModule(&calendarDaysEngine{})
	_, err := module.ClosingDayDates(context.Background(), "", "2026-08-01")
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidClosingDay)
	_, err = module.ClosingDayDates(context.Background(), "2026-08-01", "01.08.2026")
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidClosingDay)
}

func TestClosingDayDatesPropagatesStoreError(t *testing.T) {
	t.Parallel()

	_, err := newTestModule(&calendarDaysEngine{closingErr: errors.New("boom")}).
		ClosingDayDates(context.Background(), "2026-08-01", "2026-08-03")
	require.Error(t, err)
}

func TestNonWorkingDayDatesUnionsHolidaysAndClosingDays(t *testing.T) {
	t.Parallel()

	// Allerheiligen 2026-11-01 lies inside a closure week: it must collapse
	// into one entry so a Soll is never subtracted twice.
	engine := &calendarDaysEngine{
		region:  "DE-NW",
		dates:   map[string]bool{"2026-10-03": true, "2026-11-01": true},
		closing: []schoolcalendar.ClosingDay{closingRange("2026-10-12", "2026-10-12", "Pädagogischer Tag"), closingRange("2026-10-30", "2026-11-01", "Brücke")},
	}

	set, err := newTestModule(engine).NonWorkingDayDates(context.Background(), "2026-10-01", "2026-11-30")
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"2026-10-03": true, "2026-10-12": true, "2026-10-30": true, "2026-10-31": true, "2026-11-01": true}, set)
	assert.Len(t, engine.dates, 2, "the union must not mutate the holiday set")
}

func TestNonWorkingDayDatesKeepsTenantHolidaysPure(t *testing.T) {
	t.Parallel()

	engine := &calendarDaysEngine{
		region:   "DE-NW",
		holidays: []schoolcalendar.Holiday{{Date: "2026-05-01", Name: "Tag der Arbeit"}},
		closing:  []schoolcalendar.ClosingDay{closingRange("2026-05-04", "2026-05-04", "Brückentag")},
	}

	list, err := newTestModule(engine).TenantHolidays(context.Background(), "2026-05-01", "2026-05-31")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Tag der Arbeit", list[0].Name, "closing days must never leak into the holiday listing")
}

func TestNonWorkingDayDatesPropagatesErrors(t *testing.T) {
	t.Parallel()

	_, err := newTestModule(&calendarDaysEngine{region: "DE-NW", holidayErr: errors.New("holiday boom")}).
		NonWorkingDayDates(context.Background(), "2026-05-01", "2026-05-31")
	require.Error(t, err)

	_, err = newTestModule(&calendarDaysEngine{region: "DE-NW", dates: map[string]bool{}, closingErr: errors.New("closing boom")}).
		NonWorkingDayDates(context.Background(), "2026-05-01", "2026-05-31")
	require.Error(t, err)
}

func TestListActiveOverlapsSkipsInactivePeriods(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := newTestModule(engine)

	overlaps, err := module.ListActiveOverlaps(context.Background(), schoolcalendar.CalendarPeriod{ID: 7, StartDate: "2030-08-01", EndDate: "2031-07-31", IsActive: false})
	require.NoError(t, err)
	assert.Nil(t, overlaps)
	assert.Equal(t, 0, engine.calls, "an inactive period never reaches the store")

	_, err = module.ListActiveOverlaps(context.Background(), schoolcalendar.CalendarPeriod{ID: 7, StartDate: "2030-08-01", EndDate: "2031-07-31", IsActive: true})
	require.NoError(t, err)
	assert.Equal(t, schoolcalendar.CalendarPeriodFilter{ActiveOnly: true, OverlappingFrom: "2030-08-01", OverlappingTo: "2031-07-31", ExcludeID: 7}, engine.periodFilter)
}

func TestPeriodAdministrationValidatesBeforeDelegating(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := newTestModule(engine)

	_, err := module.AddCalendarPeriod(context.Background(), schoolcalendar.CreateCalendarPeriod{})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
	_, err = module.ChangeCalendarPeriod(context.Background(), schoolcalendar.UpdateCalendarPeriod{CalendarPeriodFields: schoolcalendar.CalendarPeriodFields{Name: "x", PeriodType: schoolcalendar.PeriodTypeCustom, StartDate: "2030-01-01", EndDate: "2030-02-01", WeekCycleLength: 1}})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod, "the ID is required")
	require.ErrorIs(t, module.RemoveCalendarPeriod(context.Background(), 0), schoolcalendar.ErrInvalidCalendarPeriod)
	assert.Equal(t, 0, engine.calls)

	_, err = module.AddCalendarPeriod(context.Background(), schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: schoolcalendar.CalendarPeriodFields{Name: "x", PeriodType: schoolcalendar.PeriodTypeCustom, StartDate: "2030-01-01", EndDate: "2030-02-01", WeekCycleLength: 1}})
	require.NoError(t, err)
	_, _, err = module.EnsureDefaultSchoolYear(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, engine.calls)
}

func TestErrorCodeNamesTheAdministrationOutcomes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "calendar_period_overlap_conflict", schoolcalendar.ErrorCode(&schoolcalendar.CalendarPeriodOverlapError{}))
	assert.Equal(t, "calendar_period_care_offering_conflict", schoolcalendar.ErrorCode(schoolcalendar.ErrCalendarPeriodRequiredByCareOffering))
	assert.Equal(t, "federal_state_unavailable", schoolcalendar.ErrorCode(schoolcalendar.ErrFederalStateUnavailable))
}

func TestNewRecordsAndAdministeredModuleRequireTheirParts(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { New(nil, nil, Administration{}, nil) })
	_, err := WithAdministration(nil, Administration{}, nil)
	require.Error(t, err)
}
