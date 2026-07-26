// Hermetic integration tests for the ReplanWeek deviation-reapply path (#1840).
//
// Focus: a substitute row only ever exists to cover an absent planned position.
// When the "edit all occurrences" flow removes the absent employee from the
// template, re-plan regenerates the occurrence without that position — so the
// substitute is an orphan and must NOT be recreated. When the absent employee
// stays on the template, the absence is reapplied and the substitute is
// recreated. Both cases are exercised end-to-end against the real materializer.
package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// loadInstanceStaffRows returns every instance_staff row for one instance.
func loadInstanceStaffRows(t *testing.T, db *bun.DB, ctx context.Context, instanceID int64) []*scheduleModels.InstanceStaff {
	t.Helper()
	var rows []*scheduleModels.InstanceStaff
	err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Where(`"instance_staff".instance_id = ?`, instanceID).
		Order("staff_id ASC").
		Scan(ctx)
	require.NoError(t, err)
	return rows
}

// applyDeviation flips the planned supervisor's row to absent and inserts a
// substitute row for subStaffID — the persisted state the /deviations and
// /substitute endpoints produce. Returns nothing; asserts on write errors.
func applyDeviation(t *testing.T, s *scenarioSetup, instanceID, absentStaffID, subStaffID int64) {
	t.Helper()
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.InstanceStaff)(nil)).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Set("is_absent = ?", true).
		Where(`"instance_staff".instance_id = ?`, instanceID).
		Where(`"instance_staff".staff_id = ?`, absentStaffID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	sub := &scheduleModels.InstanceStaff{InstanceID: instanceID, StaffID: subStaffID, IsSubstitute: true}
	sub.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(sub).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)
}

// seedDeviatedOccurrence materializes the single Monday occurrence, adds a
// second staff to act as substitute, and applies the absent+substitute
// deviation. Returns the substitute staff id.
func seedDeviatedOccurrence(t *testing.T, s *scenarioSetup, date timezone.Date) (substituteStaffID int64) {
	t.Helper()
	_, err := s.factory.Materialization.MaterializeForTenant(s.ctx, date, date, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	instances := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, instances, 1, "one occurrence materialized")
	instanceID := instances[0].ID
	s.registerCleanup("schedule.activity_instances", instanceID)

	subStaff := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Sub", fmt.Sprintf("Stitute-%d", time.Now().UnixNano()))
	// extraCleanups run AFTER activity_instances (and their cascaded
	// instance_staff) are removed, so the staff/person rows have no referencing
	// child by then.
	s.extraCleanups = append(s.extraCleanups, func() {
		testpkg.CleanupTableRecords(t, s.db, "users.staff", subStaff.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.persons", subStaff.PersonID)
	})

	applyDeviation(t, s, instanceID, s.staffID, subStaff.ID)
	return subStaff.ID
}

// TestInstance_ReplanWeek_DropsOrphanedSubstituteWhenAbsentStaffRemoved covers
// the #1840 finding: removing the absent employee from the template must not
// leave their substitute behind as an extra supervisor on the regenerated
// block.
func TestInstance_ReplanWeek_DropsOrphanedSubstituteWhenAbsentStaffRemoved(t *testing.T) {
	date := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, date)
	defer s.runCleanup(t)

	subStaffID := seedDeviatedOccurrence(t, s, date)

	// Remove the absent employee from the template so re-plan regenerates the
	// occurrence WITHOUT their planned position.
	testpkg.CleanupTableRecords(t, s.db, "activities.supervisors", s.supervisorIDs...)

	_, err := s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID, nil)
	require.NoError(t, err)

	regen := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, regen, 1, "re-plan regenerated the occurrence")
	s.registerCleanup("schedule.activity_instances", regen[0].ID)

	rows := loadInstanceStaffRows(t, s.db, s.ctx, regen[0].ID)
	for _, r := range rows {
		assert.NotEqual(t, subStaffID, r.StaffID,
			"orphaned substitute must NOT be recreated once the absent position is gone")
	}
	assert.Empty(t, rows,
		"template lost its only supervisor, so the regenerated block has no staff at all")
}

