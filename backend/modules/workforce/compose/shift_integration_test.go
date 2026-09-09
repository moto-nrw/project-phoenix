package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The four Dienstplan tables Workforce owns (#2689) are tenant-scoped
// through the ambient tenant of the context plus row-level security. The
// tests below write the same shape of row into two tenants and assert that
// reads, updates, deletes and caps through the capability never cross that
// boundary, and that the row rules the legacy repositories enforced hold.

func testShift(staffID int64, day timezone.Date, start, end string) workforce.StaffShift {
	return workforce.StaffShift{
		StaffID: staffID, Date: day.String(), StartTime: start, EndTime: end, CreatedBy: staffID,
	}
}

func TestStaffShiftsAreTenantIsolated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)

	local := testpkg.CreateTestStaff(t, db, "Shift", "Local")
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Shift", "Foreign")
	day := timezone.NewDate(2026, 7, 6)
	capability := buildWorkforce(t, db)

	localShift, err := capability.CreateStaffShift(ctx, testShift(local.ID, day, "08:00:00", "16:30:00"))
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), localShift.TenantID, "the ambient tenant is stamped on the row")
	foreignShift, err := capability.CreateStaffShift(foreignCtx, testShift(foreign.ID, day, "08:00:00", "16:30:00"))
	require.NoError(t, err)
	assert.Equal(t, foreignTenantID, foreignShift.TenantID)

	listed, err := capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{From: day.String(), To: day.String()})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, localShift.ID, listed[0].ID)

	_, err = capability.FindStaffShift(ctx, foreignShift.ID)
	require.ErrorIs(t, err, workforce.ErrStaffShiftNotFound, "a foreign row is invisible, not an error of another kind")

	weeks, err := capability.UsedStaffShiftWeeks(foreignCtx, day.String(), day.AddDays(6).String())
	require.NoError(t, err)
	assert.Equal(t, []string{day.String()}, weeks, "each tenant sees only its own planned weeks")

	// An update addressed at a foreign row reports not found instead of
	// rewriting the other tenant's row.
	foreignShift.Notes = "crossed"
	_, err = capability.UpdateStaffShift(ctx, foreignShift)
	require.ErrorIs(t, err, workforce.ErrStaffShiftNotFound)
	absenceType := testpkg.CreateTestStaffAbsenceType(t, db, "Shift isolation")
	absence := testpkg.CreateTestStaffAbsenceToday(t, db, local.ID, absenceType.ID)
	stamped, err := capability.SetStaffShiftSickAbsence(ctx, foreignShift.ID, &absence.ID)
	require.NoError(t, err)
	assert.Zero(t, stamped, "a partial update addressed at a foreign row touches nothing")

	// Deletes addressed at foreign rows are no-ops in the caller's tenant.
	require.NoError(t, capability.DeleteStaffShift(ctx, foreignShift.ID))
	deleted, err := capability.DeleteUpcomingStaffShifts(ctx, foreign.ID, day.AddDays(-1).String())
	require.NoError(t, err)
	assert.Zero(t, deleted)
	stillThere, err := capability.FindStaffShift(foreignCtx, foreignShift.ID)
	require.NoError(t, err)
	assert.Equal(t, foreignShift.ID, stillThere.ID)
	assert.Empty(t, stillThere.Notes)
	assert.Nil(t, stillThere.SickAbsenceID)

	// A tenant cannot plan a shift for another tenant's staff member.
	_, err = capability.CreateStaffShift(foreignCtx, testShift(local.ID, day.AddDays(1), "08:00:00", "12:00:00"))
	require.Error(t, err, "the composite staff FK rejects a cross-tenant reference")
	crossed, err := capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{From: day.AddDays(1).String(), To: day.AddDays(1).String()})
	require.NoError(t, err)
	assert.Empty(t, crossed, "the rejected insert leaves nothing behind")
}

