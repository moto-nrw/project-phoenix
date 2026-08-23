package schedule_test

import (
	"fmt"
	"testing"
	"time"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateSplit_PreservesDeviationsOnSuccessor(t *testing.T) {
	t.Parallel()

	effective := futureMonday(2)
	before := effective.AddDays(-7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, before)
	defer s.runCleanup(t)

	secondStaff := testpkg.CreateTestStaffForTenant(
		t, s.db, s.tenantID, "Second", fmt.Sprintf("Planned-%d", time.Now().UnixNano()),
	)
	substitute := testpkg.CreateTestStaffForTenant(
		t, s.db, s.tenantID, "Sub", fmt.Sprintf("Stitute-%d", time.Now().UnixNano()),
	)
	secondSupervisor := &activitiesModels.SupervisorPlanned{
		StaffID:   secondStaff.ID,
		GroupID:   s.template.ID,
		ValidFrom: before.AddDays(-30),
	}
	secondSupervisor.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(secondSupervisor).ModelTableExpr(`activities.supervisors`).Exec(s.ctx)
	require.NoError(t, err)
	s.extraCleanups = append(s.extraCleanups, func() {
	})

	_, err = s.factory.Materialization.MaterializeForTenant(
		s.ctx, before, effective, scheduleSvc.MaterializationSourceManual,
	)
	require.NoError(t, err)
	beforeInstances := listInstancesForDate(t, s.db, s.template.ID, before)
	targetInstances := listInstancesForDate(t, s.db, s.template.ID, effective)
	require.Len(t, beforeInstances, 1)
	require.Len(t, targetInstances, 1)
	beforeInstance := beforeInstances[0]
	targetInstance := targetInstances[0]

	beforeReason := "Fortbildung"
	_, err = s.db.NewUpdate().
		Model((*scheduleModels.InstanceStaff)(nil)).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Set("is_absent = ?", true).
		Set("absence_reason = ?", beforeReason).
		Where(`"instance_staff".instance_id = ?`, beforeInstance.ID).
		Where(`"instance_staff".staff_id = ?`, s.staffID).
		Where(`"instance_staff".tenant_id = ?`, s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	reasons := map[int64]string{
		s.staffID:      "Krank",
		secondStaff.ID: "Termin",
	}
	for staffID, reason := range reasons {
		_, err = s.db.NewUpdate().
			Model((*scheduleModels.InstanceStaff)(nil)).
			ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
			Set("is_absent = ?", true).
			Set("absence_reason = ?", reason).
			Where(`"instance_staff".instance_id = ?`, targetInstance.ID).
			Where(`"instance_staff".staff_id = ?`, staffID).
			Where(`"instance_staff".tenant_id = ?`, s.tenantID).
			Exec(s.ctx)
		require.NoError(t, err)
	}

	substituteRow := &scheduleModels.InstanceStaff{
		InstanceID:   targetInstance.ID,
		StaffID:      substitute.ID,
		IsSubstitute: true,
		IsPrimary:    true,
	}
	substituteRow.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(substituteRow).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	ackNote := "Eine Position bleibt bewusst offen"
	_, err = s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("understaffed_ack = ?", true).
		Set("understaffed_note = ?", ackNote).
		Set("required_staff = ?", 4).
		Where(`"activity_instance".id = ?`, targetInstance.ID).
		Where(`"activity_instance".tenant_id = ?`, s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	in := baseSplitInput(s, effective, fmt.Sprintf("Renamed-%d", time.Now().UnixNano()))
	in.MaterializeFrom = &effective
	in.MaterializeTo = &effective
	result, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)

	require.False(t, splitInstanceExists(t, s, targetInstance.ID), "split must replace the old future instance")
	successors := listInstancesForDate(t, s.db, result.NewTemplateID, effective)
	require.Len(t, successors, 1)
	successor := successors[0]
	require.NotEqual(t, targetInstance.ID, successor.ID)
	require.NotNil(t, successor.ActivityGroupID)
	assert.Equal(t, result.NewTemplateID, *successor.ActivityGroupID)
	assert.Equal(t, "15:00:00", successor.StartTime.Format("15:04:05"), "deviations must follow the sole occurrence when its time changes")
	assert.True(t, successor.UnderstaffedAck)
	require.NotNil(t, successor.UnderstaffedNote)
	assert.Equal(t, ackNote, *successor.UnderstaffedNote)
	require.NotNil(t, successor.RequiredStaff)
	assert.Equal(t, 4, *successor.RequiredStaff)

	rows := loadInstanceStaffRows(t, s.db, s.ctx, successor.ID)
	require.Len(t, rows, 3)
	for _, row := range rows {
		switch row.StaffID {
		case s.staffID, secondStaff.ID:
			assert.False(t, row.IsSubstitute)
			assert.True(t, row.IsAbsent)
			require.NotNil(t, row.AbsenceReason)
			assert.Equal(t, reasons[row.StaffID], *row.AbsenceReason)
		case substitute.ID:
			assert.True(t, row.IsSubstitute)
			assert.False(t, row.IsAbsent)
			assert.True(t, row.IsPrimary)
		default:
			t.Fatalf("unexpected staff row for staff %d", row.StaffID)
		}
	}

	require.True(t, splitInstanceExists(t, s, beforeInstance.ID), "instance before the split date must survive")
	beforeRows := loadInstanceStaffRows(t, s.db, s.ctx, beforeInstance.ID)
	require.Len(t, beforeRows, 2)
	var beforeDeviationFound bool
	for _, row := range beforeRows {
		if row.StaffID == s.staffID {
			beforeDeviationFound = true
			assert.True(t, row.IsAbsent)
			require.NotNil(t, row.AbsenceReason)
			assert.Equal(t, beforeReason, *row.AbsenceReason)
		}
	}
	assert.True(t, beforeDeviationFound)
}

func TestTemplateSplit_DropsDeviationWhenWeekdayNoLongerExists(t *testing.T) {
	t.Parallel()

	effective := futureMonday(2)
	tuesday := effective.AddDays(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)

	_, err := s.factory.Materialization.MaterializeForTenant(
		s.ctx, effective, effective, scheduleSvc.MaterializationSourceManual,
	)
	require.NoError(t, err)
	oldInstances := listInstancesForDate(t, s.db, s.template.ID, effective)
	require.Len(t, oldInstances, 1)
	old := oldInstances[0]

	_, err = s.db.NewUpdate().
		Model((*scheduleModels.InstanceStaff)(nil)).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Set("is_absent = ?", true).
		Where(`"instance_staff".instance_id = ?`, old.ID).
		Where(`"instance_staff".staff_id = ?`, s.staffID).
		Where(`"instance_staff".tenant_id = ?`, s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	in := baseSplitInput(s, effective, fmt.Sprintf("Tuesday-%d", time.Now().UnixNano()))
	in.Weekdays = []int{activitiesModels.WeekdayTuesday}
	in.MaterializeFrom = &effective
	in.MaterializeTo = &tuesday
	result, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)

	assert.Empty(t, listInstancesForDate(t, s.db, result.NewTemplateID, effective))
	tuesdayInstances := listInstancesForDate(t, s.db, result.NewTemplateID, tuesday)
	require.Len(t, tuesdayInstances, 1)
	rows := loadInstanceStaffRows(t, s.db, s.ctx, tuesdayInstances[0].ID)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].IsAbsent, "a removed Monday's deviation must not move to Tuesday")
	assert.False(t, rows[0].IsSubstitute)
}
