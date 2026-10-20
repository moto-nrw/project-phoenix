package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_ schoolcalendar.Days = (schoolcalendar.Calendar)(nil)
)

func federalState(context.Context) (string, error) { return "DE-NW", nil }

func TestNewRecordsRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	observe := func(Observation) {}

	for name, dependencies := range map[string]RecordsDependencies{
		"database":      {Observe: observe, FederalState: federalState},
		"observer":      {DB: db, FederalState: federalState},
		"federal state": {DB: db, Observe: observe},
	} {
		_, err := NewRecords(dependencies)
		require.EqualError(t, err, "school calendar compose: all dependencies are required", name)
	}
}

func TestNewAdministrationRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	observe := func(Observation) {}
	records, err := NewRecords(RecordsDependencies{DB: db, Observe: observe, FederalState: federalState})
	require.NoError(t, err)
	gate := func(context.Context) error { return nil }
	guard := func(context.Context, int64, *schoolcalendar.CalendarPeriodFields) error { return nil }

	_, err = NewAdministration(nil, AdministrationDependencies{DB: db, Observe: observe, RecurrenceGate: gate, CareOfferingGuard: guard})
	require.EqualError(t, err, "school calendar compose: all dependencies are required", "records")
	for name, dependencies := range map[string]AdministrationDependencies{
		"database":            {Observe: observe, RecurrenceGate: gate, CareOfferingGuard: guard},
		"observer":            {DB: db, RecurrenceGate: gate, CareOfferingGuard: guard},
		"recurrence gate":     {DB: db, Observe: observe, CareOfferingGuard: guard},
		"care-offering guard": {DB: db, Observe: observe, RecurrenceGate: gate},
	} {
		_, err := NewAdministration(records, dependencies)
		require.EqualError(t, err, "school calendar compose: all dependencies are required", name)
	}
}

// The Records serve the calendar and its tenant holidays before the
// administration exists; the administration added later runs its gate and
// guard over the same rows.
func TestRecordsServeTheCalendarAndTheAdministrationGuardsItLater(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	log := &observationLog{}
	records, err := NewRecords(RecordsDependencies{DB: db, Observe: log.record, FederalState: federalState})
	require.NoError(t, err)

	createdDay, err := records.CreateClosingDay(ctx, schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2026-05-04", EndDate: "2026-05-04", Reason: "Pädagogischer Tag",
	}})
	require.NoError(t, err)
	set, err := records.NonWorkingDayDates(ctx, "2026-05-01", "2026-05-31")
	require.NoError(t, err)
	assert.True(t, set["2026-05-01"], "Tag der Arbeit through the federal state")
	assert.True(t, set[createdDay.StartDate], "closing day")

	var gated, guarded int
	module, err := NewAdministration(records, AdministrationDependencies{
		DB: db, Observe: log.record, Today: func() string { return fixedToday },
		RecurrenceGate: func(gateCtx context.Context) error {
			gated++
			return recurrenceGate(db)(gateCtx)
		},
		CareOfferingGuard: func(guardCtx context.Context, _ int64, replacement *schoolcalendar.CalendarPeriodFields) error {
			_, hasTx := tenant.TransactionFromContext(guardCtx)
			assert.True(t, hasTx, "the guard runs under the mutation transaction")
			assert.Nil(t, replacement, "a removal has no replacement")
			guarded++
			return schoolcalendar.ErrCalendarPeriodRequiredByCareOffering
		},
	})
	require.NoError(t, err)

	period := addPeriod(t, ctx, module, schoolYear("Schuljahr", "2026-08-01", "2027-07-31", true))
	found, err := records.FindCalendarPeriod(ctx, period.ID)
	require.NoError(t, err)
	assert.Equal(t, period, found)
	require.ErrorIs(t, module.RemoveCalendarPeriod(ctx, period.ID), schoolcalendar.ErrCalendarPeriodRequiredByCareOffering)
	assert.Equal(t, 2, gated)
	assert.Equal(t, 1, guarded)
	holidays, err := module.TenantHolidays(ctx, "2026-05-01", "2026-05-01")
	require.NoError(t, err)
	assert.Len(t, holidays, 1, "the whole module reads holidays through the records")
	assert.Equal(t, []string{
		"create_closing_day", "holiday_dates", "list_closing_days",
		"add_calendar_period", "find_calendar_period", "remove_calendar_period", "list_holidays",
	}, log.operations())
}