// TestInstance_ReplanWeek_ReapplySubstituteWhenAbsenceRestored is the positive
// counterpart: when the absent employee stays on the template, the absence is
// reapplied and the substitute is recreated.
func TestInstance_ReplanWeek_ReapplySubstituteWhenAbsenceRestored(t *testing.T) {
	date := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, date)
	defer s.runCleanup(t)

	subStaffID := seedDeviatedOccurrence(t, s, date)

	// Template unchanged: the absent employee is still planned.
	_, err := s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID, nil)
	require.NoError(t, err)

	regen := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, regen, 1)
	s.registerCleanup("schedule.activity_instances", regen[0].ID)

	rows := loadInstanceStaffRows(t, s.db, s.ctx, regen[0].ID)
	var absentReapplied, substituteRecreated bool
	for _, r := range rows {
		if r.StaffID == s.staffID && r.IsAbsent && !r.IsSubstitute {
			absentReapplied = true
		}
		if r.StaffID == subStaffID && r.IsSubstitute {
			substituteRecreated = true
		}
	}
	assert.True(t, absentReapplied, "planned absence must be reapplied onto the regenerated roster")
	assert.True(t, substituteRecreated, "substitute covering a restored absence must be recreated")
}

// TestInstance_ReplanWeek_CapsRecreatedSubstitutesToSurvivingAbsences covers the
// #1840 finding for the PARTIAL-removal case: two planned employees were each
// absent with their own substitute; the series edit removes ONE of them from the
// template. One absence still survives, so absencesReapplied stays positive — but
// only the surviving absence's substitute may return. Recreating both would leave
// the removed position's substitute as an orphaned extra supervisor and overstaff
// the block. The recreate must be capped at the number of surviving absences.
func TestInstance_ReplanWeek_CapsRecreatedSubstitutesToSurvivingAbsences(t *testing.T) {
	date := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, date)
	defer s.runCleanup(t)

	// Second planned supervisor B so the occurrence has TWO absent-able positions.
	staffB := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Second", fmt.Sprintf("Planned-%d", time.Now().UnixNano()))
	supB := &activitiesModels.SupervisorPlanned{StaffID: staffB.ID, GroupID: s.template.ID, ValidFrom: date.AddDays(-30)}
	supB.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(supB).ModelTableExpr(`activities.supervisors`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("activities.supervisors", supB.ID)

	subX := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "SubX", fmt.Sprintf("X-%d", time.Now().UnixNano()))
	subY := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "SubY", fmt.Sprintf("Y-%d", time.Now().UnixNano()))
	s.extraCleanups = append(s.extraCleanups, func() {
		testpkg.CleanupTableRecords(t, s.db, "users.staff", staffB.ID, subX.ID, subY.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.persons", staffB.PersonID, subX.PersonID, subY.PersonID)
	})

	// Materialize the single Monday occurrence — now with two planned staff.
	_, err = s.factory.Materialization.MaterializeForTenant(s.ctx, date, date, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	instances := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, instances, 1, "one occurrence, two planned staff")
	instanceID := instances[0].ID
	s.registerCleanup("schedule.activity_instances", instanceID)

	// A (s.staffID) absent → covered by X; B absent → covered by Y.
	applyDeviation(t, s, instanceID, s.staffID, subX.ID)
	applyDeviation(t, s, instanceID, staffB.ID, subY.ID)

	// Remove ONLY staff B from the template; A's position stays.
	_, err = s.db.NewDelete().Model((*activitiesModels.SupervisorPlanned)(nil)).
		ModelTableExpr(`activities.supervisors`).
		Where("id = ?", supB.ID).Where("tenant_id = ?", s.tenantID).Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID, nil)
	require.NoError(t, err)

	regen := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, regen, 1, "re-plan regenerated the occurrence")
	s.registerCleanup("schedule.activity_instances", regen[0].ID)

	rows := loadInstanceStaffRows(t, s.db, s.ctx, regen[0].ID)
	substituteCount, aAbsent := 0, false
	for _, r := range rows {
		if r.IsSubstitute {
			substituteCount++
		}
		if r.StaffID == s.staffID && r.IsAbsent && !r.IsSubstitute {
			aAbsent = true
		}
		assert.NotEqual(t, staffB.ID, r.StaffID, "removed planned staff must not reappear on the regenerated block")
	}
	assert.True(t, aAbsent, "the surviving planned absence must be reapplied")
	assert.Equal(t, 1, substituteCount,
		"only the surviving absence's substitute may return; the orphaned one is dropped so the block is not overstaffed")
}

