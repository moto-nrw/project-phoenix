package enrollment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	phaseFixture "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	serviceFixture "github.com/moto-nrw/project-phoenix/services/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The guards the Care Plan catalog (#3559) offers the owners of calendar
// periods, timeframes, rooms and phases, driven through the catalog composed
// over the test database as the server binds it.

// calendarPeriodReplacementOf describes a stored period as the catalog's
// proposed replacement.
func calendarPeriodReplacementOf(period scheduleModels.CalendarPeriod) *careplan.CalendarPeriodReplacement {
	replacement := &careplan.CalendarPeriodReplacement{
		StartDate:       calendar.Date(period.StartDate),
		EndDate:         calendar.Date(period.EndDate),
		IsActive:        period.IsActive,
		WeekCycleLength: period.WeekCycleLength,
	}
	if period.WeekCycleAnchor != nil {
		replacement.WeekCycleAnchor = string(*period.WeekCycleAnchor)
	}
	return replacement
}

// timeframeReplacementOf describes a stored timeframe as the catalog's
// proposed replacement with HH:MM:SS clock values.
func timeframeReplacementOf(timeframe scheduleModels.Timeframe) *careplan.TimeframeReplacement {
	replacement := &careplan.TimeframeReplacement{
		StartTime:   timeframe.StartTime.Format("15:04:05"),
		IsActive:    timeframe.IsActive,
		Description: timeframe.Description,
	}
	if timeframe.EndTime != nil {
		end := timeframe.EndTime.Format("15:04:05")
		replacement.EndTime = &end
	}
	return replacement
}

type calendarPeriodValidationFixture struct {
	db       *bun.DB
	tenantID int64
	ctx      context.Context
	phase    *phaseFixture.Phase
	catalog  careplan.CareOfferingCatalogCapability
}

func newCalendarPeriodValidationFixture(t *testing.T) *calendarPeriodValidationFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := tenant.WithTenantID(testpkg.Ctx(t), tenantID)
	repos := testRepositories(t, db)

	phase := &phaseFixture.Phase{
		Name:             fmt.Sprintf("calendar-period-validation-%d", time.Now().UnixNano()),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: phaseFixture.Date(timezone.NewDate(2026, time.September, 1)),
		ServiceEndDate:   phaseFixture.Date(timezone.NewDate(2027, time.July, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = tenantID
	require.NoError(t, enrollmentService.InsertOwnerPhaseForTest(ctx, repos.Enrollment(), phase))

	return &calendarPeriodValidationFixture{
		db:       db,
		tenantID: tenantID,
		ctx:      ctx,
		phase:    phase,
		catalog:  testCareOfferingCatalog(t, db),
	}
}

func (f *calendarPeriodValidationFixture) createPeriod(
	t *testing.T,
	name string,
) *scheduleModels.CalendarPeriod {
	t.Helper()
	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeCustom,
		StartDate:       scheduleModels.NewDate(2026, time.August, 1),
		EndDate:         scheduleModels.NewDate(2027, time.August, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(f.tenantID)
	require.NoError(t, repositories.NewFactory(f.db, repositories.NewUnobservedTimetableDependencies(f.db)).CalendarPeriod.Create(f.ctx, period))
	return period
}

func (f *calendarPeriodValidationFixture) createLinkedTemplate(
	t *testing.T,
	templatePeriodID *int64,
	schedulePeriodID *int64,
) (*activitiesModels.Group, *enrollmentModels.CareOffering) {
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
	group := &activitiesModels.Group{
		Name:             fmt.Sprintf("calendar-period-validation-%d", time.Now().UnixNano()),
		MaxParticipants:  20,
		IsOpen:           true,
		CategoryID:       category.ID,
		Type:             activitiesModels.GroupTypeCare,
		IsTemplate:       true,
		PlannedRoomID:    &room.ID,
		CalendarPeriodID: templatePeriodID,
	}
	group.SetTenantID(f.tenantID)
	repos := testRepositories(t, f.db)
	require.NoError(t, repos.ActivityGroup.Create(f.ctx, group))
	timeframe := testpkg.CreateTestTimeframeForTenant(
		t,
		f.db,
		f.tenantID,
		fmt.Sprintf("calendar-period-validation-%d", time.Now().UnixNano()),
	)

	schedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  group.ID,
		WeekPattern:      0,
		CalendarPeriodID: schedulePeriodID,
	}
	schedule.SetTenantID(f.tenantID)
	require.NoError(t, repos.ActivitySchedule.Create(f.ctx, schedule))

	offering := &enrollmentModels.CareOffering{
		PhaseID:         f.phase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Linked Template",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon"},
		PickupTimes:     carePickupTimes("mon"),
		IsActive:        true,
	}
	offering.TenantID = f.tenantID
	created, err := enrollmentService.NewCareOfferingRows(f.catalog).Create(f.ctx, offering)
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

	row := &capability.RequestChildOffering{
		RequestChildID: childID,
		CareOfferingID: offeringID,
		SelectedDays:   []string{"mon"},
	}
	require.NoError(t, repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).InsertRequestChildOffering(f.ctx, row))
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
	period := fixture.createPeriod(t, "care-period-mutation")
	fixture.createLinkedTemplate(t, &period.ID, nil)

	replacement := calendarPeriodReplacementOf(*period)
	replacement.EndDate = timezone.Date(fixture.phase.ServiceEndDate).AddDays(-1)

	err := fixture.validate(t, period.ID, replacement)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingTemplatePeriodMismatch)

	err = fixture.validate(t, period.ID, nil)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)

	replacement = calendarPeriodReplacementOf(*period)
	replacement.IsActive = false
	err = fixture.validate(t, period.ID, replacement)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
}

