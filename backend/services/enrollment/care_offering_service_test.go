package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	phaseFixture "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/enrollment/enrollmenttest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func bindTestTimetable(t *testing.T, factory *repositories.Factory, db *bun.DB) {
	t.Helper()
	factory.BindTimetable(timetabletest.New(t, db))
}

func testRepositories(t *testing.T, db *bun.DB) *repositories.Factory {
	t.Helper()
	factory, err := repositories.NewFactoryWithPeopleDirectory(db, repositories.NewUnobservedTimetableDependencies(db))
	require.NoError(t, err)
	return factory
}

func testActivityScheduleRepository(t *testing.T, db *bun.DB) activitiesModels.ScheduleRepository {
	t.Helper()
	return testRepositories(t, db).ActivitySchedule
}

// setupCareTest wires a real DB-backed CareOfferingService and creates a
// phase the offerings can FK against. Phase model replaced calendar
// periods as the parent entity for care offerings (migration 1.15.68).
func setupCareTest(t *testing.T) (*bun.DB, enrollmentService.CareOfferingService, *phaseFixture.Phase, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	// Register pool closure before fixture cleanups. testing.Cleanup runs in
	// LIFO order, so groups/periods created later are deleted while the pool is
	// still open instead of leaking into subsequent package tests.
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	bindTestTimetable(t, repoFactory, db)
	svc := enrollmentService.NewCareOfferingService(enrollmentService.CareOfferingServiceConfig{
		Repo:                  repoFactory.CareOffering,
		Bookings:              repoFactory.Enrollment(),
		ActivityGroupRepo:     repoFactory.ActivityGroup,
		ActivityScheduleRepo:  repoFactory.ActivitySchedule,
		CalendarPeriodRepo:    repoFactory.CalendarPeriod,
		TimeframeRepo:         repoFactory.Timeframe,
		ActivityExceptionRepo: repoFactory.ActivityException,
		Phases:                repoFactory.Enrollment(),
		Logger:                slog.Default(),
	})
	bindTestPickupResyncer(t, svc)

	phase := &phaseFixture.Phase{
		Name:             uniqueSchemaName("phase-" + t.Name()),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: phaseFixture.Date(timezone.NewDate(2026, 9, 1)),
		ServiceEndDate:   phaseFixture.Date(timezone.NewDate(2027, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.TenantID = testpkg.Tenant(t)
	ctx := testpkg.Ctx(t)
	require.NoError(t, enrollmentService.InsertOwnerPhaseForTest(ctx, repoFactory.Enrollment(), phase))

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

	return db, svc, phase, cleanup
}

func createCareOfferingTestPeriod(t *testing.T, db *bun.DB, name string, start, end timezone.Date) *scheduleModels.CalendarPeriod {
	t.Helper()
	period := &scheduleModels.CalendarPeriod{
		Name:            uniqueSchemaName(name + "-" + t.Name()),
		PeriodType:      scheduleModels.PeriodTypeCustom,
		StartDate:       scheduleModels.Date(start),
		EndDate:         scheduleModels.Date(end),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CalendarPeriod.Create(testpkg.Ctx(t), period))
	return period
}

func createCareOfferingTemplateGroup(t *testing.T, db *bun.DB, name string) *activitiesModels.Group {
	t.Helper()
	category := testpkg.CreateTestActivityCategory(t, db, "CareTemplate-"+name)
	room := testpkg.CreateTestRoom(t, db, "CareTemplate-"+name)
	group := &activitiesModels.Group{
		Name:            uniqueSchemaName(name + "-" + t.Name()),
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
		Type:            activitiesModels.GroupTypeCare,
		IsTemplate:      true,
	}
	group.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActivityGroup.Create(testpkg.Ctx(t), group))
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			TableExpr("activities.schedules").
			Where("activity_group_id = ?", group.ID).
			Exec(context.Background())
	})
	return group
}

