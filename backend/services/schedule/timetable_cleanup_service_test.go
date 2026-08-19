package schedule_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	auditRepoPkg "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	scheduleRepoPkg "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Hermetic: every instance/exception row is created per-subtest under a unique
// tenant and cleaned up via registerCleanup. No hardcoded entity IDs.

// instFixture holds the DB rows a test created so runCleanup can remove them
// in child-first order.
type instFixture struct {
	db               *bun.DB
	ctx              context.Context
	tenantID         int64
	instanceIDs      []int64
	exceptionIDs     []int64
	instanceStaffIDs []int64
	instStudentIDs   []int64
	studentIDs       []int64
	staffIDs         []int64
	roomIDs          []int64
	activityGrpIDs   []int64
	templateIDs      []int64
	categoryIDs      []int64
}

func (f *instFixture) cleanup(t *testing.T) {
	t.Helper()
	// Children first. Most instance_students + instance_staff rows disappear
	// via CASCADE when the instance row is deleted by the test, but some
	// tests intentionally leave rows un-deleted and want the child tables
	// cleared explicitly.
	if len(f.instStudentIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "schedule.instance_students", f.instStudentIDs...)
	}
	if len(f.instanceStaffIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "schedule.instance_staff", f.instanceStaffIDs...)
	}
	if len(f.instanceIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "schedule.activity_instances", f.instanceIDs...)
	}
	if len(f.exceptionIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "schedule.activity_exceptions", f.exceptionIDs...)
	}
	if len(f.templateIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "activities.groups", f.templateIDs...)
	}
	if len(f.activityGrpIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "activities.groups", f.activityGrpIDs...)
	}
	if len(f.categoryIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "activities.categories", f.categoryIDs...)
	}
	if len(f.studentIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "users.students", f.studentIDs...)
	}
	if len(f.staffIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "users.staff", f.staffIDs...)
	}
	if len(f.roomIDs) > 0 {
		testpkg.CleanupTableRecords(t, f.db, "facilities.rooms", f.roomIDs...)
	}
	// Clean up orphaned audit rows for the test's affected students. Audit
	// rows are tenant-scoped and use DELETE CASCADE on student_id, so most
	// are gone when the student is deleted — but rows written by tests
	// where we prevent the DELETE (mock rollback) remain.
	for _, sid := range f.studentIDs {
		_, _ = f.db.NewDelete().Table("audit.data_deletions").
			Where("tenant_id = ? AND student_id = ? AND deletion_type = ?", f.tenantID, sid, auditModels.DeletionTypeTimetableRetention).
			Exec(f.ctx)
	}
}

// newInstanceFixture inserts an ActivityInstance row and returns the ID. The
// template-backed flag controls whether activity_group_id is set.
func (f *instFixture) newInstance(t *testing.T, date timezone.Date, status string, roomID int64, templateID *int64) int64 {
	t.Helper()
	title := fmt.Sprintf("Inst-%s-%s", date, status)
	inst := &scheduleModels.ActivityInstance{
		Date:            date,
		Title:           title,
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          roomID,
		Status:          status,
		ActivityGroupID: templateID,
	}
	inst.SetTenantID(f.tenantID)
	_, err := f.db.NewInsert().Model(inst).
		ModelTableExpr(`schedule.activity_instances`).
		Exec(f.ctx)
	require.NoError(t, err, "insert activity_instance")
	f.instanceIDs = append(f.instanceIDs, inst.ID)
	return inst.ID
}

// newException inserts an ActivityException row for the given date.
func (f *instFixture) newException(t *testing.T, activityGroupID int64, date timezone.Date, exceptionType string) int64 {
	t.Helper()
	exc := &scheduleModels.ActivityException{
		ActivityGroupID: activityGroupID,
		ExceptionDate:   date,
		ExceptionType:   exceptionType,
	}
	exc.SetTenantID(f.tenantID)
	_, err := f.db.NewInsert().Model(exc).
		ModelTableExpr(`schedule.activity_exceptions`).
		Exec(f.ctx)
	require.NoError(t, err, "insert activity_exception")
	f.exceptionIDs = append(f.exceptionIDs, exc.ID)
	return exc.ID
}

