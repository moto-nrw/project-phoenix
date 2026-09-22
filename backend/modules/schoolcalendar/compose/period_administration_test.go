package compose

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// fixedToday keeps the bootstrap independent of the wall clock: 2026-08-24
// belongs to the school year 2026/2027.
const fixedToday = "2026-08-24"

// recurrenceGate takes the tenant-wide recurrence advisory lock on the
// ambient transaction, exactly like the production binding.
func recurrenceGate(db *bun.DB) func(context.Context) error {
	return func(ctx context.Context) error {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return errors.New("recurrence gate requires a transaction")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return fmt.Errorf("recurrence gate: unsupported transaction %T", transaction)
		}
		_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("template-recurrence:%d", tenant.FromContext(ctx)))
		return err
	}
}

func buildAdministration(t *testing.T, db *bun.DB, guard func(context.Context, int64, *schoolcalendar.CalendarPeriodFields) error) *schoolcalendar.Module {
	t.Helper()
	runtime := AdministrationRuntime{RecurrenceGate: recurrenceGate(db), CareOfferingGuard: guard}
	module, err := New(Dependencies{
		DB: db, Observe: func(Observation) {},
		Administration: func() AdministrationRuntime { return runtime },
		Today:          func() string { return fixedToday },
	})
	require.NoError(t, err)
	return module
}

func schoolYear(name, start, end string, active bool) schoolcalendar.CalendarPeriodFields {
	return schoolcalendar.CalendarPeriodFields{
		Name: name, PeriodType: schoolcalendar.PeriodTypeSchoolYear, StartDate: start, EndDate: end,
		WeekCycleLength: 1, IsActive: active,
	}
}

func semester(name, start, end string, active bool) schoolcalendar.CalendarPeriodFields {
	return schoolcalendar.CalendarPeriodFields{
		Name: name, PeriodType: schoolcalendar.PeriodTypeSemester, StartDate: start, EndDate: end,
		WeekCycleLength: 1, IsActive: active,
	}
}

func addPeriod(t *testing.T, ctx context.Context, module *schoolcalendar.Module, fields schoolcalendar.CalendarPeriodFields) schoolcalendar.CalendarPeriod {
	t.Helper()
	period, err := module.AddCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: fields})
	require.NoError(t, err)
	return period
}

func countPeriods(t *testing.T, ctx context.Context, db *bun.DB) int {
	t.Helper()
	var rowCount int
	require.NoError(t, db.NewSelect().
		TableExpr("schedule.calendar_periods").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", tenant.FromContext(ctx)).
		Scan(ctx, &rowCount))
	return rowCount
}

func TestAdministrationAddsPeriods(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildAdministration(t, db, nil)

	t.Run("adds a period", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		period := addPeriod(t, ctx, module, schoolYear("Schuljahr", "2025-08-01", "2026-07-31", true))
		assert.Positive(t, period.ID)
		assert.Equal(t, tenant.FromContext(ctx), period.TenantID)
	})

	t.Run("adds a period with a week cycle", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		fields := schoolYear("Zyklus", "2025-08-01", "2026-07-31", true)
		fields.WeekCycleLength, fields.WeekCycleAnchor = 2, "2025-09-01"
		period := addPeriod(t, ctx, module, fields)
		assert.Equal(t, "2025-09-01", period.WeekCycleAnchor)
	})

	t.Run("rejects a duplicate name", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		addPeriod(t, ctx, module, schoolYear("Doppelt", "2025-08-01", "2026-07-31", true))
		_, err := module.AddCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: semester("Doppelt", "2025-08-01", "2026-01-31", true)})
		require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNameConflict)
		assert.Equal(t, 1, countPeriods(t, ctx, db))
	})

	t.Run("rejects invalid period data", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		_, err := module.AddCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: schoolYear("", "2025-08-01", "2026-07-31", true)})
		require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
		assert.Contains(t, err.Error(), "name is required")

		_, err = module.AddCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: schoolYear("Verkehrt", "2026-08-01", "2025-07-31", true)})
		require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
		assert.Contains(t, err.Error(), "end_date must be after start_date")
		assert.Equal(t, 0, countPeriods(t, ctx, db))
	})
}