func TestStaffShiftRowRulesMatchTheLegacyRepository(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Shift", "Rules")
	other := testpkg.CreateTestStaff(t, db, "Shift", "Batch")
	monday := timezone.NewDate(2026, 7, 6)
	capability := buildWorkforce(t, db)

	created, err := capability.CreateStaffShift(ctx, testShift(staff.ID, monday, "08:00:00", "16:30:00"))
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, "08:00:00", created.StartTime, "TIME columns come back as bare wall clocks")
	assert.Equal(t, "16:30:00", created.EndTime)

	found, err := capability.FindStaffShift(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, monday.String(), found.Date)
	assert.Equal(t, "16:30:00", found.EndTime)

	// The cancellation-aware unique index rejects a second active shift with
	// the same start, and the conflict keeps the driver error reachable.
	_, err = capability.CreateStaffShift(ctx, testShift(staff.ID, monday, "08:00:00", "14:00:00"))
	require.ErrorIs(t, err, workforce.ErrStaffShiftDuplicate)
	var conflict *workforce.ConflictError
	require.ErrorAs(t, err, &conflict)
	assert.NotNil(t, conflict.Cause)

	// Validation runs before any statement.
	_, err = capability.CreateStaffShift(ctx, testShift(staff.ID, monday, "16:00:00", "08:00:00"))
	require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
	_, err = capability.SetStaffShiftSickAbsence(ctx, 0, nil)
	require.ErrorIs(t, err, workforce.ErrInvalidStaffShift, "a shift ID is required")

	created.EndTime = "17:00:00"
	created.Notes = "Updated through the facade"
	updated, err := capability.UpdateStaffShift(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "17:00:00", updated.EndTime)
	assert.Equal(t, "Updated through the facade", updated.Notes)

	absenceType := testpkg.CreateTestStaffAbsenceType(t, db, "Shift rules")
	absence := testpkg.CreateTestStaffAbsenceToday(t, db, staff.ID, absenceType.ID)
	absenceID := absence.ID
	stamped, err := capability.SetStaffShiftSickAbsence(ctx, updated.ID, &absenceID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, stamped)
	bySickAbsence, err := capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{SickAbsenceID: &absenceID})
	require.NoError(t, err)
	require.Len(t, bySickAbsence, 1)
	assert.Equal(t, created.ID, bySickAbsence[0].ID)

	// Batch reads address exact staff/date sets; an explicit empty set
	// matches nobody.
	saturday := monday.AddDays(5)
	nextMonday := monday.AddDays(7)
	thirdMonday := monday.AddDays(14)
	cancelledOnly := testShift(staff.ID, thirdMonday, "08:00:00", "12:00:00")
	cancelledOnly.Cancelled = true
	rows, err := capability.CreateStaffShifts(ctx, []workforce.StaffShift{
		testShift(staff.ID, saturday, "08:00:00", "12:00:00"),
		testShift(staff.ID, nextMonday, "08:00:00", "12:00:00"),
		testShift(other.ID, monday, "08:00:00", "12:00:00"),
		cancelledOnly,
	})
	require.NoError(t, err)
	require.Len(t, rows, 4)
	for _, row := range rows {
		assert.NotZero(t, row.ID, "bulk inserts return the identities")
	}

	exact, err := capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{
		StaffIDs: []int64{staff.ID}, Dates: []string{monday.String(), nextMonday.String()},
		Order: []workforce.StaffShiftOrder{{Field: workforce.StaffShiftOrderDate}},
	})
	require.NoError(t, err)
	require.Len(t, exact, 2)
	assert.Equal(t, monday.String(), exact[0].Date)
	assert.Equal(t, nextMonday.String(), exact[1].Date)

	nobody, err := capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{StaffIDs: []int64{}, From: monday.String(), To: thirdMonday.String()})
	require.NoError(t, err)
	assert.Empty(t, nobody)

	weeks, err := capability.UsedStaffShiftWeeks(ctx, monday.String(), thirdMonday.AddDays(6).String())
	require.NoError(t, err)
	assert.Equal(t, []string{monday.String(), nextMonday.String()}, weeks, "the cancelled-only week must not report as used")

	deleted, err := capability.DeleteUpcomingStaffShifts(ctx, staff.ID, nextMonday.String())
	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted, "the same day and later rows go, earlier rows stay")
	remaining, err := capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{StaffID: staff.ID, From: monday.String(), To: thirdMonday.String()})
	require.NoError(t, err)
	assert.Len(t, remaining, 2)
}