// attachStudent inserts an instance_students row linking studentID to
// instanceID. Optionally attaches a GDPR-sensitive note (WP-B10 field).
func (f *instFixture) attachStudent(t *testing.T, instanceID, studentID int64, note *string) int64 {
	t.Helper()
	row := &scheduleModels.InstanceStudent{
		InstanceID: instanceID,
		StudentID:  studentID,
		Note:       note,
	}
	row.SetTenantID(f.tenantID)
	_, err := f.db.NewInsert().Model(row).
		ModelTableExpr(`schedule.instance_students`).
		Exec(f.ctx)
	require.NoError(t, err, "insert instance_students")
	f.instStudentIDs = append(f.instStudentIDs, row.ID)
	return row.ID
}

// attachStaff inserts an instance_staff row linking staffID to instanceID.
func (f *instFixture) attachStaff(t *testing.T, instanceID, staffID int64) int64 {
	t.Helper()
	row := &scheduleModels.InstanceStaff{
		InstanceID: instanceID,
		StaffID:    staffID,
	}
	row.SetTenantID(f.tenantID)
	_, err := f.db.NewInsert().Model(row).
		ModelTableExpr(`schedule.instance_staff`).
		Exec(f.ctx)
	require.NoError(t, err, "insert instance_staff")
	f.instanceStaffIDs = append(f.instanceStaffIDs, row.ID)
	return row.ID
}

// setupFixture sets up db + ctx + a room for cleanup tests.
func setupFixture(t *testing.T) (*instFixture, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, fmt.Sprintf("Room-%d", suffix))
	f := &instFixture{db: db, ctx: ctx, tenantID: tenantID, roomIDs: []int64{room.ID}}
	t.Cleanup(func() { f.cleanup(t) })
	return f, room.ID
}

// newCleanupSvc builds a TimetableCleanupService wired against the real
// audit repo + no settings service (so retention falls to the 365-day
// literal default). Tests that need to override retention build their own
// service with a stubSettingsService.
func newCleanupSvc(db *bun.DB) scheduleSvc.TimetableCleanupService {
	return scheduleSvc.NewTimetableCleanupService(
		scheduleRepoPkg.NewActivityInstanceRepository(db),
		scheduleRepoPkg.NewActivityExceptionRepository(db),
		scheduleRepoPkg.NewInstanceStudentRepository(db),
		auditRepoPkg.NewDataDeletionRepository(db),
		auditRepoPkg.NewDeviationEventRepository(db),
		nil, // no settings — retention falls through to the 365-day default
		slog.Default(),
	)
}

// --- Tests ---

func TestCleanup_HappyPath_DeletesOldRowsKeepsFreshRows(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := newCleanupSvc(f.db)

	old := timezone.TodayDate().AddDays(-400) // past 365d retention
	recent := timezone.TodayDate().AddDays(-30)

	// 3 instances: 2 old (should delete), 1 recent (should survive).
	oldID1 := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)
	oldID2 := f.newInstance(t, old, scheduleModels.InstanceStatusCancelled, roomID, nil)
	recentID := f.newInstance(t, recent, scheduleModels.InstanceStatusPlanned, roomID, nil)

	// 2 exceptions: 1 old (delete), 1 recent (survive). Needs a template parent.
	cat := testpkg.CreateTestActivityCategoryForTenant(t, f.db, f.tenantID, fmt.Sprintf("Cat-%d", time.Now().UnixNano()))
	f.categoryIDs = append(f.categoryIDs, cat.ID)
	template := createTemplate(t, f, roomID, cat.ID)
	oldExcID := f.newException(t, template.ID, old, scheduleModels.ActivityExceptionCancelled)
	recentExcID := f.newException(t, template.ID, recent, scheduleModels.ActivityExceptionCancelled)
	_ = recentExcID

	result, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 2, result.InstancesDeleted, "2 old instances deleted")
	assert.Equal(t, 1, result.ExceptionsDeleted, "1 old exception deleted")
	assert.Equal(t, 365, result.RetentionDays)

	// Verify: old IDs gone, recent IDs still present.
	assertInstanceExists(t, f, oldID1, false)
	assertInstanceExists(t, f, oldID2, false)
	assertInstanceExists(t, f, recentID, true)
	assertExceptionExists(t, f, oldExcID, false)
}

