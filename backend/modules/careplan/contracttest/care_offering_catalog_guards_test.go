package contracttest_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The guards the Care Plan catalog (#3559) offers the owners of calendar
// periods, rooms and phases, driven through the catalog composed
// over the test database as the server binds it.

type calendarPeriodValidationFixture struct {
	db       *bun.DB
	tenantID int64
	ctx      context.Context
	phase    *enrollmentTest.Phase
	catalog  careplan.CareOfferingCatalogCapability
}

func newCalendarPeriodValidationFixture(t *testing.T) *calendarPeriodValidationFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := tenant.WithTenantID(testpkg.Ctx(t), tenantID)
	_, _, reposEnrollment := testRepositories(t, db)

	phase := &enrollmentTest.Phase{
		Name:             fmt.Sprintf("calendar-period-validation-%d", time.Now().UnixNano()),
		Kind:             enrollmentTest.PhaseKindSchoolYear,
		ServiceStartDate: enrollmentTest.Date(calendar.NewDate(2026, time.September, 1)),
		ServiceEndDate:   enrollmentTest.Date(calendar.NewDate(2027, time.July, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentTest.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = tenantID
	require.NoError(t, insertOwnerPhaseForTest(ctx, reposEnrollment.Enrollment(), phase))

	return &calendarPeriodValidationFixture{
		db:       db,
		tenantID: tenantID,
		ctx:      ctx,
		phase:    phase,
		catalog:  testCareOfferingCatalog(t, db),
	}
}

func (f *calendarPeriodValidationFixture) createLinkedTemplate(
	t *testing.T,
	templatePeriodID *int64,
	schedulePeriodID *int64,
) (*timetable.Group, *offeringContract.CareOfferingRecord) {
	t.Helper()
	category := testpkg.CreateTestActivityCategoryForTenant(
		t,
		f.db,
		f.tenantID,
		fmt.Sprintf("calendar-period-validation-%d", time.Now().UnixNano()),
	)
	room := testpkg.CreateTestRoomForTenant(
		t,
		f.db,
		f.tenantID,
		fmt.Sprintf("calendar-period-validation-%d", time.Now().UnixNano()),
	)
	group := &timetable.Group{
		Name:             fmt.Sprintf("calendar-period-validation-%d", time.Now().UnixNano()),
		MaxParticipants:  20,
		IsOpen:           true,
		CategoryID:       category.ID,
		Type:             timetable.GroupTypeCare,
		IsTemplate:       true,
		PlannedRoomID:    &room.ID,
		CalendarPeriodID: templatePeriodID,
	}
	group.TenantID = f.tenantID
	timetableRepos := testTimetableRepositories(f.db)
	require.NoError(t, timetableCompose.NativeGroupRecords(timetableRepos.Timetable).Create(f.ctx, group))
	timeframe := testpkg.CreateTestTimeframeForTenant(
		t,
		f.db,
		f.tenantID,
		fmt.Sprintf("calendar-period-validation-%d", time.Now().UnixNano()),
	)

	schedule := &timetable.Schedule{
		Weekday:          timetable.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  group.ID,
		WeekPattern:      0,
		CalendarPeriodID: schedulePeriodID,
	}
	schedule.TenantID = f.tenantID
	require.NoError(t, timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).Create(f.ctx, schedule))

	offering := &offeringContract.CareOfferingRecord{
		PhaseID:         f.phase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Linked Template",
		DaysOfWeekMode:  offeringContract.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon"},
		PickupTimes:     carePickupTimes("mon"),
		IsActive:        true,
	}
	offering.TenantID = f.tenantID
	created, err := newScopedOfferingRows(f.catalog).Create(f.ctx, offering)
	require.NoError(t, err)
	return group, created
}

func (f *calendarPeriodValidationFixture) selectOfferingForSubmittedChild(
	t *testing.T,
	offeringID int64,
) {
	t.Helper()
	suffix := time.Now().UnixNano()
	var requestID int64
	require.NoError(t, f.db.NewRaw(`
		INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name,
			 guardian_email, consent_flags, custom_data, status_token, submitted_at)
		VALUES (?, ?, 'Anna', 'Auswahl', ?, '{}'::jsonb, '{}'::jsonb, ?, NOW())
		RETURNING id
	`, f.tenantID, f.phase.ID,
		fmt.Sprintf("calendar-period-selection-%d@example.test", suffix),
		fmt.Sprintf("calendar-period-selection-%d", suffix),
	).Scan(f.ctx, &requestID))

	var childID int64
	require.NoError(t, f.db.NewRaw(`
		INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth,
			 status, activation_mode, sort_order, custom_data)
		VALUES (?, ?, 'Lina', 'Auswahl', '2018-04-15',
			'submitted', 'scheduled', 0, '{}'::jsonb)
		RETURNING id
	`, f.tenantID, requestID).Scan(f.ctx, &childID))

	row := &enrollmentTest.RequestChildOffering{
		RequestChildID: childID,
		CareOfferingID: offeringID,
		SelectedDays:   []string{"mon"},
	}
	require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).InsertRequestChildOffering(f.ctx, row))
}

