package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingEngine captures what reaches the engine so the tests can prove the
// facade validated and normalised before delegating.
type recordingEngine struct {
	periodFilter    schoolcalendar.CalendarPeriodFilter
	closingFilter   schoolcalendar.ClosingDayFilter
	dateframeFilter schoolcalendar.DateframeFilter
	calls           int
}

func (e *recordingEngine) FindCalendarPeriod(context.Context, int64) (schoolcalendar.CalendarPeriod, error) {
	e.calls++
	return schoolcalendar.CalendarPeriod{}, nil
}

func (e *recordingEngine) ListCalendarPeriods(_ context.Context, filter schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, error) {
	e.calls++
	e.periodFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateCalendarPeriod(context.Context, schoolcalendar.CreateCalendarPeriod, bool) (schoolcalendar.CalendarPeriod, bool, error) {
	e.calls++
	return schoolcalendar.CalendarPeriod{}, true, nil
}

func (e *recordingEngine) UpdateCalendarPeriod(context.Context, schoolcalendar.UpdateCalendarPeriod) (schoolcalendar.CalendarPeriod, error) {
	e.calls++
	return schoolcalendar.CalendarPeriod{}, nil
}

func (e *recordingEngine) DeleteCalendarPeriod(context.Context, int64) error { e.calls++; return nil }

func (e *recordingEngine) FederalState(context.Context) (string, error) {
	e.calls++
	return "DE-NW", nil
}

func (e *recordingEngine) AddCalendarPeriod(context.Context, schoolcalendar.CreateCalendarPeriod) (schoolcalendar.CalendarPeriod, error) {
	e.calls++
	return schoolcalendar.CalendarPeriod{}, nil
}

func (e *recordingEngine) ChangeCalendarPeriod(context.Context, schoolcalendar.UpdateCalendarPeriod) (schoolcalendar.CalendarPeriod, error) {
	e.calls++
	return schoolcalendar.CalendarPeriod{}, nil
}

func (e *recordingEngine) RemoveCalendarPeriod(context.Context, int64) error { e.calls++; return nil }

func (e *recordingEngine) EnsureDefaultSchoolYear(context.Context) ([]schoolcalendar.CalendarPeriod, bool, error) {
	e.calls++
	return nil, false, nil
}

func (e *recordingEngine) FindClosingDay(context.Context, int64) (schoolcalendar.ClosingDay, error) {
	e.calls++
	return schoolcalendar.ClosingDay{}, nil
}

func (e *recordingEngine) ListClosingDays(_ context.Context, filter schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error) {
	e.calls++
	e.closingFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateClosingDay(context.Context, schoolcalendar.CreateClosingDay) (schoolcalendar.ClosingDay, error) {
	e.calls++
	return schoolcalendar.ClosingDay{}, nil
}

func (e *recordingEngine) UpdateClosingDay(context.Context, schoolcalendar.UpdateClosingDay) (schoolcalendar.ClosingDay, error) {
	e.calls++
	return schoolcalendar.ClosingDay{}, nil
}

func (e *recordingEngine) DeleteClosingDay(context.Context, int64) error { e.calls++; return nil }

func (e *recordingEngine) FindDateframe(context.Context, int64) (schoolcalendar.Dateframe, error) {
	e.calls++
	return schoolcalendar.Dateframe{}, nil
}

func (e *recordingEngine) ListDateframes(_ context.Context, filter schoolcalendar.DateframeFilter) ([]schoolcalendar.Dateframe, error) {
	e.calls++
	e.dateframeFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateDateframe(context.Context, schoolcalendar.CreateDateframe) (schoolcalendar.Dateframe, error) {
	e.calls++
	return schoolcalendar.Dateframe{}, nil
}

func (e *recordingEngine) UpdateDateframe(context.Context, schoolcalendar.UpdateDateframe) (schoolcalendar.Dateframe, error) {
	e.calls++
	return schoolcalendar.Dateframe{}, nil
}

func (e *recordingEngine) DeleteDateframe(context.Context, int64) error { e.calls++; return nil }
func (e *recordingEngine) ValidHolidayRegion(string) bool               { return true }
func (e *recordingEngine) ListHolidays(context.Context, string, string, string) ([]schoolcalendar.Holiday, error) {
	e.calls++
	return nil, nil
}
func (e *recordingEngine) HolidayDates(context.Context, string, string, string) (map[string]bool, error) {
	e.calls++
	return nil, nil
}
func (e *recordingEngine) RenderCalendar(context.Context, string, []schoolcalendar.CalendarEvent) (string, error) {
	e.calls++
	return "calendar", nil
}
func (e *recordingEngine) RenderCalendarObject(context.Context, schoolcalendar.CalendarEvent) (string, error) {
	e.calls++
	return "calendar-object", nil
}

func validPeriod() schoolcalendar.CalendarPeriodFields {
	return schoolcalendar.CalendarPeriodFields{
		Name: "Schuljahr 2030/2031", PeriodType: schoolcalendar.PeriodTypeSchoolYear,
		StartDate: "2030-08-01", EndDate: "2031-07-31", WeekCycleLength: 1, IsActive: true,
	}
}

func TestNewModuleRequiresAnEngine(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { newTestModule(nil) })
}

func TestHolidayWindowValidationRunsBeforeTheEngine(t *testing.T) {
	t.Parallel()

	for name, dates := range map[string][2]string{
		"missing dates": {"", ""},
		"missing end":   {"2026-06-01", ""},
		"invalid date":  {"01.06.2026", "2026-06-02"},
		"reverse range": {"2026-06-02", "2026-06-01"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			engine := &recordingEngine{}
			_, err := newTestModule(engine).ListHolidays(context.Background(), "DE-NW", dates[0], dates[1])
			require.ErrorIs(t, err, schoolcalendar.ErrInvalidHolidayRange)
			assert.Zero(t, engine.calls)
		})
	}
}

func TestCalendarPeriodValidationMirrorsTheLegacyRules(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		mutate  func(*schoolcalendar.CalendarPeriodFields)
		message string
	}{
		"name required":   {func(f *schoolcalendar.CalendarPeriodFields) { f.Name = "" }, "name is required"},
		"name too long":   {func(f *schoolcalendar.CalendarPeriodFields) { f.Name = string(make([]byte, 256)) }, "name cannot exceed 255 characters"},
		"period type":     {func(f *schoolcalendar.CalendarPeriodFields) { f.PeriodType = "term" }, "invalid period type"},
		"start required":  {func(f *schoolcalendar.CalendarPeriodFields) { f.StartDate = "" }, "start_date is required"},
		"end required":    {func(f *schoolcalendar.CalendarPeriodFields) { f.EndDate = "" }, "end_date is required"},
		"start format":    {func(f *schoolcalendar.CalendarPeriodFields) { f.StartDate = "01.08.2030" }, "start_date must be a calendar date in YYYY-MM-DD format"},
		"end after start": {func(f *schoolcalendar.CalendarPeriodFields) { f.EndDate = f.StartDate }, "end_date must be after start_date"},
		"cycle length":    {func(f *schoolcalendar.CalendarPeriodFields) { f.WeekCycleLength = 0 }, "week_cycle_length must be at least 1"},
		"anchor required": {func(f *schoolcalendar.CalendarPeriodFields) { f.WeekCycleLength = 2 }, "week_cycle_anchor is required when week_cycle_length > 1"},
		"anchor format":   {func(f *schoolcalendar.CalendarPeriodFields) { f.WeekCycleLength = 2; f.WeekCycleAnchor = "next monday" }, "week_cycle_anchor must be a calendar date in YYYY-MM-DD format"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			engine := &recordingEngine{}
			module := newTestModule(engine)
			fields := validPeriod()
			tc.mutate(&fields)

			_, err := module.CreateCalendarPeriod(context.Background(), schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: fields})
			require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
			assert.EqualError(t, err, tc.message)
			_, _, err = module.CreateCalendarPeriodIfAbsent(context.Background(), schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: fields})
			require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
			_, err = module.UpdateCalendarPeriod(context.Background(), schoolcalendar.UpdateCalendarPeriod{ID: 7, CalendarPeriodFields: fields})
			require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
			assert.Zero(t, engine.calls, "validation runs before any persistence")
		})
	}
}

