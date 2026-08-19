package enrollment_test

import (
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
	start := timezone.WallClock(time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC))
	timeframe := &scheduleModels.Timeframe{
		StartTime:   start,
		EndTime:     endTime,
		IsActive:    true,
		Description: "care materializability",
	}
	timeframe.SetTenantID(testpkg.Tenant(t))
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.Timeframe.Create(testpkg.Ctx(t), timeframe))
	schedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  groupID,
		CalendarPeriodID: &periodID,
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), schedule))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "activities.schedules", schedule.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.timeframes", timeframe.ID)
	})
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
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.activity_exceptions", exception.ID)
	})
}

func TestCareOfferingMaterializability_RejectsIncompleteTimeframeAndRoom(t *testing.T) {
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
		require.NoError(t, repositories.NewFactory(db).ActivitySchedule.Create(testpkg.Ctx(t), schedule))

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
		end := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
		createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)

		_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
		require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
		assert.ErrorContains(t, err, "no effective room")
	})
}

func TestCareOfferingMaterializability_ExceptionCannotRescueMissingTimeframe(t *testing.T) {
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "exception-no-timeframe",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "exception-no-timeframe")
	group.PlannedRoomID = nil
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), group))
	schedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		ActivityGroupID:  group.ID,
		CalendarPeriodID: &period.ID,
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), schedule))

	overrideRoom := testpkg.CreateTestRoom(t, db, "Care no-timeframe override")
	start := timezone.WallClock(time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC))
	end := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationException(t, db, &scheduleModels.ActivityException{
		ActivityGroupID: group.ID,
		ExceptionDate:   monday,
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &start,
		EndTime:         &end,
		RoomID:          &overrideRoom.ID,
	})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", overrideRoom.ID)
	})

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "no complete timeframe",
		"date-specific time and room overrides must not fabricate the missing base timeframe")
}

func TestCareOfferingMaterializability_CancellationCannotFabricateRecurrence(t *testing.T) {
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "cancellation-no-recurrence",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "cancellation-no-recurrence")
	end := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	timeframe, _ := createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	repos := repositories.NewFactory(db)
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
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	firstMonday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, firstMonday, firstMonday)
	period := createCareOfferingTestPeriod(t, db, "exception-room",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "exception-room")
	group.PlannedRoomID = nil
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), group))
	end := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
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
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.activity_exceptions", exception.ID)
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", overrideRoom.ID)
	})

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
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), root))
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), successor))
	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, testpkg.Tenant(t), "segment exception")
	rootSchedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  root.ID,
		CalendarPeriodID: &period.ID,
		ValidUntil:       &secondMonday,
	}
	rootSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), rootSchedule))
	successorSchedule := &activitiesModels.Schedule{
		Weekday:          activitiesModels.WeekdayMonday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  successor.ID,
		CalendarPeriodID: &period.ID,
		ValidFrom:        &secondMonday,
	}
	successorSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(testpkg.Ctx(t), successorSchedule))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "activities.schedules", rootSchedule.ID, successorSchedule.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.timeframes", timeframe.ID)
	})

	overrideRoom := testpkg.CreateTestRoom(t, db, "Care segment exception override")
	for _, date := range []timezone.Date{firstMonday, secondMonday} {
		createCareMaterializationException(t, db, &scheduleModels.ActivityException{
			ActivityGroupID: root.ID,
			ExceptionDate:   date,
			ExceptionType:   scheduleModels.ActivityExceptionModified,
			RoomID:          &overrideRoom.ID,
		})
	}
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", overrideRoom.ID)
	})

	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, root.ID))
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, secondMonday.String())
	assert.ErrorContains(t, err, "no effective room",
		"a root exception on the successor date must not apply to the successor segment")
}

func TestCareOfferingMaterializability_ValidatesTimeframeReplacementAndDeletion(t *testing.T) {
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	period := createCareOfferingTestPeriod(t, db, "resource-change",
		timezone.NewDate(2026, 8, 1), timezone.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, db, "resource-change")
	end := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	timeframe, _ := createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	_, err := svc.Create(testpkg.Ctx(t), baseLinkedOffering(t, phase.ID, group.ID))
	require.NoError(t, err)

	validator, ok := svc.(enrollmentService.CareOfferingMaterializationResourceValidator)
	require.True(t, ok)
	err = validator.ValidateRoomDeletion(testpkg.Ctx(t), *group.PlannedRoomID)
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "no effective room")

	err = validator.ValidateTimeframeDeletion(testpkg.Ctx(t), timeframe.ID)
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "no complete timeframe")

	openEnded := *timeframe
	openEnded.EndTime = nil
	err = validator.ValidateTimeframeChange(testpkg.Ctx(t), timeframe.ID, &openEnded)
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)

	inactive := *timeframe
	inactive.IsActive = false
	require.NoError(t, validator.ValidateTimeframeChange(testpkg.Ctx(t), timeframe.ID, &inactive),
		"materialization deliberately accepts inactive timeframes with complete clock times")
}

func TestCareOfferingMaterializability_RejectsCompleteReplacementWhenPartialExceptionBecomesInvalid(t *testing.T) {
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "replacement-partial-exception",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "replacement-partial-exception")
	originalEnd := timezone.WallClock(time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC))
	timeframe, _ := createCareMaterializationSchedule(t, db, group.ID, period.ID, &originalEnd)
	overrideStart := timezone.WallClock(time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC))
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
	replacementEnd := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	replacement.EndTime = &replacementEnd
	err = validator.ValidateTimeframeChange(testpkg.Ctx(t), timeframe.ID, &replacement)
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid)
	assert.ErrorContains(t, err, "invalid effective start/end time",
		"replacement and partial exception must be composed before validating effective times")
}

func TestCareOfferingMaterializability_CancellationDoesNotRequireRoom(t *testing.T) {
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "cancelled-room",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "cancelled-room")
	group.PlannedRoomID = nil
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.ActivityGroup.Update(testpkg.Ctx(t), group))
	end := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
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
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	monday := timezone.NewDate(2026, time.September, 7)
	setCareTestPhaseWindow(t, db, phase, monday, monday)
	period := createCareOfferingTestPeriod(t, db, "invalid-effective-time",
		timezone.NewDate(2026, 9, 1), timezone.NewDate(2026, 9, 30))
	group := createCareOfferingTemplateGroup(t, db, "invalid-effective-time")
	end := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	createCareMaterializationSchedule(t, db, group.ID, period.ID, &end)
	invalidStart := timezone.WallClock(time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC))
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
