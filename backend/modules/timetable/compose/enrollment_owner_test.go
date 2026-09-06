package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestModuleOwnsStudentEnrollmentLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Enrollment", "Owner", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Enrollment owner lifecycle")

	created := createOwnedEnrollment(t, module, ctx, student.ID, group.ID, "2026-09-01")
	found, err := module.FindStudentEnrollment(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, student.ID, found.StudentID)

	weekday, status := 2, timetable.AttendancePresent
	updated, err := module.UpdateStudentEnrollment(ctx, created.ID, timetable.StudentEnrollmentInput{
		StudentID: student.ID, ActivityGroupID: group.ID, ValidFrom: "2026-09-01",
		SelectedWeekdays: []int{1, 3}, AttendanceStatus: &status, Weekday: &weekday,
	})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 3}, updated.SelectedWeekdays)
	assert.Equal(t, &weekday, updated.Weekday)

	listed, err := module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_student_enrollments").Stats.Queries)

	require.NoError(t, module.DeleteStudentEnrollment(ctx, created.ID))
	_, err = module.FindStudentEnrollment(ctx, created.ID)
	require.ErrorIs(t, err, timetable.ErrStudentEnrollmentNotFound)
}

func TestModuleStudentEnrollmentsAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Owned", "Enrollment", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Owned enrollment group")
	owned := createOwnedEnrollment(t, module, ctx, student.ID, group.ID, "2026-09-01")

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreignTenantID, "Foreign", "Enrollment", "1a")
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "Foreign enrollment group")
	foreign := createOwnedEnrollment(t, module, foreignCtx, foreignStudent.ID, foreignGroup.ID, "2026-09-01")

	_, err := module.FindStudentEnrollment(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrStudentEnrollmentNotFound)
	listed, err := module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{
		ActivityGroupIDs: []int64{group.ID, foreignGroup.ID},
	})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, owned.ID, listed[0].ID)

	_, err = module.UpdateStudentEnrollment(foreignCtx, owned.ID, enrollmentInput(foreignStudent.ID, foreignGroup.ID, "2026-09-01"))
	require.ErrorIs(t, err, timetable.ErrStudentEnrollmentNotFound)
	require.NoError(t, module.DeleteStudentEnrollment(foreignCtx, owned.ID))
	_, err = module.FindStudentEnrollment(ctx, owned.ID)
	require.NoError(t, err)
	_, err = module.FindStudentEnrollment(ctx, foreign.ID)
	require.ErrorIs(t, err, timetable.ErrStudentEnrollmentNotFound)
}

func TestModuleStudentEnrollmentRejectsForeignTenantReferences(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	student := testpkg.CreateTestStudentForTenant(t, db, foreignTenantID, "Foreign", "Student", "1a")
	group := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "Foreign enrollment reference")

	_, err := module.CreateStudentEnrollment(ctx, enrollmentInput(student.ID, group.ID, "2026-09-01"))
	require.Error(t, err)
	listed, listErr := module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{})
	require.NoError(t, listErr)
	assert.Empty(t, listed)
}

func TestModuleStudentEnrollmentRecordsDuplicatePreventionConflicts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Duplicate", "Enrollment", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Duplicate enrollment group")
	input := enrollmentInput(student.ID, group.ID, "2026-09-01")

	_, err := module.CreateStudentEnrollment(ctx, input)
	require.NoError(t, err)
	_, err = module.CreateStudentEnrollment(ctx, input)
	require.Error(t, err)
	var conflicts int64
	for _, observation := range log.seen {
		conflicts += observation.Stats.DuplicatePreventionConflicts
	}
	assert.EqualValues(t, 1, conflicts)
}

func TestModuleStudentEnrollmentReadFailuresAreNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := module.FindStudentEnrollment(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestModuleStudentEnrollmentLifecycleWritesRollBackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Lifecycle", "Enrollment", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Enrollment lifecycle rollback")

	var rolledBackID int64
	rollbackEnrollmentWrite(t, ctx, func(txCtx context.Context) error {
		created, err := module.CreateStudentEnrollment(txCtx, enrollmentInput(student.ID, group.ID, "2026-09-01"))
		rolledBackID = created.ID
		return err
	})
	_, err := module.FindStudentEnrollment(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrStudentEnrollmentNotFound)
	enrollment := createOwnedEnrollment(t, module, ctx, student.ID, group.ID, "2026-09-01")

	rollbackEnrollmentWrite(t, ctx, func(txCtx context.Context) error {
		_, err := module.UpdateStudentEnrollment(txCtx, enrollment.ID, enrollmentInput(student.ID, group.ID, "2026-09-02"))
		return err
	})
	assertEnrollmentValidFrom(t, module, ctx, enrollment.ID, "2026-09-01")
	_, err = module.UpdateStudentEnrollment(ctx, enrollment.ID, enrollmentInput(student.ID, group.ID, "2026-09-02"))
	require.NoError(t, err)

	rollbackEnrollmentWrite(t, ctx, func(txCtx context.Context) error { return module.DeleteStudentEnrollment(txCtx, enrollment.ID) })
	_, err = module.FindStudentEnrollment(ctx, enrollment.ID)
	require.NoError(t, err)
	require.NoError(t, module.DeleteStudentEnrollment(ctx, enrollment.ID))
}