func TestClosingDayValidationMirrorsTheLegacyRules(t *testing.T) {
	t.Parallel()
	valid := schoolcalendar.ClosingDayFields{StartDate: "2030-11-04", EndDate: "2030-11-08", Reason: "Pädagogische Woche"}
	cases := map[string]struct {
		mutate  func(*schoolcalendar.ClosingDayFields)
		message string
	}{
		"reason required":  {func(f *schoolcalendar.ClosingDayFields) { f.Reason = "  " }, "reason is required"},
		"reason too long":  {func(f *schoolcalendar.ClosingDayFields) { f.Reason = string(make([]rune, 256)) }, "reason cannot exceed 255 characters"},
		"start required":   {func(f *schoolcalendar.ClosingDayFields) { f.StartDate = "" }, "start_date is required"},
		"end required":     {func(f *schoolcalendar.ClosingDayFields) { f.EndDate = "" }, "end_date is required"},
		"end before start": {func(f *schoolcalendar.ClosingDayFields) { f.EndDate = "2030-11-03" }, "end_date must not be before start_date"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			engine := &recordingEngine{}
			module := newTestModule(engine)
			fields := valid
			tc.mutate(&fields)

			_, err := module.CreateClosingDay(context.Background(), schoolcalendar.CreateClosingDay{ClosingDayFields: fields})
			require.ErrorIs(t, err, schoolcalendar.ErrInvalidClosingDay)
			assert.EqualError(t, err, tc.message)
			assert.Zero(t, engine.calls)
		})
	}

	engine := &recordingEngine{}
	module := newTestModule(engine)
	_, err := module.CreateClosingDay(context.Background(), schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{StartDate: "2030-11-04", EndDate: "2030-11-04", Reason: "Rosenmontag"}})
	require.NoError(t, err, "a single day closes with start = end")
}

