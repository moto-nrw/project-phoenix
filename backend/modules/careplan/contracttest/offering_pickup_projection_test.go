package contracttest_test

import (
	"context"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"

	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func pickupTimeService(t *testing.T, env *decisionTestEnv) careplan.OfferingPickupTimes {
	t.Helper()
	svc, ok := env.bookings.(careplan.OfferingPickupTimes)
	require.True(t, ok, "Care Plan's booking materialization must implement OfferingPickupTimes")
	return svc
}

func createPickupTimeOffering(
	t *testing.T,
	env *decisionTestEnv,
	name string,
	days []string,
	times map[string]string,
) *offeringContract.CareOfferingRecord {
	t.Helper()
	offering := &offeringContract.CareOfferingRecord{
		PhaseID:        env.sourcePhase.ID,
		Name:           uniqueSchemaName(name + "-" + t.Name()),
		DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
		AvailableDays:  days,
		PickupTimes:    times,
		IsActive:       true,
	}
	offering.TenantID = testpkg.Tenant(t)
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Create(testpkg.Ctx(t), offering))
	return offering
}

func projectedPickupReader(env *decisionTestEnv) careplan.PickupScheduleService {
	result, err := careplanCompose.NewPickupSchedules(env.db, env.repos.CarePlan(), newPickupBaselineService(env.repos.CarePlan(), approvedOfferingTestProjection(env.reposEnrollment, env.reposPeople)), nil, nil, slog.Default())
	if err != nil {
		panic(err)
	}
	return result
}

func nextWeekday(from calendar.Date, weekday time.Weekday) calendar.Date {
	date := from
	for date.Weekday() != weekday {
		date = date.AddDays(1)
	}
	return date
}

func TestOfferingPickupProjection_FutureBookingEndIsNotVisibleOnEffectiveDate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, calendar.NewDate(2026, 8, 24).AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createPickupTimeOffering(t, env, "gehzeit-future-end",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	studentID, childID := submitAndApproveOfferingChild(
		t, env, offering.ID, "gehzeit-future-end@example.com", "Ende", 2,
	)

	effectiveFrom := nextWeekday(calendar.NewDate(2026, 8, 24).AddDays(1), time.Monday)
	err := api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).ScheduleRequestChildOfferings(ctx, childID, enrollmentTest.Date(effectiveFrom), nil)
	require.NoError(t, err)

	reader := projectedPickupReader(env)
	before, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), reader.GetEffectivePickupTimeForDate, studentID, effectiveFrom.AddDays(-7))
	require.NoError(t, err)
	require.NotNil(t, before.PickupTime)
	assert.Equal(t, "14:30", before.PickupTime.Format("15:04"))

	atBoundary, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), reader.GetEffectivePickupTimeForDate, studentID, effectiveFrom)
	require.NoError(t, err)
	assert.Nil(t, atBoundary.PickupTime,
		"an offering pickup time must stop at the booking's exclusive valid_until")
	rangeData, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), reader.GetStudentPickupDataForRange, studentID, effectiveFrom.AddDays(-7), effectiveFrom)
	require.NoError(t, err)
	require.Len(t, rangeData.EffectiveSchedules, 8)
	require.NotNil(t, rangeData.EffectiveSchedules[0].Schedule)
	assert.Equal(t, offering.Name, rangeData.EffectiveSchedules[0].Schedule.CareOfferingName)
	assert.Nil(t, rangeData.EffectiveSchedules[7].Schedule,
		"an empty projected date must be explicit so clients cannot fall back to today's row")

	stored, err := careplantest.NewScheduleRecords(env.repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Empty(t, stored, "approval must not materialize an undated offering row")

	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Geschützt")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayMonday, author.ID, "15:15")
	err = pickupTimeService(t, env).ResetStudentPickupDayToOffering(ctx, studentID, effectiveFrom)
	require.ErrorIs(t, err, careplan.ErrPickupResetNoOffering)
	stored, err = careplantest.NewScheduleRecords(env.repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, "15:15", stored[0].PickupTime.Format("15:04"),
		"a reset without an offering replacement must preserve the manual row")
}