func TestStaffShiftSeriesLifecycleStaysInTheTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)

	staff := testpkg.CreateTestStaff(t, db, "Series", "Owner")
	periodStart := timezone.NewDate(2026, 9, 1)
	periodEnd := timezone.NewDate(2027, 6, 30)
	period := testpkg.CreateTestCalendarPeriod(t, db, "Serienjahr", periodStart, periodEnd)
	capability := buildWorkforce(t, db)

	series, err := capability.CreateStaffShiftSeries(ctx, workforce.StaffShiftSeries{
		StaffID: staff.ID, Weekdays: []int{1, 3}, StartTime: "09:00:00", EndTime: "12:00:00", BreakMinutes: 15,
		CalendarPeriodID: period.ID, WeekPattern: workforce.WeekPatternEvery, ValidFrom: periodStart.String(), CreatedBy: staff.ID,
	})
	require.NoError(t, err)
	require.NotZero(t, series.ID)
	assert.Equal(t, []int{1, 3}, series.Weekdays)
	assert.Empty(t, series.ValidUntil, "a series without an end runs to the period end")

	_, err = capability.CreateStaffShiftSeries(ctx, workforce.StaffShiftSeries{
		StaffID: staff.ID, Weekdays: []int{8}, StartTime: "09:00:00", EndTime: "12:00:00",
		CalendarPeriodID: period.ID, ValidFrom: periodStart.String(), CreatedBy: staff.ID,
	})
	require.ErrorIs(t, err, workforce.ErrInvalidShiftSeries)

	// The materialized rows carry the series and their source slot.
	firstMonday := timezone.NewDate(2026, 9, 7)
	secondMonday := firstMonday.AddDays(7)
	seriesID := series.ID
	materialized := make([]workforce.StaffShift, 0, 2)
	for _, day := range []timezone.Date{firstMonday, secondMonday} {
		shift := testShift(staff.ID, day, "09:00:00", "12:00:00")
		shift.SeriesID = &seriesID
		shift.SeriesOccurrenceDate = day.String()
		materialized = append(materialized, shift)
	}
	rows, err := capability.CreateStaffShifts(ctx, materialized)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, firstMonday.String(), rows[0].SeriesOccurrenceDate)

	// A detached row survives re-plans; the second occurrence is removed.
	detached := rows[0]
	detached.Detached = true
	_, err = capability.UpdateStaffShift(ctx, detached)
	require.NoError(t, err)
	exception := workforce.StaffShiftSeriesException{SeriesID: series.ID, Date: secondMonday.String(), CreatedBy: staff.ID}
	require.NoError(t, capability.RecordSeriesException(ctx, exception))
	require.NoError(t, capability.RecordSeriesException(ctx, exception), "recording the same slot again is a no-op")
	dates, err := capability.SeriesExceptionDates(ctx, series.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{secondMonday.String()}, dates)

	// The foreign tenant sees none of it and cannot touch it.
	_, err = capability.FindStaffShiftSeries(foreignCtx, series.ID)
	require.ErrorIs(t, err, workforce.ErrShiftSeriesNotFound)
	foreignDates, err := capability.SeriesExceptionDates(foreignCtx, series.ID)
	require.NoError(t, err)
	assert.Empty(t, foreignDates)
	require.NoError(t, capability.CapStaffShiftSeries(foreignCtx, series.ID, firstMonday.String()))
	require.NoError(t, capability.DeleteStaffShiftSeries(foreignCtx, series.ID))
	unchanged, err := capability.FindStaffShiftSeries(ctx, series.ID)
	require.NoError(t, err)
	assert.Empty(t, unchanged.ValidUntil, "a foreign cap does not bound the series")
	require.Error(t, capability.RecordSeriesException(foreignCtx, exception), "the composite series FK rejects a foreign exception")

	// A split caps the predecessor, creates the successor in the lineage and
	// repoints detached rows and exceptions from the effective date.
	effective := secondMonday
	require.NoError(t, capability.CapStaffShiftSeries(ctx, series.ID, effective.String()))
	capped, err := capability.FindStaffShiftSeries(ctx, series.ID)
	require.NoError(t, err)
	assert.Equal(t, effective.String(), capped.ValidUntil)
	rootID := series.ID
	successor, err := capability.CreateStaffShiftSeries(ctx, workforce.StaffShiftSeries{
		StaffID: staff.ID, Weekdays: []int{1}, StartTime: "10:00:00", EndTime: "13:00:00",
		CalendarPeriodID: period.ID, ValidFrom: effective.String(), SeriesRootID: &rootID, CreatedBy: staff.ID,
	})
	require.NoError(t, err)
	regenerable, err := capability.DeleteRegenerableSeriesShifts(ctx, series.ID, effective.String())
	require.NoError(t, err)
	assert.EqualValues(t, 1, regenerable, "only the non-detached occurrence on or after the effective date goes")
	moved, err := capability.RepointDetachedSeriesShifts(ctx, series.ID, successor.ID, firstMonday.String())
	require.NoError(t, err)
	assert.EqualValues(t, 1, moved)
	repointed, err := capability.RepointSeriesExceptions(ctx, series.ID, successor.ID, effective.String())
	require.NoError(t, err)
	assert.EqualValues(t, 1, repointed)
	successorDates, err := capability.SeriesExceptionDates(ctx, successor.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{secondMonday.String()}, successorDates)
	detachedNow, err := capability.FindStaffShift(ctx, detached.ID)
	require.NoError(t, err)
	require.NotNil(t, detachedNow.SeriesID)
	assert.Equal(t, successor.ID, *detachedNow.SeriesID)

	next, err := capability.FindOverlappingSeriesInLineage(ctx, rootID, series.ID, effective.String())
	require.NoError(t, err)
	assert.Equal(t, successor.ID, next.ID)
	_, err = capability.FindOverlappingSeriesInLineage(ctx, rootID, successor.ID, effective.String())
	require.ErrorIs(t, err, workforce.ErrShiftSeriesNotFound, "the capped predecessor is no longer active there")

	// Offboarding bounds every segment of the staff member; a cap before
	// valid_from clamps to valid_from instead of violating the check.
	cappedAll, err := capability.CapStaffShiftSeriesForStaff(ctx, staff.ID, periodStart.String())
	require.NoError(t, err)
	assert.EqualValues(t, 2, cappedAll)
	clamped, err := capability.FindStaffShiftSeries(ctx, successor.ID)
	require.NoError(t, err)
	assert.Equal(t, effective.String(), clamped.ValidUntil)

	successor.Notes = "Nachfolger"
	updated, err := capability.UpdateStaffShiftSeries(ctx, successor)
	require.NoError(t, err)
	assert.Equal(t, "Nachfolger", updated.Notes)
	require.NoError(t, capability.DeleteStaffShiftSeries(ctx, successor.ID))
	_, err = capability.FindStaffShiftSeries(ctx, successor.ID)
	require.ErrorIs(t, err, workforce.ErrShiftSeriesNotFound)
}