func createCareOfferingTemplateSchedule(t *testing.T, db *bun.DB, groupID int64, weekday int, periodID *int64) {
	t.Helper()
	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, testpkg.Tenant(t), "CareTemplate")
	schedule := &activitiesModels.Schedule{
		Weekday:          weekday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  groupID,
		WeekPattern:      0,
		CalendarPeriodID: periodID,
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, testActivityScheduleRepository(t, db).Create(testpkg.Ctx(t), schedule))
}

func baseLinkedOffering(t *testing.T, phaseID int64, groupID int64) *enrollmentModels.CareOffering {
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

func carePickupTimes(days ...string) map[string]string {
	times := make(map[string]string, len(days))
	for _, day := range days {
		times[day] = "14:30"
	}
	return times
}

func bindTestPickupResyncer(
	t *testing.T,
	service enrollmentService.CareOfferingService,
) *enrollmenttest.PickupResyncer {
	t.Helper()
	resyncer := &enrollmenttest.PickupResyncer{}
	binder, ok := service.(enrollmentService.CareOfferingPickupResyncBinder)
	require.True(t, ok)
	binder.SetPickupResyncer(resyncer)
	return resyncer
}

func TestCareOfferingService_Create_RequiresPickupTimesForActiveCareDays(t *testing.T) {
	t.Parallel()

	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	offering := &enrollmentModels.CareOffering{
		PhaseID: phase.ID, Name: "Ohne Gehzeit", AvailableDays: []string{"mon"}, IsActive: true,
	}
	offering.TenantID = testpkg.Tenant(t)

	_, err := svc.Create(testpkg.Ctx(t), offering)

	require.ErrorIs(t, err, enrollmentModels.ErrCareOfferingPickupTimesRequired)
}

func TestCareOfferingService_Create_AllowsMissingPickupTimesOutsideActiveCare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		active  bool
		care    bool
		careSet bool
	}{
		{name: "inactive", active: false},
		{name: "not care", active: true, care: false, careSet: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, svc, phase, cleanup := setupCareTest(t)
			defer cleanup()
			offering := &enrollmentModels.CareOffering{
				PhaseID: phase.ID, Name: tt.name, AvailableDays: []string{"mon"},
				IsActive: tt.active, CountsAsCare: tt.care, CountsAsCareSet: tt.careSet,
			}
			offering.TenantID = testpkg.Tenant(t)

			_, err := svc.Create(testpkg.Ctx(t), offering)

			require.NoError(t, err)
		})
	}
}

func TestCareOfferingService_Create_AndListByPhase(t *testing.T) {
	t.Parallel()

	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	offering := &enrollmentModels.CareOffering{
		PhaseID:             phase.ID,
		Name:                "Regelbetreuung",
		DaysOfWeekMode:      enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:       []string{"mon", "tue", "wed", "thu", "fri"},
		IncludesHolidayCare: false,
		IncludesLunch:       true,
		IsActive:            true,
		PickupTimes:         carePickupTimes("mon", "tue", "wed", "thu", "fri"),
		SortOrder:           0,
	}
	offering.TenantID = testpkg.Tenant(t)

	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, phase.ID, created.PhaseID)

	list, err := svc.ListByPhase(ctx, phase.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Regelbetreuung", list[0].Name)
}

func TestCareOfferingService_MutationsAcquireTemplateRecurrenceGate(t *testing.T) {
	t.Parallel()

	db, _, phase, cleanup := setupCareTest(t)
	defer cleanup()
	repos := testRepositories(t, db)
	lockCalls := 0
	svc := enrollmentService.NewCareOfferingService(enrollmentService.CareOfferingServiceConfig{
		Repo:                  repos.CareOffering,
		Bookings:              repos.Enrollment(),
		ActivityGroupRepo:     repos.ActivityGroup,
		ActivityScheduleRepo:  repos.ActivitySchedule,
		CalendarPeriodRepo:    repos.CalendarPeriod,
		TimeframeRepo:         repos.Timeframe,
		ActivityExceptionRepo: repos.ActivityException,
		Phases:                repos.Enrollment(),
		LockTemplateRecurrence: func(context.Context) error {
			lockCalls++
			return nil
		},
		Logger: slog.Default(),
	})
	bindTestPickupResyncer(t, svc)
	ctx := testpkg.Ctx(t)
	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Recurrence-gated offering",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		PickupTimes:    carePickupTimes("mon"),
		IsActive:       true,
	}

	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)
	created.Name = "Recurrence-gated offering updated"
	require.NoError(t, svc.Update(ctx, created))
	_, err = svc.Clone(ctx, created.ID, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, lockCalls, "create, update, and clone must all share the recurrence gate")
}

