package enrollment_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func pickupTimeService(t *testing.T, env *decisionTestEnv) enrollmentService.OfferingPickupTimeService {
	t.Helper()
	svc, ok := env.decision.(enrollmentService.OfferingPickupTimeService)
	require.True(t, ok, "decision service must implement OfferingPickupTimeService")
	return svc
}

func createPickupTimeOffering(
	t *testing.T,
	env *decisionTestEnv,
	name string,
	days []string,
	times map[string]string,
) *enrollmentModels.CareOffering {
	t.Helper()
	offering := &enrollmentModels.CareOffering{
		PhaseID:        env.sourcePhase.ID,
		Name:           uniqueSchemaName(name + "-" + t.Name()),
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  days,
		PickupTimes:    times,
		IsActive:       true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(testpkg.Ctx(t), offering))
	return offering
}

func projectedPickupReader(env *decisionTestEnv) scheduleService.PickupScheduleService {
	return scheduleService.NewPickupScheduleServiceWithBulk(
		env.repos.StudentPickupSchedule,
		env.repos.StudentPickupException,
		env.repos.StudentPickupNote,
		env.repos.Student,
		env.repos.Person,
		nil,
		scheduletest.NewPickupBaselineService(
			env.repos.StudentPickupSchedule,
			env.repos.RequestChildOffering,
			env.repos.CareOffering,
		),
		env.db,
		slog.Default(),
	)
}