func TestCleanup_EmptyTenant_WritesNoAuditRowsForStudents(t *testing.T) {
	t.Parallel()

	f, _ := setupFixture(t)
	svc := newCleanupSvc(f.db)

	// No instances, no exceptions, no students.
	result, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, result.InstancesDeleted)
	assert.Equal(t, 0, result.ExceptionsDeleted)
	assert.Equal(t, 0, result.StudentsAffected)

	// No audit rows for a non-existent affected student — the per-student
	// policy means nothing gets logged when nothing was about a student.
	count, err := f.db.NewSelect().
		Table("audit.data_deletions").
		Where("tenant_id = ? AND deletion_type = ?", f.tenantID, auditModels.DeletionTypeTimetableRetention).
		Count(f.ctx)
	require.NoError(t, err)
	// The "zero rows" assertion is indirect: an empty-tenant run should add
	// zero new audit rows. We measure delta by running cleanup twice — both
	// runs should yield the same count.
	result2, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, result2.StudentsAffected)
	count2, err := f.db.NewSelect().
		Table("audit.data_deletions").
		Where("tenant_id = ? AND deletion_type = ?", f.tenantID, auditModels.DeletionTypeTimetableRetention).
		Count(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, count, count2, "empty-tenant cleanup adds no audit rows")
}

func TestCleanup_Idempotent_SecondRunDeletesZero(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := newCleanupSvc(f.db)

	old := timezone.TodayDate().AddDays(-400)
	f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)

	r1, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, r1.InstancesDeleted)

	r2, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, r2.InstancesDeleted, "second run is a no-op")
	assert.True(t, r2.Success)
}

func TestCleanup_CASCADE_DeletesInstanceChildren(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := newCleanupSvc(f.db)

	old := timezone.TodayDate().AddDays(-400)
	instID := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)

	staff := testpkg.CreateTestStaffForTenant(t, f.db, f.tenantID, "Cascade", fmt.Sprintf("Staff-%d", time.Now().UnixNano()))
	student := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Cascade", fmt.Sprintf("Stud-%d", time.Now().UnixNano()), "3a")
	f.staffIDs = append(f.staffIDs, staff.ID)
	f.studentIDs = append(f.studentIDs, student.ID)

	staffRowID := f.attachStaff(t, instID, staff.ID)
	studRowID := f.attachStudent(t, instID, student.ID, nil)

	_, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)

	// CASCADE should have removed both child rows.
	assertRowExists(t, f, "schedule.instance_staff", staffRowID, false)
	assertRowExists(t, f, "schedule.instance_students", studRowID, false)
	// Our fixture tracker should not try to delete them again on teardown.
	f.instanceStaffIDs = nil
	f.instStudentIDs = nil
	f.instanceIDs = nil
}

func TestCleanup_RetentionOverride_UsesOverriddenDays(t *testing.T) {
	t.Parallel()

	// Tenant override narrows the window to 30 days; a 60-day-old instance
	// becomes deletable even though the registry default (365) would have
	// spared it. Wires a stubSettingsService that reports HasTenantOverride
	// = true and ResolveInt = 30 — the same contract the real settings
	// service provides when a school admin sets the value in the UI.
	f, roomID := setupFixture(t)
	svc := scheduleSvc.NewTimetableCleanupService(
		scheduleRepoPkg.NewActivityInstanceRepository(f.db),
		scheduleRepoPkg.NewActivityExceptionRepository(f.db),
		scheduleRepoPkg.NewInstanceStudentRepository(f.db),
		auditRepoPkg.NewDataDeletionRepository(f.db),
		auditRepoPkg.NewDeviationEventRepository(f.db),
		newStubSettingsService(true, nil, 30, nil),
		slog.Default(),
	)

	sixtyDaysOld := timezone.TodayDate().AddDays(-60)
	tenDaysOld := timezone.TodayDate().AddDays(-10)

	oldID := f.newInstance(t, sixtyDaysOld, scheduleModels.InstanceStatusCompleted, roomID, nil)
	freshID := f.newInstance(t, tenDaysOld, scheduleModels.InstanceStatusCompleted, roomID, nil)

	r, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 30, r.RetentionDays, "tenant override must win over registry default")
	assert.Equal(t, 1, r.InstancesDeleted, "60-day-old row past 30-day window")

	assertRowExists(t, f, "schedule.activity_instances", oldID, false)
	assertRowExists(t, f, "schedule.activity_instances", freshID, true)
}

func TestCleanup_AllStatuses_DeletesPlannedActiveCompletedCancelled(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := newCleanupSvc(f.db)

	old := timezone.TodayDate().AddDays(-400)
	for _, status := range []string{
		scheduleModels.InstanceStatusPlanned,
		scheduleModels.InstanceStatusActive,
		scheduleModels.InstanceStatusCompleted,
		scheduleModels.InstanceStatusCancelled,
	} {
		f.newInstance(t, old, status, roomID, nil)
	}

	r, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, r.InstancesDeleted, "all four statuses deleted past retention")
}

