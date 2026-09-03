package schoolcalendar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingEngine captures what reaches the engine so the tests can prove the
// facade validated and normalised before delegating.
type recordingEngine struct {
	periodFilter    CalendarPeriodFilter
	closingFilter   ClosingDayFilter
	dateframeFilter DateframeFilter
	calls           int
}

func (e *recordingEngine) FindCalendarPeriod(context.Context, int64) (CalendarPeriod, error) {
	e.calls++
	return CalendarPeriod{}, nil
}

func (e *recordingEngine) ListCalendarPeriods(_ context.Context, filter CalendarPeriodFilter) ([]CalendarPeriod, error) {
	e.calls++
	e.periodFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateCalendarPeriod(context.Context, CreateCalendarPeriod, bool) (CalendarPeriod, bool, error) {
	e.calls++
	return CalendarPeriod{}, true, nil
}

func (e *recordingEngine) UpdateCalendarPeriod(context.Context, UpdateCalendarPeriod) (CalendarPeriod, error) {
	e.calls++
	return CalendarPeriod{}, nil
}

func (e *recordingEngine) DeleteCalendarPeriod(context.Context, int64) error { e.calls++; return nil }

func (e *recordingEngine) FindClosingDay(context.Context, int64) (ClosingDay, error) {
	e.calls++
	return ClosingDay{}, nil
}

func (e *recordingEngine) ListClosingDays(_ context.Context, filter ClosingDayFilter) ([]ClosingDay, error) {
	e.calls++
	e.closingFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateClosingDay(context.Context, CreateClosingDay) (ClosingDay, error) {
	e.calls++
	return ClosingDay{}, nil
}

func (e *recordingEngine) UpdateClosingDay(context.Context, UpdateClosingDay) (ClosingDay, error) {
	e.calls++
	return ClosingDay{}, nil
}

func (e *recordingEngine) DeleteClosingDay(context.Context, int64) error { e.calls++; return nil }

func (e *recordingEngine) FindDateframe(context.Context, int64) (Dateframe, error) {
	e.calls++
	return Dateframe{}, nil
}

func (e *recordingEngine) ListDateframes(_ context.Context, filter DateframeFilter) ([]Dateframe, error) {
	e.calls++
	e.dateframeFilter = filter
	return nil, nil
}

func (e *recordingEngine) CreateDateframe(context.Context, CreateDateframe) (Dateframe, error) {
	e.calls++
	return Dateframe{}, nil
}

func (e *recordingEngine) UpdateDateframe(context.Context, UpdateDateframe) (Dateframe, error) {
	e.calls++
	return Dateframe{}, nil
}

func (e *recordingEngine) DeleteDateframe(context.Context, int64) error { e.calls++; return nil }

func validPeriod() CalendarPeriodFields {
	return CalendarPeriodFields{
		Name: "Schuljahr 2030/2031", PeriodType: PeriodTypeSchoolYear,
		StartDate: "2030-08-01", EndDate: "2031-07-31", WeekCycleLength: 1, IsActive: true,
	}
}

func TestNewModuleRequiresAnEngine(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { NewModule(nil) })
}

func TestCalendarPeriodValidationMirrorsTheLegacyRules(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		mutate  func(*CalendarPeriodFields)
		message string
	}{
		"name required":   {func(f *CalendarPeriodFields) { f.Name = "" }, "name is required"},
		"name too long":   {func(f *CalendarPeriodFields) { f.Name = string(make([]byte, 256)) }, "name cannot exceed 255 characters"},
		"period type":     {func(f *CalendarPeriodFields) { f.PeriodType = "term" }, "invalid period type"},
		"start required":  {func(f *CalendarPeriodFields) { f.StartDate = "" }, "start_date is required"},
		"end required":    {func(f *CalendarPeriodFields) { f.EndDate = "" }, "end_date is required"},
		"start format":    {func(f *CalendarPeriodFields) { f.StartDate = "01.08.2030" }, "start_date must be a calendar date in YYYY-MM-DD format"},
		"end after start": {func(f *CalendarPeriodFields) { f.EndDate = f.StartDate }, "end_date must be after start_date"},
		"cycle length":    {func(f *CalendarPeriodFields) { f.WeekCycleLength = 0 }, "week_cycle_length must be at least 1"},
		"anchor required": {func(f *CalendarPeriodFields) { f.WeekCycleLength = 2 }, "week_cycle_anchor is required when week_cycle_length > 1"},
		"anchor format":   {func(f *CalendarPeriodFields) { f.WeekCycleLength = 2; f.WeekCycleAnchor = "next monday" }, "week_cycle_anchor must be a calendar date in YYYY-MM-DD format"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			engine := &recordingEngine{}
			module := NewModule(engine)
			fields := validPeriod()
			tc.mutate(&fields)

			_, err := module.CreateCalendarPeriod(context.Background(), CreateCalendarPeriod{CalendarPeriodFields: fields})
			require.ErrorIs(t, err, ErrInvalidCalendarPeriod)
			assert.EqualError(t, err, tc.message)
			_, _, err = module.CreateCalendarPeriodIfAbsent(context.Background(), CreateCalendarPeriod{CalendarPeriodFields: fields})
			require.ErrorIs(t, err, ErrInvalidCalendarPeriod)
			_, err = module.UpdateCalendarPeriod(context.Background(), UpdateCalendarPeriod{ID: 7, CalendarPeriodFields: fields})
			require.ErrorIs(t, err, ErrInvalidCalendarPeriod)
			assert.Zero(t, engine.calls, "validation runs before any persistence")
		})
	}
}

