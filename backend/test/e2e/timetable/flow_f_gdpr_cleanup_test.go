package e2e_timetable

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestFlowF_GDPRCleanup covers the retention-based cleanup job:
//   - Seed old instances (far past) + fresh ones + an exception row.
//   - Set gdpr.timetable_retention_days to 30 so the cutoff falls between them.
//   - Run the cleanup service; verify old rows + CASCADE children gone, fresh
//     rows untouched, audit.data_deletions row per affected student.
//   - Re-run (idempotency): zero deletions.
func TestFlowF_GDPRCleanup(t *testing.T) {
	t.Parallel()

	s := setupTimetableScenarioRoute(t)
	defer s.teardown()

	// --- Fixtures: students, room, old + fresh instances -------------------
	room := testpkg.CreateTestRoom(t, s.db, "FlowF-Room")
	s.extraCleanup = append(s.extraCleanup, func() {
	})

	studA := testpkg.CreateTestStudent(t, s.db, "Anna", "FlowF", "3a")
	studB := testpkg.CreateTestStudent(t, s.db, "Bob", "FlowF", "3a")
	s.extraCleanup = append(s.extraCleanup, func() {
	})

	// Old instance: 60 days ago. Should be deleted.
	oldDate := timezone.TodayDate().AddDays(-60)
	// Fresh instance: 5 days ago. Should survive.
	freshDate := timezone.TodayDate().AddDays(-5)

	// Old exception on a separate past date (template scope; no student audit).
	oldExceptionDate := timezone.TodayDate().AddDays(-70)

	oldInstance := testpkg.CreateTestActivityInstance(t, s.db, oldDate, room.ID,
		testpkg.ActivityInstanceOpts{Title: "FlowF-Old", IsSpontaneous: true})
	freshInstance := testpkg.CreateTestActivityInstance(t, s.db, freshDate, room.ID,
		testpkg.ActivityInstanceOpts{Title: "FlowF-Fresh", IsSpontaneous: true})

	// Old instance has two students: both get an audit row on cleanup, one
	// of them even carries a note to confirm the sensitive field gets wiped
	// via CASCADE.
	_ = testpkg.CreateTestInstanceStudent(t, s.db, oldInstance.ID, studA.ID, "")
	isB := testpkg.CreateTestInstanceStudent(t, s.db, oldInstance.ID, studB.ID, "")
	note := "sensitive context — must be CASCADE-deleted"
	_, err := s.db.NewUpdate().
		Model((*scheduleModel.InstanceStudent)(nil)).
		ModelTableExpr(`schedule.instance_students`).
		Set("note = ?", note).
		Where("id = ?", isB.ID).
		Exec(s.tenantCtx())
	require.NoError(t, err, "annotate note on instance_student")

	// Fresh instance also has a student; must survive.
	_ = testpkg.CreateTestInstanceStudent(t, s.db, freshInstance.ID, studA.ID, "")

	// Cleanup registration — old instance IDs may vanish, but re-registration
	// is idempotent from the teardown order's perspective (DELETEs skip).
	s.registerCleanup("schedule.activity_instances", oldInstance.ID, freshInstance.ID)

	// Old exception (template-scoped cleanup).
	oldExc := &scheduleModel.ActivityException{
		ActivityGroupID: 0, // doesn't reference a real template; tenant-scope + date drive the delete
		ExceptionDate:   oldExceptionDate,
		ExceptionType:   scheduleModel.ActivityExceptionCancelled,
	}
	oldExc.SetTenantID(s.primaryTenant)
	// activity_group_id is FK-constrained; point it at a throwaway template.
	tmpl := s.buildTemplate(templateSpec{
		name:       "FlowF-TemplatePlaceholder",
		weekday:    1,
		startHHMM:  "09:00",
		endHHMM:    "10:00",
		roomID:     room.ID,
		staffIDs:   []int64{studAStaffProxy(t, s)}, // need at least one staff for buildTemplate
		studentIDs: []int64{},
	})
	oldExc.ActivityGroupID = tmpl.group.ID
	_, err = s.db.NewInsert().Model(oldExc).
		ModelTableExpr(`schedule.activity_exceptions`).Exec(s.tenantCtx())
	require.NoError(t, err, "insert old activity_exception")
	// Exception cleanup is FK'd to template: if cleanup doesn't reach it,
	// teardown will. Register explicitly so we don't leak.
	s.registerCleanup("schedule.activity_exceptions", oldExc.ID)

	// --- Set retention to 30 days via the settings service ----------------
	setRetentionDays(t, s, 30)

	// --- Preview first (dry-run) ------------------------------------------
	preview := runInTenantTx(t, s, func(ctx context.Context) (any, error) {
		return s.previewTimetableCleanup(ctx)
	}).(*scheduleSvc.TimetableCleanupPreview)
	assert.GreaterOrEqual(t, preview.InstancesToDelete, 1,
		"at least the old instance is previewed for delete")
	assert.GreaterOrEqual(t, preview.ExceptionsToDelete, 1,
		"at least the old exception is previewed for delete")
	assert.Equal(t, 30, preview.RetentionDays)

	// --- Run the actual cleanup -------------------------------------------
	result := runInTenantTx(t, s, func(ctx context.Context) (any, error) {
		return s.cleanupTimetable(ctx)
	}).(*scheduleSvc.TimetableCleanupResult)

	assert.True(t, result.Success)
	assert.Equal(t, 1, result.InstancesDeleted, "exactly one old instance deleted")
	assert.Equal(t, 1, result.ExceptionsDeleted, "exactly one old exception deleted")
	assert.Equal(t, 2, result.StudentsAffected, "studA + studB, both had old instance_students rows")

	// --- Verify DB state --------------------------------------------------
	assert.False(t, instanceExists(t, s, oldInstance.ID), "old instance deleted")
	assert.True(t, instanceExists(t, s, freshInstance.ID), "fresh instance survived")
	assert.False(t, exceptionExists(t, s, oldExc.ID), "old exception deleted")

	// instance_students CASCADE delete for old instance
	count := countInstanceStudents(t, s, oldInstance.ID)
	assert.Equal(t, 0, count, "instance_students CASCADE-deleted with old instance")

	// Audit rows: one per affected student, deletion_type='timetable_retention'.
	auditCount := countAuditRows(t, s, studA.ID, auditModel.DeletionTypeTimetableRetention)
	assert.Equal(t, 1, auditCount, "studA audit row")
	auditCount = countAuditRows(t, s, studB.ID, auditModel.DeletionTypeTimetableRetention)
	assert.Equal(t, 1, auditCount, "studB audit row")

	// --- Idempotency: second run deletes nothing --------------------------
	result2 := runInTenantTx(t, s, func(ctx context.Context) (any, error) {
		return s.cleanupTimetable(ctx)
	}).(*scheduleSvc.TimetableCleanupResult)
	assert.True(t, result2.Success)
	assert.Equal(t, 0, result2.InstancesDeleted, "idempotent: nothing left to delete")
	assert.Equal(t, 0, result2.ExceptionsDeleted)
	assert.Equal(t, 0, result2.StudentsAffected, "no new audit rows")

	// --- Tenant isolation: cleanup on T2 does not touch T1 data -----------
	// Seed an old instance in T2; run cleanup there; verify T1 fresh
	// instance is still intact.
	t2Scope := tenant.WithTenantID(context.Background(), s.secondaryTenant)
	err = testpkg.WithTenantTx(t, t2Scope, s.db, s.secondaryTenant, func(ctx context.Context, _ bun.Tx) error {
		_, err := s.cleanupTimetable(ctx)
		return err
	})
	require.NoError(t, err, "T2 cleanup ran cleanly")
	assert.True(t, instanceExists(t, s, freshInstance.ID),
		"T1 fresh instance untouched after T2 cleanup")

	// cleanup: wipe audit rows we created so teardown doesn't report them
	// from stale state.
	_, _ = s.db.NewDelete().
		TableExpr("audit.data_deletions").
		Where("tenant_id = ?", s.primaryTenant).
		Where("student_id IN (?, ?)", studA.ID, studB.ID).
		Exec(s.tenantCtx())
}