// validate runs the validation inside a tenant transaction, which is how it
// runs in production: the sweep it performs (ListByTenant and friends) has no
// tenant_id filter of its own and relies on RLS to narrow it. Called with a
// plain tenant context — as these tests did while per-row teardowns removed
// every other test's rows — it sees the care offerings of the whole clone
// (#2419).
func (f *calendarPeriodValidationFixture) validate(
	t *testing.T,
	periodID int64,
	replacement *careplan.CalendarPeriodReplacement,
) error {
	t.Helper()
	var validationErr error
	require.NoError(t, testpkg.WithTenantTx(t, f.ctx, f.db, f.tenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			validationErr = f.catalog.ValidateCalendarPeriodChange(txCtx, periodID, replacement)
			return nil
		}))
	return validationErr
}

func TestCareOfferingCalendarPeriodValidation_RejectsRangeUpdateAndDelete(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	fixture := newCalendarPeriodValidationFixture(t)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-mutation", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	fixture.createLinkedTemplate(t, &period.ID, nil)

	replacement := api.CalendarPeriodReplacementOf(*period)
	replacement.EndDate = calendar.Date(fixture.phase.ServiceEndDate).AddDays(-1)

	err := fixture.validate(t, period.ID, replacement)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	assert.ErrorIs(t, err, careplan.ErrCareOfferingTemplatePeriodMismatch)

	err = fixture.validate(t, period.ID, nil)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)

	replacement = api.CalendarPeriodReplacementOf(*period)
	replacement.IsActive = false
	err = fixture.validate(t, period.ID, replacement)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
}

func TestCareOfferingCalendarPeriodValidation_RejectsWeekCycleCoverageGap(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	fixture := newCalendarPeriodValidationFixture(t)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-cycle-change", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	group, _ := fixture.createLinkedTemplate(t, &period.ID, nil)

	schedules, err := testActivityScheduleRepository(t, fixture.db).FindByGroupID(fixture.ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	schedules[0].WeekPattern = 1
	require.NoError(t, testActivityScheduleRepository(t, fixture.db).Update(fixture.ctx, schedules[0]))

	replacement := api.CalendarPeriodReplacementOf(*period)
	anchor := calendar.NewDate(2026, time.August, 31) // Monday, week A
	replacement.WeekCycleLength = 2
	replacement.WeekCycleAnchor = string(anchor)
	err = fixture.validate(t, period.ID, replacement)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	assert.ErrorContains(t, err, "does not cover")
}

func TestCareOfferingCalendarPeriodValidation_ProtectsInactiveReferencedOffering(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	fixture := newCalendarPeriodValidationFixture(t)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-inactive-referenced", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	_, offering := fixture.createLinkedTemplate(t, &period.ID, nil)
	fixture.selectOfferingForSubmittedChild(t, offering.ID)
	offering.IsActive = false
	require.NoError(t, newCareOfferingFixtureRecords(api.NewRepositoryTestFactory(testpkg.NewDatabase(t, fixture.db)).CarePlan()).Update(fixture.ctx, offering))

	replacement := api.CalendarPeriodReplacementOf(*period)
	replacement.IsActive = false
	err := fixture.validate(t, period.ID, replacement)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
}

func TestCareOfferingCalendarPeriodValidation_ProtectsNonOverlappingLinkedRootDelete(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	fixture := newCalendarPeriodValidationFixture(t)
	rootPeriod := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-linked-root", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	successorPeriod := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-linked-successor", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	root, _ := fixture.createLinkedTemplate(t, &rootPeriod.ID, nil)
	timetableRepos := testTimetableRepositories(fixture.db)

	rootSchedules, err := timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).FindByGroupID(fixture.ctx, root.ID)
	require.NoError(t, err)
	require.Len(t, rootSchedules, 1)
	rootSchedules[0].ValidUntil = activityDatePtr(&fixture.phase.ServiceStartDate)
	require.NoError(t, timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).Update(fixture.ctx, rootSchedules[0]))

	seriesRootID := root.ID
	successor := &timetable.Group{
		Name:             fmt.Sprintf("calendar-period-successor-%d", time.Now().UnixNano()),
		MaxParticipants:  root.MaxParticipants,
		IsOpen:           true,
		CategoryID:       root.CategoryID,
		Type:             root.Type,
		IsTemplate:       true,
		SeriesRootID:     &seriesRootID,
		CalendarPeriodID: &successorPeriod.ID,
	}
	successor.TenantID = fixture.tenantID
	require.NoError(t, timetableCompose.NativeGroupRecords(timetableRepos.Timetable).Create(fixture.ctx, successor))
	successorSchedule := &timetable.Schedule{
		Weekday:          timetable.WeekdayMonday,
		ActivityGroupID:  successor.ID,
		WeekPattern:      0,
		CalendarPeriodID: &successorPeriod.ID,
		ValidFrom:        activityDatePtr(&fixture.phase.ServiceStartDate),
	}
	successorSchedule.TenantID = fixture.tenantID
	require.NoError(t, timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).Create(fixture.ctx, successorSchedule))

	err = fixture.validate(t, rootPeriod.ID, nil)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
}