func TestCareOfferingCalendarPeriodValidation_RejectsWeekCycleCoverageGap(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	fixture := newCalendarPeriodValidationFixture(t)
	period := fixture.createPeriod(t, "care-period-cycle-change")
	group, _ := fixture.createLinkedTemplate(t, &period.ID, nil)

	schedules, err := testActivityScheduleRepository(t, fixture.db).FindByGroupID(fixture.ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	schedules[0].WeekPattern = 1
	require.NoError(t, testActivityScheduleRepository(t, fixture.db).Update(fixture.ctx, schedules[0]))

	replacement := calendarPeriodReplacementOf(*period)
	anchor := scheduleModels.NewDate(2026, time.August, 31) // Monday, week A
	replacement.WeekCycleLength = 2
	replacement.WeekCycleAnchor = string(anchor)
	err = fixture.validate(t, period.ID, replacement)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "does not cover")
}

func TestCareOfferingCalendarPeriodValidation_ProtectsInactiveReferencedOffering(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	fixture := newCalendarPeriodValidationFixture(t)
	period := fixture.createPeriod(t, "care-period-inactive-referenced")
	_, offering := fixture.createLinkedTemplate(t, &period.ID, nil)
	fixture.selectOfferingForSubmittedChild(t, offering.ID)
	offering.IsActive = false
	require.NoError(t, enrollmentService.NewCareOfferingRepository(repositories.NewFactory(fixture.db, repositories.NewUnobservedTimetableDependencies(fixture.db)).CarePlan()).Update(fixture.ctx, offering))

	replacement := calendarPeriodReplacementOf(*period)
	replacement.IsActive = false
	err := fixture.validate(t, period.ID, replacement)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
}

func TestCareOfferingCalendarPeriodValidation_ProtectsNonOverlappingLinkedRootDelete(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	fixture := newCalendarPeriodValidationFixture(t)
	rootPeriod := fixture.createPeriod(t, "care-period-linked-root")
	successorPeriod := fixture.createPeriod(t, "care-period-linked-successor")
	root, _ := fixture.createLinkedTemplate(t, &rootPeriod.ID, nil)
	repos := testRepositories(t, fixture.db)

	rootSchedules, err := repos.ActivitySchedule.FindByGroupID(fixture.ctx, root.ID)
	require.NoError(t, err)
	require.Len(t, rootSchedules, 1)
	rootSchedules[0].ValidUntil = activityDatePtr(&fixture.phase.ServiceStartDate)
	require.NoError(t, repos.ActivitySchedule.Update(fixture.ctx, rootSchedules[0]))

	seriesRootID := root.ID
	successor := &activitiesModels.Group{
		Name:             fmt.Sprintf("calendar-period-successor-%d", time.Now().UnixNano()),
		MaxParticipants:  root.MaxParticipants,
		IsOpen:           true,
		CategoryID:       root.CategoryID,
		Type:             root.Type,
		IsTemplate:       true,
		SeriesRootID:     &seriesRootID,
		CalendarPeriodID: &successorPeriod.ID,
	}
	successor.SetTenantID(fixture.tenantID)
	require.NoError(t, repos.ActivityGroup.Create(fixture.ctx, successor))
	successorSchedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		ActivityGroupID:  successor.ID,
		WeekPattern:      0,
		CalendarPeriodID: &successorPeriod.ID,
		ValidFrom:        activityDatePtr(&fixture.phase.ServiceStartDate),
	}
	successorSchedule.SetTenantID(fixture.tenantID)
	require.NoError(t, repos.ActivitySchedule.Create(fixture.ctx, successorSchedule))

	err = fixture.validate(t, rootPeriod.ID, nil)
	require.ErrorIs(t, err, careplan.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
}

