package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Integration tests for the Angebots-Gehzeit rollout/reconcile service
// (#2290, ADR 0001): offering pickup_times materialize into
// schedule.student_pickup_schedules rows with source=care_offering.

func rolloutActorAccountID(t *testing.T, env *decisionTestEnv) int64 {
	t.Helper()
	_, account := testpkg.CreateTestStaffWithAccount(t, env.db, "Gehzeit", "Rollout")
	return account.ID
}

func pickupTimeService(t *testing.T, env *decisionTestEnv) enrollmentService.OfferingPickupTimeService {
	t.Helper()
	svc, ok := env.decision.(enrollmentService.OfferingPickupTimeService)
	require.True(t, ok, "decision service must implement OfferingPickupTimeService")
	return svc
}

func createPickupTimeOffering(t *testing.T, env *decisionTestEnv, name string, days []string, times map[string]string) *enrollmentModels.CareOffering {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	offering := &enrollmentModels.CareOffering{
		PhaseID:        env.sourcePhase.ID,
		Name:           uniqueSchemaName(name + "-" + t.Name()),
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  days,
		PickupTimes:    times,
		IsActive:       true,
	}
	offering.SetTenantID(1)
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("id = ?", offering.ID).
			Exec(context.Background())
	})
	return offering
}

func pickupRowsByWeekday(t *testing.T, env *decisionTestEnv, studentID int64) map[int]*scheduleModels.StudentPickupSchedule {
	t.Helper()
	rows, err := env.repos.StudentPickupSchedule.FindByStudentID(testpkg.TenantContext(1), studentID)
	require.NoError(t, err)
	out := make(map[int]*scheduleModels.StudentPickupSchedule, len(rows))
	for _, row := range rows {
		out[row.Weekday] = row
	}
	return out
}

func TestOfferingPickupRollout_CreatesSourcedRows(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	offering := createPickupTimeOffering(t, env, "gehzeit-basic",
		[]string{"mon", "tue"}, map[string]string{"mon": "14:30"})
	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "gehzeit-basic@example.com", "Gina", 2)

	result, err := pickupTimeService(t, env).RolloutOfferingPickupTimes(ctx, offering.ID, nil, rolloutActorAccountID(t, env))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.CreatedRows, 1)

	rows := pickupRowsByWeekday(t, env, studentID)
	require.Contains(t, rows, scheduleModels.WeekdayMonday)
	monday := rows[scheduleModels.WeekdayMonday]
	assert.Equal(t, "14:30", monday.PickupTime.Format("15:04"))
	assert.Equal(t, scheduleModels.PickupScheduleSourceCareOffering, monday.Source)
	require.NotNil(t, monday.CareOfferingID)
	assert.Equal(t, offering.ID, *monday.CareOfferingID)
	assert.NotContains(t, rows, scheduleModels.WeekdayTuesday,
		"a day without an Angebots-Gehzeit must not get a row")
}

func TestOfferingPickupRollout_OverwritesStaffRowsUnlessSkipped(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	offering := createPickupTimeOffering(t, env, "gehzeit-overwrite",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	overwrittenID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "gehzeit-ow-a@example.com", "Owa", 2)
	keptID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "gehzeit-ow-b@example.com", "Owb", 2)

	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Autor")
	testpkg.CreateTestPickupSchedule(t, env.db, overwrittenID, scheduleModels.WeekdayMonday, author.ID, "15:00")
	testpkg.CreateTestPickupSchedule(t, env.db, keptID, scheduleModels.WeekdayMonday, author.ID, "15:00")

	result, err := pickupTimeService(t, env).RolloutOfferingPickupTimes(ctx, offering.ID, []int64{keptID}, rolloutActorAccountID(t, env))
	require.NoError(t, err)
	assert.Equal(t, 1, result.SkippedStudents)

	overwritten := pickupRowsByWeekday(t, env, overwrittenID)[scheduleModels.WeekdayMonday]
	require.NotNil(t, overwritten)
	assert.Equal(t, "14:30", overwritten.PickupTime.Format("15:04"))
	assert.Equal(t, scheduleModels.PickupScheduleSourceCareOffering, overwritten.Source)

	kept := pickupRowsByWeekday(t, env, keptID)[scheduleModels.WeekdayMonday]
	require.NotNil(t, kept)
	assert.Equal(t, "15:00", kept.PickupTime.Format("15:04"))
	assert.Equal(t, scheduleModels.PickupScheduleSourceStaff, kept.Source)
}

func TestOfferingPickupRolloutPreview_ClassifiesWithoutWriting(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	offering := createPickupTimeOffering(t, env, "gehzeit-preview",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	freshID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "gehzeit-pv-a@example.com", "Pva", 2)
	conflictID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "gehzeit-pv-b@example.com", "Pvb", 2)

	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Konflikt")
	testpkg.CreateTestPickupSchedule(t, env.db, conflictID, scheduleModels.WeekdayMonday, author.ID, "15:00")

	preview, err := pickupTimeService(t, env).PreviewOfferingPickupRollout(ctx, offering.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, preview.AffectedStudents)
	assert.Equal(t, 1, preview.NewRows)
	require.Len(t, preview.Conflicts, 1)
	conflict := preview.Conflicts[0]
	assert.Equal(t, conflictID, conflict.StudentID)
	assert.Equal(t, scheduleModels.WeekdayMonday, conflict.Weekday)
	assert.Equal(t, "15:00", conflict.CurrentTime)
	assert.Equal(t, "14:30", conflict.NewTime)
	assert.NotEmpty(t, conflict.StudentName)

	// Dry run: no rows may have been written.
	assert.Empty(t, pickupRowsByWeekday(t, env, freshID))
	unchanged := pickupRowsByWeekday(t, env, conflictID)[scheduleModels.WeekdayMonday]
	require.NotNil(t, unchanged)
	assert.Equal(t, scheduleModels.PickupScheduleSourceStaff, unchanged.Source)
}

