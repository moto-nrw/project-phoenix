// Hermetic integration tests for the moved-slot exception (UpdatePlanned).
//
// Moving a template-backed instance to a different date or start time vacates
// its original (template, date, start) key; without a consuming exception the
// next materialization run re-creates the original occurrence as a duplicate.
// UpdatePlanned therefore writes a cancelled activity_exception for the
// ORIGINAL date. Covered here:
//
//   - date move writes the exception AND a re-materialization of the original
//     date creates nothing (skipped_exception)
//   - start-time-only move writes the exception too
//   - updates that keep date+start_time write nothing
//   - spontaneous instances never write exceptions
//
// Fixtures via makeScenario (materialization_integration_test.go) — unique
// tenant per test, cleanup via scenarioSetup.runCleanup. No hardcoded IDs.
package schedule_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadExceptions returns all activity_exceptions rows for the template,
// registering them for cleanup.
func loadExceptions(t *testing.T, s *scenarioSetup, groupID int64) []*scheduleModels.ActivityException {
	t.Helper()
	var rows []*scheduleModels.ActivityException
	err := s.db.NewSelect().Model(&rows).
		ModelTableExpr(`schedule.activity_exceptions AS "activity_exception"`).
		Where(`"activity_exception".activity_group_id = ?`, groupID).
		Where(`"activity_exception".tenant_id = ?`, s.tenantID).
		Order("exception_date ASC").
		Scan(s.ctx)
	require.NoError(t, err)
	return rows
}

// clockInput converts a scanned TIME value into the 2000-01-01-anchored shape
// the update handlers send, optionally shifted by hours.
func clockInput(scanned time.Time, addHours int) time.Time {
	return time.Date(2000, 1, 1, scanned.Hour()+addHours, scanned.Minute(), scanned.Second(), 0, time.UTC)
}

func moveInput(s *scenarioSetup, inst *scheduleModels.ActivityInstance, date timezone.Date, startShiftHours int) scheduleSvc.UpdateInstanceInput {
	return scheduleSvc.UpdateInstanceInput{
		Date:            date,
		StartTime:       clockInput(inst.StartTime, startShiftHours),
		EndTime:         clockInput(inst.EndTime, startShiftHours),
		Title:           inst.Title,
		RoomID:          s.roomID,
		ActivityGroupID: inst.ActivityGroupID,
	}
}

func TestUpdatePlanned_DateMove_WritesExceptionAndBlocksRematerialization(t *testing.T) {
	t.Parallel()

	origDate := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, origDate)
	defer s.runCleanup(t)

	r0, err := s.svc.MaterializeForTenant(s.ctx, origDate, origDate, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 1, r0.InstancesCreated)
	rows := listInstancesForDate(t, s.db, s.template.ID, origDate)
	require.Len(t, rows, 1)
	inst := rows[0]

	// Move the occurrence to Tuesday, keeping times and template binding.
	newDate := origDate.AddDays(1)
	_, err = s.factory.Instance.UpdatePlanned(s.ctx, inst.ID, moveInput(s, inst, newDate, 0), nil)
	require.NoError(t, err)

	excs := loadExceptions(t, s, s.template.ID)
	require.Len(t, excs, 1, "date move must consume the original slot")
	assert.Equal(t, origDate, excs[0].ExceptionDate)
	assert.Equal(t, scheduleModels.ActivityExceptionCancelled, excs[0].ExceptionType)
	require.NotNil(t, excs[0].Reason)
	assert.NotEmpty(t, *excs[0].Reason)

	// Re-materializing the original date must NOT resurrect the old slot.
	r1, err := s.svc.MaterializeForTenant(s.ctx, origDate, origDate, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r1.InstancesCreated, "original slot must stay consumed")
	assert.Equal(t, 1, r1.CandidatesSkippedException)
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, origDate))
}

