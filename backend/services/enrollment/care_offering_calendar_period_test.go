package enrollment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type calendarPeriodValidationFixture struct {
	db       *bun.DB
	tenantID int64
	ctx      context.Context
	phase    *enrollmentModels.Phase
	service  enrollmentService.CareOfferingService
}

func newCalendarPeriodValidationFixture(t *testing.T) *calendarPeriodValidationFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	repos := repositories.NewFactory(db)

	phase := &enrollmentModels.Phase{
		Name:             fmt.Sprintf("calendar-period-validation-%d", time.Now().UnixNano()),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2026, time.September, 1),
		ServiceEndDate:   timezone.NewDate(2027, time.July, 31),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.SetTenantID(tenantID)
	require.NoError(t, repos.Phase.Create(ctx, phase))

	service := enrollmentService.NewCareOfferingService(enrollmentService.CareOfferingServiceConfig{
		Repo:                     repos.CareOffering,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		ActivityGroupRepo:        repos.ActivityGroup,
		ActivityScheduleRepo:     repos.ActivitySchedule,
		CalendarPeriodRepo:       repos.CalendarPeriod,
		TimeframeRepo:            repos.Timeframe,
		ActivityExceptionRepo:    repos.ActivityException,
		PhaseRepo:                repos.Phase,
	})

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, table := range []string{
			"enrollment.request_child_offerings",
			"enrollment.request_children",
			"enrollment.requests",
			"enrollment.care_offerings",
			"enrollment.phases",
			"schedule.activity_exceptions",
			"activities.schedules",
			"activities.groups",
			"activities.categories",
			"schedule.calendar_periods",
			"schedule.timeframes",
			"facilities.rooms",
		} {
			_, err := db.NewDelete().
				TableExpr(table).
				Where("tenant_id = ?", tenantID).
				Exec(cleanupCtx)
			require.NoError(t, err, "clean up %s", table)
		}
		testpkg.CleanupTenantTestData(t, db, tenantID)
		_, err := db.NewDelete().TableExpr("platform.schools").Where("id = ?", tenantID).Exec(cleanupCtx)
		require.NoError(t, err)
		_, err = db.NewDelete().TableExpr("platform.organizations").Where("id = ?", tenantID).Exec(cleanupCtx)
		require.NoError(t, err)
	})

	return &calendarPeriodValidationFixture{
		db:       db,
		tenantID: tenantID,
		ctx:      ctx,
		phase:    phase,
		service:  service,
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
		StartDate:       timezone.NewDate(2026, time.August, 1),
		EndDate:         timezone.NewDate(2027, time.August, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(f.tenantID)
	require.NoError(t, repositories.NewFactory(f.db).CalendarPeriod.Create(f.ctx, period))
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
	repos := repositories.NewFactory(f.db)
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
		IsActive:        true,
	}
	offering.SetTenantID(f.tenantID)
	created, err := f.service.Create(f.ctx, offering)
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

	row := &enrollmentModels.RequestChildOffering{
		RequestChildID: childID,
		CareOfferingID: offeringID,
		SelectedDays:   []string{"mon"},
	}
	require.NoError(t, repositories.NewFactory(f.db).RequestChildOffering.Create(f.ctx, row))
}

func (f *calendarPeriodValidationFixture) validator(
	t *testing.T,
) enrollmentService.CareOfferingCalendarPeriodValidator {
	t.Helper()
	validator, ok := f.service.(enrollmentService.CareOfferingCalendarPeriodValidator)
	require.True(t, ok)
	return validator
}

func TestCareOfferingCalendarPeriodValidation_RejectsRangeUpdateAndDelete(t *testing.T) {
	fixture := newCalendarPeriodValidationFixture(t)
	period := fixture.createPeriod(t, "care-period-mutation")
	fixture.createLinkedTemplate(t, &period.ID, nil)

	replacement := *period
	replacement.EndDate = fixture.phase.ServiceEndDate.AddDays(-1)

	err := fixture.validator(t).ValidateCalendarPeriodChange(fixture.ctx, period.ID, &replacement)
	require.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingTemplatePeriodMismatch)

	err = fixture.validator(t).ValidateCalendarPeriodChange(fixture.ctx, period.ID, nil)
	require.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)

	replacement = *period
	replacement.IsActive = false
	err = fixture.validator(t).ValidateCalendarPeriodChange(fixture.ctx, period.ID, &replacement)
	require.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
}