func TestCareOfferingCalendarPeriodValidation_AllowsCompatibleFallbacks(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	t.Run("period update still contains phase", func(t *testing.T) {
		fixture := newCalendarPeriodValidationFixture(t)
		period := fixture.createPeriod(t, "care-period-compatible-update")
		fixture.createLinkedTemplate(t, &period.ID, nil)

		// The rename of the old test is not part of the catalog's replacement
		// (the name never reached the guard); the unchanged range must pass.
		replacement := calendarPeriodReplacementOf(*period)
		err := fixture.validate(t, period.ID, replacement)
		require.NoError(t, err)
	})

	t.Run("schedule override survives template fallback deletion", func(t *testing.T) {
		fixture := newCalendarPeriodValidationFixture(t)
		deletedPeriod := fixture.createPeriod(t, "care-period-deleted-fallback")
		overridePeriod := fixture.createPeriod(t, "care-period-schedule-override")
		fixture.createLinkedTemplate(t, &deletedPeriod.ID, &overridePeriod.ID)

		err := fixture.validate(t, deletedPeriod.ID, nil)
		require.NoError(t, err)
	})

	t.Run("template fallback replaces deleted schedule pin", func(t *testing.T) {
		fixture := newCalendarPeriodValidationFixture(t)
		deletedPeriod := fixture.createPeriod(t, "care-period-deleted-schedule-pin")
		fallbackPeriod := fixture.createPeriod(t, "care-period-template-fallback")
		fixture.createLinkedTemplate(t, &fallbackPeriod.ID, &deletedPeriod.ID)

		err := fixture.validate(t, deletedPeriod.ID, nil)
		require.NoError(t, err)
	})
}

// setupCareGuardTest replaces the deleted setupCareTest of the old catalog
// suite for the materializability tests: a catalog over the test database,
// its row adapter and a school-year phase of the test tenant.
func setupCareGuardTest(t *testing.T) (*bun.DB, enrollmentService.CareOfferingRows, careplan.CareOfferingCatalogCapability, *phaseFixture.Phase, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	catalog := testCareOfferingCatalog(t, db,
		testutil.WithCareOfferingPickupResync(&serviceFixture.PickupResyncer{}))

	phase := &phaseFixture.Phase{
		Name:             uniqueSchemaName("phase-" + t.Name()),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: phaseFixture.Date(timezone.NewDate(2026, 9, 1)),
		ServiceEndDate:   phaseFixture.Date(timezone.NewDate(2027, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = testpkg.Tenant(t)
	require.NoError(t, enrollmentService.InsertOwnerPhaseForTest(testpkg.Ctx(t), repoFactory.Enrollment(), phase))

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

	return db, enrollmentService.NewCareOfferingRows(catalog), catalog, phase, cleanup
}

func baseGuardLinkedOffering(t *testing.T, phaseID int64, groupID int64) *enrollmentModels.CareOffering {
	offering := &enrollmentModels.CareOffering{
		PhaseID:         phaseID,
		ActivityGroupID: &groupID,
		Name:            "Linked Template",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
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
) (*scheduleModels.Timeframe, *activitiesModels.Schedule) {
	t.Helper()
	start := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC))
	timeframe := &scheduleModels.Timeframe{
		StartTime:   start,
		EndTime:     endTime,
		IsActive:    true,
		Description: "care materializability",
	}
	timeframe.SetTenantID(testpkg.Tenant(t))
	repos := testRepositories(t, db)
	require.NoError(t, repos.Timeframe.Create(testpkg.Ctx(t), timeframe))
	schedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  groupID,
		CalendarPeriodID: &periodID,
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), schedule))
	return timeframe, schedule
}

func setCareTestPhaseWindow(
	t *testing.T,
	db *bun.DB,
	phase *phaseFixture.Phase,
	start, end timezone.Date,
) {
	t.Helper()
	phase.ServiceStartDate = phaseFixture.Date(start)
	phase.ServiceEndDate = phaseFixture.Date(end)
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Enrollment().UpdatePhase(testpkg.Ctx(t), enrollmentService.OwnerPhaseForTest(phase)))
}

func createCareMaterializationException(
	t *testing.T,
	db *bun.DB,
	exception *scheduleModels.ActivityException,
) {
	t.Helper()
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActivityException.Create(testpkg.Ctx(t), exception))
}

