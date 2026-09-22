package schoolcalendar

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// calendarDaysEngine stubs the holiday and closing-day reads the tenant
// non-working-day queries compose; every other engine method is unreachable.
type calendarDaysEngine struct {
	recordingEngine
	region      string
	regionErr   error
	holidays    []Holiday
	dates       map[string]bool
	holidayErr  error
	closing     []ClosingDay
	closingErr  error
	seenRegion  string
	seenClosing ClosingDayFilter
}

func (e *calendarDaysEngine) FederalState(context.Context) (string, error) {
	return e.region, e.regionErr
}

func (e *calendarDaysEngine) ValidHolidayRegion(region string) bool {
	return region == "DE-NW" || region == "DE-SN"
}

func (e *calendarDaysEngine) ListHolidays(_ context.Context, region, _, _ string) ([]Holiday, error) {
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

func (e *calendarDaysEngine) ListClosingDays(_ context.Context, filter ClosingDayFilter) ([]ClosingDay, error) {
	e.seenClosing = filter
	return e.closing, e.closingErr
}

func closingRange(start, end, reason string) ClosingDay {
	return ClosingDay{StartDate: start, EndDate: end, Reason: reason}
}

func TestTenantHolidaysResolveTheRegionFromTheTenant(t *testing.T) {
	t.Parallel()

	engine := &calendarDaysEngine{region: "DE-SN", holidays: []Holiday{{Date: "2026-11-18", Name: "Buß- und Bettag"}}}
	module := NewModule(engine)

	list, err := module.TenantHolidays(context.Background(), "2026-11-01", "2026-11-30")
	require.NoError(t, err)
	assert.Equal(t, "DE-SN", engine.seenRegion)
	require.Len(t, list, 1)
	assert.Equal(t, "Buß- und Bettag", list[0].Name)
}

func TestTenantHolidayDates(t *testing.T) {
	t.Parallel()

	module := NewModule(&calendarDaysEngine{region: "DE-NW", dates: map[string]bool{"2026-05-01": true, "2026-05-25": true}})

	set, err := module.TenantHolidayDates(context.Background(), "2026-05-01", "2026-05-31")
	require.NoError(t, err)
	assert.True(t, set["2026-05-01"], "Tag der Arbeit")
	assert.True(t, set["2026-05-25"], "Pfingstmontag")
	assert.False(t, set["2026-05-02"])
}

func TestTenantHolidayErrors(t *testing.T) {
	t.Parallel()

	_, err := NewModule(&calendarDaysEngine{regionErr: errors.New("boom")}).
		TenantHolidayDates(context.Background(), "2026-01-01", "2026-01-02")
	require.ErrorIs(t, err, ErrFederalStateUnavailable)

	_, err = NewModule(&calendarDaysEngine{region: "XX"}).
		TenantHolidayDates(context.Background(), "2026-01-01", "2026-01-02")
	require.ErrorIs(t, err, ErrFederalStateUnavailable)
	assert.Contains(t, err.Error(), "unsupported region")

	_, err = NewModule(&calendarDaysEngine{regionErr: errors.New("boom")}).
		TenantHolidays(context.Background(), "2026-01-01", "2026-01-02")
	require.ErrorIs(t, err, ErrFederalStateUnavailable)

	_, err = NewModule(&calendarDaysEngine{region: "DE-NW"}).
		TenantHolidays(context.Background(), "2026-01-02", "2026-01-01")
	require.ErrorIs(t, err, ErrInvalidHolidayRange, "the window is validated before the region is resolved")
}

func TestClosingDayDatesExpandsRanges(t *testing.T) {
	t.Parallel()

	engine := &calendarDaysEngine{closing: []ClosingDay{
		closingRange("2026-12-24", "2026-12-27", "Weihnachten"),
		closingRange("2026-12-31", "2026-12-31", "Silvester"),
	}}

	set, err := NewModule(engine).ClosingDayDates(context.Background(), "2026-12-01", "2026-12-31")
	require.NoError(t, err)
	assert.Equal(t, ClosingDayFilter{OverlappingFrom: "2026-12-01", OverlappingTo: "2026-12-31"}, engine.seenClosing)
	assert.Len(t, set, 5)
	assert.True(t, set["2026-12-24"])
	assert.True(t, set["2026-12-27"])
	assert.True(t, set["2026-12-31"])
	assert.False(t, set["2026-12-28"])
}

func TestClosingDayDatesClampsToWindow(t *testing.T) {
	t.Parallel()

	// Range extends past both window edges; only in-window days may appear.
	module := NewModule(&calendarDaysEngine{closing: []ClosingDay{closingRange("2026-07-20", "2026-08-07", "Sommerschließung")}})

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
	set, err := NewModule(engine).ClosingDayDates(context.Background(), "2026-08-03", "2026-08-01")
	require.NoError(t, err)
	assert.Empty(t, set)
}

func TestClosingDayDatesRejectsInvalidWindow(t *testing.T) {
	t.Parallel()

	module := NewModule(&calendarDaysEngine{})
	_, err := module.ClosingDayDates(context.Background(), "", "2026-08-01")
	require.ErrorIs(t, err, ErrInvalidClosingDay)
	_, err = module.ClosingDayDates(context.Background(), "2026-08-01", "01.08.2026")
	require.ErrorIs(t, err, ErrInvalidClosingDay)
}

func TestClosingDayDatesPropagatesStoreError(t *testing.T) {
	t.Parallel()

	_, err := NewModule(&calendarDaysEngine{closingErr: errors.New("boom")}).
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
		closing: []ClosingDay{closingRange("2026-10-12", "2026-10-12", "Pädagogischer Tag"), closingRange("2026-10-30", "2026-11-01", "Brücke")},
	}

	set, err := NewModule(engine).NonWorkingDayDates(context.Background(), "2026-10-01", "2026-11-30")
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"2026-10-03": true, "2026-10-12": true, "2026-10-30": true, "2026-10-31": true, "2026-11-01": true}, set)
	assert.Len(t, engine.dates, 2, "the union must not mutate the holiday set")
}

