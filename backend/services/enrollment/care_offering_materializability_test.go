package enrollment_test

import (
	"context"
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
	phase *enrollmentModels.Phase,
	start, end timezone.Date,
) {
	t.Helper()
	phase.ServiceStartDate = start
	phase.ServiceEndDate = end
	require.NoError(t, repositories.NewFactory(db).Phase.Update(testpkg.Ctx(t), phase))
}

func createCareMaterializationException(
	t *testing.T,
	db *bun.DB,
	exception *scheduleModels.ActivityException,
) {
	t.Helper()
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db).ActivityException.Create(testpkg.Ctx(t), exception))
}

func TestCareOfferingMaterializability_RejectsIncompleteTimeframeAndRoom(t *testing.T) {
	t.Parallel()

	t.Run("missing timeframe", func(t *testing.T) {
		db, svc, phase, cleanup := setupCareTest(t)
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

		_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
		assert.ErrorContains(t, err, "no complete timeframe")
	})

	t.Run("open ended timeframe", func(t *testing.T) {
		db, svc, phase, cleanup := setupCareTest(t)
		defer cleanup()
		period := createCareOfferingTestPeriod(t, db, "open-timeframe",
			timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
		group := createCareOfferingTemplateGroup(t, db, "open-timeframe")
		createCareMaterializationSchedule(t, db, group.ID, period.ID, nil)

		_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
		assert.ErrorContains(t, err, "no complete timeframe")
	})

	t.Run("missing effective room", func(t *testing.T) {
		db, svc, phase, cleanup := setupCareTest(t)
		defer cleanup()
		period := createCareOfferingTestPeriod(t, db, "missing-room",
			timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
		group := createCareOfferingTemplateGroup(t, db, "missing-room")
		group.PlannedRoomID = nil
		require.NoError(t, repositories.NewFactory(db).ActivityGroup.Update(testpkg.Ctx(t), group))
		end := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
		createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)

		_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
		assert.ErrorContains(t, err, "no effective room")
	})
}

func TestCareOfferingMaterializability_ExceptionCannotRescueMissingTimeframe(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
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
		ExceptionDate:   monday,
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &start,
		EndTime:         &end,
		RoomID:          &overrideRoom.ID,
	})

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "no complete timeframe",
		"date-specific time and room overrides must not fabricate the missing base timeframe")
}

func TestCareOfferingMaterializability_CancellationCannotFabricateRecurrence(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
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
		ExceptionDate:   monday,
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	})

	_, err = svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "recurrence does not cover",
		"a cancellation suppresses a real occurrence; it cannot create one on another weekday")
}

func TestCareOfferingMaterializability_UsesDateSpecificExceptionRoom(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
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
		ExceptionDate:   firstMonday,
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		RoomID:          &overrideRoom.ID,
	}
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivityException.Create(testpkg.Ctx(t), exception))

	created, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err, "the exact occurrence room override must satisfy materialization")

	secondMonday := firstMonday.AddDays(7)
	phaseValidator, ok := svc.(enrollmentService.CareOfferingPhaseValidator)
	require.True(t, ok)
	replacement := *phase
	replacement.ServiceEndDate = secondMonday
	err = phaseValidator.ValidatePhaseChange(testpkg.Ctx(t), phase.ID, &replacement)
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

	db, svc, phase, cleanup := setupCareTest(t)
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
			ExceptionDate:   date,
			ExceptionType:   scheduleModels.ActivityExceptionModified,
			RoomID:          &overrideRoom.ID,
		})
	}

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, root.ID))
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
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	period := createCareOfferingTestPeriod(t, db, "resource-change",
		timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "resource-change")
	end := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	timeframe, _ := createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err)

	validator, ok := svc.(enrollmentService.CareOfferingMaterializationResourceValidator)
	require.True(t, ok)
	err = inCareTenantTx(t, db, func(ctx context.Context) error {
		return validator.ValidateRoomDeletion(ctx, *group.PlannedRoomID)
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "no effective room")

	err = inCareTenantTx(t, db, func(ctx context.Context) error {
		return validator.ValidateTimeframeDeletion(ctx, timeframe.ID)
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "no complete timeframe")

	openEnded := *timeframe
	openEnded.EndTime = nil
	err = inCareTenantTx(t, db, func(ctx context.Context) error {
		return validator.ValidateTimeframeChange(ctx, timeframe.ID, &openEnded)
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)

	inactive := *timeframe
	inactive.IsActive = false
	require.NoError(t, inCareTenantTx(t, db, func(ctx context.Context) error {
		return validator.ValidateTimeframeChange(ctx, timeframe.ID, &inactive)
	}),
		"materialization deliberately accepts inactive timeframes with complete clock times")
}

func TestCareOfferingMaterializability_RejectsCompleteReplacementWhenPartialExceptionBecomesInvalid(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, svc, phase, cleanup := setupCareTest(t)
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
		ExceptionDate:   monday,
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &overrideStart,
	})
	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err, "the partial override is valid against the stored 17:00 end")

	validator, ok := svc.(enrollmentService.CareOfferingMaterializationResourceValidator)
	require.True(t, ok)
	replacement := *timeframe
	replacementEnd := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	replacement.EndTime = &replacementEnd
	err = inCareTenantTx(t, db, func(ctx context.Context) error {
		return validator.ValidateTimeframeChange(ctx, timeframe.ID, &replacement)
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "invalid effective start/end time",
		"replacement and partial exception must be composed before validating effective times")
}

func TestCareOfferingMaterializability_CancellationDoesNotRequireRoom(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
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
		ExceptionDate:   monday,
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivityException.Create(testpkg.Ctx(t), exception))

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err, "a genuine recurrence that is intentionally cancelled needs no room")
}

func TestCareOfferingMaterializability_RejectsInvalidEffectiveTimes(t *testing.T) {
	t.Parallel()

	db, svc, phase, cleanup := setupCareTest(t)
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
		ExceptionDate:   monday,
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &invalidStart,
	}
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db).ActivityException.Create(testpkg.Ctx(t), exception))

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "invalid effective start/end time")
}