func TestOfferingPickupRollout_LatestTimeWinsAcrossOfferings(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	early := createPickupTimeOffering(t, env, "gehzeit-early",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	late := createPickupTimeOffering(t, env, "gehzeit-late",
		[]string{"mon"}, map[string]string{"mon": "16:00"})
	studentID, childID := submitAndApproveOfferingChild(t, env, early.ID, "gehzeit-latest@example.com", "Spät", 2)
	attachOfferingLink(t, env, childID, late.ID, []string{"mon"})

	_, err := pickupTimeService(t, env).RolloutOfferingPickupTimes(ctx, early.ID, nil, rolloutActorAccountID(t, env))
	require.NoError(t, err)

	monday := pickupRowsByWeekday(t, env, studentID)[scheduleModels.WeekdayMonday]
	require.NotNil(t, monday)
	assert.Equal(t, "16:00", monday.PickupTime.Format("15:04"),
		"the latest Gehzeit across the day's offerings must win")
	require.NotNil(t, monday.CareOfferingID)
	assert.Equal(t, late.ID, *monday.CareOfferingID)
}

func TestOfferingPickupReconcile_RemovesStaleSourcedRowsKeepsStaff(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	offering := createPickupTimeOffering(t, env, "gehzeit-stale",
		[]string{"mon", "tue"}, map[string]string{"mon": "14:30"})
	studentID, childID := submitAndApproveOfferingChild(t, env, offering.ID, "gehzeit-stale@example.com", "Stella", 2)

	svc := pickupTimeService(t, env)
	_, err := svc.RolloutOfferingPickupTimes(ctx, offering.ID, nil, rolloutActorAccountID(t, env))
	require.NoError(t, err)
	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Bleibt")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, scheduleModels.WeekdayTuesday, author.ID, "13:15")

	// The child loses the offering: close the link's validity in the past.
	yesterday := timezone.TodayDate().AddDays(-1)
	longAgo := timezone.TodayDate().AddDays(-30)
	_, err = env.db.NewUpdate().
		Model((*enrollmentModels.RequestChildOffering)(nil)).
		ModelTableExpr(`enrollment.request_child_offerings AS "request_child_offering"`).
		Set("valid_from = ?", longAgo).
		Set("valid_until = ?", yesterday).
		Where(`"request_child_offering".request_child_id = ?`, childID).
		Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.ReconcileOfferingPickupForStudents(ctx, []int64{studentID}, 0))

	rows := pickupRowsByWeekday(t, env, studentID)
	assert.NotContains(t, rows, scheduleModels.WeekdayMonday,
		"the offering-sourced row must be removed with the booking")
	tuesday := rows[scheduleModels.WeekdayTuesday]
	require.NotNil(t, tuesday, "manually maintained rows must survive reconciliation")
	assert.Equal(t, scheduleModels.PickupScheduleSourceStaff, tuesday.Source)
}

func TestOfferingPickupReset_RestoresOfferingTimeOrDeletes(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	offering := createPickupTimeOffering(t, env, "gehzeit-reset",
		[]string{"mon", "tue"}, map[string]string{"mon": "14:30"})
	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "gehzeit-reset@example.com", "Resa", 2)

	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Reset")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, scheduleModels.WeekdayMonday, author.ID, "15:00")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, scheduleModels.WeekdayTuesday, author.ID, "12:45")

	svc := pickupTimeService(t, env)
	row, err := svc.ResetStudentPickupDayToOffering(ctx, studentID, scheduleModels.WeekdayMonday, rolloutActorAccountID(t, env))
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, "14:30", row.PickupTime.Format("15:04"))
	assert.Equal(t, scheduleModels.PickupScheduleSourceCareOffering, row.Source)

	// Tuesday has no Angebots-Gehzeit: reset deletes the manual row.
	row, err = svc.ResetStudentPickupDayToOffering(ctx, studentID, scheduleModels.WeekdayTuesday, rolloutActorAccountID(t, env))
	require.NoError(t, err)
	assert.Nil(t, row)
	assert.NotContains(t, pickupRowsByWeekday(t, env, studentID), scheduleModels.WeekdayTuesday)
}

// attachOfferingLink adds a second approved offering link to an existing
// request child, the way a parent booking two offerings would have submitted.
func attachOfferingLink(t *testing.T, env *decisionTestEnv, requestChildID, offeringID int64, days []string) {
	t.Helper()
	link := &enrollmentModels.RequestChildOffering{
		RequestChildID: requestChildID,
		CareOfferingID: offeringID,
		SelectedDays:   days,
	}
	link.SetTenantID(1)
	require.NoError(t, env.repos.RequestChildOffering.Create(testpkg.TenantContext(1), link))
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("enrollment.request_child_offerings").
			Where("id = ?", link.ID).
			Exec(context.Background())
	})
}