func TestCleanup_TemplatesUntouched_OnlyInstancesAndExceptionsDeleted(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := newCleanupSvc(f.db)

	cat := testpkg.CreateTestActivityCategoryForTenant(t, f.db, f.tenantID, fmt.Sprintf("Cat-%d", time.Now().UnixNano()))
	f.categoryIDs = append(f.categoryIDs, cat.ID)
	template := createTemplate(t, f, roomID, cat.ID)

	old := timezone.TodayDate().AddDays(-400)
	f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, &template.ID)
	f.newException(t, template.ID, old, scheduleModels.ActivityExceptionCancelled)

	r, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, r.InstancesDeleted)
	assert.Equal(t, 1, r.ExceptionsDeleted)

	// Template survives.
	exists, err := f.db.NewSelect().
		Table("activities.groups").
		Where("id = ?", template.ID).
		Exists(f.ctx)
	require.NoError(t, err)
	assert.True(t, exists, "template must survive the cleanup — only instances + exceptions are retention-scoped")
}

func TestCleanup_PerStudentAuditRows_OneRowPerAffectedStudent(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := newCleanupSvc(f.db)

	old := timezone.TodayDate().AddDays(-400)
	inst1 := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)
	inst2 := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)

	s1 := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Audit", fmt.Sprintf("One-%d", time.Now().UnixNano()), "3a")
	s2 := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Audit", fmt.Sprintf("Two-%d", time.Now().UnixNano()), "3a")
	f.studentIDs = append(f.studentIDs, s1.ID, s2.ID)

	// s1 attends both instances (2 rows), s2 attends one (1 row).
	f.attachStudent(t, inst1, s1.ID, nil)
	f.attachStudent(t, inst2, s1.ID, nil)
	f.attachStudent(t, inst1, s2.ID, nil)

	r, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, r.StudentsAffected)

	// The DELETE has fired — student IDs are gone via CASCADE? No: students
	// are in users.students, which is not cascaded by activity_instances
	// deletion. So s1/s2 still exist, and their audit rows persist.
	var auditCount int
	auditCount, err = f.db.NewSelect().
		Table("audit.data_deletions").
		Where("tenant_id = ? AND deletion_type = ?", f.tenantID, auditModels.DeletionTypeTimetableRetention).
		Where("student_id IN (?, ?)", s1.ID, s2.ID).
		Count(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, auditCount, "one audit row per affected student")

	// Verify per-student counts.
	var s1Row auditModels.DataDeletion
	err = f.db.NewSelect().Model(&s1Row).
		Where("tenant_id = ? AND student_id = ? AND deletion_type = ?",
			f.tenantID, s1.ID, auditModels.DeletionTypeTimetableRetention).
		Scan(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, s1Row.RecordsDeleted, "s1 had 2 instance_students rows")
	assert.Equal(t, "system", s1Row.DeletedBy)
	assert.Contains(t, s1Row.Metadata, "retention_days")
	assert.Contains(t, s1Row.Metadata, "cutoff_date")
	assert.Contains(t, s1Row.Metadata, "instance_ids_sample")

	// Tracker cleanup: instances are gone, skip them. instance_students
	// rows were CASCADEd.
	f.instanceIDs = nil
	f.instStudentIDs = nil
}

func TestCleanup_DeletesGDPRSensitiveNotesViaCascade(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := newCleanupSvc(f.db)

	old := timezone.TodayDate().AddDays(-400)
	instID := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)

	student := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Sensitive", fmt.Sprintf("Note-%d", time.Now().UnixNano()), "3a")
	f.studentIDs = append(f.studentIDs, student.ID)
	sensitiveNote := "contains GDPR-sensitive free text about this student's behavior"
	studRowID := f.attachStudent(t, instID, student.ID, &sensitiveNote)

	// Verify the note exists before cleanup.
	var existing struct {
		Note *string `bun:"note"`
	}
	err := f.db.NewSelect().Table("schedule.instance_students").
		Column("note").
		Where("id = ?", studRowID).
		Scan(f.ctx, &existing)
	require.NoError(t, err)
	require.NotNil(t, existing.Note)
	require.Equal(t, sensitiveNote, *existing.Note)

	// Run cleanup.
	_, err = svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)

	// The instance_students row (with its note) must be gone via CASCADE.
	assertRowExists(t, f, "schedule.instance_students", studRowID, false)
	f.instStudentIDs = nil
	f.instanceIDs = nil
}