func TestDeletePlanned_WritesExceptionAndBlocksRematerialization(t *testing.T) {
	t.Parallel()

	origDate := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, origDate)
	defer s.runCleanup(t)

	r0, err := s.svc.MaterializeForTenant(s.ctx, origDate, origDate, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 1, r0.InstancesCreated)
	rows := listInstancesForDate(t, s.db, s.template.ID, origDate)
	require.Len(t, rows, 1)
	inst := rows[0]

	require.NoError(t, s.factory.Instance.DeleteCancelled(s.ctx, inst.ID))

	excs := loadExceptions(t, s, s.template.ID)
	require.Len(t, excs, 1, "delete must consume the original slot")
	assert.Equal(t, origDate, excs[0].ExceptionDate)
	assert.Equal(t, scheduleModels.ActivityExceptionCancelled, excs[0].ExceptionType)

	r1, err := s.svc.MaterializeForTenant(s.ctx, origDate, origDate, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r1.InstancesCreated, "deleted slot must stay consumed")
	assert.Equal(t, 1, r1.CandidatesSkippedException)
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, origDate))
}

func TestUpdatePlanned_StartTimeOnlyMove_WritesException(t *testing.T) {
	t.Parallel()

	origDate := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, origDate)
	defer s.runCleanup(t)

	r0, err := s.svc.MaterializeForTenant(s.ctx, origDate, origDate, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 1, r0.InstancesCreated)
	rows := listInstancesForDate(t, s.db, s.template.ID, origDate)
	require.Len(t, rows, 1)
	inst := rows[0]

	// Same date, start shifted by one hour.
	_, err = s.factory.Instance.UpdatePlanned(s.ctx, inst.ID, moveInput(s, inst, origDate, 1), nil)
	require.NoError(t, err)

	excs := loadExceptions(t, s, s.template.ID)
	require.Len(t, excs, 1, "start-time move vacates the original slot key too")
	assert.Equal(t, origDate, excs[0].ExceptionDate)
	assert.Equal(t, scheduleModels.ActivityExceptionCancelled, excs[0].ExceptionType)

	// The exception consumes the (template, date): re-materialization must
	// not re-create the original start time next to the moved instance.
	r1, err := s.svc.MaterializeForTenant(s.ctx, origDate, origDate, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r1.InstancesCreated)
	assert.Equal(t, 1, r1.CandidatesSkippedException)
	after := listInstancesForDate(t, s.db, s.template.ID, origDate)
	require.Len(t, after, 1, "only the moved instance remains on the original date")
	assert.Equal(t, inst.ID, after[0].ID)
}

func TestUpdatePlanned_NoDateOrTimeChange_WritesNothing(t *testing.T) {
	t.Parallel()

	origDate := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, origDate)
	defer s.runCleanup(t)

	r0, err := s.svc.MaterializeForTenant(s.ctx, origDate, origDate, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 1, r0.InstancesCreated)
	rows := listInstancesForDate(t, s.db, s.template.ID, origDate)
	require.Len(t, rows, 1)
	inst := rows[0]

	// Title and end_time change only — the (template, date, start) key stays.
	in := moveInput(s, inst, origDate, 0)
	in.Title = fmt.Sprintf("Umbenannt-%d", time.Now().UnixNano())
	in.EndTime = clockInput(inst.EndTime, 1)
	_, err = s.factory.Instance.UpdatePlanned(s.ctx, inst.ID, in, nil)
	require.NoError(t, err)

	assert.Empty(t, loadExceptions(t, s, s.template.ID),
		"no exception when neither date nor start_time changed")
}

func TestUpdatePlanned_SpontaneousMove_WritesNothing(t *testing.T) {
	t.Parallel()

	origDate := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, origDate)
	defer s.runCleanup(t)

	// Spontaneous instance: no template binding before (or after) the edit.
	spontaneousID := splitInsertInstance(t, s, nil, origDate, scheduleModels.InstanceStatusPlanned, true, 9)
	inst := &scheduleModels.ActivityInstance{}
	err := s.db.NewSelect().Model(inst).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".id = ?`, spontaneousID).
		Scan(s.ctx)
	require.NoError(t, err)

	_, err = s.factory.Instance.UpdatePlanned(s.ctx, spontaneousID, moveInput(s, inst, origDate.AddDays(1), 0), nil)
	require.NoError(t, err)

	var count int
	err = s.db.NewSelect().
		TableExpr(`schedule.activity_exceptions AS "ae"`).
		ColumnExpr("COUNT(*)").
		Where(`"ae".tenant_id = ?`, s.tenantID).
		Scan(s.ctx, &count)
	require.NoError(t, err)
	assert.Zero(t, count, "spontaneous instances never write slot exceptions")
}