// TestInstance_ReplanWeek_DoesNotMoveDeletedSlotDeviationToSurvivor covers the
// #1840 finding for the multi-slot cardinality case: a group has TWO occurrences
// on one day (two clock slots) but only one carries a deviation. A series edit
// deletes the deviated slot while the other survives. The deleted slot's
// override must NOT be transplanted onto the survivor, whose start_time differs —
// counting snapshots (rather than the original occurrence cardinality) would have
// misread this day as "sole" and moved the override.
func TestInstance_ReplanWeek_DoesNotMoveDeletedSlotDeviationToSurvivor(t *testing.T) {
	date := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, date)
	defer s.runCleanup(t)

	// Second slot on the SAME weekday at a different clock time (10:00 vs the
	// scenario's 08:00), so the two occurrences disambiguate by start_time.
	tf2Start := time.Date(2000, 1, 1, 10, 0, 0, 0, time.Local)
	tf2End := time.Date(2000, 1, 1, 11, 0, 0, 0, time.Local)
	tf2 := &scheduleModels.Timeframe{StartTime: tf2Start, EndTime: &tf2End, IsActive: true, Description: fmt.Sprintf("Slot2-%d", time.Now().UnixNano())}
	tf2.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(tf2).ModelTableExpr(`schedule.timeframes`).Exec(s.ctx)
	require.NoError(t, err)
	s.extraCleanups = append(s.extraCleanups, func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.timeframes", tf2.ID)
	})

	sched2 := &activitiesModels.Schedule{Weekday: activitiesModels.WeekdayMonday, TimeframeID: &tf2.ID, ActivityGroupID: s.template.ID, WeekPattern: 0}
	sched2.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(sched2).ModelTableExpr(`activities.schedules`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("activities.schedules", sched2.ID)

	// Materialize both slots.
	_, err = s.factory.Materialization.MaterializeForTenant(s.ctx, date, date, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	instances := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, instances, 2, "two slots materialized")
	for _, inst := range instances {
		s.registerCleanup("schedule.activity_instances", inst.ID)
	}

	// tf2 (10:00) always materializes to the later start_time than tf1 (08:00),
	// regardless of how the TIME column round-trips, so the later occurrence is
	// slot2 — the one backed by sched2, which we delete below.
	slot2 := instances[0]
	for _, inst := range instances {
		if inst.StartTime.After(slot2.StartTime) {
			slot2 = inst
		}
	}
	require.NotEqual(t, instances[0].StartTime.Format("15:04:05"), instances[1].StartTime.Format("15:04:05"),
		"the two slots must have distinct start_times")

	subStaff := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Sub", fmt.Sprintf("Slot2-%d", time.Now().UnixNano()))
	s.extraCleanups = append(s.extraCleanups, func() {
		testpkg.CleanupTableRecords(t, s.db, "users.staff", subStaff.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.persons", subStaff.PersonID)
	})
	// Deviate ONLY slot2: mark the planned supervisor absent + assign a substitute.
	applyDeviation(t, s, slot2.ID, s.staffID, subStaff.ID)

	// Delete slot2's schedule so re-plan regenerates ONLY slot1.
	_, err = s.db.NewDelete().Model((*activitiesModels.Schedule)(nil)).
		ModelTableExpr(`activities.schedules`).
		Where("id = ?", sched2.ID).Where("tenant_id = ?", s.tenantID).Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID, nil)
	require.NoError(t, err)

	regen := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, regen, 1, "only slot1 regenerated")
	s.registerCleanup("schedule.activity_instances", regen[0].ID)
	assert.NotEqual(t, slot2.StartTime.Format("15:04:05"), regen[0].StartTime.Format("15:04:05"),
		"the surviving slot is the un-deviated one (tf1), not the deleted deviated slot")

	rows := loadInstanceStaffRows(t, s.db, s.ctx, regen[0].ID)
	plannedPresent := false
	for _, r := range rows {
		assert.False(t, r.IsAbsent, "the deleted slot's absence must not land on the surviving slot")
		assert.False(t, r.IsSubstitute, "the deleted slot's substitute must not be recreated on the surviving slot")
		assert.NotEqual(t, subStaff.ID, r.StaffID, "orphaned substitute must not appear on the surviving slot")
		if r.StaffID == s.staffID && !r.IsAbsent && !r.IsSubstitute {
			plannedPresent = true
		}
	}
	assert.True(t, plannedPresent, "the surviving slot keeps its planned supervisor, un-deviated")
}

