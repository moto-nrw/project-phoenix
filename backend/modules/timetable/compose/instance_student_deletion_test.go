package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleDeleteStudentAssignmentsPreservesSharedInstanceAndRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "student-deletion")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-01", "08:00:00", "Shared")
	target := testpkg.CreateTestStudent(t, db, "Delete", "Target", "3a")
	spared := testpkg.CreateTestStudent(t, db, "Delete", "Spared", "3a")
	createOwnedInstanceStudent(t, module, ctx, instance.ID, target.ID, timetable.InstanceAttendanceExpected)
	other := createOwnedInstanceStudent(t, module, ctx, instance.ID, spared.ID, timetable.InstanceAttendanceExpected)
	count, err := module.CountStudentAssignments(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	abort := errors.New("abort student deletion after roster write")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		deleted, deleteErr := module.DeleteStudentAssignments(txCtx, target.ID)
		require.NoError(t, deleteErr)
		assert.EqualValues(t, 1, deleted)
		return abort
	})
	require.ErrorIs(t, err, abort)
	count, err = module.CountStudentAssignments(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	deleted, err := module.DeleteStudentAssignments(ctx, target.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	deleted, err = module.DeleteStudentAssignments(ctx, target.ID)
	require.NoError(t, err)
	assert.Zero(t, deleted)
	_, err = module.FindInstanceStudent(ctx, other.ID)
	require.NoError(t, err)
	_, err = module.FindActivityInstance(ctx, instance.ID)
	require.NoError(t, err)
}