func TestShiftTypesAreTenantIsolatedAndNormalized(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	capability := buildWorkforce(t, db)

	zebra, err := capability.CreateShiftType(ctx, workforce.ShiftType{Name: " Zebra ", Color: "83cd2d", IsActive: true})
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), zebra.TenantID)
	assert.Equal(t, "Zebra", zebra.Name, "the name is trimmed")
	assert.Equal(t, "#83CD2D", zebra.Color, "the color is normalized to upper-case hex with a hash")
	alpha, err := capability.CreateShiftType(ctx, workforce.ShiftType{Name: "Alpha", Color: "#5080d8", IsActive: true})
	require.NoError(t, err)
	foreignType, err := capability.CreateShiftType(foreignCtx, workforce.ShiftType{Name: "Zebra", Color: "#5080D8", IsActive: true})
	require.NoError(t, err, "the same name may exist in another tenant")

	_, err = capability.CreateShiftType(ctx, workforce.ShiftType{Name: "zebra", Color: "#5080D8", IsActive: true})
	require.ErrorIs(t, err, workforce.ErrShiftTypeNameTaken, "the per-tenant name index is case-insensitive")
	_, err = capability.CreateShiftType(ctx, workforce.ShiftType{Name: "Bad", Color: "not-a-color"})
	require.ErrorIs(t, err, workforce.ErrInvalidShiftType)

	listed, err := capability.ListShiftTypes(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 2, "the foreign type is invisible")
	assert.Equal(t, alpha.ID, listed[0].ID, "types are listed by name")
	assert.Equal(t, zebra.ID, listed[1].ID)

	// The idempotent seed insert wins once and is a clean no-op afterwards,
	// leaving the original row untouched.
	seeded, created, err := capability.CreateShiftTypeIfAbsent(ctx, workforce.ShiftType{Name: "Pause", Color: "#6B7280", IsActive: true})
	require.NoError(t, err)
	assert.True(t, created)
	require.NotZero(t, seeded.ID)
	_, createdAgain, err := capability.CreateShiftTypeIfAbsent(ctx, workforce.ShiftType{Name: "PAUSE", Color: "#000000", IsActive: true})
	require.NoError(t, err)
	assert.False(t, createdAgain)
	pause, err := capability.FindShiftType(ctx, seeded.ID)
	require.NoError(t, err)
	assert.Equal(t, "#6B7280", pause.Color)

	zebra.Name = "Zebra neu"
	zebra.Color = "#f78c10"
	zebra.IsActive = false
	updated, err := capability.UpdateShiftType(ctx, zebra)
	require.NoError(t, err)
	assert.Equal(t, "#F78C10", updated.Color)
	assert.False(t, updated.IsActive, "an explicit false is written, not the column default")

	_, err = capability.FindShiftType(ctx, foreignType.ID)
	require.ErrorIs(t, err, workforce.ErrShiftTypeNotFound)
	foreignType.Name = "Umbenannt"
	_, err = capability.UpdateShiftType(ctx, foreignType)
	require.ErrorIs(t, err, workforce.ErrShiftTypeNotFound)
	require.NoError(t, capability.DeleteShiftType(ctx, foreignType.ID))
	unchanged, err := capability.FindShiftType(foreignCtx, foreignType.ID)
	require.NoError(t, err)
	assert.Equal(t, "Zebra", unchanged.Name)

	// Deleting a type keeps the shifts that carried it and clears the label.
	staff := testpkg.CreateTestStaff(t, db, "Type", "Bearer")
	shift := testShift(staff.ID, timezone.NewDate(2026, 7, 6), "08:00:00", "12:00:00")
	shift.ShiftTypeID = &alpha.ID
	labelled, err := capability.CreateStaffShift(ctx, shift)
	require.NoError(t, err)
	require.NoError(t, capability.DeleteShiftType(ctx, alpha.ID))
	unlabelled, err := capability.FindStaffShift(ctx, labelled.ID)
	require.NoError(t, err)
	assert.Nil(t, unlabelled.ShiftTypeID)
}