func TestCareOfferingCalendarPeriodValidation_RejectsWeekCycleCoverageGap(t *testing.T) {
	fixture := newCalendarPeriodValidationFixture(t)
	period := fixture.createPeriod(t, "care-period-cycle-change")
	group, _ := fixture.createLinkedTemplate(t, &period.ID, nil)

	schedules, err := repositories.NewFactory(fixture.db).ActivitySchedule.FindByGroupID(fixture.ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	schedules[0].WeekPattern = 1
	require.NoError(t, repositories.NewFactory(fixture.db).ActivitySchedule.Update(fixture.ctx, schedules[0]))

	replacement := *period
	anchor := timezone.NewDate(2026, time.August, 31) // Monday, week A
	replacement.WeekCycleLength = 2
	replacement.WeekCycleAnchor = &anchor
	err = fixture.validator(t).ValidateCalendarPeriodChange(fixture.ctx, period.ID, &replacement)
	require.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "does not cover")
}

func TestCareOfferingCalendarPeriodValidation_ProtectsInactiveReferencedOffering(t *testing.T) {
	fixture := newCalendarPeriodValidationFixture(t)
	period := fixture.createPeriod(t, "care-period-inactive-referenced")
	_, offering := fixture.createLinkedTemplate(t, &period.ID, nil)
	fixture.selectOfferingForSubmittedChild(t, offering.ID)
	offering.IsActive = false
	require.NoError(t, repositories.NewFactory(fixture.db).CareOffering.Update(fixture.ctx, offering))

	replacement := *period
	replacement.IsActive = false
	err := fixture.validator(t).ValidateCalendarPeriodChange(fixture.ctx, period.ID, &replacement)
	require.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
}

func TestCareOfferingCalendarPeriodValidation_ProtectsNonOverlappingLinkedRootDelete(t *testing.T) {
	fixture := newCalendarPeriodValidationFixture(t)
	rootPeriod := fixture.createPeriod(t, "care-period-linked-root")
	successorPeriod := fixture.createPeriod(t, "care-period-linked-successor")
	root, _ := fixture.createLinkedTemplate(t, &rootPeriod.ID, nil)
	repos := repositories.NewFactory(fixture.db)

	rootSchedules, err := repos.ActivitySchedule.FindByGroupID(fixture.ctx, root.ID)
	require.NoError(t, err)
	require.Len(t, rootSchedules, 1)
	rootSchedules[0].ValidUntil = &fixture.phase.ServiceStartDate
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
		ValidFrom:        &fixture.phase.ServiceStartDate,
	}
	successorSchedule.SetTenantID(fixture.tenantID)
	require.NoError(t, repos.ActivitySchedule.Create(fixture.ctx, successorSchedule))

	err = fixture.validator(t).ValidateCalendarPeriodChange(fixture.ctx, rootPeriod.ID, nil)
	require.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodCareOfferingConflict)
	assert.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
}

func TestCareOfferingCalendarPeriodValidation_AllowsCompatibleFallbacks(t *testing.T) {
	t.Run("period update still contains phase", func(t *testing.T) {
		fixture := newCalendarPeriodValidationFixture(t)
		period := fixture.createPeriod(t, "care-period-compatible-update")
		fixture.createLinkedTemplate(t, &period.ID, nil)

		replacement := *period
		replacement.Name += " renamed"
		err := fixture.validator(t).ValidateCalendarPeriodChange(fixture.ctx, period.ID, &replacement)
		require.NoError(t, err)
	})

	t.Run("schedule override survives template fallback deletion", func(t *testing.T) {
		fixture := newCalendarPeriodValidationFixture(t)
		deletedPeriod := fixture.createPeriod(t, "care-period-deleted-fallback")
		overridePeriod := fixture.createPeriod(t, "care-period-schedule-override")
		fixture.createLinkedTemplate(t, &deletedPeriod.ID, &overridePeriod.ID)

		err := fixture.validator(t).ValidateCalendarPeriodChange(fixture.ctx, deletedPeriod.ID, nil)
		require.NoError(t, err)
	})

	t.Run("template fallback replaces deleted schedule pin", func(t *testing.T) {
		fixture := newCalendarPeriodValidationFixture(t)
		deletedPeriod := fixture.createPeriod(t, "care-period-deleted-schedule-pin")
		fallbackPeriod := fixture.createPeriod(t, "care-period-template-fallback")
		fixture.createLinkedTemplate(t, &fallbackPeriod.ID, &deletedPeriod.ID)

		err := fixture.validator(t).ValidateCalendarPeriodChange(fixture.ctx, deletedPeriod.ID, nil)
		require.NoError(t, err)
	})
}
