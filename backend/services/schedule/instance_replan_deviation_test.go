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

	_, err := s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID)
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
	_, err := s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID)
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