func TestCareOfferingService_ListActiveByPhase_FiltersInactive(t *testing.T) {
	t.Parallel()

	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	active := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Aktiv",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		PickupTimes:    carePickupTimes("mon"),
		IsActive:       true,
	}
	active.TenantID = testpkg.Tenant(t)
	_, err := svc.Create(ctx, active)
	require.NoError(t, err)

	inactive := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Inaktiv",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       false,
	}
	inactive.TenantID = testpkg.Tenant(t)
	_, err = svc.Create(ctx, inactive)
	require.NoError(t, err)

	publicList, err := svc.ListActiveByPhase(ctx, phase.ID)
	require.NoError(t, err)
	require.Len(t, publicList, 1, "ListActiveByPhase must drop is_active=false rows")
	assert.Equal(t, "Aktiv", publicList[0].Name)
}

func TestCareOfferingService_GetByID_NotFoundSentinel(t *testing.T) {
	t.Parallel()

	_, svc, _, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, err := svc.GetByID(ctx, 999_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingNotFound))
}

func TestCareOfferingService_Update_AppliesChanges(t *testing.T) {
	t.Parallel()

	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Original",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		PickupTimes:    carePickupTimes("mon"),
		IsActive:       true,
	}
	offering.TenantID = testpkg.Tenant(t)
	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)

	created.Name = "Updated"
	created.IsActive = false
	require.NoError(t, svc.Update(ctx, created))

	refreshed, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", refreshed.Name)
	assert.False(t, refreshed.IsActive)
}

func TestCareOfferingService_Create_ValidatesLinkedTemplate(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()

	period := createCareOfferingTestPeriod(t, db, "care-valid-period",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "care-valid-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, &period.ID)

	created, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))

	require.NoError(t, err)
	require.NotNil(t, created.ActivityGroupID)
	assert.Equal(t, group.ID, *created.ActivityGroupID)
}

func TestCareOfferingService_Create_UsesTemplateCalendarPeriodFallback(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := createCareOfferingTestPeriod(t, db, "care-template-fallback-period",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "care-template-fallback")
	group.CalendarPeriodID = &period.ID
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActivityGroup.Update(ctx, group))
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, nil)

	created, err := svc.Create(ctx, baseLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err)
	require.NotNil(t, created.ActivityGroupID)
	assert.Equal(t, group.ID, *created.ActivityGroupID)
}

func TestCareOfferingService_Create_RejectsIncompatibleSplitSeriesSegment(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	t.Cleanup(cleanup)
	ctx := testpkg.Ctx(t)

	rootPeriod := createCareOfferingTestPeriod(t, db, "care-series-root-period",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
	successorPeriod := createCareOfferingTestPeriod(t, db, "care-series-successor-period",
		timezone.NewDate(2027, 1, 1),
		timezone.NewDate(2027, 8, 31))
	root := createCareOfferingTemplateGroup(t, db, "care-series-root")
	root.CalendarPeriodID = &rootPeriod.ID
	repos := testRepositories(t, db)
	require.NoError(t, repos.ActivityGroup.Update(ctx, root))
	successor := createCareOfferingTemplateGroup(t, db, "care-series-successor")
	successor.SeriesRootID = &root.ID
	successor.CalendarPeriodID = &successorPeriod.ID
	require.NoError(t, repos.ActivityGroup.Update(ctx, successor))

	boundary := timezone.NewDate(2027, 1, 1)
	rootSchedule := &activitiesModels.Schedule{
		Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: root.ID,
		ValidUntil: activityDatePtr(&boundary),
	}
	rootSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(ctx, rootSchedule))
	successorSchedule := &activitiesModels.Schedule{
		Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: successor.ID,
		ValidFrom: activityDatePtr(&boundary),
	}
	successorSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(ctx, successorSchedule))

	_, err := svc.Create(ctx, baseLinkedOffering(t, phase.ID, root.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingTemplatePeriodMismatch)
}