// runInTenantTx wraps fn in WithTenantTx against the primary tenant and
// returns fn's result.
func runInTenantTx(t *testing.T, s *scenario, fn func(ctx context.Context) (any, error)) any {
	t.Helper()
	var out any
	err := testpkg.WithTenantTx(t, context.Background(), s.db, s.primaryTenant,
		func(ctx context.Context, _ bun.Tx) error {
			res, err := fn(ctx)
			out = res
			return err
		})
	require.NoError(t, err, "WithTenantTx fn")
	return out
}

// setRetentionDays writes `gdpr.timetable_retention_days` for the primary
// tenant via the settings service (admin permissions inlined for the write).
func setRetentionDays(t *testing.T, s *scenario, days int) {
	t.Helper()
	perms := []string{
		permissions.ConfigManage,
		permissions.ConfigUpdate,
	}
	err := testpkg.WithTenantTx(t, context.Background(), s.db, s.primaryTenant,
		func(ctx context.Context, _ bun.Tx) error {
			return s.resource.SettingsService.SetValue(ctx, "gdpr.timetable_retention_days", days, nil, perms)
		})
	require.NoError(t, err, "set gdpr.timetable_retention_days=%d", days)
	s.extraCleanup = append(s.extraCleanup, func() {
		// Reset the override so a subsequent test sees the registry default.
		_ = testpkg.WithTenantTx(t, context.Background(), s.db, s.primaryTenant,
			func(ctx context.Context, _ bun.Tx) error {
				return s.resource.SettingsService.ResetValue(ctx, "gdpr.timetable_retention_days", nil, perms)
			})
	})
}