func TestOfferingPickupProjection_FutureReplacementStartsExactlyOnEffectiveDate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, calendar.NewDate(2026, 8, 24).AddDays(-30))
	ctx := testpkg.Ctx(t)

	oldOffering := createPickupTimeOffering(t, env, "gehzeit-switch-old",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	newOffering := createPickupTimeOffering(t, env, "gehzeit-switch-new",
		[]string{"mon"}, map[string]string{"mon": "16:00"})
	studentID, childID := submitAndApproveOfferingChild(
		t, env, oldOffering.ID, "gehzeit-switch@example.com", "Wechsel", 2,
	)

	effectiveFrom := nextWeekday(calendar.NewDate(2026, 8, 24).AddDays(1), time.Monday)
	require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).ScheduleRequestChildOfferings(
		ctx,
		childID,
		enrollmentTest.Date(effectiveFrom),
		[]*enrollmentTest.RequestChildOffering{{
			CareOfferingID: newOffering.ID,
			SelectedDays:   []string{"mon"},
		}},
	))

	reader := projectedPickupReader(env)
	before, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), reader.GetEffectivePickupTimeForDate, studentID, effectiveFrom.AddDays(-7))
	require.NoError(t, err)
	require.NotNil(t, before.PickupTime)
	assert.Equal(t, "14:30", before.PickupTime.Format("15:04"))

	atBoundary, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), reader.GetEffectivePickupTimeForDate, studentID, effectiveFrom)
	require.NoError(t, err)
	require.NotNil(t, atBoundary.PickupTime)
	assert.Equal(t, "16:00", atBoundary.PickupTime.Format("15:04"))
	weekly, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), reader.GetWeeklySchedulesByStudentIDsForDate, []int64{studentID}, effectiveFrom)
	require.NoError(t, err)
	require.Len(t, weekly, 1)
	assert.Equal(t, newOffering.Name, weekly[0].CareOfferingName)

	rangeData, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), reader.GetStudentPickupDataForRange, studentID, effectiveFrom.AddDays(-7), effectiveFrom)
	require.NoError(t, err)
	require.Len(t, rangeData.EffectiveSchedules, 8)
	assert.Equal(t, oldOffering.Name, rangeData.EffectiveSchedules[0].Schedule.CareOfferingName)
	assert.Equal(t, newOffering.Name, rangeData.EffectiveSchedules[7].Schedule.CareOfferingName)

	futureWeek, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), reader.GetStudentPickupDataForRange, studentID, effectiveFrom, effectiveFrom.AddDays(4))
	require.NoError(t, err)
	require.Len(t, futureWeek.Schedules, 1)
	assert.Equal(t, newOffering.Name, futureWeek.Schedules[0].CareOfferingName,
		"the weekly editor must receive the offering value for its requested week")

	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Zukunft")
	require.NoError(t, reader.UpsertBulkStudentPickupSchedulesForDate(ctx, studentID, effectiveFrom, []*careplan.PickupSchedule{{
		StudentID: studentID, Weekday: timetable.WeekdayMonday,
		PickupTime: calendar.NormalizeWallClock(time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC)), CreatedBy: author.ID,
	}}))
	stored, err := careplantest.NewScheduleRecords(env.repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Empty(t, stored, "a future offering value must not be saved as a staff override")
}

func TestOfferingPickupProjection_StaffOverrideSurvivesOfferingEdit(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-1))
	ctx := testpkg.Ctx(t)

	offering := createPickupTimeOffering(t, env, "gehzeit-manual-override",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "gehzeit-manual@example.com", "Override", 2,
	)
	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Manuell")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayMonday, author.ID, "15:15")
	offering.PickupTimes = map[string]string{"mon": "16:00"}
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, offering))

	monday := nextWeekday(decisionTestToday, time.Monday)
	actual, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), projectedPickupReader(env).GetEffectivePickupTimeForDate, studentID, monday)
	require.NoError(t, err)
	require.NotNil(t, actual.PickupTime)
	assert.Equal(t, "15:15", actual.PickupTime.Format("15:04"))

	require.NoError(t, pickupTimeService(t, env).ResetStudentPickupDayToOffering(ctx, studentID, monday))
	stored, err := careplantest.NewScheduleRecords(env.repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Empty(t, stored, "reset must delete the manual row when an offering pickup replaces it")
	reset, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), projectedPickupReader(env).GetEffectivePickupTimeForDate, studentID, monday)
	require.NoError(t, err)
	require.NotNil(t, reset.PickupTime)
	assert.Equal(t, "16:00", reset.PickupTime.Format("15:04"))
}