func TestNonWorkingDayDatesKeepsTenantHolidaysPure(t *testing.T) {
	t.Parallel()

	engine := &calendarDaysEngine{
		region:   "DE-NW",
		holidays: []Holiday{{Date: "2026-05-01", Name: "Tag der Arbeit"}},
		closing:  []ClosingDay{closingRange("2026-05-04", "2026-05-04", "Brückentag")},
	}

	list, err := NewModule(engine).TenantHolidays(context.Background(), "2026-05-01", "2026-05-31")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Tag der Arbeit", list[0].Name, "closing days must never leak into the holiday listing")
}

func TestNonWorkingDayDatesPropagatesErrors(t *testing.T) {
	t.Parallel()

	_, err := NewModule(&calendarDaysEngine{region: "DE-NW", holidayErr: errors.New("holiday boom")}).
		NonWorkingDayDates(context.Background(), "2026-05-01", "2026-05-31")
	require.Error(t, err)

	_, err = NewModule(&calendarDaysEngine{region: "DE-NW", dates: map[string]bool{}, closingErr: errors.New("closing boom")}).
		NonWorkingDayDates(context.Background(), "2026-05-01", "2026-05-31")
	require.Error(t, err)
}

func TestListActiveOverlapsSkipsInactivePeriods(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := NewModule(engine)

	overlaps, err := module.ListActiveOverlaps(context.Background(), CalendarPeriod{ID: 7, StartDate: "2030-08-01", EndDate: "2031-07-31", IsActive: false})
	require.NoError(t, err)
	assert.Nil(t, overlaps)
	assert.Equal(t, 0, engine.calls, "an inactive period never reaches the store")

	_, err = module.ListActiveOverlaps(context.Background(), CalendarPeriod{ID: 7, StartDate: "2030-08-01", EndDate: "2031-07-31", IsActive: true})
	require.NoError(t, err)
	assert.Equal(t, CalendarPeriodFilter{ActiveOnly: true, OverlappingFrom: "2030-08-01", OverlappingTo: "2031-07-31", ExcludeID: 7}, engine.periodFilter)
}

func TestPeriodAdministrationValidatesBeforeDelegating(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := NewModule(engine)

	_, err := module.AddCalendarPeriod(context.Background(), CreateCalendarPeriod{})
	require.ErrorIs(t, err, ErrInvalidCalendarPeriod)
	_, err = module.ChangeCalendarPeriod(context.Background(), UpdateCalendarPeriod{CalendarPeriodFields: CalendarPeriodFields{Name: "x", PeriodType: PeriodTypeCustom, StartDate: "2030-01-01", EndDate: "2030-02-01", WeekCycleLength: 1}})
	require.ErrorIs(t, err, ErrInvalidCalendarPeriod, "the ID is required")
	require.ErrorIs(t, module.RemoveCalendarPeriod(context.Background(), 0), ErrInvalidCalendarPeriod)
	assert.Equal(t, 0, engine.calls)

	_, err = module.AddCalendarPeriod(context.Background(), CreateCalendarPeriod{CalendarPeriodFields: CalendarPeriodFields{Name: "x", PeriodType: PeriodTypeCustom, StartDate: "2030-01-01", EndDate: "2030-02-01", WeekCycleLength: 1}})
	require.NoError(t, err)
	_, _, err = module.EnsureDefaultSchoolYear(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, engine.calls)
}

func TestErrorCodeNamesTheAdministrationOutcomes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "calendar_period_overlap_conflict", ErrorCode(&CalendarPeriodOverlapError{}))
	assert.Equal(t, "calendar_period_care_offering_conflict", ErrorCode(ErrCalendarPeriodRequiredByCareOffering))
	assert.Equal(t, "federal_state_unavailable", ErrorCode(ErrFederalStateUnavailable))
}
