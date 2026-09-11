package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOffboardingMembershipSerializesConcurrentAssignmentCreation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()
	membership := buildModule(t, db)
	offboarding, err := NewOffboarding(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	staff := createStaff(t, ctx, db, membership, "Concurrent", "Retirement", schoolmembership.StaffFields{})
	writerDone := make(chan error, 1)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		preview, previewErr := offboarding.Preview(txCtx, staff.ID)
		if previewErr != nil {
			return previewErr
		}
		go func() {
			_, writeErr := membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "4a"})
			writerDone <- writeErr
		}()
		// Observe a real PostgreSQL wait, not merely an unscheduled goroutine.
		require.Eventually(t, func() bool {
			var waiting int
			queryErr := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(ctx, &waiting)
			return queryErr == nil && waiting > 0
		}, 5*time.Second, 10*time.Millisecond)
		_, executeErr := offboarding.Execute(txCtx, staff.ID, preview.Revision)
		return executeErr
	})
	require.NoError(t, err)
	select {
	case err := <-writerDone:
		require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	case <-ctx.Done():
		t.Fatal("assignment writer did not finish after retirement committed")
	}
	assignments, err := membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staff.ID}})
	require.NoError(t, err)
	require.Empty(t, assignments)
}

func TestOffboardingMembershipRejectsPreviewDrift(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	membership := buildModule(t, db)
	offboarding, err := NewOffboarding(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	staff := createStaff(t, ctx, db, membership, "Preview", "Drift", schoolmembership.StaffFields{})
	preview, err := offboarding.Preview(ctx, staff.ID)
	require.NoError(t, err)
	require.NotEmpty(t, preview.Revision)
	_, err = membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "2a"})
	require.NoError(t, err)
	_, err = offboarding.Execute(ctx, staff.ID, preview.Revision)
	require.ErrorIs(t, err, schoolmembership.ErrOffboardingConflict)
	_, err = membership.FindStaff(ctx, staff.ID)
	require.NoError(t, err)
	assignments, err := membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staff.ID}})
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	preview, err = offboarding.Preview(ctx, staff.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, preview.Retirement.ClassAssignments)
	result, err := offboarding.Execute(ctx, staff.ID, preview.Revision)
	require.NoError(t, err)
	require.Equal(t, preview.Retirement, result)
	result, err = offboarding.Execute(ctx, staff.ID, preview.Revision)
	require.NoError(t, err)
	require.Equal(t, schoolmembership.Retirement{}, result)
}

func TestOffboardingMembershipCannotAcquireNewAssignmentsAfterRetirement(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	membership := buildModule(t, db)
	offboarding, err := NewOffboarding(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	staff := createStaff(t, ctx, db, membership, "Retired", "Assignments", schoolmembership.StaffFields{})
	teacher, err := membership.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.NoError(t, err)
	group := testpkg.CreateTestEducationGroup(t, db, "Retired assignments")
	_, err = offboarding.Retire(ctx, staff.ID)
	require.NoError(t, err)
	_, err = membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "3a"})
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	_, err = membership.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{TeacherID: teacher.ID, GroupID: group.ID})
	require.ErrorIs(t, err, schoolmembership.ErrTeacherNotFound)
	_, err = membership.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
}

func TestOffboardingMembershipRetirementRollsBackWithOuterWorkflow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	dependencies := Dependencies{DB: db, Observe: func(Observation) {}}
	membership, err := New(dependencies)
	require.NoError(t, err)
	offboarding, err := NewOffboarding(dependencies)
	require.NoError(t, err)
	person := testpkg.CreateTestPerson(t, db, "Retained", "History")
	staff, err := membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.NoError(t, err)
	teacher, err := membership.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.NoError(t, err)
	_, err = membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "1a"})
	require.NoError(t, err)

	failure := errors.New("later owner command failed")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		result, retireErr := offboarding.Retire(txCtx, staff.ID)
		require.NoError(t, retireErr)
		require.EqualValues(t, 1, result.ClassAssignments)
		return failure
	})
	require.ErrorIs(t, err, failure)
	_, err = membership.FindStaff(ctx, staff.ID)
	require.NoError(t, err)
	_, err = membership.FindTeacher(ctx, teacher.ID)
	require.NoError(t, err)
	assignments, err := membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staff.ID}})
	require.NoError(t, err)
	require.Len(t, assignments, 1)

	result, err := offboarding.Retire(ctx, staff.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, result.ClassAssignments)
	retained, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{staff.ID}, IncludeDeleted: true})
	require.NoError(t, err)
	require.Len(t, retained, 1)
	require.NotNil(t, retained[0].DeletedAt)
	teachers, err := membership.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: []int64{teacher.ID}, IncludeDeleted: true})
	require.NoError(t, err)
	require.Len(t, teachers, 1)
	require.NotNil(t, teachers[0].DeletedAt)
	result, err = offboarding.Retire(ctx, staff.ID)
	require.NoError(t, err)
	require.Equal(t, schoolmembership.Retirement{}, result)
}

func TestOffboardingMembershipRetirementIsTenantScopedAndDetachesPlanning(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	membership := buildModule(t, db)
	offboarding, err := NewOffboarding(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	modelID := createWorkTimeModel(t, db, testpkg.Tenant(t), "Retirement template")
	staff := createStaff(t, ctx, db, membership, "Retired", "Teacher", schoolmembership.StaffFields{WorkTimeModelID: &modelID})
	teacher, err := membership.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.NoError(t, err)
	group := testpkg.CreateTestEducationGroup(t, db, "Retirement group")
	_, err = membership.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{TeacherID: teacher.ID, GroupID: group.ID})
	require.NoError(t, err)

	otherCtx, _ := otherTenantContext(t, db)
	result, err := offboarding.Retire(otherCtx, staff.ID)
	require.NoError(t, err)
	require.Equal(t, schoolmembership.Retirement{}, result)
	live, err := membership.FindStaff(ctx, staff.ID)
	require.NoError(t, err)
	require.Equal(t, &modelID, live.WorkTimeModelID)
	groups, err := membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacher.ID}})
	require.NoError(t, err)
	require.Len(t, groups, 1)

	result, err = offboarding.Retire(ctx, staff.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, result.GroupAssignments)
	retained, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{staff.ID}, IncludeDeleted: true})
	require.NoError(t, err)
	require.Len(t, retained, 1)
	require.Nil(t, retained[0].WorkTimeModelID)
	groups, err = membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacher.ID}})
	require.NoError(t, err)
	require.Empty(t, groups)
}