func TestDateframeValidationMirrorsTheLegacyRules(t *testing.T) {
	t.Parallel()
	start := time.Date(2030, time.August, 1, 0, 0, 0, 0, time.UTC)
	engine := &recordingEngine{}
	module := newTestModule(engine)

	_, err := module.CreateDateframe(context.Background(), schoolcalendar.CreateDateframe{DateframeFields: schoolcalendar.DateframeFields{EndDate: start}})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidDateframe)
	assert.EqualError(t, err, "start date is required")
	_, err = module.CreateDateframe(context.Background(), schoolcalendar.CreateDateframe{DateframeFields: schoolcalendar.DateframeFields{StartDate: start}})
	assert.EqualError(t, err, "end date is required")
	_, err = module.CreateDateframe(context.Background(), schoolcalendar.CreateDateframe{DateframeFields: schoolcalendar.DateframeFields{StartDate: start, EndDate: start.Add(-time.Hour)}})
	assert.EqualError(t, err, "end date must be on or after start date")
	assert.Zero(t, engine.calls)

	_, err = module.CreateDateframe(context.Background(), schoolcalendar.CreateDateframe{DateframeFields: schoolcalendar.DateframeFields{StartDate: start, EndDate: start}})
	require.NoError(t, err, "a zero-length range is allowed")
}

func TestFiltersAreNormalisedBeforeDelegating(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := newTestModule(engine)
	ctx := context.Background()

	_, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{IDs: []int64{3, 0, 3, -1, 5}, PeriodType: schoolcalendar.PeriodTypeHoliday})
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 5}, engine.periodFilter.IDs, "ids are deduplicated and non-positive ones dropped")

	_, err = module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{PeriodType: "term"})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
	_, err = module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{OverlappingFrom: "2030-10-01", OverlappingTo: "2030-09-01"})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
	_, err = module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{OverlappingTo: "2030-09-01"})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
	_, err = module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{ExcludeID: -1})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)

	_, err = module.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: "2030-11-01", OverlappingTo: "2030-11-30", IDs: []int64{0}})
	require.NoError(t, err)
	assert.Equal(t, []int64{}, engine.closingFilter.IDs)
	_, err = module.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: "November"})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidClosingDay)

	from := time.Date(2030, time.August, 1, 0, 0, 0, 0, time.UTC)
	_, err = module.ListDateframes(ctx, schoolcalendar.DateframeFilter{OverlappingFrom: &from})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidDateframe)
	_, err = module.ListDateframes(ctx, schoolcalendar.DateframeFilter{Limit: -1})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidDateframe)
	_, err = module.ListDateframes(ctx, schoolcalendar.DateframeFilter{Sort: []schoolcalendar.DateframeSort{{Field: "tenant_id"}}})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidDateframe)
	_, err = module.ListDateframes(ctx, schoolcalendar.DateframeFilter{Sort: []schoolcalendar.DateframeSort{{Field: schoolcalendar.DateframeSortName, Descending: true}}, Limit: 5, Offset: 10})
	require.NoError(t, err)
	assert.Equal(t, 5, engine.dateframeFilter.Limit)
	assert.Equal(t, 10, engine.dateframeFilter.Offset)
}

func TestIDsMustBePositive(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := newTestModule(engine)
	ctx := context.Background()

	_, err := module.FindCalendarPeriod(ctx, 0)
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
	require.ErrorIs(t, module.DeleteCalendarPeriod(ctx, -4), schoolcalendar.ErrInvalidCalendarPeriod)
	_, err = module.UpdateCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{CalendarPeriodFields: validPeriod()})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
	_, err = module.FindClosingDay(ctx, 0)
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidClosingDay)
	require.ErrorIs(t, module.DeleteClosingDay(ctx, 0), schoolcalendar.ErrInvalidClosingDay)
	_, err = module.FindDateframe(ctx, 0)
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidDateframe)
	require.ErrorIs(t, module.DeleteDateframe(ctx, 0), schoolcalendar.ErrInvalidDateframe)
	assert.Zero(t, engine.calls)
}

func TestErrorCodeLabelsTheStableOutcomes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "none", schoolcalendar.ErrorCode(nil))
	assert.Equal(t, "not_found", schoolcalendar.ErrorCode(schoolcalendar.ErrCalendarPeriodNotFound))
	assert.Equal(t, "not_found", schoolcalendar.ErrorCode(schoolcalendar.ErrClosingDayNotFound))
	assert.Equal(t, "not_found", schoolcalendar.ErrorCode(schoolcalendar.ErrDateframeNotFound))
	assert.Equal(t, "invalid", schoolcalendar.ErrorCode(&schoolcalendar.InvalidCalendarPeriodError{Reason: "x"}))
	assert.Equal(t, "invalid", schoolcalendar.ErrorCode(&schoolcalendar.InvalidClosingDayError{Reason: "x"}))
	assert.Equal(t, "invalid", schoolcalendar.ErrorCode(&schoolcalendar.InvalidDateframeError{Reason: "x"}))
	assert.Equal(t, "calendar_period_name_conflict", schoolcalendar.ErrorCode(schoolcalendar.ErrCalendarPeriodNameConflict))
	assert.Equal(t, "internal_error", schoolcalendar.ErrorCode(errors.New("boom")))
}