func TestCleanup_TenantIsolation_OtherTenantDataUntouched(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := newCleanupSvc(f.db)

	// Tenant A has one old instance.
	old := timezone.TodayDate().AddDays(-400)
	f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)

	// Insert a row for a different tenant_id directly to bypass the
	// fixture helper. Track manually so we can assert presence then clean up.
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, f.db, otherTenantID)

	otherRoom := testpkg.CreateTestRoomForTenant(t, f.db, otherTenantID, fmt.Sprintf("Other-Room-%d", time.Now().UnixNano()))

	otherInst := &scheduleModels.ActivityInstance{
		Date:      old,
		Title:     fmt.Sprintf("OtherTenant-%d", time.Now().UnixNano()),
		StartTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:    otherRoom.ID,
		Status:    scheduleModels.InstanceStatusCompleted,
	}
	otherInst.SetTenantID(otherTenantID)
	_, err := f.db.NewInsert().Model(otherInst).
		ModelTableExpr(`schedule.activity_instances`).
		Exec(f.ctx)
	require.NoError(t, err)

	// Run cleanup under tenant A.
	_, err = svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)

	// Other tenant's row must survive.
	assertRowExists(t, f, "schedule.activity_instances", otherInst.ID, true)

	// Manual cleanup of the "other" tenant's fixture rows.
	_, _ = f.db.NewDelete().Table("schedule.activity_instances").Where("id = ?", otherInst.ID).Exec(f.ctx)
	_, _ = f.db.NewDelete().Table("facilities.rooms").Where("id = ?", otherRoom.ID).Exec(f.ctx)
}

// --- Audit rollback test (mock repo) ---

// failingAuditRepo implements audit.DataDeletionRepository and returns a
// predetermined error from Create; everything else panics because the
// test shouldn't exercise other paths.
type failingAuditRepo struct {
	err error
}

func (r *failingAuditRepo) Create(_ context.Context, _ *auditModels.DataDeletion) error {
	return r.err
}
func (r *failingAuditRepo) FindByID(context.Context, interface{}) (*auditModels.DataDeletion, error) {
	panic("not implemented")
}
func (r *failingAuditRepo) FindByStudentID(context.Context, int64) ([]*auditModels.DataDeletion, error) {
	panic("not implemented")
}
func (r *failingAuditRepo) FindByDateRange(context.Context, time.Time, time.Time) ([]*auditModels.DataDeletion, error) {
	panic("not implemented")
}
func (r *failingAuditRepo) FindByType(context.Context, string) ([]*auditModels.DataDeletion, error) {
	panic("not implemented")
}
func (r *failingAuditRepo) List(context.Context, map[string]interface{}) ([]*auditModels.DataDeletion, error) {
	panic("not implemented")
}
func (r *failingAuditRepo) GetDeletionStats(context.Context, time.Time) (map[string]interface{}, error) {
	panic("not implemented")
}
func (r *failingAuditRepo) CountByType(context.Context, string, time.Time) (int64, error) {
	panic("not implemented")
}

func TestCleanup_AuditWriteFailure_BubblesError(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)

	old := timezone.TodayDate().AddDays(-400)
	instID := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)

	student := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "AuditFail", fmt.Sprintf("Stud-%d", time.Now().UnixNano()), "3a")
	f.studentIDs = append(f.studentIDs, student.ID)
	f.attachStudent(t, instID, student.ID, nil)

	svc := scheduleSvc.NewTimetableCleanupService(
		scheduleRepoPkg.NewActivityInstanceRepository(f.db),
		scheduleRepoPkg.NewActivityExceptionRepository(f.db),
		scheduleRepoPkg.NewInstanceStudentRepository(f.db),
		&failingAuditRepo{err: errors.New("simulated audit failure")},
		nil,
		nil,
		slog.Default(),
	)

	// The service bubbles the error. Because we do NOT wrap in
	// tenant.WithTenantTx in this integration test (tests run as the
	// postgres superuser), the DELETEs never fire — the service returns
	// before executing them on error. Confirm the row is still present.
	_, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.Error(t, err, "audit failure must bubble up")
	assertRowExists(t, f, "schedule.activity_instances", instID, true,
		"audit-before-delete ordering means the DELETE did not fire")
}