// A plan change composed by a caller is one unit of work: when the caller's
// transaction fails after the series, its occurrences and an exception were
// written, none of them survive, and the identical retry succeeds.
func TestSeriesPlanRollsBackWithTheCallerTransaction(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Rollback", "Series")
	periodStart := timezone.NewDate(2026, 9, 1)
	period := testpkg.CreateTestCalendarPeriod(t, db, "Rollbackjahr", periodStart, timezone.NewDate(2027, 6, 30))
	capability := buildWorkforce(t, db)
	monday := timezone.NewDate(2026, 9, 7)
	failure := errors.New("materialization aborted after an authoritative write")

	// plan performs the three authoritative writes of a series materialization
	// and, when failAfter names one of them, fails right after that write.
	plan := func(txCtx context.Context, failAfter string) (int64, error) {
		series, err := capability.CreateStaffShiftSeries(txCtx, workforce.StaffShiftSeries{
			StaffID: staff.ID, Weekdays: []int{1}, StartTime: "09:00:00", EndTime: "12:00:00",
			CalendarPeriodID: period.ID, ValidFrom: periodStart.String(), CreatedBy: staff.ID,
		})
		if err != nil {
			return 0, err
		}
		if failAfter == "series" {
			return series.ID, failure
		}
		shift := testShift(staff.ID, monday, "09:00:00", "12:00:00")
		shift.SeriesID = &series.ID
		shift.SeriesOccurrenceDate = monday.String()
		if _, err := capability.CreateStaffShifts(txCtx, []workforce.StaffShift{shift}); err != nil {
			return 0, err
		}
		if failAfter == "shifts" {
			return series.ID, failure
		}
		exception := workforce.StaffShiftSeriesException{SeriesID: series.ID, Date: monday.AddDays(7).String(), CreatedBy: staff.ID}
		if err := capability.RecordSeriesException(txCtx, exception); err != nil {
			return 0, err
		}
		if failAfter == "exception" {
			return series.ID, failure
		}
		return series.ID, nil
	}

	// Injecting the failure after each authoritative write proves that none
	// of the earlier writes survive.
	for _, failAfter := range []string{"series", "shifts", "exception"} {
		var failedSeriesID int64
		err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
			id, err := plan(txCtx, failAfter)
			failedSeriesID = id
			return err
		})
		require.ErrorIs(t, err, failure, failAfter)
		require.NotZero(t, failedSeriesID, "the writes happened inside the transaction before the failure")

		_, err = capability.FindStaffShiftSeries(ctx, failedSeriesID)
		require.ErrorIs(t, err, workforce.ErrShiftSeriesNotFound, "the series must not survive the rollback after "+failAfter)
		orphaned, err := capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{StaffID: staff.ID, From: monday.String(), To: monday.String()})
		require.NoError(t, err)
		assert.Empty(t, orphaned, "the materialized occurrence must not survive the rollback after "+failAfter)
		var exceptions int
		require.NoError(t, db.NewSelect().TableExpr("schedule.staff_shift_series_exceptions").ColumnExpr("COUNT(*)").
			Where("series_id = ?", failedSeriesID).Scan(ctx, &exceptions))
		assert.Zero(t, exceptions, "the exception must not survive the rollback after "+failAfter)
	}

	var retriedSeriesID int64
	err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		id, err := plan(txCtx, "")
		retriedSeriesID = id
		return err
	})
	require.NoError(t, err)
	retried, err := capability.FindStaffShiftSeries(ctx, retriedSeriesID)
	require.NoError(t, err)
	assert.Equal(t, staff.ID, retried.StaffID)
	occurrences, err := capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{SeriesID: retriedSeriesID})
	require.NoError(t, err)
	require.Len(t, occurrences, 1)
	dates, err := capability.SeriesExceptionDates(ctx, retriedSeriesID)
	require.NoError(t, err)
	assert.Len(t, dates, 1)
	assert.WithinDuration(t, time.Now(), retried.CreatedAt, time.Minute)
}