func TestAdministrationRejectsSameTypeOverlaps(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildAdministration(t, db, nil)
	ctx := testpkg.Ctx(t)

	base := addPeriod(t, ctx, module, semester("SameType-Basis", "2035-08-01", "2036-01-31", true))

	t.Run("add rejects an active same-type overlap", func(t *testing.T) {
		_, err := module.AddCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: semester("SameType-Kollision", "2035-10-01", "2036-03-31", true)})
		require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodOverlapConflict)

		// The typed error exposes the conflicting period so the handler can
		// name it in the 409 message.
		var overlapErr *schoolcalendar.CalendarPeriodOverlapError
		require.ErrorAs(t, err, &overlapErr)
		require.Len(t, overlapErr.Overlaps, 1)
		assert.Equal(t, base.ID, overlapErr.Overlaps[0].ID)
		assert.Equal(t, base.Name, overlapErr.Overlaps[0].Name)
	})

	t.Run("add allows an inactive same-type overlap", func(t *testing.T) {
		addPeriod(t, ctx, module, semester("SameType-Inaktiv", "2035-10-01", "2036-03-31", false))
	})

	t.Run("add allows an active cross-type overlap", func(t *testing.T) {
		holiday := semester("SameType-Ferien", "2035-10-01", "2035-10-14", true)
		holiday.PeriodType = schoolcalendar.PeriodTypeHoliday
		addPeriod(t, ctx, module, holiday)
	})

	t.Run("add allows an adjacent same-type period", func(t *testing.T) {
		addPeriod(t, ctx, module, semester("SameType-Angrenzend", "2036-02-01", "2036-07-31", true))
	})

	t.Run("change rejects a date change into a same-type overlap", func(t *testing.T) {
		mover := addPeriod(t, ctx, module, semester("SameType-Verschoben", "2037-08-01", "2038-01-31", true))
		_, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: mover.ID, CalendarPeriodFields: semester(mover.Name, "2035-12-01", "2036-01-15", true)})
		require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodOverlapConflict)
	})

	t.Run("change rejects activating an overlapping same-type period", func(t *testing.T) {
		sleeper := addPeriod(t, ctx, module, semester("SameType-Schlafend", "2035-09-01", "2035-12-31", false))
		_, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: sleeper.ID, CalendarPeriodFields: semester(sleeper.Name, sleeper.StartDate, sleeper.EndDate, true)})
		require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodOverlapConflict)
	})

	t.Run("rename-only change of a legacy overlapper stays allowed", func(t *testing.T) {
		// Seed the overlap through the plain command, bypassing the
		// administrative guard — this is the pre-rule legacy data shape.
		legacy := createPeriod(t, ctx, module, semester("SameType-Bestand", "2035-09-01", "2035-11-30", true))
		_, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: legacy.ID, CalendarPeriodFields: semester("SameType-Bestand-Umbenannt", legacy.StartDate, legacy.EndDate, true)})
		require.NoError(t, err, "rename-only edits must not trip the overlap guard on pre-existing overlaps")
	})

	t.Run("change rejects a type change into a same-type overlap", func(t *testing.T) {
		// A holiday inside the base semester is a legal cross-type overlap …
		holiday := semester("SameType-Umgetypt", "2035-11-05", "2035-11-15", true)
		holiday.PeriodType = schoolcalendar.PeriodTypeHoliday
		retyped := addPeriod(t, ctx, module, holiday)
		// … but re-typing it to semester collides with the active base semester.
		_, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: retyped.ID, CalendarPeriodFields: semester(retyped.Name, retyped.StartDate, retyped.EndDate, true)})
		require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodOverlapConflict)
	})

	t.Run("re-typing a legacy overlapper out of the conflict stays allowed", func(t *testing.T) {
		// The guard checks the NEW type, so this repair path must keep working.
		legacy := createPeriod(t, ctx, module, semester("SameType-Reparatur", "2035-12-01", "2036-01-20", true))
		_, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: legacy.ID, CalendarPeriodFields: schoolYear(legacy.Name, legacy.StartDate, legacy.EndDate, true)})
		require.NoError(t, err, "re-typing out of a same-type conflict must resolve, not trip, the overlap guard")
	})
}