// TestCleanup_InsideWithTenantTx_Rollback_UndoesEverything proves the
// atomicity claim in the package doc: when the caller wraps
// CleanupExpiredTimetableData in tenant.WithTenantTx and the tx body returns
// an error, the audit rows AND the DELETEs are both rolled back — so the
// tenant's data is left exactly as it was before the cleanup attempt.
//
// This is the production contract: the scheduler and CLI both invoke the
// service inside a WithTenantTx, so a DB failure anywhere during or after
// cleanup must leave no partial state behind. The ordering-only test
// (TestCleanup_AuditWriteFailure_BubblesError) runs outside a tx and can't
// prove this — this test does.
func TestCleanup_InsideWithTenantTx_Rollback_UndoesEverything(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := newCleanupSvc(f.db)

	old := timezone.TodayDate().AddDays(-400)
	inst1 := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)
	inst2 := f.newInstance(t, old, scheduleModels.InstanceStatusCancelled, roomID, nil)

	stud := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Rollback", fmt.Sprintf("Stud-%d", time.Now().UnixNano()), "3a")
	f.studentIDs = append(f.studentIDs, stud.ID)
	f.attachStudent(t, inst1, stud.ID, nil)
	f.attachStudent(t, inst2, stud.ID, nil)

	rollbackErr := errors.New("simulated caller-side failure after cleanup")
	txCtx := tenant.WithTenantID(context.Background(), f.tenantID)
	err := tenant.WithTenantTx(txCtx, f.db, f.tenantID, func(innerCtx context.Context, _ bun.Tx) error {
		// Cleanup succeeds inside the tx: audit row is written, both
		// instances are DELETEd (CASCADE removes the instance_students rows).
		result, cErr := svc.CleanupExpiredTimetableData(innerCtx)
		require.NoError(t, cErr, "cleanup itself must succeed")
		require.Equal(t, 2, result.InstancesDeleted)
		require.Equal(t, 1, result.StudentsAffected)
		// Return an error so the tx rolls back. In production this simulates
		// any failure that happens in the caller after cleanup returns —
		// e.g., a scheduler-level commit failure or a chained operation that
		// errors. The atomicity guarantee says cleanup's writes must be
		// undone.
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)

	// Both instance rows are back — the DELETEs were rolled back.
	assertRowExists(t, f, "schedule.activity_instances", inst1, true,
		"rollback must restore inst1")
	assertRowExists(t, f, "schedule.activity_instances", inst2, true,
		"rollback must restore inst2")

	// The audit row is gone — the INSERT was rolled back too. This is the
	// key check: without atomicity, we'd have an audit row claiming data
	// was deleted when it actually wasn't.
	auditCount, err := f.db.NewSelect().Table("audit.data_deletions").
		Where("tenant_id = ? AND student_id = ? AND deletion_type = ?",
			f.tenantID, stud.ID, auditModels.DeletionTypeTimetableRetention).
		Count(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, auditCount,
		"rollback must undo audit rows — otherwise compliance log lies about deleted data")
}

// --- Helpers ---

// createTemplate inserts an activity_groups row with is_template=true.
func createTemplate(t *testing.T, f *instFixture, roomID, categoryID int64) *activitiesModels.Group {
	t.Helper()
	tpl := &activitiesModels.Group{
		Name:            fmt.Sprintf("Template-%d", time.Now().UnixNano()),
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      categoryID,
		PlannedRoomID:   &roomID,
		IsTemplate:      true,
	}
	tpl.SetTenantID(f.tenantID)
	_, err := f.db.NewInsert().Model(tpl).ModelTableExpr(`activities.groups AS "group"`).Exec(f.ctx)
	require.NoError(t, err)
	f.templateIDs = append(f.templateIDs, tpl.ID)
	return tpl
}

func assertInstanceExists(t *testing.T, f *instFixture, id int64, wantExists bool) {
	t.Helper()
	assertRowExists(t, f, "schedule.activity_instances", id, wantExists)
}

func assertExceptionExists(t *testing.T, f *instFixture, id int64, wantExists bool) {
	t.Helper()
	assertRowExists(t, f, "schedule.activity_exceptions", id, wantExists)
}

func assertRowExists(t *testing.T, f *instFixture, table string, id int64, wantExists bool, msgAndArgs ...any) {
	t.Helper()
	got, err := f.db.NewSelect().Table(table).Where("id = ?", id).Exists(f.ctx)
	require.NoError(t, err)
	if wantExists {
		assert.True(t, got, append([]any{fmt.Sprintf("%s row %d should exist", table, id)}, msgAndArgs...)...)
	} else {
		assert.False(t, got, append([]any{fmt.Sprintf("%s row %d should be deleted", table, id)}, msgAndArgs...)...)
	}
}