func TestClosingDayValidationMirrorsTheLegacyRules(t *testing.T) {
	t.Parallel()
	valid := ClosingDayFields{StartDate: "2030-11-04", EndDate: "2030-11-08", Reason: "Pädagogische Woche"}
	cases := map[string]struct {
		mutate  func(*ClosingDayFields)
		message string
	}{
		"reason required":  {func(f *ClosingDayFields) { f.Reason = "  " }, "reason is required"},
		"reason too long":  {func(f *ClosingDayFields) { f.Reason = string(make([]rune, 256)) }, "reason cannot exceed 255 characters"},
		"start required":   {func(f *ClosingDayFields) { f.StartDate = "" }, "start_date is required"},
		"end required":     {func(f *ClosingDayFields) { f.EndDate = "" }, "end_date is required"},
		"end before start": {func(f *ClosingDayFields) { f.EndDate = "2030-11-03" }, "end_date must not be before start_date"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			engine := &recordingEngine{}
			module := NewModule(engine)
			fields := valid
			tc.mutate(&fields)

			_, err := module.CreateClosingDay(context.Background(), CreateClosingDay{ClosingDayFields: fields})
			require.ErrorIs(t, err, ErrInvalidClosingDay)
			assert.EqualError(t, err, tc.message)
			assert.Zero(t, engine.calls)
		})
	}

	engine := &recordingEngine{}
	module := NewModule(engine)
	_, err := module.CreateClosingDay(context.Background(), CreateClosingDay{ClosingDayFields: ClosingDayFields{StartDate: "2030-11-04", EndDate: "2030-11-04", Reason: "Rosenmontag"}})
	require.NoError(t, err, "a single day closes with start = end")
}

func TestDateframeValidationMirrorsTheLegacyRules(t *testing.T) {
	t.Parallel()
	start := time.Date(2030, time.August, 1, 0, 0, 0, 0, time.UTC)
	engine := &recordingEngine{}
	module := NewModule(engine)

	_, err := module.CreateDateframe(context.Background(), CreateDateframe{DateframeFields: DateframeFields{EndDate: start}})
	require.ErrorIs(t, err, ErrInvalidDateframe)
	assert.EqualError(t, err, "start date is required")
	_, err = module.CreateDateframe(context.Background(), CreateDateframe{DateframeFields: DateframeFields{StartDate: start}})
	assert.EqualError(t, err, "end date is required")
	_, err = module.CreateDateframe(context.Background(), CreateDateframe{DateframeFields: DateframeFields{StartDate: start, EndDate: start.Add(-time.Hour)}})
	assert.EqualError(t, err, "end date must be on or after start date")
	assert.Zero(t, engine.calls)

	_, err = module.CreateDateframe(context.Background(), CreateDateframe{DateframeFields: DateframeFields{StartDate: start, EndDate: start}})
	require.NoError(t, err, "a zero-length range is allowed")
}