// TestAdministrationSerializesConcurrentAdds guards the serialization of
// AddCalendarPeriod: two concurrent adds of overlapping active same-type
// periods must yield exactly one row. Without the tenant recurrence gate both
// requests could pass the overlap check before either insert commits.
func TestAdministrationSerializesConcurrentAdds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildAdministration(t, db, nil)
	ctx := testpkg.Ctx(t)

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := module.AddCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: semester(fmt.Sprintf("Concurrent-Add-%d", index), "2040-08-01", "2041-01-31", true)})
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)

	conflicts := 0
	for err := range results {
		if err != nil {
			require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodOverlapConflict)
			conflicts++
		}
	}
	assert.Equal(t, 1, conflicts, "exactly one add must lose the overlap race")
	assert.Equal(t, 1, countPeriods(t, ctx, db), "the losing add must not leave a second overlapping row")
}

func TestAdministrationChangesPeriods(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildAdministration(t, db, nil)

	t.Run("changes a period", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		period := addPeriod(t, ctx, module, schoolYear("Update", "2025-08-01", "2026-07-31", true))
		changed, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: period.ID, CalendarPeriodFields: semester("Updated", "2025-08-01", "2026-07-31", false)})
		require.NoError(t, err)
		assert.Equal(t, "Updated", changed.Name)
		assert.Equal(t, schoolcalendar.PeriodTypeSemester, changed.PeriodType)
		assert.False(t, changed.IsActive)

		found, err := module.FindCalendarPeriod(ctx, period.ID)
		require.NoError(t, err)
		assert.Equal(t, changed, found)
	})

	t.Run("allows keeping the own name", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		period := addPeriod(t, ctx, module, schoolYear("SameName", "2025-08-01", "2026-07-31", true))
		_, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: period.ID, CalendarPeriodFields: schoolYear("SameName", "2025-08-01", "2026-07-31", false)})
		require.NoError(t, err)
	})

	t.Run("rejects the name of another period", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		addPeriod(t, ctx, module, schoolYear("First", "2025-08-01", "2026-07-31", true))
		second := addPeriod(t, ctx, module, semester("Second", "2025-08-01", "2026-01-31", true))
		_, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: second.ID, CalendarPeriodFields: semester("First", "2025-08-01", "2026-01-31", true)})
		require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNameConflict)
	})

	t.Run("rejects invalid data and an unknown period", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		period := addPeriod(t, ctx, module, schoolYear("InvalidUpdate", "2025-08-01", "2026-07-31", true))
		_, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: period.ID, CalendarPeriodFields: schoolYear("", "2025-08-01", "2026-07-31", true)})
		require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod)
		assert.Contains(t, err.Error(), "name is required")

		_, err = module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: period.ID + 1_000_000, CalendarPeriodFields: schoolYear("Unbekannt", "2025-08-01", "2026-07-31", true)})
		require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNotFound)
	})
}

func TestAdministrationEnsuresDefaultSchoolYear(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildAdministration(t, db, nil)

	t.Run("creates the default school year when the tenant has none", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		periods, created, err := module.EnsureDefaultSchoolYear(ctx)
		require.NoError(t, err)
		assert.True(t, created)
		require.Len(t, periods, 1)
		period := periods[0]
		assert.Equal(t, "Schuljahr 2026/2027", period.Name)
		assert.Equal(t, "2026-08-01", period.StartDate)
		assert.Equal(t, "2027-07-31", period.EndDate)
		assert.Equal(t, schoolcalendar.PeriodTypeSchoolYear, period.PeriodType)
		assert.True(t, period.IsActive)
		assert.Equal(t, 1, period.WeekCycleLength)
	})

	t.Run("is idempotent on repeated calls", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		first, created, err := module.EnsureDefaultSchoolYear(ctx)
		require.NoError(t, err)
		assert.True(t, created)
		require.Len(t, first, 1)

		second, createdAgain, err := module.EnsureDefaultSchoolYear(ctx)
		require.NoError(t, err)
		assert.False(t, createdAgain)
		require.Len(t, second, 1)
		assert.Equal(t, first[0].ID, second[0].ID)
	})

	t.Run("is a no-op when any period already exists", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		existing := semester("Vorhandener-Zeitraum", "2030-01-01", "2030-06-30", false)
		existing.PeriodType = schoolcalendar.PeriodTypeCustom
		kept := addPeriod(t, ctx, module, existing)

		periods, created, err := module.EnsureDefaultSchoolYear(ctx)
		require.NoError(t, err)
		assert.False(t, created, "must not create a school year next to an existing period")
		require.Len(t, periods, 1)
		assert.Equal(t, kept.ID, periods[0].ID)
	})

	t.Run("keeps tenants apart", func(t *testing.T) {
		ctxA := testpkg.OwnCtx(t)
		ctxB, _ := otherTenantContext(t, db)

		periodsA, createdA, err := module.EnsureDefaultSchoolYear(ctxA)
		require.NoError(t, err)
		assert.True(t, createdA)
		require.Len(t, periodsA, 1)

		// Tenant B starts empty even though A now has the same-named period.
		periodsB, createdB, err := module.EnsureDefaultSchoolYear(ctxB)
		require.NoError(t, err)
		assert.True(t, createdB, "tenant B must get its own default period")
		require.Len(t, periodsB, 1)
		assert.NotEqual(t, periodsA[0].ID, periodsB[0].ID)

		again, created, err := module.EnsureDefaultSchoolYear(ctxA)
		require.NoError(t, err)
		assert.False(t, created)
		require.Len(t, again, 1)
		assert.Equal(t, periodsA[0].ID, again[0].ID)
	})

	t.Run("concurrent calls yield one row and no error", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		type outcome struct {
			periods []schoolcalendar.CalendarPeriod
			created bool
			err     error
		}
		results := make(chan outcome, 2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				periods, created, err := module.EnsureDefaultSchoolYear(ctx)
				results <- outcome{periods: periods, created: created, err: err}
			}()
		}
		wg.Wait()
		close(results)

		createdCount := 0
		for res := range results {
			require.NoError(t, res.err)
			assert.Len(t, res.periods, 1)
			if res.created {
				createdCount++
			}
		}
		assert.Equal(t, 1, createdCount, "exactly one caller must win the insert")
		assert.Equal(t, 1, countPeriods(t, ctx, db))
	})
}