func TestCareOfferingService_Create_RejectsTemplateOutsidePhaseWindow(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	t.Cleanup(cleanup)
	ctx := testpkg.Ctx(t)
	period := createCareOfferingTestPeriod(t, db, "care-no-overlap-period",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2028, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "care-no-overlap-template")
	group.CalendarPeriodID = &period.ID
	repos := testRepositories(t, db)
	require.NoError(t, repos.ActivityGroup.Update(ctx, group))
	startsAfterPhase := timezone.Date(phase.ServiceEndDate).AddDays(1)
	schedule := &activitiesModels.Schedule{
		Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: group.ID,
		ValidFrom: activityDatePtr(&startsAfterPhase),
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(ctx, schedule))

	_, err := svc.Create(ctx, baseLinkedOffering(t, phase.ID, group.ID))
	require.ErrorContains(t, err, "no recurrence segment")
}

func TestCareOfferingService_Create_RejectsUnavailableOfferingWeekday(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	period := createCareOfferingTestPeriod(t, db, "care-weekday-period",
		timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "care-weekday-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayTuesday, &period.ID)

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	require.ErrorContains(t, err, "weekday")
}

func TestCareOfferingService_Create_RejectsAdvertisedDayAbsentFromShortPhase(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	phase.ServiceStartDate = phaseFixture.Date(timezone.NewDate(2026, time.April, 21)) // Tuesday
	phase.ServiceEndDate = phaseFixture.Date(timezone.NewDate(2026, time.April, 22))   // Wednesday
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Enrollment().UpdatePhase(testpkg.Ctx(t), enrollmentService.OwnerPhaseForTest(phase)))
	period := createCareOfferingTestPeriod(t, db, "care-zero-occurrence-period",
		timezone.NewDate(2026, 4, 1), timezone.NewDate(2026, 4, 30))
	group := createCareOfferingTemplateGroup(t, db, "care-zero-occurrence-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, &period.ID)

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	require.ErrorContains(t, err, "has no occurrence during the enrollment phase")
}

func TestCareOfferingService_Create_RequiresEveryABWeekOccurrence(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	anchor := scheduleModels.NewDate(2026, time.August, 31) // Monday, week A
	period := &scheduleModels.CalendarPeriod{
		Name: uniqueSchemaName("care-ab-period-" + t.Name()), PeriodType: scheduleModels.PeriodTypeCustom,
		StartDate: scheduleModels.NewDate(2026, 8, 1), EndDate: scheduleModels.NewDate(2027, 8, 31),
		WeekCycleLength: 2, WeekCycleAnchor: &anchor, IsActive: true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	repos := testRepositories(t, db)
	require.NoError(t, repos.CalendarPeriod.Create(testpkg.Ctx(t), period))
	group := createCareOfferingTemplateGroup(t, db, "care-ab-template")
	weekATimeframe := testpkg.CreateTestTimeframeForTenant(t, db, testpkg.Tenant(t), "Care AB A")
	weekBTimeframe := testpkg.CreateTestTimeframeForTenant(t, db, testpkg.Tenant(t), "Care AB B")
	weekA := &activitiesModels.Schedule{
		Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: group.ID,
		WeekPattern: 1, CalendarPeriodID: &period.ID, TimeframeID: &weekATimeframe.ID,
	}
	weekA.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), weekA))
	offering := baseLinkedOffering(t, phase.ID, group.ID)

	_, err := svc.Create(testpkg.Ctx(t), offering)
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)

	weekB := &activitiesModels.Schedule{
		Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: group.ID,
		WeekPattern: 2, CalendarPeriodID: &period.ID, TimeframeID: &weekBTimeframe.ID,
	}
	weekB.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), weekB))
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("activities.schedules").Where("activity_group_id = ?", group.ID).Exec(context.Background())
	})
	_, err = svc.Create(testpkg.Ctx(t), offering)
	require.NoError(t, err, "complementary A/B rows cover every advertised Monday")
}