func nextWeekday(from timezone.Date, weekday time.Weekday) timezone.Date {
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
	setSourcePhaseServiceStartDate(t, env, timezone.NewDate(2026, 8, 24).AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createPickupTimeOffering(t, env, "gehzeit-future-end",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	studentID, childID := submitAndApproveOfferingChild(
		t, env, offering.ID, "gehzeit-future-end@example.com", "Ende", 2,
	)

	effectiveFrom := nextWeekday(timezone.NewDate(2026, 8, 24).AddDays(1), time.Monday)
	_, err := env.db.NewUpdate().
		Model((*enrollmentModels.RequestChildOffering)(nil)).
		ModelTableExpr(`enrollment.request_child_offerings AS "request_child_offering"`).
		Set("valid_until = ?", effectiveFrom).
		Where(`"request_child_offering".request_child_id = ?`, childID).
		Where(`"request_child_offering".care_offering_id = ?`, offering.ID).
		Exec(ctx)
	require.NoError(t, err)

	reader := projectedPickupReader(env)
	before, err := reader.GetEffectivePickupTimeForDate(ctx, studentID, effectiveFrom.AddDays(-7))
	require.NoError(t, err)
	require.NotNil(t, before.PickupTime)
	assert.Equal(t, "14:30", before.PickupTime.Format("15:04"))

	atBoundary, err := reader.GetEffectivePickupTimeForDate(ctx, studentID, effectiveFrom)
	require.NoError(t, err)
	assert.Nil(t, atBoundary.PickupTime,
		"an offering pickup time must stop at the booking's exclusive valid_until")
	rangeData, err := reader.GetStudentPickupDataForRange(ctx, studentID, effectiveFrom.AddDays(-7), effectiveFrom)
	require.NoError(t, err)
	require.Len(t, rangeData.EffectiveSchedules, 8)
	require.NotNil(t, rangeData.EffectiveSchedules[0].Schedule)
	assert.Equal(t, offering.Name, rangeData.EffectiveSchedules[0].Schedule.CareOfferingName)
	assert.Nil(t, rangeData.EffectiveSchedules[7].Schedule,
		"an empty projected date must be explicit so clients cannot fall back to today's row")

	stored, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Empty(t, stored, "approval must not materialize an undated offering row")

	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Geschützt")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, scheduleModels.WeekdayMonday, author.ID, "15:15")
	err = pickupTimeService(t, env).ResetStudentPickupDayToOffering(ctx, studentID, effectiveFrom)
	require.ErrorIs(t, err, enrollmentService.ErrPickupResetNoOffering)
	stored, err = env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, "15:15", stored[0].PickupTime.Format("15:04"),
		"a reset without an offering replacement must preserve the manual row")
}

func TestOfferingPickupProjection_FutureReplacementStartsExactlyOnEffectiveDate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, timezone.NewDate(2026, 8, 24).AddDays(-30))
	ctx := testpkg.Ctx(t)

	oldOffering := createPickupTimeOffering(t, env, "gehzeit-switch-old",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	newOffering := createPickupTimeOffering(t, env, "gehzeit-switch-new",
		[]string{"mon"}, map[string]string{"mon": "16:00"})
	studentID, childID := submitAndApproveOfferingChild(
		t, env, oldOffering.ID, "gehzeit-switch@example.com", "Wechsel", 2,
	)

	effectiveFrom := nextWeekday(timezone.NewDate(2026, 8, 24).AddDays(1), time.Monday)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(
		ctx,
		childID,
		effectiveFrom,
		[]*enrollmentModels.RequestChildOffering{{
			CareOfferingID: newOffering.ID,
			SelectedDays:   []string{"mon"},
		}},
	))

	reader := projectedPickupReader(env)
	before, err := reader.GetEffectivePickupTimeForDate(ctx, studentID, effectiveFrom.AddDays(-7))
	require.NoError(t, err)
	require.NotNil(t, before.PickupTime)
	assert.Equal(t, "14:30", before.PickupTime.Format("15:04"))

	atBoundary, err := reader.GetEffectivePickupTimeForDate(ctx, studentID, effectiveFrom)
	require.NoError(t, err)
	require.NotNil(t, atBoundary.PickupTime)
	assert.Equal(t, "16:00", atBoundary.PickupTime.Format("15:04"))
	weekly, err := reader.GetWeeklySchedulesByStudentIDsForDate(ctx, []int64{studentID}, effectiveFrom)
	require.NoError(t, err)
	require.Len(t, weekly, 1)
	assert.Equal(t, newOffering.Name, weekly[0].CareOfferingName)

	rangeData, err := reader.GetStudentPickupDataForRange(ctx, studentID, effectiveFrom.AddDays(-7), effectiveFrom)
	require.NoError(t, err)
	require.Len(t, rangeData.EffectiveSchedules, 8)
	assert.Equal(t, oldOffering.Name, rangeData.EffectiveSchedules[0].Schedule.CareOfferingName)
	assert.Equal(t, newOffering.Name, rangeData.EffectiveSchedules[7].Schedule.CareOfferingName)

	futureWeek, err := reader.GetStudentPickupDataForRange(ctx, studentID, effectiveFrom, effectiveFrom.AddDays(4))
	require.NoError(t, err)
	require.Len(t, futureWeek.Schedules, 1)
	assert.Equal(t, newOffering.Name, futureWeek.Schedules[0].CareOfferingName,
		"the weekly editor must receive the offering value for its requested week")

	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Zukunft")
	require.NoError(t, reader.UpsertBulkStudentPickupSchedulesForDate(ctx, studentID, effectiveFrom, []*scheduleModels.StudentPickupSchedule{{
		StudentID: studentID, Weekday: scheduleModels.WeekdayMonday,
		PickupTime: timezone.NormalizeWallClock(time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC)), CreatedBy: author.ID,
	}}))
	stored, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
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
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, scheduleModels.WeekdayMonday, author.ID, "15:15")
	offering.PickupTimes = map[string]string{"mon": "16:00"}
	require.NoError(t, env.repos.CareOffering.Update(ctx, offering))

	monday := nextWeekday(decisionTestToday, time.Monday)
	actual, err := projectedPickupReader(env).GetEffectivePickupTimeForDate(ctx, studentID, monday)
	require.NoError(t, err)
	require.NotNil(t, actual.PickupTime)
	assert.Equal(t, "15:15", actual.PickupTime.Format("15:04"))

	require.NoError(t, pickupTimeService(t, env).ResetStudentPickupDayToOffering(ctx, studentID, monday))
	stored, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Empty(t, stored, "reset must delete the manual row when an offering pickup replaces it")
	reset, err := projectedPickupReader(env).GetEffectivePickupTimeForDate(ctx, studentID, monday)
	require.NoError(t, err)
	require.NotNil(t, reset.PickupTime)
	assert.Equal(t, "16:00", reset.PickupTime.Format("15:04"))
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
	require.NoError(t, env.repos.CareOffering.Update(ctx, offering))

	pickup, err := projectedPickupReader(env).GetEffectivePickupTimeForDate(
		ctx, studentID, nextWeekday(decisionTestToday, time.Monday),
	)
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
	require.NoError(t, env.repos.CareOffering.Update(ctx, offering))

	pickup, err := projectedPickupReader(env).GetEffectivePickupTimeForDate(
		ctx, studentID, nextWeekday(decisionTestToday, time.Monday),
	)
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
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, scheduleModels.WeekdayMonday, author.ID, "15:15")
	monday := nextWeekday(decisionTestToday, time.Monday)

	lockedDecision := newDecisionServiceForTest(env.rolloverTestEnv, nil, func(ctx context.Context) error {
		return scheduleService.LockTenantRecurrenceWrites(ctx, env.db)
	})
	resetter, ok := lockedDecision.(enrollmentService.OfferingPickupTimeService)
	require.True(t, ok)
	releaseHolder, holderDone := holdOfferingSourceGate(t, env)
	resetDone := startPickupReset(t, env, resetter, studentID, monday)
	assertPickupResetBlocked(t, resetDone, releaseHolder, holderDone)
	close(releaseHolder)
	require.NoError(t, <-holderDone)
	require.NoError(t, <-resetDone)
	stored, err := env.repos.StudentPickupSchedule.FindByStudentID(testpkg.Ctx(t), studentID)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

func holdOfferingSourceGate(t *testing.T, env *decisionTestEnv) (chan struct{}, chan error) {
	t.Helper()
	acquired, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, testpkg.Ctx(t), env.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
			if err := scheduleService.LockTenantRecurrenceWrites(ctx, env.db); err != nil {
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

func startPickupReset(t *testing.T, env *decisionTestEnv, resetter enrollmentService.OfferingPickupTimeService, studentID int64, date timezone.Date) chan error {
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