func TestModuleStudentEnrollmentValidityWritesRollBackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	studentA := testpkg.CreateTestStudent(t, db, "Validity", "One", "1a")
	studentB := testpkg.CreateTestStudent(t, db, "Validity", "Two", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Enrollment validity rollback")
	first := createOwnedEnrollment(t, module, ctx, studentA.ID, group.ID, "2026-09-01")

	rollbackEnrollmentWrite(t, ctx, func(txCtx context.Context) error {
		return module.SetStudentEnrollmentValidUntil(txCtx, first.ID, "2026-09-10")
	})
	assertEnrollmentValidUntil(t, module, ctx, first.ID, nil)
	require.NoError(t, module.SetStudentEnrollmentValidUntil(ctx, first.ID, "2026-09-10"))
	want := "2026-09-10"
	assertEnrollmentValidUntil(t, module, ctx, first.ID, &want)

	second := createOwnedEnrollment(t, module, ctx, studentB.ID, group.ID, "2026-09-01")
	rollbackEnrollmentWrite(t, ctx, func(txCtx context.Context) error {
		return module.CloseOpenStudentEnrollments(txCtx, group.ID, nil, "2026-09-11")
	})
	assertEnrollmentValidUntil(t, module, ctx, second.ID, nil)
	require.NoError(t, module.CloseOpenStudentEnrollments(ctx, group.ID, nil, "2026-09-11"))
	want = "2026-09-11"
	assertEnrollmentValidUntil(t, module, ctx, second.ID, &want)
}

func TestModuleCapActiveStudentEnrollmentsRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Enrollment cap rollback")
	studentA := testpkg.CreateTestStudent(t, db, "Cap", "Begun", "1a")
	studentB := testpkg.CreateTestStudent(t, db, "Cap", "Future", "1a")
	createOwnedEnrollment(t, module, ctx, studentA.ID, group.ID, "2026-09-01")
	future := createOwnedEnrollment(t, module, ctx, studentB.ID, group.ID, "2026-09-10")

	rollbackEnrollmentWrite(t, ctx, func(txCtx context.Context) error {
		rows, err := module.CapActiveStudentEnrollments(txCtx, group.ID, "2026-09-10")
		assert.EqualValues(t, 2, rows)
		return err
	})
	assertEnrollmentCount(t, module, ctx, group.ID, 2)
	rows, err := module.CapActiveStudentEnrollments(ctx, group.ID, "2026-09-10")
	require.NoError(t, err)
	assert.EqualValues(t, 2, rows)
	_, err = module.FindStudentEnrollment(ctx, future.ID)
	require.ErrorIs(t, err, timetable.ErrStudentEnrollmentNotFound)
}

func TestModuleStudentEnrollmentSourceDeleteRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Source", "Delete", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Enrollment source delete rollback")
	sourceID := createEnrollmentSource(t, db, ctx, student.ID)
	input := enrollmentInput(student.ID, group.ID, "2026-09-01")
	input.EnrollmentRequestChildID = &sourceID
	_, err := module.CreateStudentEnrollment(ctx, input)
	require.NoError(t, err)

	rollbackEnrollmentWrite(t, ctx, func(txCtx context.Context) error {
		rows, err := module.DeleteStudentEnrollmentsBySource(txCtx, student.ID, sourceID)
		assert.EqualValues(t, 1, rows)
		return err
	})
	assertEnrollmentCount(t, module, ctx, group.ID, 1)
	rows, err := module.DeleteStudentEnrollmentsBySource(ctx, student.ID, sourceID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
}