// TestAdministrationBootstrapRacesAdd verifies the recurrence-gate
// serialization of EnsureDefaultSchoolYear against AddCalendarPeriod: a
// bootstrap racing an explicit add of an overlapping active same-type period
// must never insert the default school year next to it. Either the bootstrap
// sees the committed period (created=false) or the explicit add loses with
// the overlap conflict — exactly one row survives.
func TestAdministrationBootstrapRacesAdd(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildAdministration(t, db, nil)
	ctx := testpkg.Ctx(t)

	var wg sync.WaitGroup
	var addErr error
	var bootstrapCreated bool
	var bootstrapErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, addErr = module.AddCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: schoolYear("Eigenes-Schuljahr", "2026-08-01", "2027-07-31", true)})
	}()
	go func() {
		defer wg.Done()
		_, bootstrapCreated, bootstrapErr = module.EnsureDefaultSchoolYear(ctx)
	}()
	wg.Wait()

	require.NoError(t, bootstrapErr, "bootstrap must never fail in this race")
	if addErr != nil {
		require.ErrorIs(t, addErr, schoolcalendar.ErrCalendarPeriodOverlapConflict)
		assert.True(t, bootstrapCreated, "add can only lose against a bootstrap that inserted first")
	} else {
		assert.False(t, bootstrapCreated, "bootstrap must not insert next to the committed period")
	}
	assert.Equal(t, 1, countPeriods(t, ctx, db), "the race must never leave two overlapping active school years")
}

func TestAdministrationListsActiveOverlaps(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildAdministration(t, db, nil)
	ctx := testpkg.Ctx(t)

	active := addPeriod(t, ctx, module, schoolYear("Overlaps-Aktiv", "2030-08-01", "2031-07-31", true))
	candidate := addPeriod(t, ctx, module, semester("Overlaps-Kandidat", "2031-01-01", "2031-12-31", true))

	overlaps, err := module.ListActiveOverlaps(ctx, candidate)
	require.NoError(t, err)
	require.Len(t, overlaps, 1)
	assert.Equal(t, active.ID, overlaps[0].ID)

	inactive := candidate
	inactive.IsActive = false
	overlaps, err = module.ListActiveOverlaps(ctx, inactive)
	require.NoError(t, err)
	assert.Nil(t, overlaps, "an inactive candidate short-circuits to nil")
}

func TestAdministrationRemovesPeriods(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildAdministration(t, db, nil)
	ctx := testpkg.Ctx(t)

	period := addPeriod(t, ctx, module, schoolYear("Delete", "2025-08-01", "2026-07-31", true))
	require.NoError(t, module.RemoveCalendarPeriod(ctx, period.ID))
	_, err := module.FindCalendarPeriod(ctx, period.ID)
	require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNotFound)
	require.ErrorIs(t, module.RemoveCalendarPeriod(ctx, period.ID), schoolcalendar.ErrCalendarPeriodNotFound)
}