func TestCareOfferingService_Create_AllowsNonOccurrencePhaseBoundaries(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	phase.ServiceStartDate = phaseFixture.Date(timezone.NewDate(2026, time.April, 17)) // Friday
	phase.ServiceEndDate = phaseFixture.Date(timezone.NewDate(2026, time.April, 26))   // Sunday
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Enrollment().UpdatePhase(testpkg.Ctx(t), enrollmentService.OwnerPhaseForTest(phase)))
	period := createCareOfferingTestPeriod(t, db, "care-boundary-period",
		timezone.NewDate(2026, 4, 1), timezone.NewDate(2026, 4, 30))
	group := createCareOfferingTemplateGroup(t, db, "care-boundary-template")
	validFrom := timezone.NewDate(2026, time.April, 20)  // first Monday
	validUntil := timezone.NewDate(2026, time.April, 21) // exclusive after it
	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, testpkg.Tenant(t), "Care boundary")
	schedule := &activitiesModels.Schedule{
		Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: group.ID,
		CalendarPeriodID: &period.ID, TimeframeID: &timeframe.ID, ValidFrom: activityDatePtr(&validFrom), ValidUntil: activityDatePtr(&validUntil),
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, testActivityScheduleRepository(t, db).Create(testpkg.Ctx(t), schedule))

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err)
}

func TestCareOfferingService_Create_ActiveOfferingRequiresActivePeriod(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	period := &scheduleModels.CalendarPeriod{
		Name:       uniqueSchemaName("inactive-care-period-" + t.Name()),
		PeriodType: scheduleModels.PeriodTypeCustom,
		StartDate:  scheduleModels.NewDate(2026, 8, 1), EndDate: scheduleModels.NewDate(2027, 8, 31),
		WeekCycleLength: 1, IsActive: false,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CalendarPeriod.Create(testpkg.Ctx(t), period))
	group := createCareOfferingTemplateGroup(t, db, "inactive-care-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, &period.ID)
	offering := baseLinkedOffering(t, phase.ID, group.ID)

	_, err := svc.Create(testpkg.Ctx(t), offering)
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	require.ErrorContains(t, err, "active calendar period")

	offering.IsActive = false
	_, err = svc.Create(testpkg.Ctx(t), offering)
	require.NoError(t, err, "inactive drafts may be staged against inactive future periods")
}

func TestCareOfferingService_Create_RejectsNonTemplateActivityGroup(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	group := testpkg.CreateTestActivityGroup(t, db, "Care-NonTemplate")

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must reference a timetable template")
}

func TestCareOfferingService_Create_RejectsTemplateWithoutSchedules(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	group := createCareOfferingTemplateGroup(t, db, "care-empty-template")

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one schedule")
}

func TestCareOfferingService_Create_RejectsTemplateWithoutUniquePeriod(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	periodA := createCareOfferingTestPeriod(t, db, "care-period-a",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
	periodB := createCareOfferingTestPeriod(t, db, "care-period-b",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "care-mixed-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, &periodA.ID)
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayTuesday, &periodB.ID)

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "one calendar_period_id")
}

func TestCareOfferingService_Create_RejectsTemplateWithNullPeriod(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	group := createCareOfferingTemplateGroup(t, db, "care-null-period-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, nil)

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "calendar_period_id")
}

func TestCareOfferingService_Create_RejectsPhaseOutsideTemplatePeriod(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	period := createCareOfferingTestPeriod(t, db, "care-short-period",
		timezone.NewDate(2026, 9, 1),
		timezone.NewDate(2026, 12, 31))
	group := createCareOfferingTemplateGroup(t, db, "care-short-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, &period.ID)

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))

	require.Error(t, err)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingTemplatePeriodMismatch)
	assert.Contains(t, err.Error(), "phase must be within")
}