// TestInstance_ReplanWeek_PreservesRequiredStaffPin covers the #1839 follow-up
// to the #1840 snapshot mechanism: a per-occurrence Personalbedarf pin must
// survive a series edit's re-plan. The test also proves the inherit-at-read
// contract materialization relies on: the template's own override is NOT
// copied onto materialized rows, so template-level edits keep propagating
// while a non-NULL instance value is always a deliberate pin.
func TestInstance_ReplanWeek_PreservesRequiredStaffPin(t *testing.T) {
	date := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, date)
	defer s.runCleanup(t)

	// Give the template its own override so the copy-vs-inherit distinction
	// is actually observable on the materialized row.
	_, err := s.db.NewUpdate().
		Model((*activitiesModels.Group)(nil)).
		ModelTableExpr(`activities.groups AS "group"`).
		Set("required_staff = ?", 3).
		Where(`"group".id = ?`, s.template.ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.factory.Materialization.MaterializeForTenant(s.ctx, date, date, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	instances := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, instances, 1, "one occurrence materialized")
	s.registerCleanup("schedule.activity_instances", instances[0].ID)
	require.Nil(t, instances[0].RequiredStaff,
		"materialized rows must NOT copy the template override (inherit-at-read)")

	// Pin this single occurrence to 5 — the persisted state of a
	// "Nur diesen Termin" edit of Benötigtes Personal.
	_, err = s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("required_staff = ?", 5).
		Where(`"activity_instance".id = ?`, instances[0].ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	// The series edit path: re-plan regenerates the occurrence from the template.
	_, err = s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID, nil)
	require.NoError(t, err)

	regen := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, regen, 1, "re-plan regenerated the occurrence")
	s.registerCleanup("schedule.activity_instances", regen[0].ID)
	require.NotNil(t, regen[0].RequiredStaff, "per-occurrence pin must survive the re-plan")
	assert.Equal(t, 5, *regen[0].RequiredStaff)

	// A second re-plan after clearing the pin must leave the row NULL again —
	// nothing stale in the snapshot path re-pins it.
	_, err = s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("required_staff = NULL").
		Where(`"activity_instance".id = ?`, regen[0].ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID, nil)
	require.NoError(t, err)

	regen2 := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, regen2, 1)
	s.registerCleanup("schedule.activity_instances", regen2[0].ID)
	assert.Nil(t, regen2[0].RequiredStaff,
		"an un-pinned occurrence stays NULL after re-plan so template edits keep propagating")
}