// TestAdministrationCareOfferingGuardLeavesPeriodUnchanged: the guard sees
// the proposed row under the mutation transaction, and a refusal maps to
// the public sentinel while the stored period stays as it was.
func TestAdministrationCareOfferingGuardLeavesPeriodUnchanged(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	var wantID int64
	var replacements []*schoolcalendar.CalendarPeriodFields
	module := buildAdministration(t, db, func(callCtx context.Context, periodID int64, replacement *schoolcalendar.CalendarPeriodFields) error {
		assert.Equal(t, wantID, periodID)
		_, hasTx := tenant.TransactionFromContext(callCtx)
		assert.True(t, hasTx, "preflight must run under the mutation transaction")
		replacements = append(replacements, replacement)
		return errors.New("care offering still linked")
	})

	period := createPeriod(t, ctx, module, schoolYear("Care-link-preflight", "2026-08-01", "2027-07-31", true))
	wantID = period.ID

	_, err := module.ChangeCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: period.ID, CalendarPeriodFields: schoolYear(period.Name, "2026-09-01", "2027-06-30", true)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "care offering still linked")

	stored, err := module.FindCalendarPeriod(ctx, period.ID)
	require.NoError(t, err)
	assert.Equal(t, period.StartDate, stored.StartDate)
	assert.Equal(t, period.EndDate, stored.EndDate)
	require.Len(t, replacements, 1)
	require.NotNil(t, replacements[0], "update preflight receives the proposed replacement")
	assert.Equal(t, "2026-09-01", replacements[0].StartDate)

	require.Error(t, module.RemoveCalendarPeriod(ctx, period.ID))
	stored, err = module.FindCalendarPeriod(ctx, period.ID)
	require.NoError(t, err)
	assert.Equal(t, period.ID, stored.ID, "rejected delete must leave the period present")
	require.Len(t, replacements, 2)
	assert.Nil(t, replacements[1], "delete preflight is represented by a nil replacement")
}

func TestAdministrationMapsTheCareOfferingRefusal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := buildAdministration(t, db, func(context.Context, int64, *schoolcalendar.CalendarPeriodFields) error {
		return fmt.Errorf("wrapped: %w", schoolcalendar.ErrCalendarPeriodRequiredByCareOffering)
	})
	period := createPeriod(t, ctx, module, schoolYear("Care-link-sentinel", "2026-08-01", "2027-07-31", true))

	err := module.RemoveCalendarPeriod(ctx, period.ID)
	require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodRequiredByCareOffering)
	assert.Equal(t, "calendar_period_care_offering_conflict", schoolcalendar.ErrorCode(err))
}

func TestModuleResolvesTenantHolidaysThroughTheFederalState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	var runtime AdministrationRuntime
	module, err := New(Dependencies{
		DB: db, Observe: func(Observation) {},
		Administration: func() AdministrationRuntime { return runtime },
		Today:          func() string { return fixedToday },
	})
	require.NoError(t, err)
	_, err = module.TenantHolidays(ctx, "2026-05-01", "2026-05-31")
	require.ErrorIs(t, err, schoolcalendar.ErrFederalStateUnavailable, "an unbound runtime reports a configuration error")
	runtime = AdministrationRuntime{FederalState: func(context.Context) (string, error) { return "DE-NW", nil }}
	createClosingDay(t, ctx, module, "2026-05-04", "2026-05-05", "Pädagogische Tage")

	holidays, err := module.TenantHolidays(ctx, "2026-05-01", "2026-05-31")
	require.NoError(t, err)
	names := make([]string, 0, len(holidays))
	for _, holiday := range holidays {
		names = append(names, holiday.Name)
	}
	assert.Contains(t, names, "Tag der Arbeit")

	set, err := module.NonWorkingDayDates(ctx, "2026-05-01", "2026-05-31")
	require.NoError(t, err)
	assert.True(t, set["2026-05-01"], "Tag der Arbeit")
	assert.True(t, set["2026-05-04"], "closing day")
	assert.True(t, set["2026-05-05"], "closing day")
	assert.False(t, set["2026-05-06"])

	unconfigured := buildModule(t, db)
	_, err = unconfigured.TenantHolidays(ctx, "2026-05-01", "2026-05-31")
	require.ErrorIs(t, err, schoolcalendar.ErrFederalStateUnavailable)
}