func TestCareOfferingMaterializability_RejectsIncompleteTimeframeAndRoom(t *testing.T) {
	t.Parallel()

	t.Run("missing timeframe", func(t *testing.T) {
		db, svc, _, phase, cleanup := setupCareGuardTest(t)
		defer cleanup()
		period := createCareOfferingTestPeriod(t, db, "missing-timeframe",
			timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
		group := createCareOfferingTemplateGroup(t, db, "missing-timeframe")
		schedule := &activitiesModels.Schedule{
			Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: group.ID,
			CalendarPeriodID: &period.ID,
		}
		schedule.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, testActivityScheduleRepository(t, db).Create(testpkg.Ctx(t), schedule))

		_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
		assert.ErrorContains(t, err, "no complete timeframe")
	})

	t.Run("open ended timeframe", func(t *testing.T) {
		db, svc, _, phase, cleanup := setupCareGuardTest(t)
		defer cleanup()
		period := createCareOfferingTestPeriod(t, db, "open-timeframe",
			timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
		group := createCareOfferingTemplateGroup(t, db, "open-timeframe")
		createCareMaterializationSchedule(t, db, group.ID, period.ID, nil)

		_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
		assert.ErrorContains(t, err, "no complete timeframe")
	})

	t.Run("missing effective room", func(t *testing.T) {
		db, svc, _, phase, cleanup := setupCareGuardTest(t)
		defer cleanup()
		period := createCareOfferingTestPeriod(t, db, "missing-room",
			timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
		group := createCareOfferingTemplateGroup(t, db, "missing-room")
		group.PlannedRoomID = nil
		require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActivityGroup.Update(testpkg.Ctx(t), group))
		end := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
		createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)

		_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
		assert.ErrorContains(t, err, "no effective room")
	})
}

func TestCareOfferingMaterializability_ExceptionCannotRescueMissingTimeframe(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "exception-no-timeframe",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "exception-no-timeframe")
	group.PlannedRoomID = nil
	repos := testRepositories(t, db)
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), group))
	schedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		ActivityGroupID:  group.ID,
		CalendarPeriodID: &period.ID,
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), schedule))

	overrideRoom := testpkg.CreateTestRoom(t, db, "Care no-timeframe override")
	start := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC))
	end := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationException(t, db, &scheduleModels.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   scheduleModels.Date(monday),
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &start,
		EndTime:         &end,
		RoomID:          &overrideRoom.ID,
	})

	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "no complete timeframe",
		"date-specific time and room overrides must not fabricate the missing base timeframe")
}

func TestCareOfferingMaterializability_CancellationCannotFabricateRecurrence(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "cancellation-no-recurrence",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "cancellation-no-recurrence")
	end := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	timeframe, _ := createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	repos := testRepositories(t, db)
	schedules, err := repos.ActivitySchedule.FindByGroupID(testpkg.Ctx(t), group.ID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	schedules[0].Weekday = activitiesModels.WeekdayTuesday
	schedules[0].TimeframeID = &timeframe.ID
	require.NoError(t, repos.ActivitySchedule.Update(testpkg.Ctx(t), schedules[0]))
	createCareMaterializationException(t, db, &scheduleModels.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   scheduleModels.Date(monday),
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	})

	_, err = svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "recurrence does not cover",
		"a cancellation suppresses a real occurrence; it cannot create one on another weekday")
}

func TestCareOfferingMaterializability_UsesDateSpecificExceptionRoom(t *testing.T) {
	t.Parallel()

	db, svc, catalog, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	firstMonday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, firstMonday, firstMonday)
	period := createCareOfferingTestPeriod(t, db, "exception-room",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "exception-room")
	group.PlannedRoomID = nil
	repos := testRepositories(t, db)
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), group))
	end := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	overrideRoom := testpkg.CreateTestRoom(t, db, "Care exception override")
	exception := &scheduleModels.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   scheduleModels.Date(firstMonday),
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		RoomID:          &overrideRoom.ID,
	}
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivityException.Create(testpkg.Ctx(t), exception))

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
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, secondMonday.String(),
		"phase service-window expansion must validate the newly exposed occurrence")

	setCareTestPhaseWindow(t, db, phase, firstMonday, secondMonday)
	err = svc.Update(testpkg.Ctx(t), created)
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, secondMonday.String(),
		"the first occurrence override must not become a series-wide room fallback")
}

