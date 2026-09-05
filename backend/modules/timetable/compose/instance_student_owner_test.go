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
)

func TestModuleOwnsInstanceStudentLifecycleAndQueries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "student-lifecycle")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-01", "08:00:00", "Roster")
	firstStudent := testpkg.CreateTestStudent(t, db, "Owner", "Expected", "3a")
	secondStudent := testpkg.CreateTestStudent(t, db, "Owner", "Absent", "3a")
	first := createOwnedInstanceStudent(t, module, ctx, instance.ID, firstStudent.ID, timetable.InstanceAttendanceExpected)
	second := createOwnedInstanceStudent(t, module, ctx, instance.ID, secondStudent.ID, timetable.InstanceAttendanceAbsent)

	found, err := module.FindInstanceStudent(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, firstStudent.ID, found.StudentID)
	status := timetable.InstanceAttendanceExpected
	listed, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{
		InstanceIDs: []int64{instance.ID}, Status: &status, OrderByInstanceStudent: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID}, instanceStudentIDs(listed))
	counts, err := module.CountNonAbsentInstanceStudents(ctx, []int64{instance.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, counts[instance.ID])

	input := ownedInstanceStudentInput(instance.ID, secondStudent.ID, timetable.InstanceAttendancePresent)
	updated, err := module.UpdateInstanceStudent(ctx, second.ID, input)
	require.NoError(t, err)
	assert.Equal(t, timetable.InstanceAttendancePresent, updated.Status)
	require.NoError(t, module.DeleteInstanceStudent(ctx, first.ID))
	_, err = module.FindInstanceStudent(ctx, first.ID)
	require.ErrorIs(t, err, timetable.ErrInstanceStudentNotFound)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_instance_students").Stats.Queries)
}

func TestModuleOwnsInstanceStudentCurrentAndParallelReads(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "student-current")
	first := createOwnedInstanceWithStatus(t, module, ctx, fixture, "2027-11-02", "08:00:00", "First", timetable.InstanceStatusActive)
	second := createOwnedInstanceWithStatus(t, module, ctx, fixture, "2027-11-02", "08:30:00", "Second", timetable.InstanceStatusActive)
	student := testpkg.CreateTestStudent(t, db, "Owner", "Parallel", "3a")
	checkedIn := time.Date(2027, 11, 2, 8, 35, 0, 0, time.UTC)
	input := ownedInstanceStudentInput(second.ID, student.ID, timetable.InstanceAttendancePresent)
	input.CheckedInAt = &checkedIn
	_, err := module.CreateInstanceStudent(ctx, input)
	require.NoError(t, err)

	presence, err := module.ListParallelStudentPresence(ctx, first.ID, "2027-11-02", []int64{student.ID})
	require.NoError(t, err)
	require.Len(t, presence, 1)
	assert.Equal(t, second.ID, presence[0].InstanceID)
	current, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{
		StudentIDs: []int64{student.ID}, Date: stringText("2027-11-02"), CurrentTime: stringText("08:45:00"),
		OrderByStudentActivityTime: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{second.ID}, instanceStudentInstanceIDs(current))
}

func TestModuleInstanceStudentDuplicateAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "student-isolation")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-03", "08:00:00", "Owned")
	student := testpkg.CreateTestStudent(t, db, "Owner", "Tenant", "3a")
	input := ownedInstanceStudentInput(instance.ID, student.ID, timetable.InstanceAttendanceExpected)
	created, err := module.CreateInstanceStudent(ctx, input)
	require.NoError(t, err)
	_, err = module.CreateInstanceStudent(ctx, input)
	require.Error(t, err)
	assert.EqualValues(t, 1, lastObservedOperation(log.seen, "create_instance_student").Stats.DuplicatePreventionConflicts)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreignTenantID, "Foreign", "Student", "3a")
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	_, err = module.CreateInstanceStudent(foreignCtx, ownedInstanceStudentInput(instance.ID, foreignStudent.ID, timetable.InstanceAttendanceExpected))
	require.Error(t, err)
	_, err = module.FindInstanceStudent(foreignCtx, created.ID)
	require.ErrorIs(t, err, timetable.ErrInstanceStudentNotFound)
}

func TestModuleInstanceStudentFailuresAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "student-rollback")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-04", "08:00:00", "Rollback")
	student := testpkg.CreateTestStudent(t, db, "Owner", "Rollback", "3a")
	wantErr := errors.New("abort instance student write")
	var rolledBackID int64

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created, createErr := module.CreateInstanceStudent(txCtx, ownedInstanceStudentInput(instance.ID, student.ID, timetable.InstanceAttendanceExpected))
		rolledBackID = created.ID
		if createErr != nil {
			return createErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindInstanceStudent(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrInstanceStudentNotFound)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = module.ListInstanceStudents(cancelled, timetable.InstanceStudentFilter{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestInstanceStudentListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "student-budget")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-05", "08:00:00", "Budget")
	for index := 0; index < 8; index++ {
		student := testpkg.CreateTestStudent(t, db, "Owner", fmt.Sprintf("Budget-%d", index), "3a")
		createOwnedInstanceStudent(t, module, ctx, instance.ID, student.ID, timetable.InstanceAttendanceExpected)
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instance.ID}})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.instance_students.list", counter.Queries())
}

func createOwnedInstanceStudent(t *testing.T, module *timetable.Module, ctx context.Context, instanceID, studentID int64, status string) timetable.InstanceStudent {
	t.Helper()
	value, err := module.CreateInstanceStudent(ctx, ownedInstanceStudentInput(instanceID, studentID, status))
	require.NoError(t, err)
	return value
}

func ownedInstanceStudentInput(instanceID, studentID int64, status string) timetable.InstanceStudentInput {
	return timetable.InstanceStudentInput{InstanceID: instanceID, StudentID: studentID, Status: status}
}

func createOwnedInstanceWithStatus(t *testing.T, module *timetable.Module, ctx context.Context, fixture ownedActivityInstanceFixture, date, start, title, status string) timetable.ActivityInstance {
	t.Helper()
	input := ownedActivityInstanceInput(fixture, date, start, title)
	input.Status = status
	value, err := module.CreateActivityInstance(ctx, input)
	require.NoError(t, err)
	return value
}

func instanceStudentIDs(values []timetable.InstanceStudent) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func instanceStudentInstanceIDs(values []timetable.InstanceStudent) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.InstanceID)
	}
	return result
}