func TestCareOfferingCalendarPeriodValidation_AllowsCompatibleFallbacks(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	t.Run("period update still contains phase", func(t *testing.T) {
		fixture := newCalendarPeriodValidationFixture(t)
		period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-compatible-update", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
		fixture.createLinkedTemplate(t, &period.ID, nil)

		// The rename of the old test is not part of the catalog's replacement
		// (the name never reached the guard); the unchanged range must pass.
		replacement := api.CalendarPeriodReplacementOf(*period)
		err := fixture.validate(t, period.ID, replacement)
		require.NoError(t, err)
	})

	t.Run("schedule override survives template fallback deletion", func(t *testing.T) {
		fixture := newCalendarPeriodValidationFixture(t)
		deletedPeriod := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-deleted-fallback", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
		overridePeriod := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-schedule-override", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
		fixture.createLinkedTemplate(t, &deletedPeriod.ID, &overridePeriod.ID)

		err := fixture.validate(t, deletedPeriod.ID, nil)
		require.NoError(t, err)
	})

	t.Run("template fallback replaces deleted schedule pin", func(t *testing.T) {
		fixture := newCalendarPeriodValidationFixture(t)
		deletedPeriod := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-deleted-schedule-pin", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
		fallbackPeriod := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, fixture.db), fixture.ctx, fixture.tenantID, fmt.Sprintf("%s-%d", "care-period-template-fallback", time.Now().UnixNano()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
		fixture.createLinkedTemplate(t, &fallbackPeriod.ID, &deletedPeriod.ID)

		err := fixture.validate(t, deletedPeriod.ID, nil)
		require.NoError(t, err)
	})
}