func TestCareOfferingService_Update_RejectsPhaseOutsideTemplatePeriod(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	period := createCareOfferingTestPeriod(t, db, "care-short-update-period",
		timezone.NewDate(2026, 9, 1),
		timezone.NewDate(2026, 12, 31))
	group := createCareOfferingTemplateGroup(t, db, "care-short-update-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, &period.ID)

	offering := baseLinkedOffering(t, phase.ID, group.ID)
	offering.ActivityGroupID = nil
	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)
	created.ActivityGroupID = &group.ID

	err = svc.Update(ctx, created)

	require.Error(t, err)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingTemplatePeriodMismatch)
}

func TestCareOfferingService_Update_RejectsUnavailableOfferingWeekday(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	period := createCareOfferingTestPeriod(t, db, "care-weekday-update-period",
		timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "care-weekday-update-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, &period.ID)

	created, err := svc.Create(ctx, baseLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err)

	created.AvailableDays = []string{"mon", "tue"}
	err = svc.Update(ctx, created)

	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	require.ErrorContains(t, err, "weekday")
}

func TestCareOfferingService_Update_RefreshesPickupProjectionConsumers(t *testing.T) {
	t.Parallel()

	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	offering := &enrollmentModels.CareOffering{
		PhaseID: phase.ID, Name: "Gehzeit alt",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"}, PickupTimes: map[string]string{"mon": "14:30"},
		IsActive: true,
	}
	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)

	resyncer := bindTestPickupResyncer(t, svc)
	created.PickupTimes = map[string]string{"mon": "15:00"}

	require.NoError(t, svc.Update(ctx, created))
	assert.Equal(t, []int64{created.ID}, resyncer.OfferingIDs)
}

func TestCareOfferingService_Clone_RepointsToTargetPhase(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	source := &enrollmentModels.CareOffering{
		PhaseID:             phase.ID,
		Name:                "Original",
		DaysOfWeekMode:      enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:       []string{"mon", "tue"},
		IncludesHolidayCare: false,
		IncludesLunch:       true,
		IsActive:            true,
		CountsAsCare:        false,
		CountsAsCareSet:     true,
	}
	source.TenantID = testpkg.Tenant(t)
	created, err := svc.Create(ctx, source)
	require.NoError(t, err)

	// Build a second phase as the clone target.
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	target := &phaseFixture.Phase{
		Name:             uniqueSchemaName("phase-clone-target-" + t.Name()),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: phaseFixture.Date(timezone.NewDate(2027, 9, 1)),
		ServiceEndDate:   phaseFixture.Date(timezone.NewDate(2028, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	target.TenantID = testpkg.Tenant(t)
	require.NoError(t, enrollmentService.InsertOwnerPhaseForTest(ctx, repoFactory.Enrollment(), target))
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("phase_id = ?", target.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("id = ?", target.ID).
			Exec(bg)
	})

	clone, err := svc.Clone(ctx, created.ID, target.ID)
	require.NoError(t, err)
	require.NotZero(t, clone.ID)
	assert.NotEqual(t, created.ID, clone.ID, "clone must get a fresh BIGSERIAL id")
	assert.Equal(t, target.ID, clone.PhaseID, "clone must point at the target phase")
	assert.Equal(t, "Original", clone.Name)
	assert.Equal(t, []string{"mon", "tue"}, clone.AvailableDays)
	assert.False(t, clone.CountsAsCare)

	refetched, err := svc.GetByID(ctx, clone.ID)
	require.NoError(t, err)
	assert.False(t, refetched.CountsAsCare, "clone insert must persist non-statistical offerings")
}

func TestCareOfferingService_Clone_ClearsLinkedTemplateAcrossPhases(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := createCareOfferingTestPeriod(t, db, "care-clone-source-period",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "care-clone-source-template")
	createCareOfferingTemplateSchedule(t, db, group.ID, activitiesModels.WeekdayMonday, &period.ID)

	source := baseLinkedOffering(t, phase.ID, group.ID)
	created, err := svc.Create(ctx, source)
	require.NoError(t, err)

	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	target := &phaseFixture.Phase{
		Name:             uniqueSchemaName("phase-clone-linked-target-" + t.Name()),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: phaseFixture.Date(timezone.NewDate(2027, 9, 1)),
		ServiceEndDate:   phaseFixture.Date(timezone.NewDate(2028, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	target.TenantID = testpkg.Tenant(t)
	require.NoError(t, enrollmentService.InsertOwnerPhaseForTest(ctx, repoFactory.Enrollment(), target))
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("phase_id = ?", target.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("id = ?", target.ID).
			Exec(bg)
	})

	clone, err := svc.Clone(ctx, created.ID, target.ID)
	require.NoError(t, err)
	require.NotZero(t, clone.ID)
	assert.Equal(t, target.ID, clone.PhaseID)
	assert.Nil(t, clone.ActivityGroupID)
}

func TestCareOfferingService_Delete_RemovesRow(t *testing.T) {
	t.Parallel()

	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Soon-deleted",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		PickupTimes:    carePickupTimes("mon"),
		IsActive:       true,
	}
	offering.TenantID = testpkg.Tenant(t)
	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, created.ID))

	_, err = svc.GetByID(ctx, created.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingNotFound))
}