func TestOfferingPickupResetClearsManualWeekdayExtension(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-1))
	ctx := testpkg.Ctx(t)
	offering := createPickupTimeOffering(t, env, "gehzeit-reset-extension",
		[]string{"mon"}, map[string]string{"mon": "16:00"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "gehzeit-reset-extension@example.com", "Aufgabe", 2,
	)
	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Aufgabe")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayMonday, author.ID, "16:00")
	monday := nextWeekday(decisionTestToday, time.Monday)

	var clearedStudentID int64
	var clearedWeekday int
	reset := newBookingMaterialization(t, env.db,
		api.WithBookingCatalog(env.offeringCatalog),
		api.WithBookingPickupWeekdayExtension(func(_ context.Context, gotStudentID int64, gotWeekday int) error {
			clearedStudentID, clearedWeekday = gotStudentID, gotWeekday
			return nil
		}),
	).Bookings
	require.NoError(t, reset.ResetStudentPickupDayToOffering(ctx, studentID, monday))
	assert.Equal(t, studentID, clearedStudentID)
	assert.Equal(t, timetable.WeekdayMonday, clearedWeekday)
}

func TestOfferingPickupProjection_IgnoresInactiveOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-1))
	ctx := testpkg.Ctx(t)

	offering := createPickupTimeOffering(t, env, "gehzeit-inaktiv",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "gehzeit-inaktiv@example.com", "Inaktiv", 2,
	)
	offering.IsActive = false
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, offering))

	pickup, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), projectedPickupReader(env).GetEffectivePickupTimeForDate, studentID, nextWeekday(decisionTestToday, time.Monday))
	require.NoError(t, err)
	assert.Nil(t, pickup.PickupTime)
}

func TestOfferingPickupProjection_IgnoresNonCareOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-1))
	ctx := testpkg.Ctx(t)

	offering := createPickupTimeOffering(t, env, "gehzeit-keine-betreuung",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "gehzeit-keine-betreuung@example.com", "Keine Betreuung", 2,
	)
	offering.CountsAsCare = false
	offering.CountsAsCareSet = true
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, offering))

	pickup, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), projectedPickupReader(env).GetEffectivePickupTimeForDate, studentID, nextWeekday(decisionTestToday, time.Monday))
	require.NoError(t, err)
	assert.Nil(t, pickup.PickupTime)
}

func TestOfferingPickupProjection_ResetWaitsForOfferingSourceGate(t *testing.T) {
	t.Parallel()
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))

	offering := createPickupTimeOffering(t, env, "gehzeit-reset-gate",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "gehzeit-reset-gate@example.com", "Sperre", 2,
	)
	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Sperre")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayMonday, author.ID, "15:15")
	monday := nextWeekday(decisionTestToday, time.Monday)

	resetter := newBookingsForTest(env, nil, func(ctx context.Context) error {
		return owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, env.db)).LockRecurrenceWrites(ctx)
	}, nil)
	releaseHolder, holderDone := holdOfferingSourceGate(t, env)
	resetDone := startPickupReset(t, env, resetter, studentID, monday)
	assertPickupResetBlocked(t, resetDone, releaseHolder, holderDone)
	close(releaseHolder)
	require.NoError(t, <-holderDone)
	require.NoError(t, <-resetDone)
	stored, err := careplantest.NewScheduleRecords(env.repos.CarePlan()).StudentPickupSchedule.FindByStudentID(testpkg.Ctx(t), studentID)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

func holdOfferingSourceGate(t *testing.T, env *decisionTestEnv) (chan struct{}, chan error) {
	t.Helper()
	acquired, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, testpkg.Ctx(t), env.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
			if err := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, env.db)).LockRecurrenceWrites(ctx); err != nil {
				return err
			}
			close(acquired)
			<-release
			return nil
		})
	}()
	select {
	case <-acquired:
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out acquiring offering source gate")
	}
	return release, done
}

func startPickupReset(t *testing.T, env *decisionTestEnv, resetter careplan.OfferingPickupTimes, studentID int64, date calendar.Date) chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, testpkg.Ctx(t), env.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
			return resetter.ResetStudentPickupDayToOffering(ctx, studentID, date)
		})
	}()
	return done
}

func assertPickupResetBlocked(t *testing.T, resetDone chan error, releaseHolder chan struct{}, holderDone chan error) {
	t.Helper()
	select {
	case err := <-resetDone:
		close(releaseHolder)
		require.NoError(t, <-holderDone)
		require.FailNow(t, "pickup reset bypassed offering source gate", "returned early with %v", err)
	case <-time.After(200 * time.Millisecond):
	}
}