func TestStaffShiftSickAbsencePreservesOtherFieldsAndRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Sick", "Association")
	absenceType := testpkg.CreateTestStaffAbsenceType(t, db, "Shift association")
	absence := testpkg.CreateTestStaffAbsenceToday(t, db, staff.ID, absenceType.ID)
	capability := buildWorkforce(t, db)
	shift := testShift(staff.ID, timezone.NewDate(2026, 9, 7), "08:00:00", "16:00:00")
	shift.Notes = "Preserve notes and cancellation"
	shift.Cancelled = true
	created, err := capability.CreateStaffShift(ctx, shift)
	require.NoError(t, err)

	for _, invalidID := range []int64{0, -1} {
		_, err := capability.SetStaffShiftSickAbsence(ctx, created.ID, &invalidID)
		require.ErrorIs(t, err, workforce.ErrInvalidStaffShift)
	}

	for _, absenceID := range []*int64{&absence.ID, &absence.ID, nil, nil} {
		failure := errors.New("abort after sick-absence association")
		err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
			affected, err := capability.SetStaffShiftSickAbsence(txCtx, created.ID, absenceID)
			if err != nil {
				return err
			}
			require.EqualValues(t, 1, affected)
			return failure
		})
		require.ErrorIs(t, err, failure)
		afterRollback, err := capability.FindStaffShift(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created, afterRollback, "the caller rollback restores the complete row")

		affected, err := capability.SetStaffShiftSickAbsence(ctx, created.ID, absenceID)
		require.NoError(t, err)
		assert.EqualValues(t, 1, affected)
		created.SickAbsenceID = absenceID
		persisted, err := capability.FindStaffShift(ctx, created.ID)
		require.NoError(t, err)
		// The existing database trigger advances updated_at on every write.
		assert.False(t, persisted.UpdatedAt.Before(created.UpdatedAt))
		created.UpdatedAt = persisted.UpdatedAt
		assert.Equal(t, created, persisted, "only the sick-absence association and audit timestamp change")
	}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = capability.SetStaffShiftSickAbsence(cancelledCtx, created.ID, &absence.ID)
	require.ErrorIs(t, err, context.Canceled)
}