// setupCareGuardTest replaces the deleted setupCareTest of the old catalog
// suite for the materializability tests: a catalog over the test database,
// its row adapter and a school-year phase of the test tenant.
func setupCareGuardTest(t *testing.T) (*bun.DB, *scopedCareOfferingRows, careplan.CareOfferingCatalogCapability, *enrollmentTest.Phase, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	repoFactory := api.NewEnrollmentSuiteRepositories(testpkg.NewDatabase(t, db))
	catalog := testCareOfferingCatalog(t, db,
		api.WithCareOfferingPickupResync(&pickupResyncer{}))

	phase := &enrollmentTest.Phase{
		Name:             uniqueSchemaName("phase-" + t.Name()),
		Kind:             enrollmentTest.PhaseKindSchoolYear,
		ServiceStartDate: enrollmentTest.Date(calendar.NewDate(2026, 9, 1)),
		ServiceEndDate:   enrollmentTest.Date(calendar.NewDate(2027, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentTest.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = testpkg.Tenant(t)
	require.NoError(t, insertOwnerPhaseForTest(testpkg.Ctx(t), repoFactory.Enrollment(), phase))

	cleanup := func() {
		bg := context.Background()
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("phase_id = ?", phase.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("id = ?", phase.ID).
			Exec(bg)
	}

	return db, newScopedOfferingRows(catalog), catalog, phase, cleanup
}

func baseGuardLinkedOffering(t *testing.T, phaseID int64, groupID int64) *offeringContract.CareOfferingRecord {
	offering := &offeringContract.CareOfferingRecord{
		PhaseID:         phaseID,
		ActivityGroupID: &groupID,
		Name:            "Linked Template",
		DaysOfWeekMode:  offeringContract.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon"},
		PickupTimes:     map[string]string{"mon": "14:30"},
		IsActive:        true,
	}
	offering.TenantID = testpkg.Tenant(t)
	return offering
}

func createCareMaterializationSchedule(
	t *testing.T,
	db *bun.DB,
	groupID int64,
	periodID int64,
	endTime *time.Time,
) (*timetable.Timeframe, *timetable.Schedule) {
	t.Helper()
	start := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC))
	timeframe := &timetable.Timeframe{
		StartTime:   start.Format("15:04:05"),
		EndTime:     activityClockPtr(endTime),
		IsActive:    true,
		Description: "care materializability",
	}
	timeframe.TenantID = testpkg.Tenant(t)
	timetableRepos := testTimetableRepositories(db)
	require.NoError(t, timetableCompose.NativeTimeframeRecords(timetableRepos.Timeframe).Create(testpkg.Ctx(t), timeframe))
	schedule := &timetable.Schedule{
		Weekday:          timetable.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  groupID,
		CalendarPeriodID: &periodID,
	}
	schedule.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).Create(testpkg.Ctx(t), schedule))
	return timeframe, schedule
}

func setCareTestPhaseWindow(
	t *testing.T,
	db *bun.DB,
	phase *enrollmentTest.Phase,
	start, end calendar.Date,
) {
	t.Helper()
	phase.ServiceStartDate = enrollmentTest.Date(start)
	phase.ServiceEndDate = enrollmentTest.Date(end)
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), testpkg.SetupTestDB(t), api.NewEnrollmentSuiteRepositories(testpkg.NewDatabase(t, db)).Enrollment().UpdatePhase, ownerPhaseForTest(phase)))
}

func createCareMaterializationException(
	t *testing.T,
	db *bun.DB,
	exception *timetable.ActivityException,
) {
	t.Helper()
	exception.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeExceptionRecords(testTimetableRepositories(db).ActivityException).Create(testpkg.Ctx(t), exception))
}

func TestCareOfferingMaterializability_RejectsIncompleteTimeframeAndRoom(t *testing.T) {
	t.Parallel()

	t.Run("missing timeframe", func(t *testing.T) {
		db, svc, _, phase, cleanup := setupCareGuardTest(t)
		defer cleanup()
		period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("missing-timeframe"+"-"+
			t.Name()),

			calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
		group := createCareOfferingTemplateGroup(t, db, "missing-timeframe")
		schedule := &timetable.Schedule{
			Weekday: timetable.WeekdayMonday, ActivityGroupID: group.ID,
			CalendarPeriodID: &period.ID,
		}
		schedule.TenantID = testpkg.Tenant(t)
		require.NoError(t, testActivityScheduleRepository(t, db).Create(testpkg.Ctx(t), schedule))

		_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
		assert.ErrorContains(t, err, "no complete timeframe")
	})

	t.Run("open ended timeframe", func(t *testing.T) {
		db, svc, _, phase, cleanup := setupCareGuardTest(t)
		defer cleanup()
		period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("open-timeframe"+"-"+
			t.Name()),

			calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
		group := createCareOfferingTemplateGroup(t, db, "open-timeframe")
		createCareMaterializationSchedule(t, db, group.ID, period.ID, nil)

		_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
		assert.ErrorContains(t, err, "no complete timeframe")
	})

	t.Run("missing effective room", func(t *testing.T) {
		db, svc, _, phase, cleanup := setupCareGuardTest(t)
		defer cleanup()
		period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("missing-room"+"-"+
			t.Name()),

			calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
		group := createCareOfferingTemplateGroup(t, db, "missing-room")
		group.PlannedRoomID = nil
		require.NoError(t, timetableCompose.NativeGroupRecords(testTimetableRepositories(db).Timetable).Update(testpkg.Ctx(t), group))
		end := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
		createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)

		_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
		assert.ErrorContains(t, err, "no effective room")
	})
}