// #2147 review round 11: deleting an offering must retire its sourced
// templates' rosters BEFORE the row delete — the FK's ON DELETE SET NULL
// alone only flips the templates to manual rosters and would leave their
// offering-derived enrollment rows behind.
func TestCareOfferingService_Delete_DetachesSourcedTemplates(t *testing.T) {
	t.Parallel()

	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Detached-on-delete",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		PickupTimes:    carePickupTimes("mon"),
		IsActive:       true,
	}
	offering.TenantID = testpkg.Tenant(t)
	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)

	resyncer := &recordingSourcedTemplateResyncer{}
	binder, ok := svc.(enrollmentService.CareOfferingSourceResyncBinder)
	require.True(t, ok, "care offering service must accept the sourced-template resyncer")
	binder.SetSourcedTemplateResyncer(resyncer)

	require.NoError(t, svc.Delete(ctx, created.ID))

	assert.Equal(t, []int64{created.ID}, resyncer.detachedOfferingIDs,
		"the delete must retire sourced templates before the row is removed")
}

func TestCareOfferingService_RejectsMixedRuleInSameGroup(t *testing.T) {
	t.Parallel()

	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	groupOffering := func(name, rule string) *enrollmentModels.CareOffering {
		o := &enrollmentModels.CareOffering{
			PhaseID:        phase.ID,
			Name:           name,
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays:  []string{"mon"},
			PickupTimes:    carePickupTimes("mon"),
			IsActive:       true,
			SelectionGroup: "tag",
			SelectionRule:  rule,
		}
		o.TenantID = testpkg.Tenant(t)
		return o
	}

	first, err := svc.Create(ctx, groupOffering("A", enrollmentModels.SelectionRuleExactlyOne))
	require.NoError(t, err)

	// A non-optional rule differing from the group's existing rule is rejected.
	_, err = svc.Create(ctx, groupOffering("B", enrollmentModels.SelectionRuleAtLeastOne))
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingGroupRuleConflict))

	// An optional (or empty-rule) sibling in a non-optional group is ALSO
	// rejected — the submit path counts all members, so the group must be
	// homogeneous.
	_, err = svc.Create(ctx, groupOffering("C", enrollmentModels.SelectionRuleOptional))
	require.Error(t, err, "optional sibling in an exactly_one group must be rejected")
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingGroupRuleConflict))

	// A matching rule is accepted.
	_, err = svc.Create(ctx, groupOffering("D", enrollmentModels.SelectionRuleExactlyOne))
	require.NoError(t, err)

	// Updating the first offering to a different rule is likewise rejected
	// while siblings still hold the old rule.
	first.SelectionRule = enrollmentModels.SelectionRuleAtMostOne
	err = svc.Update(ctx, first)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingGroupRuleConflict))
}