func TestCareOfferingMaterializability_ExceptionIsScopedToSplitSeriesSegment(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	firstMonday := timezone.NewDate(2026, time.September, 7)
	secondMonday := firstMonday.AddDays(7)
	setCareTestPhaseWindow(t, db, phase, firstMonday, secondMonday)
	period := createCareOfferingTestPeriod(t, db, "segment-exception-period",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	root := createCareOfferingTemplateGroup(t, db, "segment-exception-root")
	successor := createCareOfferingTemplateGroup(t, db, "segment-exception-successor")
	root.PlannedRoomID = nil
	successor.PlannedRoomID = nil
	successor.SeriesRootID = &root.ID
	repos := testRepositories(t, db)
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), root))
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), successor))
	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, testpkg.Tenant(t), "segment exception")
	rootSchedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  root.ID,
		CalendarPeriodID: &period.ID,
		ValidUntil:       activityDatePtr(&secondMonday),
	}
	rootSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), rootSchedule))
	successorSchedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  successor.ID,
		CalendarPeriodID: &period.ID,
		ValidFrom:        activityDatePtr(&secondMonday),
	}
	successorSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), successorSchedule))

	overrideRoom := testpkg.CreateTestRoom(t, db, "Care segment exception override")
	for _, date := range []timezone.Date{firstMonday, secondMonday} {
		createCareMaterializationException(t, db, &scheduleModels.ActivityException{
			ActivityGroupID: root.ID,
			ExceptionDate:   scheduleModels.Date(date),
			ExceptionType:   scheduleModels.ActivityExceptionModified,
			RoomID:          &overrideRoom.ID,
		})
	}

	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, root.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
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

func TestCareOfferingMaterializability_ValidatesTimeframeReplacementAndDeletion(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, svc, catalog, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	period := createCareOfferingTestPeriod(t, db, "resource-change",
		timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "resource-change")
	end := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	timeframe, _ := createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err)

	err = inCareTenantTx(t, db, func(ctx context.Context) error {
		return catalog.ValidateRoomDeletion(ctx, *group.PlannedRoomID)
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "no effective room")

	err = inCareTenantTx(t, db, func(ctx context.Context) error {
		return catalog.ValidateTimeframeChange(ctx, timeframe.ID, nil)
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "no complete timeframe")

	openEnded := timeframeReplacementOf(*timeframe)
	openEnded.EndTime = nil
	err = inCareTenantTx(t, db, func(ctx context.Context) error {
		return catalog.ValidateTimeframeChange(ctx, timeframe.ID, openEnded)
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)

	inactive := timeframeReplacementOf(*timeframe)
	inactive.IsActive = false
	require.NoError(t, inCareTenantTx(t, db, func(ctx context.Context) error {
		return catalog.ValidateTimeframeChange(ctx, timeframe.ID, inactive)
	}),
		"materialization deliberately accepts inactive timeframes with complete clock times")
}

func TestCareOfferingMaterializability_RejectsCompleteReplacementWhenPartialExceptionBecomesInvalid(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, svc, catalog, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "replacement-partial-exception",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "replacement-partial-exception")
	originalEnd := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC))
	timeframe, _ := createCareMaterializationSchedule(t, db, group.ID, period.ID, &originalEnd)
	overrideStart := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC))
	createCareMaterializationException(t, db, &scheduleModels.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   scheduleModels.Date(monday),
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &overrideStart,
	})
	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err, "the partial override is valid against the stored 17:00 end")

	replacement := timeframeReplacementOf(*timeframe)
	replacementEnd := "15:00:00"
	replacement.EndTime = &replacementEnd
	err = inCareTenantTx(t, db, func(ctx context.Context) error {
		return catalog.ValidateTimeframeChange(ctx, timeframe.ID, replacement)
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "invalid effective start/end time",
		"replacement and partial exception must be composed before validating effective times")
}

func TestCareOfferingMaterializability_CancellationDoesNotRequireRoom(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "cancelled-room",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "cancelled-room")
	group.PlannedRoomID = nil
	repos := testRepositories(t, db)
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), group))
	end := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	exception := &scheduleModels.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   scheduleModels.Date(monday),
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivityException.Create(testpkg.Ctx(t), exception))

	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err, "a genuine recurrence that is intentionally cancelled needs no room")
}

func TestCareOfferingMaterializability_RejectsInvalidEffectiveTimes(t *testing.T) {
	t.Parallel()

	db, svc, _, phase, cleanup := setupCareGuardTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "invalid-effective-time",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "invalid-effective-time")
	end := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	invalidStart := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC))
	exception := &scheduleModels.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   scheduleModels.Date(monday),
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &invalidStart,
	}
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActivityException.Create(testpkg.Ctx(t), exception))

	_, err := svc.Create(testpkg.Ctx(t), baseGuardLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "invalid effective start/end time")
}