func TestCareOfferingMaterializability_ExceptionCannotRescueMissingTimeframe(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	monday := calendar.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("exception-no-timeframe"+"-"+
		t.Name()),

		calendar.NewDate(2026, 9, 1), calendar.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "exception-no-timeframe")
	group.PlannedRoomID = nil
	timetableRepos := testTimetableRepositories(db)
	require.NoError(t, timetableCompose.NativeGroupRecords(timetableRepos.Timetable).Update(testpkg.Ctx(t), group))
	schedule := &timetable.Schedule{
		Weekday:          timetable.WeekdayMonday,
		ActivityGroupID:  group.ID,
		CalendarPeriodID: &period.ID,
	}
	schedule.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).Create(testpkg.Ctx(t), schedule))

	overrideRoom := testpkg.CreateTestRoom(t, db, "Care no-timeframe override")
	start := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC))
	end := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationException(t, db, &timetable.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   string(string(careplan.Date(monday))),
		ExceptionType:   timetable.ActivityExceptionModified,
		StartTime:       activityClockPtr(&start),
		EndTime:         activityClockPtr(&end),
		RoomID:          &overrideRoom.ID,
	})

	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	assert.ErrorContains(t, err, "no complete timeframe",
		"date-specific time and room overrides must not fabricate the missing base timeframe")
}

func TestCareOfferingMaterializability_CancellationCannotFabricateRecurrence(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	monday := calendar.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("cancellation-no-recurrence"+"-"+
		t.Name()),

		calendar.NewDate(2026, 9, 1), calendar.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "cancellation-no-recurrence")
	end := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	timeframe, _ := createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	timetableRepos := testTimetableRepositories(db)
	schedules, err := timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).FindByGroupID(testpkg.Ctx(t), group.ID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	schedules[0].Weekday = timetable.WeekdayTuesday
	schedules[0].TimeframeID = &timeframe.ID
	require.NoError(t, timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).Update(testpkg.Ctx(t), schedules[0]))
	createCareMaterializationException(t, db, &timetable.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   string(string(careplan.Date(monday))),
		ExceptionType:   timetable.ActivityExceptionCancelled,
	})

	_, err = svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	assert.ErrorContains(t, err, "recurrence does not cover",
		"a cancellation suppresses a real occurrence; it cannot create one on another weekday")
}

func TestCareOfferingMaterializability_UsesDateSpecificExceptionRoom(t *testing.T) {
	t.Parallel()

	db, svc, catalog, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	firstMonday := calendar.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, firstMonday, firstMonday)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("exception-room"+"-"+
		t.Name()),

		calendar.NewDate(2026, 9, 1), calendar.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "exception-room")
	group.PlannedRoomID = nil
	timetableRepos := testTimetableRepositories(db)
	require.NoError(t, timetableCompose.NativeGroupRecords(timetableRepos.Timetable).Update(testpkg.Ctx(t), group))
	end := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	overrideRoom := testpkg.CreateTestRoom(t, db, "Care exception override")
	exception := &timetable.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   string(string(careplan.Date(firstMonday))),
		ExceptionType:   timetable.ActivityExceptionModified,
		RoomID:          &overrideRoom.ID,
	}
	exception.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeExceptionRecords(timetableRepos.ActivityException).Create(testpkg.Ctx(t), exception))

	created, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err, "the exact occurrence room override must satisfy materialization")

	secondMonday := firstMonday.AddDays(7)
	replacement := &careplan.OfferingPhase{
		ID:           phase.ID,
		Name:         phase.Name,
		ServiceStart: calendar.Date(phase.ServiceStartDate),
		ServiceEnd:   secondMonday,
	}
	err = catalog.ValidatePhaseChange(testpkg.Ctx(t), phase.ID, replacement)
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	assert.ErrorContains(t, err, secondMonday.String(),
		"phase service-window expansion must validate the newly exposed occurrence")

	setCareTestPhaseWindow(t, db, phase, firstMonday, secondMonday)
	err = svc.Update(testpkg.Ctx(t), created)
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	assert.ErrorContains(t, err, secondMonday.String(),
		"the first occurrence override must not become a series-wide room fallback")
}

