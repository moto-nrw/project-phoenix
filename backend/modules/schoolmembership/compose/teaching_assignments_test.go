package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleRunsTeachingAssignmentLifecycles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Ada", "Klasse")
	teacher := testpkg.CreateTestTeacher(t, db, "Grace", "Gruppe")
	group := testpkg.CreateTestEducationGroup(t, db, "Membership lifecycle")
	otherGroup := testpkg.CreateTestEducationGroup(t, db, "Membership lifecycle update")

	classAssignment, err := module.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{
		StaffID: staff.ID, SchoolClass: " 1A ",
	})
	require.NoError(t, err)
	assert.Positive(t, classAssignment.ID)
	assert.Equal(t, testpkg.Tenant(t), classAssignment.TenantID)
	assert.Equal(t, "1A", classAssignment.SchoolClass)

	groupAssignment, err := module.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{
		GroupID: group.ID, TeacherID: teacher.ID,
	})
	require.NoError(t, err)
	assert.Positive(t, groupAssignment.ID)
	assert.Equal(t, testpkg.Tenant(t), groupAssignment.TenantID)

	classes, err := module.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staff.ID}})
	require.NoError(t, err)
	require.Len(t, classes, 1)
	assert.Equal(t, classAssignment, classes[0])

	groups, err := module.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacher.ID}})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, groupAssignment, groups[0])

	updated, err := module.UpdateClassAssignment(ctx, schoolmembership.UpdateClassAssignment{
		ID: classAssignment.ID, StaffID: staff.ID, SchoolClass: "2b",
	})
	require.NoError(t, err)
	assert.Equal(t, "2b", updated.SchoolClass)
	updatedGroup, err := module.UpdateGroupAssignment(ctx, schoolmembership.UpdateGroupAssignment{
		ID: groupAssignment.ID, GroupID: otherGroup.ID, TeacherID: teacher.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, otherGroup.ID, updatedGroup.GroupID)

	require.NoError(t, module.DeleteClassAssignment(ctx, classAssignment.ID))
	require.NoError(t, module.DeleteGroupAssignment(ctx, groupAssignment.ID))

	classes, err = module.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staff.ID}})
	require.NoError(t, err)
	assert.Empty(t, classes)
	groups, err = module.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacher.ID}})
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestModuleTenantIsolationCoversTeachingAssignmentTables(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Own", "Class")
	teacher := testpkg.CreateTestTeacher(t, db, "Own", "Group")
	group := testpkg.CreateTestEducationGroup(t, db, "Own membership")
	ownClass, err := module.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "1a"})
	require.NoError(t, err)
	ownGroup, err := module.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{GroupID: group.ID, TeacherID: teacher.ID})
	require.NoError(t, err)

	otherCtx, otherTenantID := otherTenantContext(t, db)
	otherStaff := testpkg.CreateTestStaffForTenant(t, db, otherTenantID, "Other", "Class")
	otherTeacher, err := module.CreateTeacher(otherCtx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: otherStaff.ID}})
	require.NoError(t, err)
	otherGroup := testpkg.CreateTestEducationGroupForTenant(t, db, otherTenantID, "Other membership")
	foreignClass, err := module.CreateClassAssignment(otherCtx, schoolmembership.CreateClassAssignment{StaffID: otherStaff.ID, SchoolClass: "1a"})
	require.NoError(t, err)
	foreignGroup, err := module.CreateGroupAssignment(otherCtx, schoolmembership.CreateGroupAssignment{GroupID: otherGroup.ID, TeacherID: otherTeacher.ID})
	require.NoError(t, err)

	classes, err := module.ListClassAssignments(otherCtx, schoolmembership.ClassAssignmentFilter{})
	require.NoError(t, err)
	require.Len(t, classes, 1)
	assert.Equal(t, foreignClass.ID, classes[0].ID)
	groups, err := module.ListGroupAssignments(otherCtx, schoolmembership.GroupAssignmentFilter{})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, foreignGroup.ID, groups[0].ID)

	_, err = module.UpdateClassAssignment(otherCtx, schoolmembership.UpdateClassAssignment{
		ID: ownClass.ID, StaffID: otherStaff.ID, SchoolClass: "9z",
	})
	require.ErrorIs(t, err, schoolmembership.ErrClassAssignmentNotFound)
	require.NoError(t, module.DeleteClassAssignment(otherCtx, ownClass.ID))
	require.NoError(t, module.DeleteGroupAssignment(otherCtx, ownGroup.ID))

	classes, err = module.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{IDs: []int64{ownClass.ID}})
	require.NoError(t, err)
	require.Len(t, classes, 1)
	groups, err = module.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{IDs: []int64{ownGroup.ID}})
	require.NoError(t, err)
	require.Len(t, groups, 1)
}

func TestModuleClassifiesDuplicateTeachingAssignments(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Duplicate", "Class")
	teacher := testpkg.CreateTestTeacher(t, db, "Duplicate", "Group")
	group := testpkg.CreateTestEducationGroup(t, db, "Duplicate membership")

	_, err := module.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "1a"})
	require.NoError(t, err)
	_, err = module.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: " 1A "})
	require.ErrorIs(t, err, schoolmembership.ErrClassAssignmentConflict)
	assert.Equal(t, "class_assignment_conflict", schoolmembership.ErrorCode(err))

	_, err = module.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{GroupID: group.ID, TeacherID: teacher.ID})
	require.NoError(t, err)
	_, err = module.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{GroupID: group.ID, TeacherID: teacher.ID})
	require.ErrorIs(t, err, schoolmembership.ErrGroupAssignmentConflict)
	assert.Equal(t, "group_assignment_conflict", schoolmembership.ErrorCode(err))
}

func TestModuleClassAssignmentWritesRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Rollback", "Class")
	wantErr := errors.New("fail after class assignment write")

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, createErr := module.CreateClassAssignment(txCtx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "1a"})
		require.NoError(t, createErr)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	rolledBack, err := module.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staff.ID}})
	require.NoError(t, err)
	assert.Empty(t, rolledBack)

	created, err := module.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "1a"})
	require.NoError(t, err)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, updateErr := module.UpdateClassAssignment(txCtx, schoolmembership.UpdateClassAssignment{ID: created.ID, StaffID: staff.ID, SchoolClass: "2b"})
		require.NoError(t, updateErr)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	unchanged, err := module.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{IDs: []int64{created.ID}})
	require.NoError(t, err)
	require.Len(t, unchanged, 1)
	assert.Equal(t, "1a", unchanged[0].SchoolClass)

	_, err = module.UpdateClassAssignment(ctx, schoolmembership.UpdateClassAssignment{ID: created.ID, StaffID: staff.ID, SchoolClass: "2b"})
	require.NoError(t, err)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		rows, deleteErr := module.DeleteClassAssignmentsByStaff(txCtx, staff.ID)
		require.NoError(t, deleteErr)
		assert.EqualValues(t, 1, rows)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	rows, err := module.DeleteClassAssignmentsByStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	rows, err = module.DeleteClassAssignmentsByStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.Zero(t, rows, "the retry is idempotent once the row is gone")
}

func TestModuleGroupAssignmentWritesRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	teacher := testpkg.CreateTestTeacher(t, db, "Rollback", "Group")
	group := testpkg.CreateTestEducationGroup(t, db, "Rollback membership")
	otherGroup := testpkg.CreateTestEducationGroup(t, db, "Rollback membership update")
	wantErr := errors.New("fail after group assignment write")

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, createErr := module.CreateGroupAssignment(txCtx, schoolmembership.CreateGroupAssignment{GroupID: group.ID, TeacherID: teacher.ID})
		require.NoError(t, createErr)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	rolledBack, err := module.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacher.ID}})
	require.NoError(t, err)
	assert.Empty(t, rolledBack)

	created, err := module.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{GroupID: group.ID, TeacherID: teacher.ID})
	require.NoError(t, err)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, updateErr := module.UpdateGroupAssignment(txCtx, schoolmembership.UpdateGroupAssignment{ID: created.ID, GroupID: otherGroup.ID, TeacherID: teacher.ID})
		require.NoError(t, updateErr)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	unchanged, err := module.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{IDs: []int64{created.ID}})
	require.NoError(t, err)
	require.Len(t, unchanged, 1)
	assert.Equal(t, group.ID, unchanged[0].GroupID)

	_, err = module.UpdateGroupAssignment(ctx, schoolmembership.UpdateGroupAssignment{ID: created.ID, GroupID: otherGroup.ID, TeacherID: teacher.ID})
	require.NoError(t, err)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		rows, deleteErr := module.DeleteGroupAssignmentsByTeacher(txCtx, teacher.ID)
		require.NoError(t, deleteErr)
		assert.EqualValues(t, 1, rows)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	rows, err := module.DeleteGroupAssignmentsByTeacher(ctx, teacher.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	rows, err = module.DeleteGroupAssignmentsByTeacher(ctx, teacher.ID)
	require.NoError(t, err)
	assert.Zero(t, rows, "the retry is idempotent once the row is gone")
}

func TestModuleObservesTeachingAssignmentOperations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var observations []Observation
	module := buildModule(t, db, func(observation Observation) { observations = append(observations, observation) })
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Observed", "Class")

	created, err := module.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: staff.ID, SchoolClass: "1a"})
	require.NoError(t, err)
	_, err = module.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{IDs: []int64{created.ID}})
	require.NoError(t, err)
	rows, err := module.DeleteClassAssignmentsByStaff(ctx, staff.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)

	require.Len(t, observations, 3)
	assert.Equal(t, "create_class_assignment", observations[0].Operation)
	assert.EqualValues(t, 1, observations[0].Stats.Queries)
	assert.EqualValues(t, 1, observations[0].Stats.Rows)
	assert.Positive(t, observations[0].Stats.StatementDuration)
	assert.Equal(t, "list_class_assignments", observations[1].Operation)
	assert.Equal(t, "delete_class_assignments_by_staff", observations[2].Operation)
	assert.EqualValues(t, 1, observations[2].Stats.Rows)
}

func TestModuleKeepsTeachingAssignmentReadFailuresVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	cancelled, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, classErr := module.ListClassAssignments(cancelled, schoolmembership.ClassAssignmentFilter{})
	require.ErrorIs(t, classErr, context.Canceled)
	assert.Equal(t, "internal_error", schoolmembership.ErrorCode(classErr))
	_, groupErr := module.ListGroupAssignments(cancelled, schoolmembership.GroupAssignmentFilter{})
	require.ErrorIs(t, groupErr, context.Canceled)
	assert.Equal(t, "internal_error", schoolmembership.ErrorCode(groupErr))
}