// studAStaffProxy creates a single staff member used only to satisfy
// buildTemplate's FK requirements for the placeholder-template exception
// seeding.
func studAStaffProxy(t *testing.T, s *scenario) int64 {
	t.Helper()
	st := testpkg.CreateTestStaff(t, s.db, "FlowF", fmt.Sprintf("Proxy-%d", time.Now().UnixNano()))
	s.extraCleanup = append(s.extraCleanup, func() {
	})
	return st.ID
}

// instanceExists checks whether an activity_instance row is still in the DB.
func instanceExists(t *testing.T, s *scenario, id int64) bool {
	t.Helper()
	n, err := s.db.NewSelect().
		Model((*scheduleModel.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".id = ?`, id).
		Where(`"activity_instance".tenant_id = ?`, s.primaryTenant).
		Count(s.tenantCtx())
	require.NoError(t, err)
	return n > 0
}

// exceptionExists checks whether an activity_exception row is still present.
func exceptionExists(t *testing.T, s *scenario, id int64) bool {
	t.Helper()
	n, err := s.db.NewSelect().
		Model((*scheduleModel.ActivityException)(nil)).
		ModelTableExpr(`schedule.activity_exceptions AS "activity_exception"`).
		Where(`"activity_exception".id = ?`, id).
		Where(`"activity_exception".tenant_id = ?`, s.primaryTenant).
		Count(s.tenantCtx())
	require.NoError(t, err)
	return n > 0
}

// countInstanceStudents returns the number of instance_students rows left
// for a given instance (should be 0 after CASCADE-delete).
func countInstanceStudents(t *testing.T, s *scenario, instanceID int64) int {
	t.Helper()
	n, err := s.db.NewSelect().
		Model((*scheduleModel.InstanceStudent)(nil)).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".tenant_id = ?`, s.primaryTenant).
		Count(s.tenantCtx())
	require.NoError(t, err)
	return n
}

// countAuditRows counts audit.data_deletions rows for (student, deletionType).
func countAuditRows(t *testing.T, s *scenario, studentID int64, deletionType string) int {
	t.Helper()
	n, err := s.db.NewSelect().
		Model((*auditModel.DataDeletion)(nil)).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`).
		Where(`"data_deletion".student_id = ?`, studentID).
		Where(`"data_deletion".deletion_type = ?`, deletionType).
		Where(`"data_deletion".tenant_id = ?`, s.primaryTenant).
		Count(s.tenantCtx())
	require.NoError(t, err)
	return n
}