func TestCareOfferingService_RejectsAutoAddTriggersInExclusiveSelectionGroup(t *testing.T) {
	t.Parallel()

	for _, rule := range []string{
		enrollmentModels.SelectionRuleExactlyOne,
		enrollmentModels.SelectionRuleAtMostOne,
	} {
		t.Run(rule, func(t *testing.T) {
			_, svc, phase, cleanup := setupCareTest(t)
			defer cleanup()
			ctx := testpkg.Ctx(t)

			trigger := &enrollmentModels.CareOffering{
				PhaseID:        phase.ID,
				Name:           "Ganztag",
				DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
				AvailableDays:  []string{"mon"},
				PickupTimes:    carePickupTimes("mon"),
				IsActive:       true,
				SelectionGroup: "umfang",
				SelectionRule:  rule,
			}
			trigger.TenantID = testpkg.Tenant(t)
			createdTrigger, err := svc.Create(ctx, trigger)
			require.NoError(t, err)

			target := &enrollmentModels.CareOffering{
				PhaseID:                   phase.ID,
				Name:                      "Randstunde",
				DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
				AvailableDays:             []string{"mon"},
				PickupTimes:               carePickupTimes("mon"),
				IsActive:                  true,
				SelectionGroup:            "umfang",
				SelectionRule:             rule,
				AutoAddTriggerOfferingIDs: []int64{createdTrigger.ID},
			}
			target.TenantID = testpkg.Tenant(t)

			_, err = svc.Create(ctx, target)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "exclusive selection group")
		})
	}
}

func TestCareOfferingService_AllowsAutoAddTriggersOutsideExclusiveSelectionGroups(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		triggerRule string
		targetRule  string
		targetGroup string
	}{
		{
			name:        "at least one group",
			triggerRule: enrollmentModels.SelectionRuleAtLeastOne,
			targetRule:  enrollmentModels.SelectionRuleAtLeastOne,
			targetGroup: "umfang",
		},
		{
			name:        "different groups",
			triggerRule: enrollmentModels.SelectionRuleAtMostOne,
			targetRule:  enrollmentModels.SelectionRuleAtMostOne,
			targetGroup: "zusatz",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, svc, phase, cleanup := setupCareTest(t)
			defer cleanup()
			ctx := testpkg.Ctx(t)

			trigger := &enrollmentModels.CareOffering{
				PhaseID:        phase.ID,
				Name:           "Ganztag",
				DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
				AvailableDays:  []string{"mon"},
				PickupTimes:    carePickupTimes("mon"),
				IsActive:       true,
				SelectionGroup: "umfang",
				SelectionRule:  tc.triggerRule,
			}
			trigger.TenantID = testpkg.Tenant(t)
			createdTrigger, err := svc.Create(ctx, trigger)
			require.NoError(t, err)

			target := &enrollmentModels.CareOffering{
				PhaseID:                   phase.ID,
				Name:                      "Randstunde",
				DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
				AvailableDays:             []string{"mon"},
				PickupTimes:               carePickupTimes("mon"),
				IsActive:                  true,
				SelectionGroup:            tc.targetGroup,
				SelectionRule:             tc.targetRule,
				AutoAddTriggerOfferingIDs: []int64{createdTrigger.ID},
			}
			target.TenantID = testpkg.Tenant(t)

			_, err = svc.Create(ctx, target)

			require.NoError(t, err)
		})
	}
}