func TestFiltersAreNormalisedBeforeDelegating(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := NewModule(engine)
	ctx := context.Background()

	_, err := module.ListCalendarPeriods(ctx, CalendarPeriodFilter{IDs: []int64{3, 0, 3, -1, 5}, PeriodType: PeriodTypeHoliday})
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 5}, engine.periodFilter.IDs, "ids are deduplicated and non-positive ones dropped")

	_, err = module.ListCalendarPeriods(ctx, CalendarPeriodFilter{PeriodType: "term"})
	require.ErrorIs(t, err, ErrInvalidCalendarPeriod)
	_, err = module.ListCalendarPeriods(ctx, CalendarPeriodFilter{OverlappingFrom: "2030-10-01", OverlappingTo: "2030-09-01"})
	require.ErrorIs(t, err, ErrInvalidCalendarPeriod)
	_, err = module.ListCalendarPeriods(ctx, CalendarPeriodFilter{OverlappingTo: "2030-09-01"})
	require.ErrorIs(t, err, ErrInvalidCalendarPeriod)
	_, err = module.ListCalendarPeriods(ctx, CalendarPeriodFilter{ExcludeID: -1})
	require.ErrorIs(t, err, ErrInvalidCalendarPeriod)

	_, err = module.ListClosingDays(ctx, ClosingDayFilter{OverlappingFrom: "2030-11-01", OverlappingTo: "2030-11-30", IDs: []int64{0}})
	require.NoError(t, err)
	assert.Equal(t, []int64{}, engine.closingFilter.IDs)
	_, err = module.ListClosingDays(ctx, ClosingDayFilter{OverlappingFrom: "November"})
	require.ErrorIs(t, err, ErrInvalidClosingDay)

	from := time.Date(2030, time.August, 1, 0, 0, 0, 0, time.UTC)
	_, err = module.ListDateframes(ctx, DateframeFilter{OverlappingFrom: &from})
	require.ErrorIs(t, err, ErrInvalidDateframe)
	_, err = module.ListDateframes(ctx, DateframeFilter{Limit: -1})
	require.ErrorIs(t, err, ErrInvalidDateframe)
	_, err = module.ListDateframes(ctx, DateframeFilter{Sort: []DateframeSort{{Field: "tenant_id"}}})
	require.ErrorIs(t, err, ErrInvalidDateframe)
	_, err = module.ListDateframes(ctx, DateframeFilter{Sort: []DateframeSort{{Field: DateframeSortName, Descending: true}}, Limit: 5, Offset: 10})
	require.NoError(t, err)
	assert.Equal(t, 5, engine.dateframeFilter.Limit)
	assert.Equal(t, 10, engine.dateframeFilter.Offset)
}

func TestIDsMustBePositive(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := NewModule(engine)
	ctx := context.Background()

	_, err := module.FindCalendarPeriod(ctx, 0)
	require.ErrorIs(t, err, ErrInvalidCalendarPeriod)
	require.ErrorIs(t, module.DeleteCalendarPeriod(ctx, -4), ErrInvalidCalendarPeriod)
	_, err = module.UpdateCalendarPeriod(ctx, UpdateCalendarPeriod{CalendarPeriodFields: validPeriod()})
	require.ErrorIs(t, err, ErrInvalidCalendarPeriod)
	_, err = module.FindClosingDay(ctx, 0)
	require.ErrorIs(t, err, ErrInvalidClosingDay)
	require.ErrorIs(t, module.DeleteClosingDay(ctx, 0), ErrInvalidClosingDay)
	_, err = module.FindDateframe(ctx, 0)
	require.ErrorIs(t, err, ErrInvalidDateframe)
	require.ErrorIs(t, module.DeleteDateframe(ctx, 0), ErrInvalidDateframe)
	assert.Zero(t, engine.calls)
}

func TestErrorCodeLabelsTheStableOutcomes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "none", ErrorCode(nil))
	assert.Equal(t, "not_found", ErrorCode(ErrCalendarPeriodNotFound))
	assert.Equal(t, "not_found", ErrorCode(ErrClosingDayNotFound))
	assert.Equal(t, "not_found", ErrorCode(ErrDateframeNotFound))
	assert.Equal(t, "invalid", ErrorCode(&InvalidCalendarPeriodError{Reason: "x"}))
	assert.Equal(t, "invalid", ErrorCode(&InvalidClosingDayError{Reason: "x"}))
	assert.Equal(t, "invalid", ErrorCode(&InvalidDateframeError{Reason: "x"}))
	assert.Equal(t, "calendar_period_name_conflict", ErrorCode(ErrCalendarPeriodNameConflict))
	assert.Equal(t, "internal_error", ErrorCode(errors.New("boom")))
}