func TestCareOfferingMaterializability_ExceptionIsScopedToSplitSeriesSegment(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	firstMonday := calendar.NewDate(2026, time.September, 7)
	secondMonday := firstMonday.AddDays(7)
	setCareTestPhaseWindow(t, db, phase, firstMonday, secondMonday)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("segment-exception-period"+"-"+
		t.Name()),

		calendar.NewDate(2026, 9, 1), calendar.NewDate(2026, 9, 30))
	root := createCareOfferingTemplateGroup(t, db, "segment-exception-root")
	successor := createCareOfferingTemplateGroup(t, db, "segment-exception-successor")
	root.PlannedRoomID = nil
	successor.PlannedRoomID = nil
	successor.SeriesRootID = &root.ID
	timetableRepos := testTimetableRepositories(db)
	require.NoError(t, timetableCompose.NativeGroupRecords(timetableRepos.Timetable).Update(testpkg.Ctx(t), root))
	require.NoError(t, timetableCompose.NativeGroupRecords(timetableRepos.Timetable).Update(testpkg.Ctx(t), successor))
	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, testpkg.Tenant(t), "segment exception")
	rootSchedule := &timetable.Schedule{
		Weekday:          timetable.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  root.ID,
		CalendarPeriodID: &period.ID,
		ValidUntil:       activityDatePtr(&secondMonday),
	}
	rootSchedule.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).Create(testpkg.Ctx(t), rootSchedule))
	successorSchedule := &timetable.Schedule{
		Weekday:          timetable.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  successor.ID,
		CalendarPeriodID: &period.ID,
		ValidFrom:        activityDatePtr(&secondMonday),
	}
	successorSchedule.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeScheduleRecords(timetableRepos.ActivitySchedule).Create(testpkg.Ctx(t), successorSchedule))

	overrideRoom := testpkg.CreateTestRoom(t, db, "Care segment exception override")
	for _, date := range []calendar.Date{firstMonday, secondMonday} {
		createCareMaterializationException(t, db, &timetable.ActivityException{
			ActivityGroupID: root.ID,
			ExceptionDate:   string(string(careplan.Date(date))),
			ExceptionType:   timetable.ActivityExceptionModified,
			RoomID:          &overrideRoom.ID,
		})
	}

	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, root.ID))
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	assert.ErrorContains(t, err, secondMonday.String())
	assert.ErrorContains(t, err, "no effective room",
		"a root exception on the successor date must not apply to the successor segment")
}

// inCareTenantTx runs fn inside a tenant transaction. The materializability
// validators sweep care offerings without a tenant_id filter of their own and
// rely on RLS to narrow the sweep; a plain tenant context leaves it open to
// the whole clone, which used to be invisible only because per-row teardowns
// removed every other test's offerings (#2419).
func inCareTenantTx(t *testing.T, db *bun.DB, fn func(ctx context.Context) error) error {
	t.Helper()
	var inner error
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t),
		func(txCtx context.Context, _ bun.Tx) error {
			inner = fn(txCtx)
			return nil
		}))
	return inner
}

func TestCareOfferingMaterializability_ValidatesRoomDeletion(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, svc, catalog, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("resource-change"+"-"+
		t.Name()),

		calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "resource-change")
	end := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err)

	err = inCareTenantTx(t, db, func(ctx context.Context) error {
		return catalog.ValidateRoomDeletion(ctx, *group.PlannedRoomID)
	})
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	assert.ErrorContains(t, err, "no effective room")
}

func TestCareOfferingMaterializability_CancellationDoesNotRequireRoom(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	monday := calendar.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("cancelled-room"+"-"+
		t.Name()),

		calendar.NewDate(2026, 9, 1), calendar.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "cancelled-room")
	group.PlannedRoomID = nil
	timetableRepos := testTimetableRepositories(db)
	require.NoError(t, timetableCompose.NativeGroupRecords(timetableRepos.Timetable).Update(testpkg.Ctx(t), group))
	end := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	exception := &timetable.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   string(string(careplan.Date(monday))),
		ExceptionType:   timetable.ActivityExceptionCancelled,
	}
	exception.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeExceptionRecords(timetableRepos.ActivityException).Create(testpkg.Ctx(t), exception))

	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err, "a genuine recurrence that is intentionally cancelled needs no room")
}

func TestCareOfferingMaterializability_RejectsInvalidEffectiveTimes(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	monday := calendar.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName("invalid-effective-time"+"-"+
		t.Name()),

		calendar.NewDate(2026, 9, 1), calendar.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "invalid-effective-time")
	end := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	invalidStart := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC))
	exception := &timetable.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   string(string(careplan.Date(monday))),
		ExceptionType:   timetable.ActivityExceptionModified,
		StartTime:       activityClockPtr(&invalidStart),
	}
	exception.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeExceptionRecords(testTimetableRepositories(db).ActivityException).Create(testpkg.Ctx(t), exception))

	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	assert.ErrorContains(t, err, "invalid effective start/end time")
}