func TestModuleStudentEnrollmentSourceBackfillRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Source", "Backfill", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Enrollment source backfill rollback")
	sourceID := createEnrollmentSource(t, db, ctx, student.ID)
	enrollment := createOwnedEnrollment(t, module, ctx, student.ID, group.ID, "2026-09-01")

	rollbackEnrollmentWrite(t, ctx, func(txCtx context.Context) error {
		rows, err := module.BackfillStudentEnrollmentSource(txCtx, student.ID, sourceID, []int64{group.ID})
		assert.EqualValues(t, 1, rows)
		return err
	})
	assertEnrollmentSource(t, module, ctx, enrollment.ID, nil)
	rows, err := module.BackfillStudentEnrollmentSource(ctx, student.ID, sourceID, []int64{group.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	assertEnrollmentSource(t, module, ctx, enrollment.ID, &sourceID)
}

func TestStudentEnrollmentListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Enrollment", "Budget", "1a")
	for i := 0; i < 8; i++ {
		group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("enrollment-owner-budget-%d", i))
		createOwnedEnrollment(t, module, ctx, student.ID, group.ID, "2026-09-01")
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.enrollments.list", counter.Queries())
}

func createOwnedEnrollment(t *testing.T, module *timetable.Module, ctx context.Context, studentID, groupID int64, validFrom string) timetable.StudentEnrollment {
	t.Helper()
	value, err := module.CreateStudentEnrollment(ctx, enrollmentInput(studentID, groupID, validFrom))
	require.NoError(t, err)
	return value
}

func enrollmentInput(studentID, groupID int64, validFrom string) timetable.StudentEnrollmentInput {
	return timetable.StudentEnrollmentInput{StudentID: studentID, ActivityGroupID: groupID, ValidFrom: validFrom}
}

func rollbackEnrollmentWrite(t *testing.T, ctx context.Context, write func(context.Context) error) {
	t.Helper()
	wantErr := errors.New("abort student enrollment write")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := write(txCtx); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
}

func assertEnrollmentValidFrom(t *testing.T, module *timetable.Module, ctx context.Context, id int64, want string) {
	t.Helper()
	value, err := module.FindStudentEnrollment(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, value.ValidFrom)
}

func assertEnrollmentValidUntil(t *testing.T, module *timetable.Module, ctx context.Context, id int64, want *string) {
	t.Helper()
	value, err := module.FindStudentEnrollment(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, value.ValidUntil)
}

func assertEnrollmentCount(t *testing.T, module *timetable.Module, ctx context.Context, groupID int64, want int) {
	t.Helper()
	values, err := module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{ActivityGroupIDs: []int64{groupID}})
	require.NoError(t, err)
	assert.Len(t, values, want)
}

func assertEnrollmentSource(t *testing.T, module *timetable.Module, ctx context.Context, id int64, want *int64) {
	t.Helper()
	value, err := module.FindStudentEnrollment(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, value.EnrollmentRequestChildID)
}

func createEnrollmentSource(t *testing.T, db *bun.DB, ctx context.Context, studentID int64) int64 {
	t.Helper()
	var phaseID, requestID, childID int64
	suffix := time.Now().UnixNano()
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.phases (tenant_id, name, kind, service_start_date, service_end_date)
		VALUES (?, ?, 'custom', '2026-09-01', '2027-07-31') RETURNING id`, testpkg.Tenant(t), fmt.Sprintf("Owner source %d", suffix)).Scan(ctx, &phaseID))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.requests (tenant_id, phase_id, guardian_first_name, guardian_last_name,
		guardian_email, consent_flags, custom_data, status_token, submitted_at)
		VALUES (?, ?, 'Owner', 'Source', ?, '{}'::jsonb, '{}'::jsonb, ?, NOW()) RETURNING id`,
		testpkg.Tenant(t), phaseID, fmt.Sprintf("owner-%d@example.test", suffix), fmt.Sprintf("owner-%d", suffix)).Scan(ctx, &requestID))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_children (tenant_id, request_id, first_name, last_name,
		date_of_birth, status, activation_mode, created_student_id, reviewed_at, sort_order, custom_data)
		VALUES (?, ?, 'Owner', 'Child', '2018-01-01', 'approved', 'scheduled', ?, NOW(), 0, '{}'::jsonb) RETURNING id`,
		testpkg.Tenant(t), requestID, studentID).Scan(ctx, &childID))
	tenantID := testpkg.Tenant(t)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.NewDelete().Table("enrollment.request_children").Where("tenant_id = ?", tenantID).Where("id = ?", childID).Exec(cleanupCtx)
		_, _ = db.NewDelete().Table("enrollment.requests").Where("tenant_id = ?", tenantID).Where("id = ?", requestID).Exec(cleanupCtx)
		_, _ = db.NewDelete().Table("enrollment.phases").Where("tenant_id = ?", tenantID).Where("id = ?", phaseID).Exec(cleanupCtx)
	})
	return childID
}
