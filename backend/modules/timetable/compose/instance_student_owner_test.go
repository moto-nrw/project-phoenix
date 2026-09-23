package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
	first := createOwnedInstanceStudent(t, module, ctx, instance.ID, firstStudent.ID)
	second := createOwnedInstanceStudent(t, module, ctx, instance.ID, secondStudent.ID)

	found, err := module.FindInstanceStudent(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, firstStudent.ID, found.StudentID)
	listed, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{
		InstanceIDs: []int64{instance.ID}, StudentIDs: []int64{firstStudent.ID}, OrderByInstanceStudent: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID}, instanceStudentIDs(listed))
	planned, err := module.ListPlannedInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instance.ID}})
	require.NoError(t, err)
	require.Len(t, planned, 2)
	assert.Equal(t, timetable.PlannedInstanceStudent{ID: first.ID, InstanceID: instance.ID, StudentID: firstStudent.ID, Date: "2027-11-01", StartTime: "08:00:00"}, planned[0])
	ensured, inserted, err := module.EnsureInstanceStudent(ctx, instance.ID, secondStudent.ID)
	require.NoError(t, err)
	assert.False(t, inserted, "an already planned child is returned, not duplicated")
	assert.Equal(t, second.ID, ensured.ID)

	input := ownedInstanceStudentInput(instance.ID, secondStudent.ID)
	input.RoomID = &fixture.roomID
	updated, err := module.UpdateInstanceStudent(ctx, second.ID, input)
	require.NoError(t, err)
	assert.Equal(t, &fixture.roomID, updated.RoomID)
	require.NoError(t, module.DeleteInstanceStudent(ctx, first.ID))
	_, err = module.FindInstanceStudent(ctx, first.ID)
	require.ErrorIs(t, err, timetable.ErrInstanceStudentNotFound)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_instance_students").Stats.Queries)
}

func TestModuleInstanceStudentDuplicateAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "student-isolation")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-03", "08:00:00", "Owned")
	student := testpkg.CreateTestStudent(t, db, "Owner", "Tenant", "3a")
	input := ownedInstanceStudentInput(instance.ID, student.ID)
	created, err := module.CreateInstanceStudent(ctx, input)
	require.NoError(t, err)
	_, err = module.CreateInstanceStudent(ctx, input)
	require.Error(t, err)
	assert.EqualValues(t, 1, lastObservedOperation(log.seen, "create_instance_student").Stats.DuplicatePreventionConflicts)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreignTenantID, "Foreign", "Student", "3a")
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	_, err = module.CreateInstanceStudent(foreignCtx, ownedInstanceStudentInput(instance.ID, foreignStudent.ID))
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
		created, createErr := module.CreateInstanceStudent(txCtx, ownedInstanceStudentInput(instance.ID, student.ID))
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

func TestModuleOwnsInstanceStudentDayReads(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "student-day")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-08", "08:00:00", "Day")
	student := testpkg.CreateTestStudent(t, db, "Owner", "Day", "3a")
	participant := createOwnedInstanceStudent(t, module, ctx, instance.ID, student.ID)

	date := "2027-11-08"
	clock := "08:00:00"
	rows, err := module.ListPlannedInstanceStudents(ctx, timetable.InstanceStudentFilter{StudentIDs: []int64{student.ID}, Date: &date, FromClock: &clock, ExcludeCancelled: true})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, participant.ID, rows[0].ID)
	assert.Equal(t, instance.ID, rows[0].InstanceID)
	later := "08:00:01"
	rows, err = module.ListPlannedInstanceStudents(ctx, timetable.InstanceStudentFilter{StudentIDs: []int64{student.ID}, Date: &date, FromClock: &later})
	require.NoError(t, err)
	assert.Empty(t, rows, "a block that starts before the clock is not planned from it")
	studentIDs, err := module.ListPlannedStudentIDs(ctx, []int64{student.ID}, "2027-11-08")
	require.NoError(t, err)
	assert.Equal(t, []int64{student.ID}, studentIDs)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_planned_instance_students").Stats.Queries)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_planned_student_ids").Stats.Queries)
}

func TestInstanceStudentListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "student-budget")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-05", "08:00:00", "Budget")
	for index := 0; index < 8; index++ {
		student := testpkg.CreateTestStudent(t, db, "Owner", fmt.Sprintf("Budget-%d", index), "3a")
		createOwnedInstanceStudent(t, module, ctx, instance.ID, student.ID)
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instance.ID}})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.instance_students.list", counter.Queries())
}

func createOwnedInstanceStudent(t *testing.T, module *timetable.Module, ctx context.Context, instanceID, studentID int64) timetable.InstanceStudent {
	t.Helper()
	value, err := module.CreateInstanceStudent(ctx, ownedInstanceStudentInput(instanceID, studentID))
	require.NoError(t, err)
	return value
}

func ownedInstanceStudentInput(instanceID, studentID int64) timetable.InstanceStudentInput {
	return timetable.InstanceStudentInput{InstanceID: instanceID, StudentID: studentID}
}

func instanceStudentIDs(values []timetable.InstanceStudent) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}
